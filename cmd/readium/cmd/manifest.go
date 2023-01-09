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

// Indentation used to pretty-print
var indentFlag string

var manifestCmd = &cobra.Command{
	Use:   "manifest <pub-path>",
	Short: "Generate a Readium Web Publication Manifest for a publication",
	Long: `Generate a Readium Web Publication Manifest for a publication.

This command will parse a publication file (such as EPUB, PDF, audiobook, etc.)
and build a Readium Web Publication Manifest for it. The JSON manifest is
printed to stdout.

Examples:
  Print out a minimal JSON RWPM. 
  $ readium manifest publication.epub

  Pretty-print a JSON RWPM using tow-space indent.
  $ readium manifest --indent "  " publication.epub

  Extract the publication title with ` + "`jq`" + `.
  $ readium manifest publication.epub | jq -r .metadata.title
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
		path := filepath.Clean(args[0])
		pub, err := streamer.New(streamer.Config{}).Open(
			asset.File(path), "",
		)
		if err != nil {
			return fmt.Errorf("open failed: %w", err)
		}

		var jsonBytes []byte
		if indentFlag == "" {
			jsonBytes, err = json.Marshal(pub.Manifest)
		} else {
			jsonBytes, err = json.MarshalIndent(pub.Manifest, "", indentFlag)
		}
		if err != nil {
			return fmt.Errorf("json failed: %w", err)
		}

		fmt.Println(string(jsonBytes))
		return err
	},
}

func init() {
	rootCmd.AddCommand(manifestCmd)
	manifestCmd.Flags().StringVarP(&indentFlag, "indent", "i", "", "Indentation used to pretty-print")
}
