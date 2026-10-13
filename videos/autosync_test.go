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

func TestAutoSyncer_Sync_AddsVideo(t *testing.T) {
	searchCalled := 0
	videoCalled := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		if path == "/search" {
			searchCalled++
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": map[string]string{"videoId": "vid123"}},
				},
			})
			return
		}
		if path == "/videos" {
			videoCalled++
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id": "vid123",
						"snippet": map[string]any{
							"title":        "Miami Dolphins vs Cincinnati Bengals 1969",
							"description":  "Full game from September 14, 1969",
							"channelTitle": "NFL Classics",
							"publishedAt":  "2020-01-01T00:00:00Z",
						},
						"contentDetails": map[string]string{
							"duration": "PT1H30M0S",
						},
						"status": map[string]any{
							"embeddable": true,
						},
						"statistics": map[string]string{
							"viewCount": "50000",
						},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "videos.json")
	fc := NewFileCache(cachePath)

	syncer := &AutoSyncer{
		Client: client,
		Cache:  fc,
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	}

	games := []GameRef{
		{
			GameID:   "196909140cin",
			Year:     1969,
			HomeTeam: "Cincinnati Bengals",
			AwayTeam: "Miami Dolphins",
		},
	}

	stats, err := syncer.Sync(context.Background(), AutoSyncConfig{
		MinDurationSec:    300,
		MaxResults:        10,
		SleepBetweenCalls: 0,
	}, games)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if stats.GamesProcessed != 1 {
		t.Errorf("GamesProcessed = %d, want 1", stats.GamesProcessed)
	}
	if stats.VideosAdded != 1 {
		t.Errorf("VideosAdded = %d, want 1", stats.VideosAdded)
	}
	if searchCalled == 0 {
		t.Error("expected search API to be called")
	}
	if videoCalled == 0 {
		t.Error("expected videos API to be called")
	}

	// Verify cache was persisted
	cf, err := fc.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item, ok := cf.Items["196909140cin"]
	if !ok {
		t.Fatal("expected cache entry for 196909140cin")
	}
	if item.Status != StatusCandidate {
		t.Errorf("status = %q, want 'candidate'", item.Status)
	}
	if len(item.Videos) != 1 || item.Videos[0].ID != "vid123" {
		t.Error("expected video vid123 in cache")
	}
	if item.AutoScoring == nil {
		t.Error("expected AutoScoring metadata")
	}
}

func TestAutoSyncer_Sync_SkipsExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called when game already has a video")
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "videos.json")

	// Pre-populate cache
	cf := &CacheFile{
		Schema: CacheSchema{Name: "mundodolphins.youtube.cache", Version: "1.2.0"},
		Items: map[string]*CacheItem{
			"196909140cin": {
				Status: "selected",
				Videos: []Video{{ID: "existing123"}},
			},
		},
	}
	data, _ := json.MarshalIndent(cf, "", "  ")
	os.WriteFile(cachePath, data, 0644)

	fc := NewFileCache(cachePath)
	syncer := &AutoSyncer{
		Client: client,
		Cache:  fc,
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	}

	games := []GameRef{
		{GameID: "196909140cin", Year: 1969, HomeTeam: "Cincinnati Bengals", AwayTeam: "Miami Dolphins"},
	}

	stats, err := syncer.Sync(context.Background(), AutoSyncConfig{
		Overwrite: false,
	}, games)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if stats.GamesSkipped != 1 {
		t.Errorf("GamesSkipped = %d, want 1", stats.GamesSkipped)
	}
	if stats.VideosAdded != 0 {
		t.Errorf("VideosAdded = %d, want 0", stats.VideosAdded)
	}
}

func TestAutoSyncer_Sync_DryRunDoesNotWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/search" {
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": map[string]string{"videoId": "dryrun1"}},
				},
			})
			return
		}
		if r.URL.Path == "/videos" {
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id": "dryrun1",
						"snippet": map[string]any{
							"title":        "Dolphins at Bengals 1969",
							"description":  "Cincinnati hosts Miami in September 1969",
							"channelTitle": "OldNFL",
							"publishedAt":  "2022-01-01T00:00:00Z",
						},
						"contentDetails": map[string]string{"duration": "PT1H"},
						"status":         map[string]any{"embeddable": true},
						"statistics":     map[string]string{"viewCount": "1000"},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "videos.json")
	fc := NewFileCache(cachePath)

	syncer := &AutoSyncer{
		Client: client,
		Cache:  fc,
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	}

	games := []GameRef{
		{GameID: "196909140cin", Year: 1969, HomeTeam: "Cincinnati Bengals", AwayTeam: "Miami Dolphins"},
	}

	stats, err := syncer.Sync(context.Background(), AutoSyncConfig{
		DryRun:         true,
		MinDurationSec: 300,
		MaxResults:     5,
	}, games)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if stats.VideosAdded != 1 {
		t.Errorf("VideosAdded = %d, want 1", stats.VideosAdded)
	}

	// Cache file should NOT exist (dry run)
	if _, err := os.Stat(cachePath); err == nil {
		t.Error("cache file should NOT be created in dry-run mode")
	}
}

func TestAutoSyncer_Sync_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "videos.json")
	fc := NewFileCache(cachePath)

	syncer := &AutoSyncer{
		Client: client,
		Cache:  fc,
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	}

	games := []GameRef{
		{GameID: "196909140cin", Year: 1969, HomeTeam: "Cincinnati Bengals", AwayTeam: "Miami Dolphins"},
	}

	stats, err := syncer.Sync(context.Background(), AutoSyncConfig{
		MinDurationSec: 300,
		MaxResults:     5,
	}, games)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if stats.GamesNoResult != 1 {
		t.Errorf("GamesNoResult = %d, want 1", stats.GamesNoResult)
	}

	// Cache should still be written with "none" status
	cf, err := fc.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item, ok := cf.Items["196909140cin"]
	if !ok {
		t.Fatal("expected cache entry for no-result game")
	}
	if item.Status != "none" {
		t.Errorf("status = %q, want 'none'", item.Status)
	}
}

func TestAutoSyncer_Sync_NilClient(t *testing.T) {
	fc := NewFileCache(filepath.Join(t.TempDir(), "x.json"))
	syncer := &AutoSyncer{Client: nil, Cache: fc}
	_, err := syncer.Sync(context.Background(), AutoSyncConfig{}, nil)
	if err == nil {
		t.Error("expected error for nil client")
	}
}

func TestAutoSyncer_Sync_NilCache(t *testing.T) {
	client := youtubeapi.NewClient("key", http.DefaultClient)
	syncer := &AutoSyncer{Client: client, Cache: nil}
	_, err := syncer.Sync(context.Background(), AutoSyncConfig{}, nil)
	if err == nil {
		t.Error("expected error for nil cache")
	}
}

func TestAutoSyncer_Sync_LimitRespected(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	fc := NewFileCache(filepath.Join(tmpDir, "cache.json"))

	syncer := &AutoSyncer{
		Client: client,
		Cache:  fc,
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	}

	games := []GameRef{
		{GameID: "196909140cin", Year: 1969, HomeTeam: "Cincinnati Bengals", AwayTeam: "Miami Dolphins"},
		{GameID: "196909210mia", Year: 1969, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders"},
		{GameID: "196909280buf", Year: 1969, HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins"},
	}

	stats, err := syncer.Sync(context.Background(), AutoSyncConfig{
		Limit:      2,
		MaxResults: 5,
	}, games)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if stats.GamesProcessed != 2 {
		t.Errorf("GamesProcessed = %d, want 2 (limit)", stats.GamesProcessed)
	}
}

func TestSeasonAwareGameLoader_LoadAllGames(t *testing.T) {
	tmpDir := t.TempDir()
	seasonDir := filepath.Join(tmpDir, "1969")
	os.MkdirAll(seasonDir, 0755)

	gameJSON := `{
		"game_id": "196909140cin",
		"data": {
			"week": "1",
			"scorebox": {
				"meta": { "date": "September 14, 1969" },
				"teams": [
					{ "name": { "content": "Miami Dolphins" } },
					{ "name": { "content": "Cincinnati Bengals" } }
				]
			}
		}
	}`
	os.WriteFile(filepath.Join(seasonDir, "196909140cin.json"), []byte(gameJSON), 0644)

	loader := NewSeasonAwareGameLoader(tmpDir)
	games, err := loader.LoadAllGames()
	if err != nil {
		t.Fatalf("LoadAllGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].GameID != "196909140cin" {
		t.Errorf("GameID = %q", games[0].GameID)
	}
	if games[0].Week != "1" {
		t.Errorf("Week = %q", games[0].Week)
	}
}

func TestSeasonAwareGameLoader_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewSeasonAwareGameLoader(tmpDir)
	games, err := loader.LoadAllGames()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(games) != 0 {
		t.Errorf("expected 0 games, got %d", len(games))
	}
}

func TestSeasonAwareGameLoader_InvalidBaseDir(t *testing.T) {
	loader := NewSeasonAwareGameLoader("/nonexistent/path/that/does/not/exist")
	_, err := loader.LoadAllGames()
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestSeasonAwareGameLoader_EmptyBaseDir(t *testing.T) {
	loader := NewSeasonAwareGameLoader("")
	_, err := loader.LoadAllGames()
	if err == nil {
		t.Error("expected error for empty base dir")
	}
}

func TestDiscoverAllGames_SeasonFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 2 seasons
	for _, s := range []string{"1969", "1970"} {
		dir := filepath.Join(tmpDir, s)
		os.MkdirAll(dir, 0755)
		gameID := s + "09140cin"
		gameJSON := `{
			"game_id": "` + gameID + `",
			"data": { "scorebox": { "meta": {}, "teams": [ {"name":{"content":"A"}}, {"name":{"content":"B"}} ] } }
		}`
		os.WriteFile(filepath.Join(dir, gameID+".json"), []byte(gameJSON), 0644)
	}

	games, err := DiscoverAllGames(tmpDir, 1969, "")
	if err != nil {
		t.Fatalf("DiscoverAllGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].GameID != "196909140cin" {
		t.Errorf("GameID = %q", games[0].GameID)
	}
}

func TestDiscoverAllGames_GameIDFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "1969")
	os.MkdirAll(dir, 0755)

	for _, gid := range []string{"196909140cin", "196909210mia"} {
		j := `{"game_id":"` + gid + `","data":{"scorebox":{"meta":{},"teams":[{"name":{"content":"A"}},{"name":{"content":"B"}}]}}}`
		os.WriteFile(filepath.Join(dir, gid+".json"), []byte(j), 0644)
	}

	games, err := DiscoverAllGames(tmpDir, 0, "196909210mia")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].GameID != "196909210mia" {
		t.Errorf("GameID = %q", games[0].GameID)
	}
}
