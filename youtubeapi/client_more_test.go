package youtubeapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSearchAndHydrateWithDoer(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/search":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"items":[{"id":{"videoId":"a"}},{"id":{"videoId":"b"}}]}`)),
				Header:     make(http.Header),
			}, nil
		case "/videos":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"items":[{"id":"a","snippet":{"title":"x","description":"y","channelTitle":"z"},"contentDetails":{"duration":"PT10M00S"},"status":{"embeddable":true}},{"id":"b","snippet":{"title":"x2","description":"y2","channelTitle":"z2"},"contentDetails":{"duration":"PT5M00S"},"status":{"embeddable":false}}]}`)),
				Header:     make(http.Header),
			}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		}
	}))
	client.BaseURL = "https://example.test"

	ids, err := client.SearchVideoIDs(context.Background(), "q", 25)
	if err != nil {
		t.Fatalf("SearchVideoIDs error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("unexpected ids: %#v", ids)
	}
	items, err := client.HydrateVideos(context.Background(), ids)
	if err != nil {
		t.Fatalf("HydrateVideos error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: %d", len(items))
	}
	if items[0].Duration != 10*time.Minute {
		t.Fatalf("unexpected duration: %v", items[0].Duration)
	}
}

func TestHydrateVideosSkipsInvalidDuration(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/videos" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[{"id":"a","snippet":{"title":"x","description":"y","channelTitle":"z"},"contentDetails":{"duration":"BAD"},"status":{"embeddable":true}}]}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	}))
	client.BaseURL = "https://example.test"
	items, err := client.HydrateVideos(context.Background(), []string{"a"})
	if err != nil {
		t.Fatalf("HydrateVideos error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected invalid duration to be skipped")
	}
}

func TestSearchVideoIDsHTTPError(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)), Header: make(http.Header)}, nil
	}))
	client.BaseURL = "https://example.test"
	_, err := client.SearchVideoIDs(context.Background(), "q", 25)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsQuotaOrRateLimit(err) {
		t.Fatalf("expected quota error")
	}
}

func TestHydrateVideosHTTPError(t *testing.T) {
	client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":429,"message":"too many","errors":[{"reason":"rateLimitExceeded"}]}}`)),
			Header:     make(http.Header),
		}, nil
	}))
	client.BaseURL = "https://example.test"
	_, err := client.HydrateVideos(context.Background(), []string{"a"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsQuotaOrRateLimit(err) {
		t.Fatalf("expected quota/rate error")
	}
}
