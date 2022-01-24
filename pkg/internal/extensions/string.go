package extensions

import (
	"net/url"
	"time"

	"github.com/relvacode/iso8601"
)

func ToUrlOrNull(raw string) *url.URL { // TODO context URL
	url, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return url
}

func RemovePercentEncoding(raw string) string {
	dec, err := unescape(raw, encodeCUSTOM)
	if err != nil {
		return raw
	}
	return dec
}

func AddPercentEncodingPath(raw string) string {
	return escape(raw, encodeCUSTOM)
}

func ParseDate(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	t, err := iso8601.ParseString(raw)
	if err != nil {
		return nil
	}
	return &t
}

func Contains(strings []string, s string) bool {
	for _, v := range strings {
		if v == s {
			return true
		}
	}
	return false
}

func AddToSet(s []string, e string) []string {
	if !Contains(s, e) {
		s = append(s, e)
	}
	return s
}
