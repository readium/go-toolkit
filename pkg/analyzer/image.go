package analyzer

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/gif"
	_ "image/png"
	"io"
	"io/fs"
	"time"

	"github.com/azr/phash"
	"github.com/bbrks/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/kettek/apng"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"go4.org/media/heif"
	"golang.org/x/image/riff"
	"golang.org/x/image/webp"
)

type ImageProperties struct {
	Size     uint64
	ModTime  time.Time
	Width    uint32
	Height   uint32
	Animated bool
	Hashes   struct {
		Sha256   []byte
		Md5      []byte
		PhashDCT []byte
		BlurHash string
	}
}

func (p *ImageProperties) EnhanceLink(link *manifest.Link) {
	link.Height = uint(p.Height)
	link.Width = uint(p.Width)
	link.Size = uint(p.Size)
	if link.Properties == nil {
		link.Properties = manifest.Properties{}
	}

	// TODO: more sophisticated handling of pre-existing values, conversion to dedicated struct like encryption is
	hashes := []map[string]string{
		{
			"algorithm": "sha256",
			"value":     base64.StdEncoding.EncodeToString(p.Hashes.Sha256),
		},
		{
			"algorithm": "md5",
			"value":     base64.StdEncoding.EncodeToString(p.Hashes.Md5),
		},
	}
	if len(p.Hashes.PhashDCT) > 0 {
		hashes = append(hashes, map[string]string{
			"algorithm": "phash-dct",
			"value":     base64.StdEncoding.EncodeToString(p.Hashes.PhashDCT),
		})
	}
	if len(p.Hashes.BlurHash) > 0 {
		hashes = append(hashes, map[string]string{
			"algorithm": "https://blurha.sh",
			"value":     p.Hashes.BlurHash,
		})
	}
	link.Properties["hash"] = hashes
	link.Properties["animated"] = p.Animated
}

func Image(system fs.FS, link manifest.Link, visualHash bool) (*manifest.Link, *ImageProperties, error) {
	path := link.Href.String()
	file, err := system.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reopen := func() error {
		if of, ok := file.(io.ReadSeeker); ok {
			of.Seek(0, 0)
		} else {
			file.Close()
			file, err = system.Open(path)
			if err != nil {
				return err
			}
		}
		return nil
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if stat.IsDir() {
		return nil, nil, errors.New("must be a file, not a directory")
	}

	p := &ImageProperties{
		Size:    uint64(stat.Size()),
		ModTime: stat.ModTime(),
	}
	if p.Size == 0 {
		return nil, nil, errors.New("file is empty")
	}

	var mt *mediatype.MediaType
	if link.MediaType != nil {
		mt = link.MediaType
	} else {
		mt = mediatype.OfFileOnly(context.TODO(), file)
		if mt == nil {
			return nil, nil, errors.New("file has unknown media type")
		}
	}
	if !mt.IsBitmap() {
		return nil, nil, errors.New("file is not a bitmap image")
	}
	// Reopen because the sniffer may have read the file
	err = reopen()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed reopening file")
	}

	// Gather image width/height, and weed out unsuppored formats
	var iconfig image.Config
	if mt.Equal(&mediatype.AVIF) {
		var hf *heif.File
		if of, ok := file.(io.ReaderAt); ok {
			hf = heif.Open(of)
		} else {
			// Fall back to reading the file into memory
			buf, err := fs.ReadFile(system, path)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed reading AVIF file into memory")
			}
			hf = heif.Open(bytes.NewReader(buf))
		}
		pi, err := hf.PrimaryItem()
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed decoding supposed AVIF file metadata")
		}
		w, h, ok := pi.VisualDimensions()
		if !ok {
			return nil, nil, errors.New("failed reading AVIF image dimensions")
		}
		iconfig.Width = w
		iconfig.Height = h
	} else if mt.Equal(&mediatype.JXL) {
		magicBytes := make([]byte, 12)
		_, err = io.ReadFull(file, magicBytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed reading JXL file for magic numbers")
		}
		jxlCodestream := []byte{0xFF, 0x0A}
		jxlBmff := []byte{0x00, 0x00, 0x00, 0x0C, 0x4A, 0x58, 0x4C, 0x20, 0x0D, 0x0A, 0x87, 0x0A}
		if !bytes.Equal(magicBytes[:2], jxlCodestream) && !bytes.Equal(magicBytes, jxlBmff) {
			return nil, nil, errors.New("supposed JXL file is invalid")
		}
		return nil, nil, errors.New("JXL file format is currently unsupported")
	} else {
		var format string
		iconfig, format, err = image.DecodeConfig(file)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed decoding image metadata")
		}

		// Special case for animated PNG which gets registered by the apng package
		if format == "apng" {
			if !mt.Equal(&mediatype.PNG) {
				return nil, nil, errors.New("file mediatype not equal to decoded image format")
			}
		} else {
			imt := mediatype.OfExtension(format)
			if imt == nil {
				return nil, nil, errors.New("failed determining mediatype from image format \"" + format + "\"")
			}
			if !mt.Equal(imt) {
				return nil, nil, errors.New("file mediatype not equal to decoded image format")
			}
		}
	}
	p.Width = uint32(iconfig.Width)
	p.Height = uint32(iconfig.Height)
	if p.Width == 0 || p.Height == 0 {
		return nil, nil, errors.New("image has zero width or height")
	}

	// Decoder the image so the animation can be checked, and the perceptual hash calculated
	err = reopen()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed reopening file")
	}
	hashVisually := func(img image.Image) {
		if !visualHash {
			return
		}
		// First downsize the image because:
		// - Phash/DCT already does this, down to 32x32px
		// - Blurhash encoding with a large image is very slow
		if img.Bounds().Dx() > 128 {
			img = imaging.Resize(img, 128, 0, imaging.Lanczos)
		}

		// Create phash and put it in a byte array
		p.Hashes.PhashDCT = make([]byte, 8)
		binary.BigEndian.PutUint64(p.Hashes.PhashDCT, phash.DTC(img))

		// Create the blurhash
		blurhash, _ := blurhash.Encode(5, 5, img)
		p.Hashes.BlurHash = blurhash
	}
	if mt.Equal(&mediatype.GIF) {
		gi, err := gif.DecodeAll(file)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed decoding GIF file")
		}
		if len(gi.Image) > 1 {
			p.Animated = true
		}
		hashVisually(gi.Image[0])
	} else if mt.Equal(&mediatype.PNG) {
		pi, err := apng.DecodeAll(file)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed decoding (A)PNG file")
		}
		if len(pi.Frames) > 1 {
			p.Animated = true
		}
		hashVisually(pi.Frames[0].Image)
	} else if mt.Equal(&mediatype.AVIF) {
		// Not sure how to determine if an AVIF is animated!
		if visualHash {
			return nil, nil, errors.New("AVIF perceptual hash is not yet supported")
		}
	} else if mt.Equal(&mediatype.WEBP) {
		var wi image.Image
		if _, ok := file.(io.ReadSeeker); ok {
			p.Animated, err = isWEBPAnimated(file)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed checking if WEBP file is animated")
			}
			if visualHash {
				if p.Animated {
					return nil, nil, errors.New("perceptual hash of animated WEBP is not yet supported")
				}
				err = reopen()
				if err != nil {
					return nil, nil, errors.Wrap(err, "failed reopening file")
				}
				wi, err = webp.Decode(file)
			}
		} else {
			// Only read the file once into memory since we need to read it two times in a row
			buf := make([]byte, p.Size)
			_, err = io.ReadFull(file, buf)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed reading WEBP file into memory")
			}
			r := bytes.NewReader(buf)
			p.Animated, err = isWEBPAnimated(r)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed checking if WEBP file is animated")
			}
			if visualHash {
				if p.Animated {
					return nil, nil, errors.New("perceptual hash of animated WEBP is not yet supported")
				}
				r.Seek(0, 0)
				wi, err = webp.Decode(r)
			}
		}
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed decoding WEBP file")
		}
		if visualHash {
			hashVisually(wi)
		}
	} else if visualHash {
		// Any other format can be generically decoded since it doesn't support animation
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed decoding image file")
		}
		hashVisually(img)
	}

	// Now compute the cryptographic hashes
	err = reopen()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed reopening file")
	}
	s2hash := sha256.New()
	mdhash := md5.New()
	mw := io.MultiWriter(s2hash, mdhash)
	if _, err := io.Copy(mw, file); err != nil {
		panic(err)
	}
	p.Hashes.Sha256 = s2hash.Sum(nil)
	p.Hashes.Md5 = mdhash.Sum(nil)

	p.EnhanceLink(&link)
	return &link, p, nil
}

func isWEBPAnimated(file io.Reader) (bool, error) {
	_, data, err := riff.NewReader(file)
	if err != nil {
		return false, errors.Wrap(err, "failed reading RIFF data from WEBP file")
	}
	id, _, _, err := data.Next()
	var frames uint32
	for err == nil {
		if id == riff.FourCC([4]byte{'A', 'N', 'M', 'F'}) {
			frames++
		}
		id, _, _, err = data.Next()
	}
	if err != io.EOF {
		return false, errors.Wrap(err, "failed reading RIFF chunks from WEBP file")
	}
	return frames > 1, nil
}
