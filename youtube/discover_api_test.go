package youtube

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type ytDoer func(req *http.Request) (*http.Response, error)

func (f ytDoer) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestDiscoverViaAPI(t *testing.T) {
	searchResp := `{"items":[{"id":{"videoId":"a"}},{"id":{"videoId":"b"}}]}`
	videosResp := `{"items":[
		{"id":"a","snippet":{"title":"Oakland Raiders vs Miami Dolphins 1966-09-02","description":"full game","channelTitle":"Vintage"},"contentDetails":{"duration":"PT44M28S"},"status":{"embeddable":true}},
		{"id":"b","snippet":{"title":"Highlights only","description":"clip","channelTitle":"Highlights"},"contentDetails":{"duration":"PT10M00S"},"status":{"embeddable":true}}
	]}`

	client := youtubeapi.NewClient("k", ytDoer(func(req *http.Request) (*http.Response, error) {
		payload := "{}"
		status := http.StatusOK
		switch req.URL.Path {
		case "/search":
			payload = searchResp
		case "/videos":
			payload = videosResp
		default:
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
	}))
	client.BaseURL = "https://example.test"

	criteria := Criteria{HomeTeam: "Miami Dolphins", VisitorTeam: "Oakland Raiders", DateISO: "1966-09-02"}
	items, err := DiscoverViaAPI(context.Background(), client, criteria, 25, 5*time.Minute)
	if err != nil {
		t.Fatalf("DiscoverViaAPI error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item after filtering, got %d", len(items))
	}
	if items[0].ID != "a" {
		t.Fatalf("unexpected selected id: %s", items[0].ID)
	}
}

func TestDiscoverViaAPINilClient(t *testing.T) {
	items, err := DiscoverViaAPI(context.Background(), nil, Criteria{}, 25, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %#v", items)
	}
}
