package videos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// channelScanServer returns an httptest.Server that responds to
// channels.list and playlistItems.list with canned responses.
func channelScanServer(t *testing.T, uploadItems []youtubeapi.PlaylistItem) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels":
			resp := map[string]any{
				"items": []map[string]any{
					{
						"id":      r.URL.Query().Get("id"),
						"snippet": map[string]string{"title": "Test Channel"},
						"contentDetails": map[string]any{
							"relatedPlaylists": map[string]string{
								"uploads": "UU_test_playlist",
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		case "/playlistItems":
			items := make([]map[string]any, 0, len(uploadItems))
			for _, it := range uploadItems {
				items = append(items, map[string]any{
					"snippet": map[string]any{
						"title":        it.Title,
						"description":  it.Description,
						"publishedAt":  it.PublishedAt,
						"channelId":    it.ChannelID,
						"channelTitle": it.ChannelName,
						"resourceId":   map[string]string{"videoId": it.VideoID},
					},
				})
			}
			resp := map[string]any{
				"items":    items,
				"pageInfo": map[string]int{"totalResults": len(items)},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestChannelScanner_ScanAll(t *testing.T) {
	playlistItems := []youtubeapi.PlaylistItem{
		{VideoID: "vid1", Title: "Game 1 Video", ChannelID: "ch1"},
		{VideoID: "vid2", Title: "Game 2 Video", ChannelID: "ch1"},
	}

	srv := channelScanServer(t, playlistItems)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cache := NewDiskCacheManager(tmpDir)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)

	channels := []ChannelConfig{
		{ID: "ch_test", Name: "TestChan"},
	}

	scanner := &ChannelScanner{
		Client:   client,
		Cache:    cache,
		Quota:    quota,
		Channels: channels,
	}

	state := &PipelineState{
		ChannelMatches: make(map[string]string),
	}

	results, err := scanner.ScanAll(context.Background(), state)
	if err != nil {
		t.Fatalf("ScanAll: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].VideoCount != 2 {
		t.Errorf("VideoCount = %d, want 2", results[0].VideoCount)
	}
	if results[0].FromCache {
		t.Error("FromCache should be false on first scan")
	}
	if results[0].PagesUsed != 1 {
		t.Errorf("PagesUsed = %d, want 1", results[0].PagesUsed)
	}
	if !state.IsChannelScanned("ch_test") {
		t.Error("channel should be marked as scanned")
	}
	if !state.ChannelScanCompleted {
		t.Error("ChannelScanCompleted should be true")
	}

	// Quota consumed: 1 (channels.list) + 1 (playlistItems) = 2
	if quota.Used() != 2 {
		t.Errorf("quota used = %d, want 2", quota.Used())
	}
}

func TestChannelScanner_SkipsCachedChannels(t *testing.T) {
	srv := channelScanServer(t, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cache := NewDiskCacheManager(tmpDir)
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	channels := []ChannelConfig{
		{ID: "ch_cached", Name: "CachedChan"},
	}

	// Pre-populate cache
	err := cache.SaveChannelCache(&ChannelCacheEntry{
		ChannelID:  "ch_cached",
		Title:      "CachedChan",
		FetchedAt:  now,
		TotalItems: 3,
		Videos: []youtubeapi.PlaylistItem{
			{VideoID: "v1"}, {VideoID: "v2"}, {VideoID: "v3"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	quota := NewQuotaManager(10000)
	scanner := &ChannelScanner{
		Client:   client,
		Cache:    cache,
		Quota:    quota,
		Channels: channels,
	}

	state := &PipelineState{
		ChannelsScanned: []string{"ch_cached"},
		ChannelMatches:  make(map[string]string),
	}

	results, err := scanner.ScanAll(context.Background(), state)
	if err != nil {
		t.Fatalf("ScanAll: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if !results[0].FromCache {
		t.Error("FromCache should be true for already-scanned channel")
	}
	if results[0].VideoCount != 3 {
		t.Errorf("VideoCount from cache = %d, want 3", results[0].VideoCount)
	}
	// No quota should have been used
	if quota.Used() != 0 {
		t.Errorf("quota used = %d, want 0", quota.Used())
	}
}

func TestChannelScanner_StopsOnQuotaExhaustion(t *testing.T) {
	// Give just enough for 1 channels.list but not the playlist call
	srv := channelScanServer(t, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cache := NewDiskCacheManager(tmpDir)
	quota := NewQuotaManager(1) // Only 1 unit available

	channels := []ChannelConfig{
		{ID: "ch_expensive", Name: "ExpensiveChan"},
	}

	scanner := &ChannelScanner{
		Client:   client,
		Cache:    cache,
		Quota:    quota,
		Channels: channels,
	}

	state := &PipelineState{
		ChannelMatches: make(map[string]string),
	}

	results, err := scanner.ScanAll(context.Background(), state)
	if err != nil {
		t.Fatalf("ScanAll: %v", err)
	}

	// channels.list consumes 1 unit. The playlist page can't be fetched (0 left).
	// scanChannel still succeeds but with 0 videos (due to graceful stop in pagination loop).
	if len(results) != 1 {
		t.Errorf("results = %d, want 1", len(results))
	}
	if len(results) > 0 && results[0].VideoCount != 0 {
		t.Errorf("VideoCount = %d, want 0 (no quota for playlist page)", results[0].VideoCount)
	}
	// Even though the channel scan ran, it only collected 0 videos
	// The channel IS marked scanned because scanChannel returned without error
	if !state.IsChannelScanned("ch_expensive") {
		t.Error("channel should be marked scanned")
	}
}

func TestChannelScanner_CollectAllChannelVideos(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewDiskCacheManager(tmpDir)
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	cache.nowFunc = func() time.Time { return now }

	channels := []ChannelConfig{
		{ID: "ch_a", Name: "ChannelA"},
		{ID: "ch_b", Name: "ChannelB"},
	}

	// Save videos for ch_a
	err := cache.SaveChannelCache(&ChannelCacheEntry{
		ChannelID:  "ch_a",
		FetchedAt:  now,
		TotalItems: 2,
		Videos: []youtubeapi.PlaylistItem{
			{VideoID: "vid_a1"}, {VideoID: "vid_a2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// ch_b has no cache

	scanner := &ChannelScanner{
		Cache:    cache,
		Channels: channels,
	}

	items, err := scanner.CollectAllChannelVideos()
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Errorf("collected %d items, want 2", len(items))
	}
}
