package fetcher

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/readium/go-toolkit/pkg/pub"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// Acts as a proxy to an actual resource by handling read access.
type Resource interface {

	/**
	 * Direct file for this resource, when available.
	 *
	 * This is meant to be used as an optimization for consumers which can't work efficiently
	 * with streams. However, [file] is not guaranteed to be set, for example if the resource
	 * underwent transformations or is being read from an archive. Therefore, consumers should
	 * always fallback on regular stream reading, using [read] or [ResourceInputStream].
	 */
	File() fs.File

	// Closes this object and releases any resources associated with it.
	// If the object is already closed then invoking this method has no effect.
	Close()

	/**
	 * Returns the link from which the resource was retrieved.
	 *
	 * It might be modified by the [Resource] to include additional metadata, e.g. the
	 * `Content-Type` HTTP header in [Link.type].
	 */
	Link() pub.Link

	/**
	 * Returns data length from metadata if available, or calculated from reading the bytes otherwise.
	 *
	 * This value must be treated as a hint, as it might not reflect the actual bytes length. To get
	 * the real length, you need to read the whole resource.
	 */
	Length() (int64, *ResourceException)

	/**
	 * Reads the bytes at the given range.
	 *
	 * When [range] is null, the whole content is returned. Out-of-range indexes are clamped to the
	 * available length automatically.
	 */
	Read(start int64, end int64) ([]byte, *ResourceException)

	/**
	 * Reads the full content as a [String].
	 *
	 * If [charset] is null, then it is parsed from the `charset` parameter of link().type,
	 * or falls back on UTF-8.
	 */
	ReadAsString(charset encoding.Encoding) (string, *ResourceException) // TODO determine how charset is needed

	// Reads the full content as a JSON object.
	ReadAsJSON() (map[string]interface{}, *ResourceException)

	// Reads the full content as an XML document.
	// TODO decide on the way to represent the XML
	// ReadAsXML() (xml.Token, *ResourceException)
}

func ReadResourceAsString(r Resource) (string, *ResourceException) {
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
		ex := Other(err)
		return "", &ex
	}
	return string(utf8bytes), nil
}

func ReadResourceAsJSON(r Resource) (map[string]interface{}, *ResourceException) {
	str, ex := r.ReadAsString(unicode.UTF8)
	if ex != nil {
		return nil, ex
	}
	var object map[string]interface{}
	err := json.Unmarshal([]byte(str), object)
	if err != nil {
		ex := Other(err)
		return nil, &ex
	}
	return object, nil
}

/*func ReadResourceAsXML(r Resource) (xml.Token, *ResourceException) {
	bytes, ex := r.Read(0, 0)
	if ex != nil {
		return "", ex
	}
	xml.NewDecoder().
}*/

// Errors occurring while accessing a resource.
type ResourceException struct {
	Code  int
	Cause error
}

func (ex ResourceException) Error() string {
	return fmt.Sprintf("%d: %s", ex.Code, ex.Cause)
}

// Equivalent to a 400 HTTP error.
func BadRequest(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusBadRequest,
		Cause: cause,
	}
}

// Equivalent to a 404 HTTP error.
func NotFound(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusNotFound,
		Cause: cause,
	}
}

// Equivalent to a 403 HTTP error.
// This can be returned when trying to read a resource protected with a DRM that is not unlocked.
func Forbidden(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusForbidden,
		Cause: cause,
	}
}

// Equivalent to a 503 HTTP error.
// Used when the source can't be reached, e.g. no Internet connection, or an issue with the file system. Usually this is a temporary error.
func Unavailable(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusServiceUnavailable,
		Cause: cause,
	}
}

// Equivalent to a 507 HTTP error.
// Used when the requested range is too large to be read in memory.
func OutOfMemory(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusInsufficientStorage,
		Cause: cause,
	}
}

// Equivalent to a 416 HTTP error.
// Used when the requested range is not satisfiable (invalid)
func RangeNotSatisfiable(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusRequestedRangeNotSatisfiable,
		Cause: cause,
	}
}

// Equivalent to a 504 HTTP error.
// Used when a request for a file times out (e.g. when fetching from remote storage)
func Timeout(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusGatewayTimeout,
		Cause: cause,
	}
}

// The request was cancelled by the caller.
// For example, when a coroutine is cancelled.
// TODO

// For any other error, such as HTTP 500.
func Other(cause error) ResourceException {
	return ResourceException{
		Code:  http.StatusInternalServerError,
		Cause: cause,
	}
}

// Convert a Go os error to an exception
func OsErrorToException(err error) *ResourceException {
	var ex ResourceException
	if os.IsNotExist(err) {
		ex = NotFound(err)
	} else if os.IsPermission(err) {
		ex = Forbidden(err)
	} else if os.IsTimeout(err) {
		ex = Timeout(err)
	} else {
		ex = Other(err)
	}
	return &ex
}

// Creates a Resource that will always return the given [error].
type FailureResource struct {
	link pub.Link
	ex   ResourceException
}

func (r FailureResource) File() fs.File {
	return nil
}

func (r FailureResource) Close() {}

func (r FailureResource) Link() pub.Link {
	return r.link
}

func (r FailureResource) Length() (int64, *ResourceException) {
	return 0, &r.ex
}

func (r FailureResource) Read(start int64, end int64) ([]byte, *ResourceException) {
	return nil, &r.ex
}

func (r FailureResource) ReadAsString(charset encoding.Encoding) (string, *ResourceException) {
	return "", &r.ex
}

func (r FailureResource) ReadAsJSON() (map[string]interface{}, *ResourceException) {
	return nil, &r.ex
}

func (r FailureResource) ReadAsXML() (xml.Token, *ResourceException) {
	return nil, &r.ex
}

func NewFailureResource(link pub.Link, ex ResourceException) FailureResource {
	return FailureResource{
		link: link,
		ex:   ex,
	}
}

// TODO ProxyResource?

// TODO TransformingResource

// TODO LazyResource

// TODO BufferingResource
