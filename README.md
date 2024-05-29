# Readium Go Toolkit

More documentation coming soon! Things are changing too quickly right now.

For development, run `go run cmd/server/main.go` to start the server, which by default listens on `127.0.0.1:5080`. Check out the [example configuration file](https://github.com/readium/go-toolkit/blob/master/cmd/server/configs/config.local.toml.example) for configuration options.

## Command line utility

The `rwp` command provides utilities to parse and generate Web Publications.

To install `rwp` in `~/go/bin`, run `make install`. Use `make build` to build the binary in the current directory.

### Generating a Readium Web Publication Manifest

The `rwp manifest` command will parse a publication file (such as EPUB, PDF, audiobook, etc.) and build a Readium Web Publication Manifest for it. The JSON manifest is
printed to stdout.

Examples:

* Print out a compact JSON RWPM.
    ```sh
    rwp manifest publication.epub
    ```
* Pretty-print a JSON RWPM using two-space indent.
    ```sh
    rwp manifest --indent "  " publication.epub
    ```
* Extract the publication title with `jq`.
    ```sh
    rwp manifest publication.epub | jq -r .metadata.title
    ```

#### Accessibility inference

`rwp manifest` can infer additional accessibility metadata when they are missing, with the `--infer-a11y` flag. It takes one of the following arguments:

| Option           | Description                                                                                            |
|------------------|--------------------------------------------------------------------------------------------------------|
| `no` (*default*) | No accessibility metadata will be inferred.                                                            |
| `merged`         | Accessibility metadata will be inferred and merged with the authored ones in `metadata.accessibility`. |
| `split`          | Accessibility metadata will be inferred but stored separately in `metadata.inferredAccessibility`.     |

```sh
rwp manifest --infer-a11y=merged publication.epub  | jq .metadata
```

##### Inferred metadata

| Key | Value | Inferred? |
|-----|-------|-----------|
| `accessMode` | `auditory` | If the publication contains a reference to an audio or video resource (inspect `resources` and `readingOrder` in RWPM) |
| `accessMode` | `visual` | If the publications contains a reference to an image or a video resource (inspect `resources` and `readingOrder` in RWPM) |
| `accessModeSufficient` | `textual` | If the publication is partially or fully accessible (WCAG A or above)<br>Or if the publication does not contain any image, audio or video resource (inspect "resources" and "readingOrder" in RWPM)<br>Or if the only image available can be identified as a cover |
| `feature` | `displayTransformability` | If the publication is fully accessible (WCAG AA or above)<br>:warning: This property should only be inferred for reflowable EPUB files as it doesn't apply to other formats (FXL, PDF, audiobooks, CBZ/CBR). |
| `feature` | `printPageNumbers` | If the publications contains a page list (check for the presence of a `pageList` collection in RWPM) |
| `feature` | `tableOfContents` | If the publications contains a table of contents (check for the presence of a `toc` collection in RWPM) |
| `feature` | `MathML` | If the publication contains any resource with MathML (check for the presence of the `contains` property where the value is `mathml` in `readingOrder` or `resources` in RWPM) |
| `feature` | `synchronizedAudioText` | If the publication contains any reference to Media Overlays (TBD in RWPM) |

### HTTP streaming of local publications

`rwp serve` starts an HTTP server that serves EPUB, CBZ and other compatible formats from a given directory.
A log is printed to stdout.