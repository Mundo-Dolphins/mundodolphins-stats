package testutil

import (
	"io"
	"net/http"
	"strings"
)

// RoundTripFunc adapts a function to http.RoundTripper.
type RoundTripFunc func(*http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// NewClient returns an HTTP client that uses the provided round tripper.
func NewClient(fn RoundTripFunc) *http.Client {
	return &http.Client{Transport: RoundTripFunc(fn)}
}

// NewResponse builds an http.Response with a string body.
func NewResponse(req *http.Request, status int, body string, headers map[string]string) *http.Response {
	r := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}
