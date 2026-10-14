// Package httpx builds the HTTP client every osmctl request goes through.
package httpx

import (
	"net/http"
	"time"
)

// UserAgent is required by the OSM API, Nominatim and Overpass usage policies,
// which block generic library agents.
func UserAgent(version string) string {
	return "osmctl/" + version + " (+https://github.com/happyhackingspace/osmctl)"
}

type uaTransport struct {
	ua   string
	next http.RoundTripper
}

func (t uaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", t.ua)
	return t.next.RoundTrip(r)
}

// New returns a client that sets the osmctl User-Agent on top of next
// (http.DefaultTransport when nil).
func New(version string, next http.RoundTripper, timeout time.Duration) *http.Client {
	if next == nil {
		next = http.DefaultTransport
	}
	return &http.Client{Transport: uaTransport{ua: UserAgent(version), next: next}, Timeout: timeout}
}
