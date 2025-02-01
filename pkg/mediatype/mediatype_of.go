package mediatype

import (
	"os"
	"path/filepath"
)

// The default sniffers provided by Readium 2 to resolve a [MediaType].
// You can register additional sniffers globally by modifying this list.
// The sniffers order is important, because some formats are subsets of other formats.
var Sniffers = []Sniffer{
	SniffEPUB,
	SniffLPF,
	SniffArchive,
	SniffPDF,
	SniffXHTML,
	SniffHTML,
	SniffBitmap,
	SniffAudio,
	SniffOPDS,
	SniffLCPLicense,
	SniffW3CWPUB,
	SniffWebpub,
	// Note SniffKnown and SniffSystem aren't here!
}

// Resolves a media type from a sniffer context.
// Sniffing a media type is done in two rounds, because we want to give an opportunity to all
// sniffers to return a [MediaType] quickly before inspecting the content itself:
// - Light Sniffing checks only the provided file extension or media type hints.
// - Heavy Sniffing reads the bytes to perform more advanced sniffing.
func of(content SnifferContent, mediaTypes []string, fileExtensions []string, sniffers []Sniffer) *MediaType {

	// Light sniffing with only media type hints
	if len(mediaTypes) > 0 {
		context := SnifferContext{
			mediaTypes: mediaTypes,
		}
		for _, sniffer := range sniffers {
			mediaType := sniffer(context)
			if mediaType != nil {
				return mediaType
			}
		}
	}

	// Light sniffing with both media type hints and file extensions
	if len(fileExtensions) > 0 {
		context := SnifferContext{
			mediaTypes:     mediaTypes,
			fileExtensions: fileExtensions,
		}
		for _, sniffer := range sniffers {
			mediaType := sniffer(context)
			if mediaType != nil {
				return mediaType
			}
		}
	}

	// Heavy sniffing
	if content != nil {
		context := SnifferContext{
			content:        content,
			mediaTypes:     mediaTypes,
			fileExtensions: fileExtensions,
		}
		for _, sniffer := range sniffers {
			mediaType := sniffer(context)
			if mediaType != nil {
				return mediaType
			}
		}
	}

	context := SnifferContext{
		content:        content,
		mediaTypes:     mediaTypes,
		fileExtensions: fileExtensions,
	}

	// Check if the media type is within the well-known list
	// Note: This is done after the heavy sniffing of the provided [sniffers], because
	// otherwise it will detect JSON, XML or ZIP formats before we have a chance of sniffing
	// their content (for example, for RWPM).
	if c := SniffKnown(context); c != nil {
		return c
	}

	// Falls back on the system-wide registered media types.
	if c := SniffSystem(context); c != nil {
		return c
	}

	// If nothing else worked, we try to parse the first valid media type hint.
	for _, mediaType := range mediaTypes {
		if mediaType == "" {
			continue // Blank mediatype
		}
		mt, err := New(mediaType, "", "")
		if err == nil {
			return &mt
		}
	}

	return nil
}

// Resolves a format from a list of mediatypes, list of extensions, and list of sniffers
func Of(mediaTypes []string, extensions []string, sniffers []Sniffer) *MediaType {
	return of(nil, mediaTypes, extensions, sniffers)
}

func OfStringAndExtension(mediaType string, extension string) *MediaType {
	return of(nil, []string{mediaType}, []string{extension}, Sniffers)
}

// Resolves a format from a single mediaType string
func OfString(mediaType string) *MediaType {
	return of(nil, []string{mediaType}, nil, Sniffers)
}

// Resolves a format from a single file extension
func OfExtension(extension string) *MediaType {
	return of(nil, nil, []string{extension}, Sniffers)
}

// Resolves a format from a file
func OfFile(file *os.File, mediaTypes []string, extensions []string, sniffers []Sniffer) *MediaType {
	if file != nil {
		ext := filepath.Ext(file.Name())
		if ext != "" {
			ext = ext[1:] // Remove the leading "."
			if extensions == nil {
				extensions = []string{ext}
			} else {
				extensions = append(extensions, ext)
			}
		}
	}

	return of(NewSnifferFileContent(file), mediaTypes, extensions, sniffers)
}

// Resolves a format from a file, and nothing else
func OfFileOnly(file *os.File) *MediaType {
	return OfFile(file, nil, nil, Sniffers)
}

// Resolves a format from bytes, e.g. from an HTTP response.
func OfBytes(bytes []byte, mediaTypes []string, extensions []string, sniffers []Sniffer) *MediaType {
	return of(NewSnifferBytesContent(bytes), mediaTypes, extensions, sniffers)
}

// Resolves a format from bytes, e.g. from an HTTP response, and nothing else
func OfBytesOnly(bytes []byte) *MediaType {
	return of(NewSnifferBytesContent(bytes), nil, nil, Sniffers)
}
