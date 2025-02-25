# Changelog

All notable changes to this project will be documented in this file.

**Warning:** Features marked as *alpha* may change or be removed in a future release without notice. Use with caution.

## [Unreleased]

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