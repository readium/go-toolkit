package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

/**
 * Implements the transformation of a Resource. It can be used, for example, to decrypt,
 * deobfuscate, inject CSS or JavaScript, correct content – e.g. adding a missing dir="rtl" in an
 * HTML document, pre-process – e.g. before indexing a publication's content, etc.
 *
 * If the transformation doesn't apply, simply return resource unchanged.
 */
type ResourceTransformer func(Resource) Resource

// FileBacked is implemented by resources read from a file of the local file
// system, including through transformations.
type FileBacked interface {
	// FileInfo describes the file the resource is read from, whose bytes
	// may differ from the resource's when it is transformed. It identifies
	// the file, e.g. to tell with os.SameFile whether two resources are read
	// from the same file through symbolic or hard links.
	FileInfo() (fs.FileInfo, error)
}

// ErrNotFileBacked is returned by FileInfo and PinFile for resources which
// aren't read from a file.
var ErrNotFileBacked = errors.New("resource is not read from a file")

// ErrFileChanged is the cause of the errors returned by the reads of a
// resource pinned to a file (see [FilePinner]) when its path names another
// file since.
var ErrFileChanged = archive.ErrFileChanged

// FilePinner is implemented by resources which can be pinned to the file they
// are read from, including through transformations.
type FilePinner interface {
	FileBacked
	// PinFile makes the resource only read from the file described by info,
	// e.g. the file it was checked to be: each read checks the file it opens,
	// and fails with [ErrFileChanged] when the path of the resource names
	// another file, e.g. because it was renamed or replaced by a link since
	// info was taken. It fails with [ErrNotFileBacked] for resources which
	// aren't read from a file.
	PinFile(info fs.FileInfo) error
}

// Located is implemented by resources read from a URL, such as HTTP
// resources. Location is the URL they are read from, once their link is
// resolved: like a file identity (see [FileBacked]), it tells whether links
// spelled differently, e.g. a relative and a root-absolute reference, read
// the same resource.
type Located interface {
	Location() url.AbsoluteURL
}

// LocationPinner binds subsequent reads to the returned URL. A resource
// wrapping a non-URL source returns (nil, nil).
type LocationPinner interface {
	PinLocation() (url.URL, error)
}

// PinLocation binds reads through resource transformations to their source
// URL. It fails when a resource reports a location but cannot pin it.
func PinLocation(r Resource) (url.URL, error) {
	switch p := r.(type) {
	case LocationPinner:
		location, err := p.PinLocation()
		if location != nil || err != nil {
			return location, err
		}
	case interface{ PinLocation() url.URL }:
		if location := p.PinLocation(); location != nil {
			return location, nil
		}
	}
	if _, located := r.(Located); located {
		return nil, errors.New("resource cannot pin its source URL")
	}
	return nil, nil
}

// Stable is implemented by resources which can report whether their content
// is read from a single version of their source, which can't be replaced
// while they are open, such as the entries of a local archive (see
// [archive.StableEntry]). Remote resources, and files, which may be renamed
// or replaced unless pinned, aren't. Resources wrapping others, which may
// transform them, don't implement it.
//
// For local files, the version is the file opened: it assumes that files
// aren't rewritten in place while they are open, i.e. that their producers
// finish writing them first, and replace them by renaming new files over
// them.
type Stable interface {
	Stable() bool
}

// pinFile pins a resource to a file, if it can be.
func pinFile(res Resource, info fs.FileInfo) error {
	if p, ok := res.(FilePinner); ok {
		return p.PinFile(info)
	}
	return ErrNotFileBacked
}

// fileInfo returns the file information of a FileBacked resource.
func fileInfo(r Resource) (fs.FileInfo, error) {
	if fb, ok := r.(FileBacked); ok {
		return fb.FileInfo()
	}
	return nil, ErrNotFileBacked
}

// Acts as a proxy to an actual resource by handling read access.
type Resource interface {

	// Direct filepath for this resource, when available.
	// Not guaranteed to be set, for example if the resource underwent transformations or is being read from an archive.
	File() string

	// Closes this object and releases any resources associated with it.
	// If the object is already closed then invoking this method has no effect.
	Close()

	// Returns the link from which the resource was retrieved.
	// It might be modified by the [Resource] to include additional metadata, e.g. the `Content-Type` HTTP header in [Link.Type].
	Link() manifest.Link

	// Returns the properties associated with the resource.
	// This is opened for extensions.
	Properties() manifest.Properties

	// Returns data length from metadata if available, or calculated from reading the bytes otherwise.
	// This value must be treated as a hint, as it might not reflect the actual bytes length. To get the real length, you need to read the whole resource.
	Length(ctx context.Context) (int64, *ResourceError)

	// Reads the bytes at the given range.
	// When start and end are zero, the whole content is returned. Out-of-range indexes are clamped to the available length automatically.
	Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError)

	// Stream the bytes at the given range to a writer.
	// When start and end are zero, the whole content is returned. Out-of-range indexes are clamped to the available length automatically.
	Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError)
}

func ReadResourceAsString(ctx context.Context, r Resource) (string, *ResourceError) {
	bin, ex := r.Read(ctx, 0, 0)
	if ex != nil {
		return "", ex
	}
	return decodeString(r, bin)
}

// decodeString decodes the content of a resource into a string, from the
// charset of its media type.
func decodeString(r Resource, bin []byte) (string, *ResourceError) {
	var cs encoding.Encoding
	if mediaType := r.Link().MediaType; mediaType != nil {
		cs = mediaType.Charset()
	}
	if cs == nil {
		cs = unicode.UTF8
	}
	// Valid UTF-8 needs no decoding. Keep the decoder for malformed input so
	// replacement characters and legacy character encodings remain unchanged.
	if cs == unicode.UTF8 && utf8.Valid(bin) {
		return string(bin), nil
	}
	utf8bytes, err := cs.NewDecoder().Bytes(bin)
	if err != nil {
		return "", Other(err)
	}
	return string(utf8bytes), nil
}

func ReadResourceAsJSON(ctx context.Context, r Resource) (map[string]any, *ResourceError) {
	str, ex := ReadResourceAsString(ctx, r)
	if ex != nil {
		return nil, ex
	}
	return parseJSON(str)
}

// ReadResourceAsJSONUpTo is [ReadResourceAsJSON], reading at most limit bytes
// of the resource like [ReadResourceUpTo].
func ReadResourceAsJSONUpTo(ctx context.Context, r Resource, limit int64) (map[string]any, *ResourceError) {
	bin, ex := ReadResourceUpTo(ctx, r, limit)
	if ex != nil {
		return nil, ex
	}
	str, ex := decodeString(r, bin)
	if ex != nil {
		return nil, ex
	}
	return parseJSON(str)
}

func parseJSON(str string) (map[string]any, *ResourceError) {
	var object map[string]any
	err := json.Unmarshal([]byte(str), &object)
	if err != nil {
		return nil, Other(err)
	}
	return object, nil
}

func ReadResourceAsXML(ctx context.Context, r Resource) (*xmlquery.Node, *ResourceError) {
	bin, ex := r.Read(ctx, 0, 0)
	if ex != nil {
		return nil, ex
	}
	return parseXML(bin)
}

// ErrResourceTooLarge is the cause of the errors returned for resources
// larger than the limit they are read into memory with, such as by
// [ReadResourceUpTo].
var ErrResourceTooLarge = extensions.ErrResourceTooLarge

// ReadResourceUpTo reads a whole resource into memory, never holding more
// than limit bytes of it, whatever length it declares. It fails with an
// [OutOfMemory] error wrapping [ErrResourceTooLarge] beyond that.
func ReadResourceUpTo(ctx context.Context, r Resource, limit int64) ([]byte, *ResourceError) {
	w := &extensions.LimitedBuffer{Limit: limit}
	_, rerr := r.Stream(ctx, w, 0, 0)
	if w.Exceeded {
		return nil, OutOfMemory(fmt.Errorf("%w: %s is larger than %s", ErrResourceTooLarge, r.Link().Href, extensions.FormatSize(limit)))
	}
	if rerr != nil {
		return nil, rerr
	}
	return w.Bytes(), nil
}

// ReadResourceAsXMLUpTo is [ReadResourceAsXML], reading at most limit bytes
// of the resource like [ReadResourceUpTo].
func ReadResourceAsXMLUpTo(ctx context.Context, r Resource, limit int64) (*xmlquery.Node, *ResourceError) {
	bin, ex := ReadResourceUpTo(ctx, r, limit)
	if ex != nil {
		return nil, ex
	}
	return parseXML(bin)
}

func parseXML(bin []byte) (*xmlquery.Node, *ResourceError) {
	node, err := xmlquery.ParseWithOptions(bytes.NewReader(bin), xmlquery.ParserOptions{
		Decoder: &xmlquery.DecoderOptions{
			Strict: true,
			Entity: xml.HTMLEntity,
		},
	})
	if err != nil {
		return nil, Other(err)
	}
	return node, nil
}

type ResourceErrorCode uint16

// Error codes with HTTP equivalents
const (
	CodeBadRequest                   ResourceErrorCode = http.StatusBadGateway
	CodeNotFound                     ResourceErrorCode = http.StatusNotFound
	CodeForbidden                    ResourceErrorCode = http.StatusForbidden
	CodeServiceUnavailable           ResourceErrorCode = http.StatusServiceUnavailable
	CodeInsufficientStorage          ResourceErrorCode = http.StatusInsufficientStorage
	CodeRequestedRangeNotSatisfiable ResourceErrorCode = http.StatusRequestedRangeNotSatisfiable
	CodeGatewayTimeout               ResourceErrorCode = http.StatusGatewayTimeout
	CodeInternalServerError          ResourceErrorCode = http.StatusInternalServerError
)

// The rest of the codes
const (
	_ ResourceErrorCode = iota + 1000 // Starts at 1k to not conflict with HTTP-based codes
	Offline
	Cancelled
)

// Errors occurring while accessing a resource.
type ResourceError struct {
	Cause error
	Code  ResourceErrorCode
}

func (ex *ResourceError) HTTPStatus() int {
	if ex.Code > 999 { // HTTP status codes can only be three digits
		return http.StatusInternalServerError
	}
	return int(ex.Code)
}

func (ex *ResourceError) Error() string {
	if ex == nil {
		return "no error"
	}
	if ex.Cause == nil {
		return fmt.Sprintf("resource: error %d", ex.Code)
	}
	return fmt.Sprintf("resource: error %d: %s", ex.Code, ex.Cause.Error())
}

// Unwrap returns the cause of the error, so that errors.Is and errors.As
// inspect it.
func (ex *ResourceError) Unwrap() error {
	if ex == nil {
		return nil
	}
	return ex.Cause
}

func NewResourceError(code ResourceErrorCode) *ResourceError {
	return &ResourceError{Code: code}
}

func NewResourceErrorWithCause(code ResourceErrorCode, cause error) *ResourceError {
	return &ResourceError{Code: code, Cause: cause}
}

// Equivalent to a 400 HTTP error.
func BadRequest(cause error) *ResourceError {
	return &ResourceError{
		Cause: cause,
		Code:  CodeBadRequest,
	}
}

// Equivalent to a 404 HTTP error.
func NotFound(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeNotFound,
		Cause: cause,
	}
}

// Equivalent to a 403 HTTP error.
// This can be returned when trying to read a resource protected with a DRM that is not unlocked.
func Forbidden(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeForbidden,
		Cause: cause,
	}
}

// Equivalent to a 503 HTTP error.
// Used when the source can't be reached, e.g. no Internet connection, or an issue with the file system. Usually this is a temporary error.
func Unavailable(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeServiceUnavailable,
		Cause: cause,
	}
}

// Equivalent to a 507 HTTP error.
// Used when the requested range is too large to be read in memory.
func OutOfMemory(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeInsufficientStorage,
		Cause: cause,
	}
}

// Equivalent to a 416 HTTP error.
// Used when the requested range is not satisfiable (invalid)
func RangeNotSatisfiable(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeRequestedRangeNotSatisfiable,
		Cause: cause,
	}
}

// Equivalent to a 504 HTTP error.
// Used when a request for a file times out (e.g. when fetching from remote storage)
func Timeout(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeGatewayTimeout,
		Cause: cause,
	}
}

// The request was cancelled by the caller.
// For example, when a coroutine is cancelled.
// TODO

// For any other error, such as HTTP 500.
func Other(cause error) *ResourceError {
	return &ResourceError{
		Code:  CodeInternalServerError,
		Cause: cause,
	}
}

// Convert a Go os error to an exception
func OsErrorToException(err error) *ResourceError {
	if os.IsNotExist(err) {
		return NotFound(err)
	} else if os.IsPermission(err) {
		return Forbidden(err)
	} else if os.IsTimeout(err) {
		return Timeout(err)
	} else {
		return Other(err)
	}
}

// Creates a Resource that will always return the given [error].
type FailureResource struct {
	link manifest.Link
	ex   *ResourceError
}

// FileInfo preserves lookup failures for callers checking file identities.
func (r FailureResource) FileInfo() (fs.FileInfo, error) {
	if r.ex.Code == CodeNotFound {
		return nil, ErrNotFileBacked
	}
	return nil, r.ex
}

// File implements Resource
func (r FailureResource) File() string {
	return ""
}

// Close implements Resource
func (r FailureResource) Close() {}

// Link implements Resource
func (r FailureResource) Link() manifest.Link {
	return r.link
}

func (r FailureResource) Properties() manifest.Properties {
	return manifest.Properties{}
}

// Length implements Resource
func (r FailureResource) Length(ctx context.Context) (int64, *ResourceError) {
	return 0, r.ex
}

// Read implements Resource
func (r FailureResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	return nil, r.ex
}

// Stream implements Resource
func (r FailureResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	return -1, r.ex
}

func NewFailureResource(link manifest.Link, ex *ResourceError) FailureResource {
	return FailureResource{
		link: link,
		ex:   ex,
	}
}

// A base class for a [Resource] which acts as a proxy to another one.
// Every function is delegating to the proxied resource, and subclasses should override some of them.
type ProxyResource struct {
	Res Resource
}

// FileInfo implements FileBacked
func (r ProxyResource) FileInfo() (fs.FileInfo, error) {
	return fileInfo(r.Res)
}

// PinLocation binds reads to the underlying URL, when present.
func (r ProxyResource) PinLocation() (url.URL, error) { return PinLocation(r.Res) }

// PinFile implements FilePinner
func (r ProxyResource) PinFile(info fs.FileInfo) error {
	return pinFile(r.Res, info)
}

func (r ProxyResource) File() string {
	return r.Res.File()
}

// Close implements Resource
func (r ProxyResource) Close() {
	r.Res.Close()
}

// Link implements Resource
func (r ProxyResource) Link() manifest.Link {
	return r.Res.Link()
}

func (r ProxyResource) Properties() manifest.Properties {
	return r.Res.Properties()
}

// Length implements Resource
func (r ProxyResource) Length(ctx context.Context) (int64, *ResourceError) {
	return r.Res.Length(ctx)
}

// Read implements Resource
func (r ProxyResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	return r.Res.Read(ctx, start, end)
}

// Stream implements Resource
func (r ProxyResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	return r.Res.Stream(ctx, w, start, end)
}

// CompressedAs implements CompressedResource
func (r ProxyResource) CompressedAs(compressionMethod archive.CompressionMethod) bool {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return false
	}
	return cres.CompressedAs(compressionMethod)
}

// CompressedLength implements CompressedResource
func (r ProxyResource) CompressedLength(ctx context.Context) int64 {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return -1
	}
	return cres.CompressedLength(ctx)
}

// CRC32Checksum implements CompressedResource
func (r ProxyResource) CRC32Checksum(ctx context.Context) *uint32 {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return nil
	}
	return cres.CRC32Checksum(ctx)
}

// StreamCompressed implements CompressedResource
func (r ProxyResource) StreamCompressed(ctx context.Context, w io.Writer) (int64, *ResourceError) {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return -1, Other(errors.New("resource is not compressed"))
	}
	return cres.StreamCompressed(ctx, w)
}

// StreamCompressedGzip implements CompressedResource
func (r ProxyResource) StreamCompressedGzip(ctx context.Context, w io.Writer) (int64, *ResourceError) {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return -1, Other(errors.New("resource is not compressed"))
	}
	return cres.StreamCompressedGzip(ctx, w)
}

// ReadCompressed implements CompressedResource
func (r ProxyResource) ReadCompressed(ctx context.Context) ([]byte, *ResourceError) {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return nil, Other(errors.New("resource is not compressed"))
	}
	return cres.ReadCompressed(ctx)
}

// ReadCompressedGzip implements CompressedResource
func (r ProxyResource) ReadCompressedGzip(ctx context.Context) ([]byte, *ResourceError) {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return nil, Other(errors.New("resource is not compressed"))
	}
	return cres.ReadCompressedGzip(ctx)
}

/**
 * Transforms the bytes of [resource] on-the-fly.
 *
 * Warning: The transformation runs on the full content of [resource], so it's not appropriate for
 * large resources which can't be held in memory. Pass [cacheBytes] = true to cache the result of
 * the transformation. This may be useful if multiple ranges will be read.
 */
type TransformingResource struct {
	mu         sync.Mutex // serializes cached reads and source pinning
	resource   Resource
	transform  func([]byte) []byte
	cacheBytes bool
	_bytes     []byte
}

func NewTransformingResource(resource Resource, cacheBytes bool, transform func([]byte) []byte) *TransformingResource {
	return &TransformingResource{
		resource:   resource,
		cacheBytes: cacheBytes,
		transform:  transform,
	}
}

func (r *TransformingResource) bytes(ctx context.Context) ([]byte, *ResourceError) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r._bytes) > 0 {
		return r._bytes, nil
	}
	bin, err := r.resource.Read(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	bytes := r.transform(bin)
	if len(bytes) == 0 {
		return nil, Other(errors.New("TransformingResource has empty bytes"))
	}

	if r.cacheBytes {
		r._bytes = bytes
	}
	return bytes, nil
}

// FileInfo implements FileBacked
func (r *TransformingResource) FileInfo() (fs.FileInfo, error) {
	return fileInfo(r.resource)
}

// PinLocation binds reads to the underlying URL, when present.
func (r *TransformingResource) PinLocation() (url.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, err := PinLocation(r.resource)
	if u != nil || err != nil {
		r._bytes = nil
	}
	return u, err
}

// PinFile implements FilePinner
func (r *TransformingResource) PinFile(info fs.FileInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r._bytes = nil
	return pinFile(r.resource, info)
}

// File implements Resource. The transformed bytes differ from the file's, so
// it is not exposed: a consumer serving it directly would bypass the
// transformation.
func (r *TransformingResource) File() string {
	return ""
}

// Close implements Resource
func (r *TransformingResource) Close() {
	r.resource.Close()
}

// Link implements Resource
func (r *TransformingResource) Link() manifest.Link {
	return r.resource.Link()
}

func (r *TransformingResource) Properties() manifest.Properties {
	return r.resource.Properties()
}

// Length implements Resource
func (r *TransformingResource) Length(ctx context.Context) (int64, *ResourceError) {
	data, err := r.bytes(ctx)
	return int64(len(data)), err
}

// Read implements Resource
func (r *TransformingResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	data, err := r.bytes(ctx)
	if err != nil {
		return nil, err
	}
	part, err := sliceRange(data, start, end)
	if err != nil {
		return nil, err
	}
	return bytes.Clone(part), nil
}

// Stream implements Resource
func (r *TransformingResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	bytes, err := r.bytes(ctx)
	if err != nil {
		return 0, err
	}
	data, err := sliceRange(bytes, start, end)
	if err != nil {
		return 0, err
	}
	n, nerr := w.Write(data)
	if nerr != nil {
		// Other(nil) is not a nil error.
		return int64(n), Other(nerr)
	}
	return int64(n), nil
}

// Wraps a [Resource] which will be created only when first accessing one of its members.
type LazyResource struct {
	mu        sync.Mutex
	_resource Resource
	factory   func() Resource
}

func NewLazyResource(factory func() Resource) *LazyResource {
	return &LazyResource{
		factory: factory,
	}
}

func (r *LazyResource) resource() Resource {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r._resource == nil {
		r._resource = r.factory()
	}
	return r._resource
}

// FileInfo implements FileBacked
func (r *LazyResource) FileInfo() (fs.FileInfo, error) {
	return fileInfo(r.resource())
}

// PinLocation binds reads to the underlying URL, when present.
func (r *LazyResource) PinLocation() (url.URL, error) { return PinLocation(r.resource()) }

// PinFile implements FilePinner
func (r *LazyResource) PinFile(info fs.FileInfo) error {
	return pinFile(r.resource(), info)
}

func (r *LazyResource) File() string {
	return r.resource().File()
}

// Close implements Resource
func (r *LazyResource) Close() {
	r.mu.Lock()
	res := r._resource
	r.mu.Unlock()
	if res != nil {
		res.Close()
	}
}

// Link implements Resource
func (r *LazyResource) Link() manifest.Link {
	return r.resource().Link()
}

func (r *LazyResource) Properties() manifest.Properties {
	return r.resource().Properties()
}

// Length implements Resource
func (r *LazyResource) Length(ctx context.Context) (int64, *ResourceError) {
	return r.resource().Length(ctx)
}

// Read implements Resource
func (r *LazyResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	return r.resource().Read(ctx, start, end)
}

// Stream implements Resource
func (r *LazyResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	return r.resource().Stream(ctx, w, start, end)
}

// CompressedAs implements CompressedResource
func (r *LazyResource) CompressedAs(compressionMethod archive.CompressionMethod) bool {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return false
	}
	return cres.CompressedAs(compressionMethod)
}

// CompressedLength implements CompressedResource
func (r *LazyResource) CompressedLength(ctx context.Context) int64 {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return -1
	}
	return cres.CompressedLength(ctx)
}

// CRC32Checksum implements CompressedResource
func (r *LazyResource) CRC32Checksum(ctx context.Context) *uint32 {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return nil
	}
	return cres.CRC32Checksum(ctx)
}

// StreamCompressed implements CompressedResource
func (r *LazyResource) StreamCompressed(ctx context.Context, w io.Writer) (int64, *ResourceError) {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return -1, Other(errors.New("resource is not compressed"))
	}
	return cres.StreamCompressed(ctx, w)
}

// StreamCompressedGzip implements CompressedResource
func (r *LazyResource) StreamCompressedGzip(ctx context.Context, w io.Writer) (int64, *ResourceError) {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return -1, Other(errors.New("resource is not compressed"))
	}
	return cres.StreamCompressedGzip(ctx, w)
}

// ReadCompressed implements CompressedResource
func (r *LazyResource) ReadCompressed(ctx context.Context) ([]byte, *ResourceError) {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return nil, Other(errors.New("resource is not compressed"))
	}
	return cres.ReadCompressed(ctx)
}

// ReadCompressedGzip implements CompressedResource
func (r *LazyResource) ReadCompressedGzip(ctx context.Context) ([]byte, *ResourceError) {
	cres, ok := r.resource().(CompressedResource)
	if !ok {
		return nil, Other(errors.New("resource is not compressed"))
	}
	return cres.ReadCompressedGzip(ctx)
}

// TODO FallbackResource, SynchronizedResource, BufferingResource
