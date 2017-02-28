package epub

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

// Epub represent epub data
type Epub struct {
	Ncx        Ncx        `json:"ncx"`
	Opf        Opf        `json:"opf"`
	Container  Container  `json:"-"`
	Encryption Encryption `json:"-"`

	zipFd     *zip.ReadCloser
	directory string
}

//Open open resource file
func (epub *Epub) Open(filepath string) (io.ReadCloser, error) {
	return epub.open(epub.filename(filepath))
}

//Close close file reader
func (epub *Epub) Close() {
	epub.zipFd.Close()
}

//-----------------------------------------------------------------------------
func (epub *Epub) filename(name string) string {
	return path.Join(path.Dir(epub.Container.Rootfile.Path), name)
}

func (epub *Epub) parseXML(filename string, v interface{}) error {
	fd, err := epub.open(filename)
	if err != nil {
		return nil
	}
	defer fd.Close()
	dec := xml.NewDecoder(fd)
	return dec.Decode(v)
}

func (epub *Epub) parseJSON(filename string, v interface{}) error {
	fd, err := epub.open(filename)
	if err != nil {
		return nil
	}
	defer fd.Close()
	dec := json.NewDecoder(fd)
	return dec.Decode(v)
}

func (epub *Epub) getData(filename string) ([]byte, error) {
	fd, err := epub.open(filename)
	if err != nil {
		return nil, nil
	}
	defer fd.Close()

	return ioutil.ReadAll(fd)

}

func (epub *Epub) open(filename string) (io.ReadCloser, error) {
	if epub.directory != "" {
		filenameEpub := path.Join(path.Dir(epub.directory+string(filepath.Separator)), filename)
		return os.Open(filenameEpub)
	}

	for _, f := range epub.zipFd.File {
		if f.Name == filename {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("can't find file or directory %s", filename)
}

// ZipReader return the internal file descriptor
func (epub *Epub) ZipReader() *zip.ReadCloser {
	return epub.zipFd
}

// GetSMIL parse and return SMIL structure
func (epub *Epub) GetSMIL(ressouce string) SMIL {
	var smil SMIL

	epub.parseXML(ressouce, &smil)

	return smil
}

//OpenEpub open and parse epub
func OpenEpub(fn string) (*Epub, error) {
	zipFile, err := zip.OpenReader(fn)
	if err != nil {
		return nil, err
	}
	defer zipFile.Close()

	epb := Epub{zipFd: zipFile}
	errCont := epb.parseXML("META-INF/container.xml", &epb.Container)
	if errCont != nil {
		return nil, err
	}

	errOpf := epb.parseXML(epb.Container.Rootfile.Path, &epb.Opf)
	if errOpf != nil {
		return nil, errOpf
	}

	errEnc := epb.parseXML("META-INF/encryption.xml", &epb.Encryption)
	if errEnc != nil {
		return nil, errEnc
	}

	for _, manf := range epb.Opf.Manifest {
		if manf.ID == epb.Opf.Spine.Toc {
			errToc := epb.parseXML(epb.filename(manf.Href), &epb.Ncx)
			if errToc != nil {
				return nil, errToc
			}
			break
		}
	}

	return &epb, nil
}

//OpenDir open a opf file
func OpenDir(filename string) (*Epub, error) {

	epb := Epub{directory: filename}

	errCont := epb.parseXML("META-INF/container.xml", &epb.Container)
	if errCont != nil {
		return nil, errCont
	}

	errOpf := epb.parseXML(epb.Container.Rootfile.Path, &epb.Opf)
	if errOpf != nil {
		return nil, errOpf
	}

	errEnc := epb.parseXML("META-INF/encryption.xml", &epb.Encryption)
	if errEnc != nil {
		return nil, errEnc
	}

	for _, manf := range epb.Opf.Manifest {
		if manf.ID == epb.Opf.Spine.Toc {
			errToc := epb.parseXML(epb.filename(manf.Href), &epb.Ncx)
			if errToc != nil {
				return nil, errToc
			}
			break
		}
	}

	return &epb, nil
}
