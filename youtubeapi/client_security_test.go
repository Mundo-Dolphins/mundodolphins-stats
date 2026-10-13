package youtubeapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestRequestErrorsDoNotLeakTheAPIKey(t *testing.T) {
	const key = "AIzaSySECRET-KEY-VALUE"
	// A real http.Client wraps transport errors in *url.Error, which embeds the URL (and so the key).
	client := NewClient(key, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})})
	_, err := client.SearchVideoIDs(context.Background(), "dolphins", 5)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "SECRET-KEY-VALUE") {
		t.Fatalf("error leaks the API key: %v", err)
	}
	if !strings.Contains(err.Error(), "network down") {
		t.Fatalf("error lost its cause: %v", err)
	}
}

func TestDefaultHTTPClientHasTimeout(t *testing.T) {
	if defaultHTTPClient.Timeout == 0 {
		t.Fatal("the fallback HTTP client must have a timeout")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
