# Changelog

All notable changes to this project will be documented in this file.

**Warning:** Features marked as *alpha* may change or be removed in a future release without notice. Use with caution.

## [0.15.1]

### Added

- New `audio.WithRetainedCache()` option for the rich audiobook parser: the read-cache blocks fetched while probing stay attached to the parsed publication, and serving its audio files answers those byte ranges — plus the resources' lengths — straight from memory. Chapter-title samples are also pulled through the block cache (costing up to one block of extra transfer per chapter while parsing) so they are retained too. The retained ranges (container headers and chapter samples) are exactly what a browser's demuxer requests before starting playback of an M4B, so with the option enabled those startup requests are served without touching the remote source: in a replay of Chrome's real request sequence against an archive.org M4B with 18 chapters, time to playback dropped from ~6.5s to ~1.6s (and the per-chapter probes from ~0.4s to ~4ms), for roughly one block of memory per chapter and a slightly more expensive parse. Recommended for servers that open once and serve many times; skip it for one-shot parsing

### Changed

- The HTTP, S3 and GCS fetchers now share resource metadata (content length) across the resources they create, so serving many requests for the same bare file performs a single HEAD (or HeadObject/Attrs) call over the fetcher's lifetime instead of one per request. The size is typically already known from parsing the publication, which removes an origin round trip from the time-to-first-byte of every range request a browser makes while streaming media — this matters especially for M4B audiobooks with chapters, where browsers reads every chapter-title sample (one ranged request each) before starting playback

## [0.15.0] - 2026-07-08

### Added

- Parsing of bare Readium Web Publication Manifests (e.g. a standalone `manifest.json`, `.audiobook` or `.divina` manifest file) is now implemented. Previously, trying to open one would panic with `remote HttpFetcher not implemented!`. The publication's resources are served relative to the manifest's location, whether it lives on a local file system or on a remote source (HTTP(S), S3, GCS):
    - HREFs climbing above the manifest's directory (e.g. `../audio/track.mp3`) are supported: the publication is rooted at the topmost directory reached by the manifest's HREFs, and all HREFs are normalized relative to that root
    - Absolute HTTP(S) HREFs in a manifest are fetched as-is using the parser's HTTP client, since a manifest is free to reference resources hosted anywhere
    - For implementers: this is enabled by the new `asset.RelativePublicationAsset` interface (`Location` + `CreateRelativeFetcher`), which the file, HTTP, S3 and GCS assets all implement
- Publications parsed from a WebPub manifest or package now get services attached: a positions service for publications conforming to the PDF (single-PDF reading order only), Divina, or EPUB profiles, and a content (text extraction) service when the reading order contains HTML resources. Previously, WebPubs got no services at all
- "Heavy" (content-based) sniffing of Readium Web Publications is now implemented in the `mediatype` package, closing a long-standing TODO. Bare manifests and packages containing a `manifest.json` are now recognized by their content, including the profile-specific (audiobook, Divina) and LCP-protected variants
- OPDS 2 feeds and publications are now also detected by heavy sniffing, matching the behavior of the other toolkits
- New `TransformHREFs` functions on `Manifest`, `Metadata`, `Link`, `LinkList`, `Contributors` and `PublicationCollectionMap` to apply a transformation to every HREF in a manifest (used for the HREF normalization mentioned above)
- New `fetcher.NewHTTPResource` function to create a `Resource` serving the contents of an HTTP(S) URL
- New `fetcher.EfficientStreamer` interface: a `Resource` can report whether its `Stream` retrieves only the requested byte range from the underlying source, making it at least as efficient as `Read` even for remote sources. The HTTP, S3 and GCS resources report true (their `Stream` is a single ranged request piped through as it arrives), and archive entry resources report true for stored entries but false for deflate-compressed ones (whose ranged `Stream` decompresses from the entry start, unlike `Read` which uses a persistent random-access index). Consumers serving remote publications over HTTP can use this to stream media instead of buffering entire ranges in memory
- Guided navigation documents can now be generated from XHTML/HTML content (*alpha*): the new `guidednavigation/converter` package converts an HTML resource into a guided navigation document
- The EPUB guided navigation service now covers the entire reading order: resources with a SMIL media overlay are served from it as before, while XHTML/HTML resources without one fall back to the new HTML conversion. Next/prev links traverse all guidable resources in reading order
- WebPubs with (X)HTML contents now get a guided navigation service too

### Changed

- EPUB parsing has been optimized:
    - ZIP archive entries are now looked up through a lazily-built index instead of scanning the archive's entire file list for every resource access
    - `encryption.xml` is now read and parsed only once: `protection.IdentifyEPUBProtection` returns the document it already parsed as a new second return value, and the EPUB parser reuses it
    - DRM detection now probes directly for the well-known protection files instead of listing every resource in the publication first
    - Whitespace collapsing of navigation titles no longer uses a regexp
- Streaming publication resources to network connections (e.g. HTTP responses) can now use the kernel's zero-copy `sendfile` fast path: `Stream` calls on local file resources (whole and ranged), stored (uncompressed) entries of local ZIP archives, and the raw deflate passthrough (`StreamCompressed`/`StreamCompressedGzip`) all copy straight from a bare file handle instead of bouncing through userspace buffers. Streams also use a private file handle per call, so concurrent streams of the same resource no longer serialize
- Streaming stored ZIP entries no longer pays a CRC32 verification pass on every serve; checksums are still verified by `Read`-based access
- Streaming stored entries of remote (or otherwise reader-backed) ZIP archives now copies with a 2 MiB buffer instead of `io.Copy`'s 32 KiB, so a large media stream costs one range request per 2 MiB rather than one per 32 KiB. The buffer is deliberately larger than the default remote range-cache threshold so streamed media blocks don't churn the range cache
- `CompressedAs(CompressionMethodStore)` now correctly reports true for stored ZIP entries and exploded-archive entries; it previously returned false for anything but the deflate method
- MP4 (M4B) chapter-title samples, which are often scattered throughout the file, are now fetched in parallel. This can significantly reduce the time it takes to open remote audiobooks with many chapters. The audio parser's `WithConcurrency` option now also bounds this within-file read parallelism
- Parsing a publication whose mediatype promises a profile is now validated like the existing LCPDF check: an Audiobook or Divina publication (or manifest) that doesn't conform to its profile is rejected
- The order of content sniffers has changed: OPDS, LCP license, and WebPub sniffing now run before archive sniffing, so that a package containing a RWPM `manifest.json` isn't misdetected as a CBZ or ZAB
- `mediatype.NewSnifferFileContent` now returns a pointer, and the sniffer context has a new `Close` function which the sniffer uses to release resources once sniffing is done. Previously, archives opened during sniffing were leaked until garbage collection
- `url.FromFilepath` now makes relative paths absolute against the working directory, and handles Windows drive paths (`C:\dir` becomes `file:///C:/dir`). `AbsoluteURL.ToFilepath` performs the reverse conversion, fixing the opening of publications via file URLs on Windows
- Updated dependencies

### Fixed

- Ranged `Read` calls on S3 resources ignored the computed byte range and downloaded the entire object
- Path traversal in fetchers: a `..` in a link HREF handed to the HTTP, S3, or GCS fetcher could address resources outside the fetcher's root (base URL, key or object name prefix). Resources are now contained within the root, the way the file fetcher sandboxes its directory. The file fetcher also no longer matches sibling paths sharing a prefix (e.g. `dir-other` passing as inside `dir`)
- The S3 and GCS fetchers now decode percent-encoded HREFs before looking up objects, so links to object keys containing e.g. spaces resolve correctly
- Concurrent ranged reads of the same local file resource could interfere with each other because they shared the file handle's offset; ranged reads are now position-independent and full reads are serialized
- `Relativize` on URLs only gave up when both the scheme and the host differed, so URLs could get relativized across different schemes or hosts
- Errors while probing for EPUB DRM files (e.g. a timeout on a remote source) are now surfaced instead of being treated as "no DRM"
- Archive entries treated a negative range start as widening the requested range; it is now clamped to 0, consistent with the file fetcher
- Opening an entry of a remote ZIP archive could download the rest of the archive from the origin: the local-file-header probe issued an open-ended range request and then drained it to EOF before closing. Since an entry is re-opened for every ranged read, seeking through a large media file inside a remote archive re-downloaded the archive tail on every request. The probe now drains at most 4 KiB before aborting the transfer, and the headers of entries too large to precache are now cached, so subsequent opens of the same entry make no remote request at all
- Ranged `Read` calls on stored ZIP entries read and discarded every byte before the range start — downloading the whole prefix, for remote archives — instead of seeking. They now seek directly to the range via the raw entry reader (also skipping the CRC32 pass, consistent with `Stream`)
- `Stream` on an HTTP resource required a `206 Partial Content` response even when streaming the whole resource, where no `Range` header is sent and origins correctly answer with `200 OK`, so whole-resource streams always failed. HTTP response bodies are now also closed on error paths in the HTTP resource's `Read` and `Stream`

### Removed

- `SnifferContext.ContentAsRWPM`, a placeholder that panicked when called, has been removed. Heavy sniffing of RWPM content is now actually implemented (see above)
- The content (text extraction) service is no longer attached to EPUB and WebPub publications; the guided navigation service replaces it. It's still available in the code if you need it

## [0.14.0] - 2026-06-06

### Added

- The audiobook parser has been dramatically improved (coded largely by Claude Opus 4.8, xhigh setting). Audiobook refers to any non-WebPub audiobook, such as a folder of audio files, a ZIP with audio files, or a single audio file (.m4b, .mp3, .opus etc.). The previous implementation was techincally non-compliant, as `bitrate` and `duration` were not included in the generated manifest. The new implementation does the folowing:
    - Parses ISO-BMFF (MP4) containers (.m4a, .m4b, .mp4 etc.), OGG containers (.ogg, .opus) etc.
    - Extracts basic metadata, such as title, author, subject, and adds it to the metadata of the WebPub manifest, along with duration and bitrate
    - Exposes the audiobook's cover as a link in the WebPub manifest
    - Builds a WebPub TOC based on individual file names or TOCs inside audio files. .m3u, .m3u8, .pls, .xspf, .cue are also supported as indpendent metadata files
    - Tries to reduce the amount of range requests that have to be made through caching, and tries to reduce latency the initial metadata parsing causes when opening files remotely by making requests in parallel
- Detection of other popular DRM schemes besides LCP. This includes: Adobe ADEPT, Apple Fairplay, Kobo, B&N, and any other generic encryption scheme using `encryption.xml`. There's still not any particular action taken for these DRM schemes, but it lays the groundwork for future decryption or error throwing when a particular DRM scheme is detected

### Changed

- The zran code now uses the [Readium-hosted fork](https://github.com/readium/zran) instead of the personal fork created by @chocolatkey
- When calling `ConformsTo` on a manifest (and publication), the contents of the manifest's `metadata.conformsTo` is now checked for conformance *before* scanning of the reading order and other checks are performed. Whether this is the best approach is TBD
- Various parsers (audio, image, pdf) were moved to their own folders
- The `mediatype` package has gotten some changes based on the other toolkits:
    - `MP3` is now `MPEGAudio`
    - FLAC, MP4, MPEGVideo were added as new mimetypes
    - Some more audio extensions were added to the sniffer

### Fixed

- The HTTP fetcher would, when making byte range requests, only accept an HTTP 206 response. But if the full range is requested (start = 0 and end = 0), HTTP 200 is also an acceptable response
- A slash was being added as a prefix to WebPub manifest links for exploded ebook folders

## [0.13.4] - 2026-03-09

### Changed

- The mimetype for OPUS is now the modern `audio/opus`, to align with the other toolkits
- The mimetype string matcher now removes spaces, so that `audio/ogg; codecs=opus` and `audio/ogg;codecs=opus` are the same when searching for a match

### Fixed

- Fixed race condition causing local file reads to fail due to early closure

## [0.13.3] - 2026-02-27

### Fixed

- The `ConformsTo` function on manifests (and by extension, publications) mistakenly checked the manifest's `links`, not its `readingOrder`, for mediatypes that determine whether the manifest is conforming to a certain profile. This may have caused mistaken cases of a11y inferrence or Divina/Audiobook/EPUB/PDF profile detection.

## [0.13.2] - 2026-02-26

### Fixed

- Fixed a typo in the accessibility metadata (`describeMath` should be `describedMath`).
- File fetcher (for exploded local publications) would panic due to runtime cleanup bug
- Fetcher for archives and exploded publications were [not able to retrieve resources at paths containing special characters](https://github.com/readium/cli/issues/94), such as spaces

## [0.13.1] - 2025-12-08

### Changed

- Now that we not longer make full releases on GitHub for the go-toolkit, it's confusing to have `https://github.com/readium/go-toolkit/releases` as the JSON key for the toolkit version in manifests. The new value is `https://github.com/readium/go-toolkit#version`

## [0.13.0] - 2025-12-04

### Added

- Implemented [zran](https://github.com/madler/zlib/blob/master/examples/zran.h)-like reading of ZIP entries. This only applies to the start offset, not the total length of the resources. Particularly useful for e.g. streaming compressed videos where a browser will send `Range: bytes=0-` then abort when satisfied, which is already hopeless for us to handle. At least when users scrub through video/audio, we can take a `Range: bytes=XXXXXX-` header and start the remote range request for the resource closer to the start of the byte range. This happens in increments of 1MB
- Add CRC32 checksum function for `CompressedResource` to make direct copying of compressed resources to new archives easier

### Fixed

- Fixed bugged `Read` calls that aren't obligated to return the entirety of a resource. This was resulting in errors when reading partial ranges of resources, most notably ones that are range-read by browsers such as audio and video

### Changed

- Updated dependencies

## [0.12.1] - 2025-10-14

### Fixed

- If the `links` array in a manifest is empty, it was set to `null` when the manifest was serialized to JSON. Now, it is not included, which is the proper behavior

## [0.12.0] - 2025-10-14

All services are now hidden by default. This mainly affects implementers creating webservers, but you also shouldn't need to manually remove services just to produce clean manifest output. To restore previous functionality, set the `streamer.Config`'s `AddServiceLinks` to `true`

### Added

- ServicesBuilder now has a convenience function `Services` to get the names of all services currently in the builder
- ServicesBuilder now has a `ExposeLinks` and `HideLinks` function to toggle the exposure of a service via the links that get added to the WebPub manifest, as well as access to the service via its well-known link path. By default, services are **private**
- `streamer.Config` has a new property, `AddServiceLinks`. Setting this property is equivalent to calling the aforementioned `ExposeLinks` function for every service

### Changed

- `ServiceFactory` now has a required `public` property. This lets a service expose itself via the `Get` and `Links` function if set to true. This also means that by default, all services are now "private". That means they will not be added to manifests as links, or callable by said link's path. They are still directly accessible in Go code using e.g. `Publication.FindService`, and then directly calling their functions by casting them to the correct service type (see e.g. `Publication.Positions`)
- Upgraded dependencies

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

- In order to support remote streaming, a lot of APIs have been altered to accept a `context.Context` as the first parameter, to provide implementers with the ability to e.g. cancel a request to fetch a resource.
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