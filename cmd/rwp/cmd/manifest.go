package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/analyzer"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/spf13/cobra"
)

// Indentation used to pretty-print.
var indentFlag string

// Infer accessibility metadata.
var inferA11yFlag InferA11yMetadata

// Infer the number of pages from the generated position list.
var inferPageCountFlag bool

/*var inferIgnoreImageHashesFlag []string

var inferIgnoreImageDirectoryFlag string*/

var hash []string

var inspectImagesFlag bool

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

		path, err := url.FromFilepath(filepath.Clean(args[0]))
		if err != nil {
			return fmt.Errorf("failed creating URL from filepath: %w", err)
		}
		pub, err := streamer.New(streamer.Config{
			InferA11yMetadata: streamer.InferA11yMetadata(inferA11yFlag),
			InferPageCount:    inferPageCountFlag,
		}).Open(
			context.TODO(),
			asset.File(path), "",
		)
		if err != nil {
			return fmt.Errorf("failed opening %s: %w", path, err)
		}

		if inspectImagesFlag {
			hashAlgorithms := make([]manifest.HashAlgorithm, len(hash))
			for i, h := range hash {
				hashAlgorithms[i] = manifest.HashAlgorithm(h)
			}
			inspector := &ImageInspector{
				Algorithms: hashAlgorithms,
				Filesystem: fetcher.ToFS(context.TODO(), pub.Fetcher),
			}

			// Inspect publication files and overwrite the links
			pub.Manifest.ReadingOrder = pub.Manifest.ReadingOrder.Copy(inspector)
			if inspector.Error() != nil {
				return fmt.Errorf("failed inspecting images in reading order: %w", inspector.Error())
			}
			pub.Manifest.Resources = pub.Manifest.Resources.Copy(inspector)
			if inspector.Error() != nil {
				return fmt.Errorf("failed inspecting images in resources: %w", inspector.Error())
			}
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
	manifestCmd.Flags().StringSliceVar(&hash, "hash", []string{string(manifest.HashAlgorithmSHA256), string(manifest.HashAlgorithmMD5)}, "Hashes to use when enhancing links, such as with image inspection. Note visual hashes are more computationally expensive. Acceptable values: sha256,md5,phash-dct,https://blurha.sh")
	manifestCmd.Flags().BoolVar(&inspectImagesFlag, "inspect-images", false, "Inspect images in the manifest. Their links will be enhanced with size, width and height, and hashes")
	// manifestCmd.Flags().StringSliceVar(&inferIgnoreImageHashesFlag, "infer-a11y-ignore-image-hashes", nil, "Ignore the given hashes when inferring textual accessibility. Hashes are in the format <algorithm>:<base64 value>, separated by commas.")
	// manifestCmd.Flags().StringVar(&inferIgnoreImageDirectoryFlag, "infer-a11y-ignore-image-dir", "", "Ignore the images in a given directory when inferring textual accessibility.")
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

type ImageInspector struct {
	Filesystem fs.FS
	Algorithms []manifest.HashAlgorithm
	err        error
}

func (n *ImageInspector) Error() error {
	return n.err
}

// TransformHREF implements ManifestTransformer
func (n *ImageInspector) TransformHREF(href manifest.HREF) manifest.HREF {
	// Identity
	return href
}

// TransformLink implements ManifestTransformer
func (n *ImageInspector) TransformLink(link manifest.Link) manifest.Link {
	if n.err != nil || link.MediaType == nil || !link.MediaType.IsBitmap() {
		return link
	}

	newLink, err := analyzer.Image(n.Filesystem, link, n.Algorithms)
	if err != nil {
		n.err = errors.Wrap(err, "failed inspecting image "+link.Href.String())
		return link
	}
	return *newLink
}

// TransformManifest implements ManifestTransformer
func (n *ImageInspector) TransformManifest(manifest manifest.Manifest) manifest.Manifest {
	// Identity
	return manifest
}

// TransformMetadata implements ManifestTransformer
func (n *ImageInspector) TransformMetadata(metadata manifest.Metadata) manifest.Metadata {
	// Identity
	return metadata
}
