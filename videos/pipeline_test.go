package videos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// writeGameJSON creates a minimal game JSON file for the pipeline to discover.
func writeGameJSON(t *testing.T, dir, gameID, homeTeam, awayTeam, date string) {
	t.Helper()
	gf := map[string]any{
		"game_id": gameID,
		"data": map[string]any{
			"scorebox": map[string]any{
				"teams": []map[string]any{
					{"name": map[string]string{"content": awayTeam}},
					{"name": map[string]string{"content": homeTeam}},
				},
				"meta": map[string]any{
					"date": date,
				},
			},
		},
	}
	data, err := json.Marshal(gf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, gameID+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// pipelineTestServer serves channels, playlistItems, videos, and search endpoints.
func pipelineTestServer(t *testing.T, channelVideos []youtubeapi.PlaylistItem, searchIDs []string, videoDetails []youtubeapi.VideoCandidate) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels":
			resp := map[string]any{
				"items": []map[string]any{{
					"id":      r.URL.Query().Get("id"),
					"snippet": map[string]string{"title": "NFL Throwback"},
					"contentDetails": map[string]any{
						"relatedPlaylists": map[string]string{"uploads": "UU_uploads"},
					},
				}},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		case "/playlistItems":
			items := make([]map[string]any, 0, len(channelVideos))
			for _, it := range channelVideos {
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

		case "/search":
			items := make([]map[string]any, 0, len(searchIDs))
			for _, id := range searchIDs {
				items = append(items, map[string]any{
					"id": map[string]string{"videoId": id},
				})
			}
			resp := map[string]any{"items": items}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		case "/videos":
			// Check if this is a CheckExistence call (part=id only)
			part := r.URL.Query().Get("part")
			if part == "id" {
				// Return all requested IDs as existing
				ids := r.URL.Query().Get("id")
				var items []map[string]string
				for _, id := range splitComma(ids) {
					items = append(items, map[string]string{"id": id})
				}
				resp := map[string]any{"items": items}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(resp)
				return
			}

			items := make([]map[string]any, 0, len(videoDetails))
			for _, v := range videoDetails {
				items = append(items, map[string]any{
					"id": v.ID,
					"snippet": map[string]any{
						"title":        v.Title,
						"description":  v.Description,
						"channelTitle": v.Channel,
						"channelId":    v.ChannelID,
					},
					"contentDetails": map[string]any{
						"duration": formatISO8601(v.Duration),
					},
					"status": map[string]any{
						"embeddable": v.Embeddable,
					},
					"statistics": map[string]any{
						"viewCount": "50000",
					},
				})
			}
			resp := map[string]any{"items": items}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func TestPipeline_BasicRun(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	channelVids := []youtubeapi.PlaylistItem{
		{
			VideoID:     "chan_vid1",
			Title:       "Oakland Raiders vs Miami Dolphins 1966 Full Game",
			Description: "Classic 1966 AFL action",
			ChannelID:   "UCpKizgBSJOBYnfn5z-VjNZg",
			ChannelName: "Vintage Pro Football",
		},
	}

	videoDetails := []youtubeapi.VideoCandidate{
		{
			ID:          "chan_vid1",
			Title:       "Oakland Raiders vs Miami Dolphins 1966 Full Game",
			Description: "Classic 1966 AFL action",
			Channel:     "Vintage Pro Football",
			ChannelID:   "UCvalidchan",
			Duration:    90 * time.Minute,
			Embeddable:  true,
			ViewCount:   50000,
		},
	}

	srv := pipelineTestServer(t, channelVids, nil, videoDetails)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")

	// Create game JSON
	seasonDir := filepath.Join(processedDir, "1966")
	writeGameJSON(t, seasonDir, "196609020mia", "Miami Dolphins", "Oakland Raiders", "September 2, 1966")
	if err := os.MkdirAll(videosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(videosDir, "channels.json"), &ChannelRegistryFile{Channels: []ChannelRecord{{
		ID:             "UCvalidchan",
		Name:           "Vintage Pro Football",
		VideosCount:    3,
		AnalyzedVideos: 3,
		Score:          1,
		LastSeen:       "2025-06-15",
	}}}); err != nil {
		t.Fatal(err)
	}

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:         10000,
			ProcessedDir:     processedDir,
			VideosDir:        videosDir,
			ValidateExisting: false,
			MinDurationSec:   300,
			MaxResults:       25,
			NoSearch:         true, // Only channel scan
		},
		Now: func() time.Time { return now },
	}

	stats, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("Pipeline.Run: %v", err)
	}

	if stats.GamesTotal != 1 {
		t.Errorf("GamesTotal = %d, want 1", stats.GamesTotal)
	}
	if stats.ChannelMatched != 1 {
		t.Errorf("ChannelMatched = %d, want 1", stats.ChannelMatched)
	}
	if stats.QuotaUsed == 0 {
		t.Error("QuotaUsed should be > 0")
	}

	// Verify state was saved
	statePath := filepath.Join(videosDir, "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("state.json should have been saved")
	}

	// Verify videos.json was saved
	cachePath := filepath.Join(videosDir, "videos.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("videos.json should have been saved")
	}
}

func TestPipeline_DryRun(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	srv := pipelineTestServer(t, nil, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")

	seasonDir := filepath.Join(processedDir, "1966")
	writeGameJSON(t, seasonDir, "196609020mia", "Miami Dolphins", "Oakland Raiders", "September 2, 1966")

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:         10000,
			ProcessedDir:     processedDir,
			VideosDir:        videosDir,
			DryRun:           true,
			ValidateExisting: false,
			NoChannelScan:    true,
			NoSearch:         true,
			MinDurationSec:   300,
			MaxResults:       25,
		},
		Now: func() time.Time { return now },
	}

	stats, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("Pipeline.Run: %v", err)
	}

	if stats.GamesTotal != 1 {
		t.Errorf("GamesTotal = %d, want 1", stats.GamesTotal)
	}

	// Dry-run: no state or cache file should be written
	statePath := filepath.Join(videosDir, "state.json")
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("state.json should NOT be saved in dry-run")
	}
}

func TestPipeline_Reset(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	srv := pipelineTestServer(t, nil, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")

	seasonDir := filepath.Join(processedDir, "1966")
	writeGameJSON(t, seasonDir, "196609020mia", "Miami Dolphins", "Oakland Raiders", "September 2, 1966")

	// Pre-create a state file with stale data
	if err := os.MkdirAll(videosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staleState := &PipelineState{
		Schema:               StateSchema{Name: "mundodolphins.youtube.state", Version: "1.0.0"},
		ProcessedGames:       []string{"196609020mia"},
		GamesWithVideo:       []string{"196609020mia"},
		ChannelScanCompleted: true,
	}
	data, _ := json.MarshalIndent(staleState, "", "  ")
	os.WriteFile(filepath.Join(videosDir, "state.json"), data, 0o644)

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:         10000,
			ProcessedDir:     processedDir,
			VideosDir:        videosDir,
			Reset:            true,
			ValidateExisting: false,
			NoChannelScan:    true,
			NoSearch:         true,
			MinDurationSec:   300,
			MaxResults:       25,
		},
		Now: func() time.Time { return now },
	}

	stats, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("Pipeline.Run: %v", err)
	}

	// Game still needs a video (reset cleared the state)
	if stats.GamesNeedingVideo != 1 {
		t.Errorf("GamesNeedingVideo = %d, want 1 (after reset)", stats.GamesNeedingVideo)
	}
}

func TestPipeline_NoClient(t *testing.T) {
	pipeline := &Pipeline{
		Config: PipelineConfig{MaxQuota: 1000},
	}
	_, err := pipeline.Run(context.Background())
	if err == nil {
		t.Error("expected error with nil client")
	}
}

func TestPipeline_NoGames(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")
	os.MkdirAll(processedDir, 0o755)

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:         10000,
			ProcessedDir:     processedDir,
			VideosDir:        videosDir,
			ValidateExisting: false,
			NoChannelScan:    true,
			NoSearch:         true,
		},
		Now: func() time.Time { return now },
	}

	stats, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("Pipeline.Run: %v", err)
	}
	if stats.GamesTotal != 0 {
		t.Errorf("GamesTotal = %d, want 0", stats.GamesTotal)
	}
}

func TestPipeline_InvalidStateFileReturnsError(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	srv := pipelineTestServer(t, nil, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")
	if err := os.MkdirAll(videosDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(videosDir, "state.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:      1000,
			ProcessedDir:  processedDir,
			VideosDir:     videosDir,
			NoChannelScan: true,
			NoSearch:      true,
		},
		Now: func() time.Time { return now },
	}

	_, err := pipeline.Run(context.Background())
	if err == nil {
		t.Fatal("expected invalid state file error")
	}
}

func TestPipeline_InvalidVideoCacheReturnsError(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	srv := pipelineTestServer(t, nil, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")
	if err := os.MkdirAll(videosDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(videosDir, "videos.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:      1000,
			ProcessedDir:  processedDir,
			VideosDir:     videosDir,
			NoChannelScan: true,
			NoSearch:      true,
		},
		Now: func() time.Time { return now },
	}

	_, err := pipeline.Run(context.Background())
	if err == nil {
		t.Fatal("expected invalid videos cache error")
	}
}

func TestPipeline_AllGamesAlreadyHaveVideos(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	srv := pipelineTestServer(t, nil, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")
	seasonDir := filepath.Join(processedDir, "1966")
	writeGameJSON(t, seasonDir, "196609020mia", "Miami Dolphins", "Oakland Raiders", "September 2, 1966")

	cache := &CacheFile{Items: map[string]*CacheItem{
		"196609020mia": {
			Status: StatusApproved,
			Videos: []Video{{ID: "existing", Title: "Existing full game"}},
		},
	}}
	if err := NewFileCache(filepath.Join(videosDir, "videos.json")).Save(cache); err != nil {
		t.Fatalf("Save cache failed: %v", err)
	}

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:       1000,
			ProcessedDir:   processedDir,
			VideosDir:      videosDir,
			NoChannelScan:  true,
			NoSearch:       true,
			MinDurationSec: 300,
		},
		Now: func() time.Time { return now },
	}

	stats, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("Pipeline.Run failed: %v", err)
	}
	if stats.GamesTotal != 1 {
		t.Fatalf("GamesTotal=%d want 1", stats.GamesTotal)
	}
	if stats.GamesNeedingVideo != 0 {
		t.Fatalf("GamesNeedingVideo=%d want 0", stats.GamesNeedingVideo)
	}
}

func TestPipeline_SearchSkippedWhenQuotaExhausted(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	srv := pipelineTestServer(t, nil, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed", "games")
	videosDir := filepath.Join(tmpDir, "videos")
	writeGameJSON(t, filepath.Join(processedDir, "1966"), "196609020mia", "Miami Dolphins", "Oakland Raiders", "September 2, 1966")

	pipeline := &Pipeline{
		Client: client,
		Config: PipelineConfig{
			MaxQuota:       0,
			ProcessedDir:   processedDir,
			VideosDir:      videosDir,
			NoChannelScan:  true,
			NoSearch:       false,
			MinDurationSec: 300,
		},
		Now: func() time.Time { return now },
	}

	stats, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("Pipeline.Run failed: %v", err)
	}
	if !stats.StoppedByQuota {
		t.Fatal("expected pipeline to report quota stop")
	}
	if stats.SearchAdded != 0 || stats.SearchNoResult != 0 {
		t.Fatalf("search should be skipped before processing queue: %+v", stats)
	}
}

func TestGamesWithoutValidVideo(t *testing.T) {
	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"game1": {Status: "selected", Videos: []Video{{ID: "v1"}}},
			"game2": {Status: "selected", Videos: nil},
			"game3": {Status: "selected", Videos: []Video{}},
			"game4": {Status: "none"},
		},
	}

	result := gamesWithoutValidVideo(cf)
	// game2 and game3 have status=selected but no videos
	if len(result) != 2 {
		t.Errorf("gamesWithoutValidVideo = %d, want 2: %v", len(result), result)
	}
}
