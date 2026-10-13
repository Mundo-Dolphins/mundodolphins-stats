package youtubeapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetChannelInfo(t *testing.T) {
	resp := channelsListResponse{
		Items: []channelsListItem{{
			ID: "UCtest123",
			Snippet: struct {
				Title string `json:"title"`
			}{Title: "Test Channel"},
			ContentDetails: struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			}{RelatedPlaylists: struct {
				Uploads string `json:"uploads"`
			}{Uploads: "UUtest123"}},
		}},
	}

	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/channels" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	info, err := client.GetChannelInfo(context.Background(), "UCtest123")
	if err != nil {
		t.Fatalf("GetChannelInfo: %v", err)
	}
	if info.ChannelID != "UCtest123" {
		t.Errorf("ChannelID = %q, want UCtest123", info.ChannelID)
	}
	if info.Title != "Test Channel" {
		t.Errorf("Title = %q, want Test Channel", info.Title)
	}
	if info.UploadsPlaylistID != "UUtest123" {
		t.Errorf("UploadsPlaylistID = %q, want UUtest123", info.UploadsPlaylistID)
	}
}

func TestGetChannelInfo_EmptyID(t *testing.T) {
	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	_, err := client.GetChannelInfo(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty channel ID")
	}
}

func TestGetChannelInfo_NotFound(t *testing.T) {
	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	_, err := client.GetChannelInfo(context.Background(), "UC_nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}
}

func TestListPlaylistPage(t *testing.T) {
	items := []playlistItemsItem{
		{Snippet: struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			ChannelTitle string `json:"channelTitle"`
			ResourceID   struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
		}{
			Title:        "Test Video 1",
			Description:  "Description 1",
			PublishedAt:  "2024-01-15T00:00:00Z",
			ChannelID:    "UCtest",
			ChannelTitle: "Test Channel",
			ResourceID: struct {
				VideoID string `json:"videoId"`
			}{VideoID: "vid1"},
		}},
		{Snippet: struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			ChannelTitle string `json:"channelTitle"`
			ResourceID   struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
		}{
			Title:        "Test Video 2",
			PublishedAt:  "2024-01-16T00:00:00Z",
			ChannelID:    "UCtest",
			ChannelTitle: "Test Channel",
			ResourceID: struct {
				VideoID string `json:"videoId"`
			}{VideoID: "vid2"},
		}},
	}

	resp := playlistItemsResponse{
		NextPageToken: "TOKEN_PAGE2",
		PageInfo: struct {
			TotalResults int `json:"totalResults"`
		}{TotalResults: 100},
		Items: items,
	}

	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlistItems" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	page, err := client.ListPlaylistPage(context.Background(), "PLtest", "", 50)
	if err != nil {
		t.Fatalf("ListPlaylistPage: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("Items = %d, want 2", len(page.Items))
	}
	if page.NextPageToken != "TOKEN_PAGE2" {
		t.Errorf("NextPageToken = %q, want TOKEN_PAGE2", page.NextPageToken)
	}
	if page.TotalResults != 100 {
		t.Errorf("TotalResults = %d, want 100", page.TotalResults)
	}
	if page.Items[0].VideoID != "vid1" {
		t.Errorf("first video ID = %q, want vid1", page.Items[0].VideoID)
	}
	if page.Items[0].Title != "Test Video 1" {
		t.Errorf("first title = %q, want Test Video 1", page.Items[0].Title)
	}
}

func TestListPlaylistPage_EmptyPlaylistID(t *testing.T) {
	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	_, err := client.ListPlaylistPage(context.Background(), "", "", 50)
	if err == nil {
		t.Error("expected error for empty playlist ID")
	}
}

func TestListPlaylistPage_MaxResultsClamping(t *testing.T) {
	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := r.URL.Query().Get("maxResults")
		if mr != "50" {
			t.Errorf("maxResults = %q, want 50 (clamped)", mr)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items":    []any{},
			"pageInfo": map[string]int{"totalResults": 0},
		})
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	_, err := client.ListPlaylistPage(context.Background(), "PLtest", "", 999)
	if err != nil {
		t.Fatalf("ListPlaylistPage: %v", err)
	}
}

func TestListPlaylistPage_SkipsEmptyVideoIDs(t *testing.T) {
	items := []playlistItemsItem{
		{Snippet: struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			ChannelTitle string `json:"channelTitle"`
			ResourceID   struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
		}{
			Title: "No Video ID",
			ResourceID: struct {
				VideoID string `json:"videoId"`
			}{VideoID: ""}, // empty
		}},
		{Snippet: struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			ChannelTitle string `json:"channelTitle"`
			ResourceID   struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
		}{
			Title: "Has Video ID",
			ResourceID: struct {
				VideoID string `json:"videoId"`
			}{VideoID: "valid_id"},
		}},
	}

	resp := playlistItemsResponse{Items: items}

	srv := newHTTPTestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	page, err := client.ListPlaylistPage(context.Background(), "PLtest", "", 50)
	if err != nil {
		t.Fatalf("ListPlaylistPage: %v", err)
	}
	if len(page.Items) != 1 {
		t.Errorf("Items = %d, want 1 (empty video ID should be skipped)", len(page.Items))
	}
}

func TestGetChannelInfo_BadBaseURL(t *testing.T) {
	client := NewClient("test-key", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":[]}`)), Header: make(http.Header)}, nil
	}))
	client.BaseURL = "://bad"

	_, err := client.GetChannelInfo(context.Background(), "UCtest123")
	if err == nil {
		t.Fatal("expected URL build error")
	}
}

func TestGetChannelInfo_HTTPError(t *testing.T) {
	client := NewClient("test-key", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)),
			Header:     make(http.Header),
		}, nil
	}))
	client.BaseURL = "https://example.test"

	_, err := client.GetChannelInfo(context.Background(), "UCtest123")
	if err == nil {
		t.Fatal("expected API error")
	}
	if !IsQuotaOrRateLimit(err) {
		t.Fatalf("expected quota/rate error, got: %v", err)
	}
}

func TestGetChannelInfo_DecodeError(t *testing.T) {
	client := NewClient("test-key", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":`)), Header: make(http.Header)}, nil
	}))
	client.BaseURL = "https://example.test"

	_, err := client.GetChannelInfo(context.Background(), "UCtest123")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestListPlaylistPage_HTTPErrorAndDecodeError(t *testing.T) {
	t.Run("http error", func(t *testing.T) {
		client := NewClient("test-key", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":429,"message":"rate","errors":[{"reason":"rateLimitExceeded"}]}}`)),
				Header:     make(http.Header),
			}, nil
		}))
		client.BaseURL = "https://example.test"

		_, err := client.ListPlaylistPage(context.Background(), "PLtest", "", 50)
		if err == nil {
			t.Fatal("expected API error")
		}
		if !IsQuotaOrRateLimit(err) {
			t.Fatalf("expected quota/rate error, got: %v", err)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		client := NewClient("test-key", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"items":`)), Header: make(http.Header)}, nil
		}))
		client.BaseURL = "https://example.test"

		_, err := client.ListPlaylistPage(context.Background(), "PLtest", "", 50)
		if err == nil {
			t.Fatal("expected decode error")
		}
	})
}
