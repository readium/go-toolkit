package fetcher

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/xmlquery"
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
	Length() (int64, *ResourceError)

	// Reads the bytes at the given range.
	// When start and end are null, the whole content is returned. Out-of-range indexes are clamped to the available length automatically.
	Read(start int64, end int64) ([]byte, *ResourceError)

	// Stream the bytes at the given range to a writer.
	// When start and end are null, the whole content is returned. Out-of-range indexes are clamped to the available length automatically.
	Stream(w io.Writer, start int64, end int64) (int64, *ResourceError)

	// Reads the full content as a string.
	// Assumes UTF-8 encoding if no Link charset is given
	ReadAsString() (string, *ResourceError)

	// Reads the full content as a JSON object.
	ReadAsJSON() (map[string]interface{}, *ResourceError)

	// Reads the full content as a generic XML document.
	ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError)
}

func ReadResourceAsString(r Resource) (string, *ResourceError) {
	bytes, ex := r.Read(0, 0)
	if ex != nil {
		return "", ex
	}
	cs := r.Link().MediaType().Charset()
	if cs == nil {
		cs = unicode.UTF8
	}
	utf8bytes, err := cs.NewDecoder().Bytes(bytes)
	if err != nil {
		return "", Other(err)
	}
	return string(utf8bytes), nil
}

func ReadResourceAsJSON(r Resource) (map[string]interface{}, *ResourceError) {
	str, ex := r.ReadAsString()
	if ex != nil {
		return nil, ex
	}
	var object map[string]interface{}
	err := json.Unmarshal([]byte(str), &object)
	if err != nil {
		return nil, Other(err)
	}
	return object, nil
}

func ReadResourceAsXML(r Resource, prefixes map[string]string) (*xmlquery.Node, *ResourceError) {
	bytes, ex := r.Read(0, 0)
	if ex != nil {
		return nil, ex
	}
	node, err := xmlquery.ParseWithOptions(strings.NewReader(string(bytes)), xmlquery.ParserOptions{
		Prefixes: prefixes,
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
func (r FailureResource) Length() (int64, *ResourceError) {
	return 0, r.ex
}

// Read implements Resource
func (r FailureResource) Read(start int64, end int64) ([]byte, *ResourceError) {
	return nil, r.ex
}

// Stream implements Resource
func (r FailureResource) Stream(w io.Writer, start int64, end int64) (int64, *ResourceError) {
	return -1, r.ex
}

// ReadAsString implements Resource
func (r FailureResource) ReadAsString() (string, *ResourceError) {
	return "", r.ex
}

// ReadAsJSON implements Resource
func (r FailureResource) ReadAsJSON() (map[string]interface{}, *ResourceError) {
	return nil, r.ex
}

// ReadAsXML implements Resource
func (r FailureResource) ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError) {
	return nil, r.ex
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

// File implements Resource
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
func (r ProxyResource) Length() (int64, *ResourceError) {
	return r.Res.Length()
}

// Read implements Resource
func (r ProxyResource) Read(start int64, end int64) ([]byte, *ResourceError) {
	return r.Res.Read(start, end)
}

// Stream implements Resource
func (r ProxyResource) Stream(w io.Writer, start int64, end int64) (int64, *ResourceError) {
	return r.Res.Stream(w, start, end)
}

// ReadAsString implements Resource
func (r ProxyResource) ReadAsString() (string, *ResourceError) {
	return r.Res.ReadAsString()
}

// ReadAsJSON implements Resource
func (r ProxyResource) ReadAsJSON() (map[string]interface{}, *ResourceError) {
	return r.Res.ReadAsJSON()
}

// ReadAsXML implements Resource
func (r ProxyResource) ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError) {
	return r.Res.ReadAsXML(prefixes)
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
func (r ProxyResource) CompressedLength() int64 {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return -1
	}
	return cres.CompressedLength()
}

// StreamCompressed implements CompressedResource
func (r ProxyResource) StreamCompressed(w io.Writer) (int64, *ResourceError) {
	cres, ok := r.Res.(CompressedResource)
	if !ok {
		return -1, Other(errors.New("resource is not compressed"))
	}
	return cres.StreamCompressed(w)
}

/**
 * Transforms the bytes of [resource] on-the-fly.
 *
 * Warning: The transformation runs on the full content of [resource], so it's not appropriate for
 * large resources which can't be held in memory. Pass [cacheBytes] = true to cache the result of
 * the transformation. This may be useful if multiple ranges will be read.
 */
type TransformingResource struct {
	resource   Resource
	cacheBytes bool
	_bytes     []byte
}

// TODO TransformingResource

// TODO LazyResource

// TODO BufferingResource
