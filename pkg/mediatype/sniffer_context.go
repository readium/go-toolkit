package mediatype

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/readium/go-toolkit/pkg/archive"
	"golang.org/x/text/encoding"
)

// A companion type of [Sniffer] holding the type hints (file extensions, media types) and providing an access to the file content.
type SnifferContext struct {
	content        SnifferContent // Underlying content holder.
	mediaTypes     []string       // Media type hints.
	fileExtensions []string       // File extension hints.

	// Memoized data
	_charset                encoding.Encoding
	_contentAsString        string
	_loadedContentAsString  bool
	_contentAsXML           *XMLNode
	_loadedContentAsXML     bool
	_contentAsJSON          map[string]interface{}
	_loadedContentAsJSON    bool
	_contentAsArchive       archive.Archive
	_loadedContentAsArchive bool
}

// Media type hints.
func (s SnifferContext) MediaTypes() []MediaType {
	marr := make([]MediaType, 0, len(s.mediaTypes))
	for _, mt := range s.mediaTypes {
		nmt, err := NewOfString(mt)
		if err == nil { // Only add if no error parsing
			marr = append(marr, nmt)
		}
	}
	return marr
}

// File extension hints.
func (s SnifferContext) FileExtensions() []string {
	arr := make([]string, len(s.fileExtensions))
	for i, v := range s.fileExtensions {
		arr[i] = strings.ToLower(v)
	}
	return arr
}

// Finds the first [Charset] declared in the media types' `charset` parameter.
func (s *SnifferContext) Charset() encoding.Encoding {
	if s._charset == nil {
		return s._charset // Memoized value
	}
	for _, mt := range s.MediaTypes() {
		cs := mt.Charset()
		if cs != nil {
			s._charset = cs
			return cs
		}
	}
	return nil
}

// Returns whether this context has any of the given file extensions, ignoring case.
func (s SnifferContext) HasFileExtension(fileExtensions ...string) bool {
	selfExtensions := s.FileExtensions()
	for _, fileExtension := range fileExtensions {
		lowerExt := strings.ToLower(fileExtension)
		for _, fext := range selfExtensions {
			if fext == lowerExt {
				return true
			}
		}
	}
	return false
}

// Returns whether this context has any of the given media type, ignoring case and extra parameters.
func (s SnifferContext) HasMediaType(mediaTypes ...string) bool {
	selfMediaTypes := s.MediaTypes()
	for _, rmt := range mediaTypes {
		nmt, err := NewOfString(rmt)
		if err == nil { // Only compare if no error parsing
			for _, mt := range selfMediaTypes {
				if nmt.Contains(&mt) {
					return true
				}
			}
		}
	}
	return false
}

// Returns whether this context has any media type with a one of the provided types (the first component of the media type).
func (s SnifferContext) HasMediaTypeWithType(types ...string) bool {
	selfMediaTypes := s.MediaTypes()
	for _, typ := range types {
		for _, mt := range selfMediaTypes {
			if mt.Type == typ {
				return true
			}
		}
	}
	return false
}

// Content as plain text.
// Extracts the charset parameter from the media type hints to figure out an encoding. Otherwise, UTF-8 is assumed.
func (s SnifferContext) ContentAsString() (string, error) {
	if !s._loadedContentAsString {
		s._loadedContentAsString = true
		if s.content == nil {
			return "", errors.New("no content has been loaded")
		}
		if s.Charset() != nil {
			decoded, err := s.Charset().NewDecoder().Bytes(s.content.Read())
			if err != nil {
				return "", err
			}
			s._contentAsString = string(decoded)
			return s._contentAsString, nil
		}
		s._contentAsString = string(s.content.Read())
	}
	return s._contentAsString, nil
}

type XMLNode struct {
	XMLName xml.Name
	Content []byte    `xml:",innerxml"`
	Nodes   []XMLNode `xml:",any"`
}

// Content as an XML document.
// TODO expand on this!
func (s SnifferContext) ContentAsXML() *XMLNode {
	if !s._loadedContentAsXML {
		s._loadedContentAsXML = true
		var n XMLNode
		stream := s.Stream()
		if stream == nil {
			return nil // No stream
		}
		err := xml.NewDecoder(stream).Decode(&n)
		if err != nil {
			return nil
		}
		s._contentAsXML = &n
	}
	return s._contentAsXML
}

// Content as an Archive instance.
// Warning: Archive is only supported for a local file, for now.
func (s *SnifferContext) ContentAsArchive() (archive.Archive, error) {
	if !s._loadedContentAsArchive {
		s._loadedContentAsArchive = true
		switch s.content.(type) {
		case SnifferFileContent:
			{
				fileSniffer := s.content.(SnifferFileContent)
				a, err := archive.NewArchiveFactory().Open(fileSniffer.file.Name(), "")
				if err != nil {
					return nil, err
				}
				s._contentAsArchive = a
			}
		case SnifferBytesContent:
			{
				fileSniffer := s.content.(SnifferBytesContent)
				a, err := archive.NewArchiveFactory().OpenBytes(fileSniffer.bytes, "")
				if err != nil {
					return nil, err
				}
				s._contentAsArchive = a
			}
		default:
			{
				return nil, errors.New("SnifferContent type does not support opening as an archive")
			}
		}
	}
	return s._contentAsArchive, nil
}

// Content parsed as generic JSON interface.
func (s SnifferContext) ContentAsJSON() map[string]interface{} {
	if !s._loadedContentAsJSON {
		s._loadedContentAsJSON = true
		stream := s.Stream()
		if stream == nil {
			return nil // No stream
		}
		var jd map[string]interface{}
		err := json.NewDecoder(stream).Decode(&jd)
		if err != nil {
			return nil
		}
		s._contentAsJSON = jd
	}
	return s._contentAsJSON
}

// Content parsed as a Readium Web Publication Manifest.
func (s SnifferContext) ContentAsRWPM() {
	panic("Not implemented!") // TODO think out the best go equivalent (without circular imports)
}

// Raw bytes stream of the content.
// A byte stream can be useful when sniffers only need to read a few bytes at the beginning of the file.
func (s SnifferContext) Stream() io.Reader {
	if s.content == nil {
		return nil
	}
	return s.content.Stream()
}

// Reads all the bytes or the given [range].
// It can be used to check a file signature, aka magic number. See https://en.wikipedia.org/wiki/List_of_file_signatures
// Warning: This ignores errors, and just returns nil
func (s SnifferContext) Read(start int64, end int64) []byte {
	if end <= start {
		return nil
	}
	if s.content == nil {
		return nil
	}
	stream := s.content.Stream()
	if stream == nil {
		return nil
	}
	if start == 0 && end == 0 {
		data, err := io.ReadAll(stream)
		if err != nil {
			return nil
		}
		return data
	}
	if start > 0 {
		_, err := io.CopyN(io.Discard, stream, start)
		if err != nil {
			return nil
		}
	}
	data := make([]byte, end-start+1)
	_, err := stream.Read(data)
	if err != nil {
		return nil
	}
	return data
}

// Returns whether the content is a JSON object containing all of the given root keys.
func (s SnifferContext) ContainsJSONKeys(keys ...string) bool {
	if len(keys) == 0 {
		return false
	}
	js := s.ContentAsJSON()
	if js == nil {
		return false
	}
	for _, key := range keys {
		_, ok := js[key]
		if !ok {
			return false
		}
	}
	return true
}

// Returns whether an Archive entry exists in this file.
func (s SnifferContext) ContainsArchiveEntryAt(path string) bool {
	a, err := s.ContentAsArchive()
	if err != nil {
		return false
	}
	_, err = a.Entry(path)
	if err != nil {
		return false
	}
	return true
}

// Returns the Archive entry data at the given [path] in this file.
func (s SnifferContext) ReadArchiveEntryAt(path string) []byte {
	a, err := s.ContentAsArchive()
	if err != nil {
		return nil
	}
	f, err := a.Entry(path)
	if err != nil {
		return nil
	}
	data, err := f.Read(0, 0)
	if err != nil {
		return nil
	}
	return data
}

func (s SnifferContext) ArchiveEntriesAllSatisfy() bool {
	panic("Not implemented!") // TODO think out the best go equivalent
}
