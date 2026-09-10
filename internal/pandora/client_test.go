package pandora

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBrowserHeadersAndEndpointErrors(t *testing.T) {
	c, err := NewClient("https://example.test", "user", "secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		for key, want := range map[string]string{
			"Origin":           "https://example.test",
			"Referer":          "https://example.test/",
			"X-Requested-With": "XMLHttpRequest",
			"Sec-Fetch-Site":   "same-origin",
			"Sec-Fetch-Mode":   "cors",
			"Sec-Fetch-Dest":   "empty",
			"Content-Type":     "application/x-www-form-urlencoded",
		} {
			if got := r.Header.Get(key); got != want {
				t.Errorf("%s: got %q, want %q", key, got, want)
			}
		}
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader("secret")), Header: make(http.Header)}, nil
	})
	err = c.login(context.Background())
	if err == nil || err.Error() != "/api/users/login: API HTTP status 400" {
		t.Fatalf("unexpected error: %v", err)
	}
}
