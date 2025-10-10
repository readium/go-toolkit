# Changelog

All notable changes to this project will be documented in this file.

**Warning:** Features marked as *alpha* may change or be removed in a future release without notice. Use with caution.

## [0.12.0] - 2025-10-10

All services are now hidden by default. This mainly affects implementors creating webservers, but you also shouldn't need to manually remove services just to produce clean manifest output. To restore previous functionality, set the `streamer.Config`'s `AddServiceLinks` to `true`

### Added

- ServicesBuilder now has a convenience function `Services` to get the names of all services currently in the builder
- ServicesBuilder now has a `Publicize` and `Privatize` function to toggle the exposure of a service via the links that get added to the WebPub manifest, as well as access to the service via its well-known link path. By default, services are **private**
- `streamer.Config` has a new property, `AddServiceLinks`. Setting this property is equivalent to calling the aforementioned `Publicize` function for every service

### Changed

- `ServiceFactory` now has a required `public` property. This lets a service expose itself via the `Get` and `Links` function if set to true. This also means that by default, all services are now "private". That means they will not be added to manifests as links, or callable by said link's path. They are still directly accessible in Go code using e.g. `Publication.FindService`, and then directly calling their functions by casting them to the correct service type (see e.g. `Publication.Positions`)

## [0.11.0] - 2025-07-30

The WebPub data the toolkit parses and provides has been updated to more closely match the latest WebPub spec. Pay close attention to these changes if you depend on the WebPub output in reading systems/libraries that use an older version of the spec!

### Added

- ServicesBuilder now has a convenience function `RemoveExcept` to remove unecessary services easily
- `metadata.altIdentifiers`, an array of alternative publication identifiers. Populated for EPUBs, current logic is rudamentary
- `contributor.altIdentifiers`, an array of alternative contributor identifiers. Populated for EPUBs, current logic is rudamentary
- `metadata.layout`, along with logic to figure out its effective value depending on the publication type. This replaces `metadata.presentation.layout`. It includes the new `scrolled` value for TTB content like webtoons

### Changed

- `metadata.readingProgression` can only be `ltr`, `rtl`, or empty. `auto`, `ttb`, `btt` are not recognized anymore

### Removed

- `metadata.presentation` no longer exists. Any properties that were parsed from EPUB will be available as keys in the metadata, using the full namespace + key URL, for example `http://www.idpf.org/vocab/rendition/#orientation`
- Link properties no longer contains helpers for `fit`, `clipped`, `orientation`, `overflow`, `spread`, `layout` values, as they are no longer recognized

### Fixed

- Potential bug in FileFetcher regarding OS file handle cleanup made safer

## [0.10.2] - 2025-07-11

### Added

- OnCreatePublication function added to Streamer config

### Changed

- Upgraded dependencies

## [0.10.1] - 2025-05-08

### Fixed

- Streamer was ignoring `InferIgnoredImages` parameter

## [0.10.0] - 2025-05-08

### Added

- New config option available when creating a `Streamer`: `InferIgnoredImages`, a list of hashes of images to ignore when when inferring nonvisual reading
- `analyzer.MatchImage` function that compares an image link's hashes with given hashes to check for a match
- `HashValue` has new `String` and `Equal` convenience functions. `HashList` has a new `Find` convenience function.

### Changed

- Renamed `analyzer.Image` to `analyzer.InspectImage`
- Slight adjustments to behavior of manifest properties functions

### Fixed

- Adds missing switch cases for WCAG 2.2 strings when inferring accessibility
- Pick up a11y metadata that's nested (due to `refines` property) in OPF `meta` elements

## [0.9.0] - 2025-04-30

### Removed

- The `cmd` folder has been removed, along with the `rwp` command and its command-line utilities including the web server. Please use [the new Readium CLI repo](https://github.com/readium/cli) as a replacement. The docker build and executable releases are also now migrated to that repository, and the `rwp` verbiage is now `readium`.

### Added

- Remote streaming of publications is now supported. Sources include HTTP servers (capable of byte range requests), Amazon S3 and S3-compatible object storage, Google Cloud Storage (GCS)
- Added a new helper to transform fetchers and resources into interfaces compatible with Go's `fs.FS` and `fs.File`
- Add support for `hash` property in links, with a list of recognized algorithms and utility functions
- A new `analyzer` package has been added that supports image analysis. We're not 100% sure this will remain in the toolkit, it could migrate to the cli repository.

### Changed

- In order to support remote streaming, a lot of APIs have been altered to accept a `context.Context` as the first parameter, to provide implementors with the ability to e.g. cancel a request to fetch a resource.
- `ReadAsString`, `ReadAsJSON`, and `ReadAsXML` functions have been removed from `Resource` and are instead available as helper functions.

## [0.8.1] - 2025-02-24

### Changed

- Docker containers & releases now properly build ARM (32-bit) images with v7 (not v6) support

## [0.8.0] - 2025-02-24

### Added

- Support for [EPUB Accessibility 1.1](https://www.w3.org/TR/epub-a11y-11/) conformance values
- `--version` flag for `rwp`
- Output of `go-toolkit` version in WebPub metadata. [Based on the Go module pseudo-version](https://github.com/readium/go-toolkit/issues/80#issuecomment-2673888192)

### Changed

- A11y `conformsTo` values are now sorted from highest to lowest conformance level

## [0.7.1] - 2025-02-07

### Added

- Add [TDMRep](https://www.w3.org/community/reports/tdmrep/CG-FINAL-tdmrep-20240510/#sec-epub3) support for EPUB 2 & 3.

### Fixed

- Fix typo in EAA exemption.

## [0.7.0] - 2025-01-31

### Added

- Implement support for [EPUB accessibility exemptions](https://www.w3.org/TR/epub-a11y-exemption/), with output in WebPub manifests

### Changed

- The a11y feature `printPageNumbers` has been renamed to `pageNavigation` as per #92
- Dependencies were updated to latest versions, code adjustments were made for changes in pdfcpu