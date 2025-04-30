# Changelog

All notable changes to this project will be documented in this file.

**Warning:** Features marked as *alpha* may change or be removed in a future release without notice. Use with caution.

## [Unreleased]

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