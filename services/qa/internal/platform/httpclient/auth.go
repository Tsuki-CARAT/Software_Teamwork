package httpclient

import (
	"net/http"
	"strings"
)

// HeaderTransport injects one configured credential header without mutating
// the caller's request. Empty tokens are allowed for local development.
type HeaderTransport struct {
	Base   http.RoundTripper
	Header string
	Token  string
}

func (t HeaderTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.Token == "" {
		return base.RoundTrip(request)
	}
	cloned := request.Clone(request.Context())
	cloned.Header = request.Header.Clone()
	value := t.Token
	if strings.EqualFold(t.Header, "Authorization") && !strings.HasPrefix(strings.ToLower(value), "bearer ") {
		value = "Bearer " + value
	}
	cloned.Header.Set(t.Header, value)
	return base.RoundTrip(cloned)
}
