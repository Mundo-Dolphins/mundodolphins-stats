package mocks

import (
	"errors"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestClientMockSearchResetAndQuery(t *testing.T) {
	client := NewClient("ignored")
	client.results = []youtubeapi.SearchResult{{ID: "video-1", Title: "Miami Dolphins full game"}}
	client.err = errors.New("quota exceeded")

	results, err := client.Search("dolphins full game")
	if err == nil || err.Error() != "quota exceeded" {
		t.Fatalf("Search error = %v, want quota exceeded", err)
	}
	if len(results) != 1 || results[0].ID != "video-1" {
		t.Fatalf("unexpected search results: %+v", results)
	}
	if got := client.Query(); got != "dolphins full game" {
		t.Fatalf("query = %q, want dolphins full game", got)
	}

	client.Reset()
	if client.Query() != "" || client.results != nil || client.err != nil {
		t.Fatalf("reset did not clear client: %+v", client)
	}
}
