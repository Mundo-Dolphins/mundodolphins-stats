package youtubeapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetChannelInfoWithDoerErrorsAndTrimming(t *testing.T) {
	t.Run("success trims response fields", func(t *testing.T) {
		client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/channels" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if req.URL.Query().Get("id") != "UCtest" {
				t.Fatalf("unexpected channel id query: %s", req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
				  "items":[{
				    "id":" UCtest ",
				    "snippet":{"title":" Test Channel "},
				    "contentDetails":{"relatedPlaylists":{"uploads":" UUtest "}}
				  }]
				}`)),
				Header: make(http.Header),
			}, nil
		}))
		client.BaseURL = "https://example.test/"

		info, err := client.GetChannelInfo(context.Background(), "UCtest")
		if err != nil {
			t.Fatalf("GetChannelInfo failed: %v", err)
		}
		if info.ChannelID != "UCtest" || info.Title != "Test Channel" || info.UploadsPlaylistID != "UUtest" {
			t.Fatalf("fields were not trimmed: %+v", info)
		}
	})

	t.Run("http error parses google error", func(t *testing.T) {
		client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)),
				Header:     make(http.Header),
			}, nil
		}))
		client.BaseURL = "https://example.test"

		_, err := client.GetChannelInfo(context.Background(), "UCtest")
		if err == nil || !IsQuotaOrRateLimit(err) {
			t.Fatalf("expected quota error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{bad json`)), Header: make(http.Header)}, nil
		}))
		client.BaseURL = "https://example.test"

		if _, err := client.GetChannelInfo(context.Background(), "UCtest"); err == nil {
			t.Fatal("expected decode error")
		}
	})
}

func TestListPlaylistPageWithDoerQueryAndErrors(t *testing.T) {
	t.Run("defaults max results and includes page token", func(t *testing.T) {
		client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/playlistItems" {
				t.Fatalf("unexpected path %s", req.URL.Path)
			}
			if got := req.URL.Query().Get("maxResults"); got != "50" {
				t.Fatalf("maxResults=%q want 50", got)
			}
			if got := req.URL.Query().Get("pageToken"); got != "NEXT" {
				t.Fatalf("pageToken=%q want NEXT", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
				  "nextPageToken":" LAST ",
				  "pageInfo":{"totalResults":2},
				  "items":[{
				    "snippet":{
				      "title":" Video ",
				      "description":" Desc ",
				      "publishedAt":" 2026-01-01T00:00:00Z ",
				      "channelId":" UC ",
				      "channelTitle":" Channel ",
				      "resourceId":{"videoId":" VID "}
				    }
				  }]
				}`)),
				Header: make(http.Header),
			}, nil
		}))
		client.BaseURL = "https://example.test/"

		page, err := client.ListPlaylistPage(context.Background(), "PL", "NEXT", 0)
		if err != nil {
			t.Fatalf("ListPlaylistPage failed: %v", err)
		}
		if page.NextPageToken != "LAST" || page.TotalResults != 2 || len(page.Items) != 1 {
			t.Fatalf("unexpected page: %+v", page)
		}
		item := page.Items[0]
		if item.VideoID != "VID" || item.Title != "Video" || item.Description != "Desc" ||
			item.PublishedAt != "2026-01-01T00:00:00Z" || item.ChannelID != "UC" || item.ChannelName != "Channel" {
			t.Fatalf("item fields were not normalized: %+v", item)
		}
	})

	t.Run("http error", func(t *testing.T) {
		client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":429,"message":"too many","errors":[{"reason":"rateLimitExceeded"}]}}`)),
				Header:     make(http.Header),
			}, nil
		}))
		client.BaseURL = "https://example.test"

		_, err := client.ListPlaylistPage(context.Background(), "PL", "", 50)
		if err == nil || !IsQuotaOrRateLimit(err) {
			t.Fatalf("expected quota error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		client := NewClient("k", clientDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{bad json`)), Header: make(http.Header)}, nil
		}))
		client.BaseURL = "https://example.test"

		if _, err := client.ListPlaylistPage(context.Background(), "PL", "", 50); err == nil {
			t.Fatal("expected decode error")
		}
	})
}
