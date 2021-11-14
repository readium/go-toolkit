package extensions

import (
	"strings"
)

func IsHiddenOrThumbs(filename string) bool {
	if strings.HasPrefix(filename, ".") || strings.HasPrefix(filename, "__MACOSX") || filename == "Thumbs.db" {
		return true
	}
	return false
}
