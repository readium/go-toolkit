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
	"golang.org/x/exp/slices"
	"golang.org/x/image/riff"
	"golang.org/x/image/webp"
)

const blurHashAlgorithm = "https://blurha.sh"

type imageProperties struct {
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

func (p *imageProperties) EnhanceLink(link *manifest.Link) {
	link.Height = uint(p.Height)
	link.Width = uint(p.Width)
	link.Size = uint(p.Size)

	hashes := make(manifest.HashList, 0, 4)
	if link.Properties == nil {
		link.Properties = manifest.Properties{}
	} else if existingHashes := link.Properties.Hash(); len(existingHashes) > 0 {
		hashes = existingHashes
	}

	if len(p.Hashes.Sha256) > 0 {
		hashes = append(hashes, manifest.HashValue{
			Algorithm: manifest.HashAlgorithmSHA256,
			Value:     base64.StdEncoding.EncodeToString(p.Hashes.Sha256),
		})
	}
	if len(p.Hashes.Md5) > 0 {
		hashes = append(hashes, manifest.HashValue{
			Algorithm: manifest.HashAlgorithmMD5,
			Value:     base64.StdEncoding.EncodeToString(p.Hashes.Md5),
		})
	}
	if len(p.Hashes.PhashDCT) > 0 {
		hashes = append(hashes, manifest.HashValue{
			Algorithm: manifest.HashAlgorithmPhashDCT,
			Value:     base64.StdEncoding.EncodeToString(p.Hashes.PhashDCT),
		})
	}
	if len(p.Hashes.BlurHash) > 0 {
		hashes = append(hashes, manifest.HashValue{
			Algorithm: blurHashAlgorithm,
			Value:     p.Hashes.BlurHash,
		})
	}
	hashes.Deduplicate()

	link.Properties["hash"] = hashes.ToJSONArray()
	link.Properties["animated"] = p.Animated
}

func hasVisualAlgorithm(hashes []manifest.HashAlgorithm) bool {
	visualHash := false
	for _, hash := range hashes {
		switch hash {
		case manifest.HashAlgorithmPhashDCT, blurHashAlgorithm:
			visualHash = true
		default:
			continue
		}
		if visualHash {
			break
		}
	}
	return visualHash
}

// InspectImage inspects an image located in the provided filesystem, using the provided link's [manifest.HREF]
// as a path. Additional properties from the link, such as the [mediatype.MediaType], may be used, and should
// be included. A copy of the provided link will be returned, with the `size`, `width`, `height` and
// `properties.animated` attributes set. A slice of [manifest.HashAlgorithm] can be provided, in which case
// the returned link will also have `properties.hash` set with the computed hashes. Currently, the supported
// algorithms are: [manifest.HashAlgorithmSHA256], [manifest.HashAlgorithmMD5], [manifest.HashAlgorithmPhashDCT],
// and `https://blurha.sh` (BlurHash). The latter two are visual hashes, which are more computationally expensive.
func InspectImage(system fs.FS, link manifest.Link, algorithms []manifest.HashAlgorithm) (*manifest.Link, error) {

	// Skip any supplied algorithms for hashes that have already been computed in the link properties
	neededAlgorithms := make([]manifest.HashAlgorithm, 0, len(algorithms))
	existingHashes := link.Properties.Hash()
	for _, algorithm := range algorithms {
		exists := false
		for _, hash := range existingHashes {
			if hash.Algorithm == algorithm {
				exists = true
				break
			}
		}
		if !exists && !slices.Contains(neededAlgorithms, algorithm) {
			neededAlgorithms = append(neededAlgorithms, algorithm)
		}
	}

	path := link.Href.String()
	file, err := system.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reopen := func() error {
		if of, ok := file.(io.ReadSeeker); ok {
			of.Seek(0, 0)
		} else {
			file, err = system.Open(path)
			if err != nil {
				return err
			}
		}
		return nil
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, errors.New("must be a file, not a directory")
	}

	p := &imageProperties{
		Size:    uint64(stat.Size()),
		ModTime: stat.ModTime(),
	}
	if p.Size == 0 {
		return nil, errors.New("file is empty")
	}

	var mt *mediatype.MediaType
	if link.MediaType != nil {
		mt = link.MediaType
	} else {
		mt = mediatype.OfFileOnly(context.TODO(), file)
		if mt == nil {
			return nil, errors.New("file has unknown media type")
		}
	}
	if !mt.IsBitmap() {
		return nil, errors.New("file is not a bitmap image")
	}
	// Reopen because the sniffer may have read the file
	err = reopen()
	if err != nil {
		return nil, errors.Wrap(err, "failed reopening file")
	}

	// Gather image width/height, and weed out unsuppored formats
	var iconfig image.Config
	if mt.Equal(&mediatype.AVIF) {
		var hf *heif.File
		if of, ok := file.(io.ReaderAt); ok {
			hf = heif.Open(of)
		} else {
			// Fall back to reading the file into memory
			stat, err := file.Stat()
			if err != nil {
				return nil, errors.Wrap(err, "failed statting AVIF file")
			}
			buf := make([]byte, stat.Size())
			_, err = io.ReadFull(file, buf)
			if err != nil {
				return nil, errors.Wrap(err, "failed reading AVIF file into memory")
			}
			hf = heif.Open(bytes.NewReader(buf))
		}
		pi, err := hf.PrimaryItem()
		if err != nil {
			return nil, errors.Wrap(err, "failed decoding supposed AVIF file metadata")
		}
		w, h, ok := pi.VisualDimensions()
		if !ok {
			return nil, errors.New("failed reading AVIF image dimensions")
		}
		iconfig.Width = w
		iconfig.Height = h
	} else if mt.Equal(&mediatype.JXL) {
		magicBytes := make([]byte, 12)
		_, err = io.ReadFull(file, magicBytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed reading JXL file for magic numbers")
		}
		jxlCodestream := []byte{0xFF, 0x0A}
		jxlBmff := []byte{0x00, 0x00, 0x00, 0x0C, 0x4A, 0x58, 0x4C, 0x20, 0x0D, 0x0A, 0x87, 0x0A}
		if !bytes.Equal(magicBytes[:2], jxlCodestream) && !bytes.Equal(magicBytes, jxlBmff) {
			return nil, errors.New("supposed JXL file is invalid")
		}
		return nil, errors.New("JXL file format is currently unsupported")
	} else {
		var format string
		iconfig, format, err = image.DecodeConfig(file)
		if err != nil {
			return nil, errors.Wrap(err, "failed decoding image metadata")
		}

		// Special case for animated PNG which gets registered by the apng package
		if format == "apng" {
			if !mt.Equal(&mediatype.PNG) {
				return nil, errors.New("file mediatype not equal to decoded image format")
			}
		} else {
			imt := mediatype.OfExtension(format)
			if imt == nil {
				return nil, errors.New("failed determining mediatype from image format \"" + format + "\"")
			}
			if !mt.Equal(imt) {
				return nil, errors.New("file mediatype not equal to decoded image format")
			}
		}
	}
	p.Width = uint32(iconfig.Width)
	p.Height = uint32(iconfig.Height)
	if p.Width == 0 || p.Height == 0 {
		return nil, errors.New("image has zero width or height")
	}

	// Decoder the image so the animation can be checked, and the perceptual hash calculated
	err = reopen()
	if err != nil {
		return nil, errors.Wrap(err, "failed reopening file")
	}
	visualHash := hasVisualAlgorithm(neededAlgorithms)
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

		if slices.Contains(neededAlgorithms, manifest.HashAlgorithmPhashDCT) {
			// Create phash and put it in a byte array
			p.Hashes.PhashDCT = make([]byte, 8)
			binary.BigEndian.PutUint64(p.Hashes.PhashDCT, phash.DTC(img))
		}
		if slices.Contains(neededAlgorithms, blurHashAlgorithm) {
			// Create the blurhash
			blurhash, _ := blurhash.Encode(5, 5, img)
			p.Hashes.BlurHash = blurhash
		}
	}
	if mt.Equal(&mediatype.GIF) {
		gi, err := gif.DecodeAll(file)
		if err != nil {
			return nil, errors.Wrap(err, "failed decoding GIF file")
		}
		if len(gi.Image) > 1 {
			p.Animated = true
		}
		hashVisually(gi.Image[0])
	} else if mt.Equal(&mediatype.PNG) {
		pi, err := apng.DecodeAll(file)
		if err != nil {
			return nil, errors.Wrap(err, "failed decoding (A)PNG file")
		}
		if len(pi.Frames) > 1 {
			p.Animated = true
		}
		hashVisually(pi.Frames[0].Image)
	} else if mt.Equal(&mediatype.AVIF) {
		// Not sure how to determine if an AVIF is animated. Very rare
		if visualHash {
			return nil, errors.New("AVIF perceptual hash is not yet supported")
		}
	} else if mt.Equal(&mediatype.WEBP) {
		var wi image.Image
		if _, ok := file.(io.ReadSeeker); ok {
			p.Animated, err = isWEBPAnimated(file)
			if err != nil {
				return nil, errors.Wrap(err, "failed checking if WEBP file is animated")
			}
			if visualHash {
				if p.Animated {
					return nil, errors.New("perceptual hash of animated WEBP is not yet supported")
				}
				err = reopen()
				if err != nil {
					return nil, errors.Wrap(err, "failed reopening file")
				}
				wi, err = webp.Decode(file)
			}
		} else {
			// Only read the file once into memory since we need to read it two times in a row
			buf := make([]byte, p.Size)
			_, err = io.ReadFull(file, buf)
			if err != nil {
				return nil, errors.Wrap(err, "failed reading WEBP file into memory")
			}
			r := bytes.NewReader(buf)
			p.Animated, err = isWEBPAnimated(r)
			if err != nil {
				return nil, errors.Wrap(err, "failed checking if WEBP file is animated")
			}
			if visualHash {
				if p.Animated {
					return nil, errors.New("perceptual hash of animated WEBP is not yet supported")
				}
				r.Seek(0, 0)
				wi, err = webp.Decode(r)
			}
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed decoding WEBP file")
		}
		if visualHash {
			hashVisually(wi)
		}
	} else if visualHash {
		// Any other format can be generically decoded since it doesn't support animation
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, errors.Wrap(err, "failed decoding image file")
		}
		hashVisually(img)
	}

	// Now compute the cryptographic hashes
	err = reopen()
	if err != nil {
		return nil, errors.Wrap(err, "failed reopening file")
	}

	// TODO: rewrite more cleanly
	s2hash := sha256.New()
	mdhash := md5.New()
	if slices.Contains(neededAlgorithms, manifest.HashAlgorithmSHA256) && slices.Contains(neededAlgorithms, manifest.HashAlgorithmMD5) {
		mw := io.MultiWriter(s2hash, mdhash)
		if _, err := io.Copy(mw, file); err != nil {
			return nil, errors.Wrap(err, "failed computing SHA256 and MD5 hashes")
		}
		p.Hashes.Sha256 = s2hash.Sum(nil)
		p.Hashes.Md5 = mdhash.Sum(nil)
	} else {
		if slices.Contains(neededAlgorithms, manifest.HashAlgorithmSHA256) {
			if _, err := io.Copy(s2hash, file); err != nil {
				return nil, errors.Wrap(err, "failed computing SHA256 hash")
			}
			p.Hashes.Sha256 = s2hash.Sum(nil)
		}
		if slices.Contains(neededAlgorithms, manifest.HashAlgorithmMD5) {
			if _, err := io.Copy(mdhash, file); err != nil {
				return nil, errors.Wrap(err, "failed computing MD5 hash")
			}
			p.Hashes.Md5 = mdhash.Sum(nil)
		}
	}

	p.EnhanceLink(&link)
	return &link, nil
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

// MatchImage compares the link with the given hashes to determine if they match.
func MatchImage(link manifest.Link, hashes manifest.HashList) (bool, error) {
	if link.MediaType == nil || !link.MediaType.IsBitmap() {
		return false, errors.New("link is not to an image that can be matched")
	}

	linkHashes := link.Properties.Hash()
	if len(linkHashes) == 0 {
		// No hashes in the link, we can't match it
		return false, nil
	}
	for _, hash := range hashes {
		if v, ok := linkHashes.Find(hash.Algorithm); ok {
			if v.Equal(hash) {
				// Simple equality
				return true, nil
			}

			// Special distance-based matching for perceptual hashes
			if v.Algorithm == manifest.HashAlgorithmPhashDCT {
				phashVal, err := base64.StdEncoding.DecodeString(v.Value)
				if err != nil {
					return false, errors.Wrap(err, "failed decoding perceptual hash value of link")
				}
				if len(phashVal) != 8 {
					return false, errors.New("perceptual hash value of link is not 8 bytes in length")
				}
				linkPerceptualHash := binary.BigEndian.Uint64(phashVal)

				phashVal, err = base64.StdEncoding.DecodeString(hash.Value)
				if err != nil {
					return false, errors.Wrap(err, "failed decoding provided perceptual hash value")
				}
				if len(phashVal) != 8 {
					return false, errors.New("provided perceptual hash value is not 8 bytes in length")
				}
				providedPerceptualHash := binary.BigEndian.Uint64(phashVal)

				if phash.Distance(linkPerceptualHash, providedPerceptualHash) == 0 {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
