package helpers

import (
	"errors"

	"github.com/readium/go-toolkit/pkg/streamer"
)

type InferA11yMetadata streamer.InferA11yMetadata

// String is used both by fmt.Print and by Cobra in help text
func (e *InferA11yMetadata) String() string {
	if e == nil {
		return "no"
	}
	switch *e {
	case InferA11yMetadata(streamer.InferA11yMetadataMerged):
		return "merged"
	case InferA11yMetadata(streamer.InferA11yMetadataSplit):
		return "split"
	default:
		return "no"
	}
}

func (e *InferA11yMetadata) Set(v string) error {
	switch v {
	case "no":
		*e = InferA11yMetadata(streamer.InferA11yMetadataNo)
	case "merged":
		*e = InferA11yMetadata(streamer.InferA11yMetadataMerged)
	case "split":
		*e = InferA11yMetadata(streamer.InferA11yMetadataSplit)
	default:
		return errors.New(`must be one of "no", "merged", or "split"`)
	}
	return nil
}

// Type is only used in help text.
func (e *InferA11yMetadata) Type() string {
	return "string"
}
