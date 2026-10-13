package youtubeapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type clientDoerFunc func(req *http.Request) (*http.Response, error)

func (f clientDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestClientSearchAndHydrate(t *testing.T) {
	searchPayload := readFixture(t, "search_list_ok.json")
	videosPayload := readFixture(t, "videos_list_ok.json")

	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(searchPayload)
		case "/videos":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(videosPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient("k", srv.Client())
	client.BaseURL = srv.URL

	ids, err := client.SearchVideoIDs(context.Background(), "miami", 25)
	if err != nil {
		t.Fatalf("SearchVideoIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != "abc123" || ids[1] != "def456" {
		t.Fatalf("unexpected ids: %#v", ids)
	}

	items, err := client.HydrateVideos(context.Background(), ids)
	if err != nil {
		t.Fatalf("HydrateVideos: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Duration != (44*time.Minute + 28*time.Second) {
		t.Fatalf("unexpected duration: %v", items[0].Duration)
	}
	if !items[0].Embeddable {
		t.Fatalf("expected embeddable true")
	}
	if items[1].Embeddable {
		t.Fatalf("expected embeddable false")
	}
}

func TestClientQuotaError(t *testing.T) {
	quotaPayload := readFixture(t, "quota_exceeded.json")

	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(quotaPayload)
	}))
	defer srv.Close()

	client := NewClient("k", srv.Client())
	client.BaseURL = srv.URL

	_, err := client.SearchVideoIDs(context.Background(), "miami", 25)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsQuotaOrRateLimit(err) {
		t.Fatalf("expected quota/rate limit error, got: %v", err)
	}
}

func TestIsQuotaOrRateLimit429(t *testing.T) {
	err := &GoogleAPIError{StatusCode: http.StatusTooManyRequests, Message: "too many requests"}
	if !IsQuotaOrRateLimit(err) {
		t.Fatalf("expected true for 429")
	}
}

func newHTTPTestServerOrSkip(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("httptest.NewServer unavailable in this environment: %v", r)
			}
		}()
		srv = httptest.NewServer(handler)
	}()
	if srv == nil {
		t.Skip("httptest.NewServer unavailable in this environment")
	}
	return srv
}
