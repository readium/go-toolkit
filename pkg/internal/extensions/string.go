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

var fallbackTimeFormats = map[int]string{
	4: "2006",
	7: "2006-01",
}
var zeroTime = time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)

// Parse DublinCore Date
// https://www.dublincore.org/specifications/dublin-core/dcmi-terms/elements11/date/
func ParseDate(raw string) *time.Time {
	if raw == "" {
		return nil
	}

	// If not long enough to be ISO8601, try YYYY or YYYY-MM
	if len(raw) < 10 {
		fallbackFormat, ok := fallbackTimeFormats[len(raw)]
		if !ok {
			return nil
		}
		t, err := time.Parse(fallbackFormat, raw)
		if err != nil {
			return nil
		}
		return &t
	}

	t, err := iso8601.ParseString(raw)
	if err != nil || t.Before(zeroTime) {
		return nil
	}
	return &t
}
