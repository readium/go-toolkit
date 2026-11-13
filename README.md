# Readium Go Toolkit

**Looking for the `rwp` command-line tools? Checkout the new [Readium CLI](https://github.com/readium/cli) repo!**

More documentation coming soon!

## Build tags

- `readium_nocloud` leaves out the S3 and Google Cloud Storage support (`S3*` and `GCS*` in the `archive`, `asset` and
  `fetcher` packages, and `AbsoluteURL.ToS3Object`/`ToGSObject`), and with it the AWS and Google Cloud SDKs and gRPC.
  For programs that only read local files or HTTP, a much smaller binary: e.g. `go build -tags readium_nocloud`.
