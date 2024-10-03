# Readium Go Toolkit

More documentation coming soon! Things are changing too quickly right now.

## Running the server

Run `go run cmd/server/main.go` to start the server, which by default listens on `127.0.0.1:5080`. 

Note: The example configuration file specifies `./publications` as the `publication-path`, but the actual test publications are located in the `test` directory in the project repository. To run the server, you will need to either create a `publications` directory and place your own publication files there, or update the `publication-path` setting in the config file to point to the `test` directory.

The server exposes the following HTTP endpoints:

- `/list.json` - Returns a list of available publications
- `/{filename}/manifest.json` - Returns the Web Publication Manifest for the specified publication  
- `/{filename}/{asset}` - Returns a publication asset (e.g. chapters, images, etc.)

### Accessing a publication

1. Get the list of publications:

```
GET http://127.0.0.1:5080/list.json
```

This returns a JSON array of available publications:

```json
[
  {"filename":"publication1", "path":"cHVibGljYXRpb24x"}, 
  {"filename":"publication2", "path":"cHVibGljYXRpb24y"}
]
```

2. Access the manifest for a publication:

```
GET http://127.0.0.1:5080/{path}/manifest.json
```

Where `{path}` is the base64 URL encoded filename of the publication.

3. Access assets for the publication:

```
GET http://127.0.0.1:5080/{path}/{asset}
``` 

Where `{asset}` is the path of the asset within the publication. These paths can be discovered from the manifest.

### Configuration 

Check out the [example configuration file](https://github.com/readium/go-toolkit/blob/master/cmd/server/configs/config.local.toml.example) for configuration options:

```toml
# Example of a local env config file, useful for development
env-name = "local"
sentry-dsn = "https://deadbeef@sentry.tld"
cache-dsn = "local://not-yet-determined-scheme" 
origins = ["example.com", "localhost", "127.0.0.1"]
log-level = "debug"
bind-address = "localhost" 
bind-port = "15080"
publication-path = "./publications"  # Update this to "./test" to use the test publications
static-path = "./public"
```

- `bind-address` - The network interface to bind the server to
- `bind-port` - The port number to run the server on 
- `origins` - Allowed CORS origins
- `publication-path` - Directory containing publication files to serve 
- `static-path` - Directory containing static assets for the server
- `sentry-dsn` - Sentry error reporting DSN
- `cache-dsn` - Connection string for caching (not yet implemented)  


## Command line utility

The `rwp` command provides utilities to parse and generate Web Publications. 

To install `rwp` in `~/go/bin`, run `make install`. Use `make build` to build the binary in the current directory.

### Generating a Readium Web Publication Manifest

The `rwp manifest` command will parse a publication file (such as EPUB, PDF, audiobook, etc.) and build a Readium Web Publication Manifest for it. The JSON manifest is printed to stdout.

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
| `no` (_default_) | No accessibility metadata will be inferred.                                                            |
| `merged`         | Accessibility metadata will be inferred and merged with the authored ones in `metadata.accessibility`. |
| `split`          | Accessibility metadata will be inferred but stored separately in `metadata.inferredAccessibility`.     |

```sh
rwp manifest --infer-a11y=merged publication.epub  | jq .metadata
```

##### Inferred metadata

| Key                    | Value                        | Inferred?                                                                                                                                                                                                                                                          |
|------------------------|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `accessMode`           | `auditory`                   | If the publication contains a reference to an audio or video resource (inspect `resources` and `readingOrder` in RWPM)                                                                                                                                          |
| `accessMode`           | `visual`                     | If the publications contains a reference to an image or a video resource (inspect `resources` and `readingOrder` in RWPM)                                                                                                                                        |
| `accessModeSufficient` | `textual`                    | If the publication is partially or fully accessible (WCAG A or above)<br>Or if the publication does not contain any image, audio or video resource (inspect "resources" and "readingOrder" in RWPM)<br>Or if the only image available can be identified as a cover |
| `feature`              | `displayTransformability`    | If the publication is fully accessible (WCAG AA or above)<br>:warning: This property should only be inferred for reflowable EPUB files as it doesn't apply to other formats (FXL, PDF, audiobooks, CBZ/CBR).                                                         |
| `feature`              | `printPageNumbers`           | If the publications contains a page list (check for the presence of a `pageList` collection in RWPM)                                                                                                                                                              |
| `feature`              | `tableOfContents`            | If the publications contains a table of contents (check for the presence of a `toc` collection in RWPM)                                                                                                                                                            |
| `feature`              | `MathML`                     | If the publication contains any resource with MathML (check for the presence of the `contains` property where the value is `mathml` in `readingOrder` or `resources` in RWPM)                                                                                       |
| `feature`              | `synchronizedAudioText`      | If the publication contains any reference to Media Overlays (TBD in RWPM)                                                                                                                                                                                          |

### HTTP streaming of local publications

`rwp serve` starts an HTTP server that serves EPUB, CBZ and other compatible formats from a given directory.
A log is printed to stdout. See the above section on "Running the server" for details on the HTTP API.  
