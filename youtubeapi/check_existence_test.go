package youtubeapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCheckExistenceSuccess(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/videos" {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		}
		ids := req.URL.Query().Get("id")
		if ids != "a,b,c" {
			t.Fatalf("unexpected ids query: %q", ids)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"items":[{"id":"a"},{"id":"c"}]}`)),
			Header:     make(http.Header),
		}, nil
	}))
	client.BaseURL = "https://example.test"

	existing, err := client.CheckExistence(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("CheckExistence error: %v", err)
	}
	if !existing["a"] {
		t.Fatalf("expected id a to exist")
	}
	if existing["b"] {
		t.Fatalf("expected id b to be missing")
	}
	if !existing["c"] {
		t.Fatalf("expected id c to exist")
	}
}

func TestCheckExistenceAPIError(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)),
			Header:     make(http.Header),
		}, nil
	}))
	client.BaseURL = "https://example.test"

	_, err := client.CheckExistence(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsQuotaOrRateLimit(err) {
		t.Fatalf("expected quota/rate limit error, got: %v", err)
	}
}

func TestCheckExistenceDecodeError(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"items":`)),
			Header:     make(http.Header),
		}, nil
	}))
	client.BaseURL = "https://example.test"

	_, err := client.CheckExistence(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("expected decode error")
	}
}
