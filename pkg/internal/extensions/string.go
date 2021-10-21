package extensions

import "net/url"

func ToUrlOrNull(raw string) *url.URL { // TODO context URL
	url, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return url
}

func RemovePercentEncoding(raw string) string {
	dec, err := url.QueryUnescape(raw)
	if err != nil {
		return raw
	}
	return dec
}
