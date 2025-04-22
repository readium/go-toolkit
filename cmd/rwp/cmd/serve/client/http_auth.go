package client

import (
	"net/http"
)

type authTransport struct {
	Authorization string
	Transport     http.RoundTripper
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if a.Authorization == "" {
		return a.transport().RoundTrip(req)
	}
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", a.Authorization)
	return a.transport().RoundTrip(req2)
}

func (a *authTransport) transport() http.RoundTripper {
	if a.Transport != nil {
		return a.Transport
	}
	return http.DefaultTransport
}

func newAuthenticatedRoundTripper(auth string, transport *http.Transport) http.RoundTripper {
	return &authTransport{
		Authorization: auth,
		Transport:     transport,
	}
}
