package mediatype

import "mime"

// Explicitly add certain mimetypes to the system sniffer to work around OS differences
func init() {
	mime.AddExtensionType(".xml", "application/xml")
	mime.AddExtensionType(".ncx", "application/x-dtbncx+xml")
	mime.AddExtensionType(".opf", "application/oebps-package+xml")
}
