package url

import "strings"

type Scheme string

const (
	SchemeHTTP  Scheme = "http"
	SchemeHTTPS Scheme = "https"
	SchemeData  Scheme = "data"
	SchemeFTP   Scheme = "ftp"
	SchemeS3    Scheme = "s3" // Amazon S3-compatible
	SchemeGS    Scheme = "gs" // Google Cloud Storage
	SchemeOPDS  Scheme = "opds"
	SchemeFile  Scheme = "file"
)

func SchemeFromString(s string) Scheme {
	s = strings.ToLower(s)
	switch s {
	case "http":
		fallthrough
	case "https":
		fallthrough
	case "data":
		fallthrough
	case "ftp":
		fallthrough
	case "s3":
		fallthrough
	case "gs":
		fallthrough
	case "opds":
		fallthrough
	case "file":
		return Scheme(s)
	default:
		// Not a known scheme.
		return ""
	}
}

func (s Scheme) String() string {
	return string(s)
}

func (s Scheme) IsHTTP() bool {
	return s == SchemeHTTP || s == SchemeHTTPS
}

func (s Scheme) IsFile() bool {
	return s == SchemeFile
}

func (s Scheme) IsCloud() bool {
	return s == SchemeS3 || s == SchemeGS
}
