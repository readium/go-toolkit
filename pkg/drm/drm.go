package drm

// TODO DRM class
const (
	SCHEME_LCP = "http://readium.org/2014/01/lcp"
)

type DRMLicense interface {
	EncryptionProfile() string
	Decipher(data []byte) []byte
	CanCopy() bool
	Copy(text string) string
}
