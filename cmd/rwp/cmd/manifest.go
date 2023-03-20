package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/spf13/cobra"
)

// Indentation used to pretty-print.
var indentFlag string

// Infer accessibility metadata.
var inferA11yFlag InferA11yMetadata

// Infer the number of pages from the generated position list.
var inferPageCountFlag bool

var manifestCmd = &cobra.Command{
	Use:   "manifest <pub-path>",
	Short: "Generate a Readium Web Publication Manifest for a publication",
	Long: `Generate a Readium Web Publication Manifest for a publication.

This command will parse a publication file (such as EPUB, PDF, audiobook, etc.)
and build a Readium Web Publication Manifest for it. The JSON manifest is
printed to stdout.

Examples:
  Print out a compact JSON RWPM. 
  $ rwp manifest publication.epub

  Pretty-print a JSON RWPM using two-space indent.
  $ rwp manifest --indent "  " publication.epub

  Extract the publication title with ` + "`jq`" + `.
  $ rwp manifest publication.epub | jq -r .metadata.title
  `,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("expects a path to the publication")
		} else if len(args) > 1 {
			return errors.New("accepts a single path to a publication")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// By the time we reach this point, we know that the arguments were
		// properly parsed, and we don't want to show the usage if an API error
		// occurs.
		cmd.SilenceUsage = true

		path := filepath.Clean(args[0])
		pub, err := streamer.New(streamer.Config{
			InferA11yMetadata: streamer.InferA11yMetadata(inferA11yFlag),
			InferPageCount:    inferPageCountFlag,
		}).Open(
			asset.File(path), "",
		)
		if err != nil {
			return fmt.Errorf("failed opening %s: %w", path, err)
		}

		var jsonBytes []byte
		if indentFlag == "" {
			jsonBytes, err = json.Marshal(pub.Manifest)
		} else {
			jsonBytes, err = json.MarshalIndent(pub.Manifest, "", indentFlag)
		}
		if err != nil {
			return fmt.Errorf("failed rendering JSON for %s: %w", path, err)
		}

		fmt.Println(string(jsonBytes))
		return err
	},
}

func init() {
	rootCmd.AddCommand(manifestCmd)
	manifestCmd.Flags().StringVarP(&indentFlag, "indent", "i", "", "Indentation used to pretty-print")
	manifestCmd.Flags().Var(&inferA11yFlag, "infer-a11y", "Infer accessibility metadata: no, merged, split")
	manifestCmd.Flags().BoolVar(&inferPageCountFlag, "infer-page-count", false, "Infer the number of pages from the generated position list.")
}

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
