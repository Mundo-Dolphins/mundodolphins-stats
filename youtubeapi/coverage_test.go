package youtubeapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSearchVideoIDsValidateBranches(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[]}`)), Header: make(http.Header)}, nil
	}))
	client.BaseURL = ""
	if _, err := client.SearchVideoIDs(context.Background(), "q", 0); err != nil {
		t.Fatalf("expected success with default base URL: %v", err)
	}

	client.BaseURL = "::bad::"
	if _, err := client.SearchVideoIDs(context.Background(), "q", 10); err == nil {
		t.Fatalf("expected url build error")
	}
}

func TestSearchVideoIDsDoError(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	}))
	client.BaseURL = "https://example.test"
	if _, err := client.SearchVideoIDs(context.Background(), "q", 25); err == nil {
		t.Fatalf("expected do error")
	}
}

func TestHydrateVideosBranches(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[]}`)), Header: make(http.Header)}, nil
	}))
	client.BaseURL = "https://example.test"
	items, err := client.HydrateVideos(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items for empty ids")
	}

	clientErr := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)), Header: make(http.Header)}, nil
	}))
	clientErr.BaseURL = "https://example.test"
	if _, err := clientErr.HydrateVideos(context.Background(), []string{"a"}); err == nil {
		t.Fatalf("expected hydrate error")
	}
}

func TestGoogleAPIErrorBranches(t *testing.T) {
	var nilErr *GoogleAPIError
	if nilErr.Error() == "" {
		t.Fatalf("expected non-empty message for nil receiver")
	}
	if (&GoogleAPIError{StatusCode: 500, Message: "m"}).Error() == "" {
		t.Fatalf("expected non-empty message")
	}
	if IsQuotaOrRateLimit(errors.New("plain")) {
		t.Fatalf("expected false for non api error")
	}
	if IsQuotaOrRateLimit(&GoogleAPIError{StatusCode: 403, Reasons: []string{"other"}}) {
		t.Fatalf("expected false for non quota reasons")
	}
}
