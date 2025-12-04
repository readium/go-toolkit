package mediatype

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"os"
)

type SnifferContent interface {
	Read() []byte
	Stream() io.Reader
}

// Used to sniff a local file.
type SnifferFileContent struct {
	file   fs.File
	name   *string
	buffer []byte
}

func NewSnifferFileContent(file fs.File) SnifferFileContent {
	return SnifferFileContent{file: file}
}

const MaxReadSize = 5 * 1024 * 1024 // 5MB

// Read implements SnifferContent
func (s SnifferFileContent) Read() []byte {
	info, err := s.file.Stat()
	if err != nil {
		return nil
	}
	if info.Size() > MaxReadSize {
		return nil
	}

	if of, ok := s.file.(io.ReadSeeker); ok {
		of.Seek(0, io.SeekStart)
		data := make([]byte, info.Size())
		if _, err := io.ReadFull(s.file, data); err != nil {
			return nil
		}
		return data
	} else {
		if s.buffer == nil {
			s.buffer = make([]byte, info.Size())
			io.ReadFull(s.file, s.buffer)
		}
		return s.buffer
	}
}

// Stream implements SnifferContent
func (s SnifferFileContent) Stream() io.Reader {
	if of, ok := s.file.(*os.File); ok {
		of.Seek(0, io.SeekStart)
		return bufio.NewReader(s.file)
	} else {
		if r := s.Read(); r != nil {
			return bytes.NewReader(r)
		}
		return nil
	}
}

func (s *SnifferFileContent) Name() string {
	if s.name != nil {
		return *s.name
	}

	if of, ok := s.file.(*os.File); ok {
		name := of.Name()
		s.name = &name
		return name
	} else {
		info, err := s.file.Stat()
		if err != nil {
			return ""
		}
		name := info.Name()
		s.name = &name
		return name
	}
}

// Used to sniff a byte array.
type SnifferBytesContent struct {
	bytes []byte
}

func NewSnifferBytesContent(bytes []byte) SnifferBytesContent {
	return SnifferBytesContent{bytes: bytes}
}

// Read implements SnifferContent
func (s SnifferBytesContent) Read() []byte {
	return s.bytes
}

// Stream implements SnifferContent
func (s SnifferBytesContent) Stream() io.Reader {
	return bytes.NewReader(s.bytes)
}

// TODO SnifferUriContent equivalent
