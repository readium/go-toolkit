package mediatype

import (
	"os"
	"path/filepath"
)

// The default sniffers provided by Readium 2 to resolve a [MediaType].
// You can register additional sniffers globally by modifying this list.
// The sniffers order is important, because some formats are subsets of other formats.
var Sniffers = []Sniffer{
	SniffHTML, SniffOPDS, SniffLCPLicense, SniffBitmap,
	SniffWebpub, SniffW3CWPUB, SniffEPUB, SniffLPF, SniffArchive, SniffPDF,
	// Note SniffSystem isn't here!
}

// Resolves a media type from a sniffer context.
// Sniffing a media type is done in two rounds, because we want to give an opportunity to all
// sniffers to return a [MediaType] quickly before inspecting the content itself:
// - Light Sniffing checks only the provided file extension or media type hints.
// - Heavy Sniffing reads the bytes to perform more advanced sniffing.
func mediaTypeOf(content SnifferContent, mediaTypes []string, fileExtensions []string, sniffers []Sniffer) *MediaType {

	// Light Sniffing
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

	// Heavy sniffing
	if content != nil {
		context = SnifferContext{
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

	// Falls back on the system-wide registered media types.
	// Note: This is done after the heavy sniffing of the provided [sniffers], because
	// otherwise it will detect JSON, XML or ZIP formats before we have a chance of sniffing
	// their content (for example, for RWPM).
	if c := SniffSystem(context); c != nil {
		return c
	}

	// If nothing else worked, we try to parse the first valid media type hint.
	for _, mediaType := range mediaTypes {
		if mediaType == "" {
			continue // Blank mediatype
		}
		mt, err := NewMediaType(mediaType, "", "")
		if err == nil {
			return &mt
		}
	}

	return nil
}

// Resolves a format from a list of mediatypes, list of extensions, and list of sniffers
func MediaTypeOf(mediaTypes []string, extensions []string, sniffers []Sniffer) *MediaType {
	return mediaTypeOf(nil, mediaTypes, extensions, sniffers)
}

func MediaTypeOfStringAndExtension(mediaType string, extension string) *MediaType {
	return mediaTypeOf(nil, []string{mediaType}, []string{extension}, Sniffers)
}

// Resolves a format from a single mediaType string
func MediaTypeOfString(mediaType string) *MediaType {
	return mediaTypeOf(nil, []string{mediaType}, nil, Sniffers)
}

// Resolves a format from a single file extension
func MediaTypeOfExtension(extension string) *MediaType {
	return mediaTypeOf(nil, nil, []string{extension}, Sniffers)
}

// Resolves a format from a file
func MediaTypeOfFile(file *os.File, mediaTypes []string, extensions []string, sniffers []Sniffer) *MediaType {
	ext := filepath.Ext(file.Name())
	if ext != "" {
		ext = ext[1:] // Remove the leading "."
		if extensions == nil {
			extensions = []string{ext}
		} else {
			extensions = append(extensions, ext)
		}
	}

	return mediaTypeOf(NewSnifferFileContent(file), mediaTypes, extensions, sniffers)
}

// Resolves a format from a file, and nothing else
func MediaTypeOfFileOnly(file *os.File) *MediaType {
	return MediaTypeOfFile(file, nil, nil, Sniffers)
}

// Resolves a format from bytes, e.g. from an HTTP response.
func MediaTypeOfBytes(bytes []byte, mediaTypes []string, extensions []string, sniffers []Sniffer) *MediaType {
	return mediaTypeOf(NewSnifferBytesContent(bytes), mediaTypes, extensions, sniffers)
}

// Resolves a format from bytes, e.g. from an HTTP response, and nothing else
func MediaTypeOfBytesOnly(bytes []byte) *MediaType {
	return mediaTypeOf(NewSnifferBytesContent(bytes), nil, nil, Sniffers)
}
