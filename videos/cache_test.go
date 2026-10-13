package videos

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileCacheLoadMissing(t *testing.T) {
	cache := NewFileCache(filepath.Join(t.TempDir(), "videos.json"))
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cf.Schema.Name != cacheSchemaName {
		t.Fatalf("unexpected schema name: %s", cf.Schema.Name)
	}
	if len(cf.Items) != 0 {
		t.Fatalf("expected empty items")
	}
}

func TestFileCacheSaveAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "videos.json")
	cache := NewFileCache(path)
	item := &CacheItem{Status: "selected", Query: "test", UpdatedAt: time.Now(), Videos: []Video{{ID: "abc", URL: "url", Title: "t"}}}
	if err := cache.Upsert("game1", item); err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	got, ok, err := cache.Get("game1")
	if err != nil || !ok {
		t.Fatalf("Get error: %v ok=%v", err, ok)
	}
	if got.Status != "selected" {
		t.Fatalf("unexpected status: %s", got.Status)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected data written")
	}
}

func TestFileCacheSaveErrors(t *testing.T) {
	cache := NewFileCache("")
	if err := cache.Save(&CacheFile{}); err == nil {
		t.Fatalf("expected error for empty path")
	}
	cache = NewFileCache(filepath.Join(t.TempDir(), "videos.json"))
	if err := cache.Save(nil); err == nil {
		t.Fatalf("expected error for nil cache")
	}
}

func TestFileCacheLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "videos.json")
	if err := os.WriteFile(path, []byte("bad-json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cache := NewFileCache(path)
	if _, err := cache.Load(); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestFileCacheLoadFillsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "videos.json")
	data := []byte(`{"schema":{"name":"","version":""},"items":null}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cache := NewFileCache(path)
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cf.Schema.Name == "" || cf.Schema.Version == "" {
		t.Fatalf("expected schema defaults")
	}
	if cf.Items == nil {
		t.Fatalf("expected items initialized")
	}
}

func TestFileCacheSaveFillsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "videos.json")
	cache := NewFileCache(path)
	cf := &CacheFile{}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Schema.Name == "" || loaded.Schema.Version == "" {
		t.Fatalf("expected schema defaults after save")
	}
	if loaded.Items == nil {
		t.Fatalf("expected items initialized after save")
	}
	if loaded.GeneratedAt.IsZero() {
		t.Fatalf("expected generated_at set")
	}
}

func TestFileCacheUpsertErrors(t *testing.T) {
	cache := NewFileCache(filepath.Join(t.TempDir(), "videos.json"))
	if err := cache.Upsert("", &CacheItem{}); err == nil {
		t.Fatalf("expected error for empty gameID")
	}
	if err := cache.Upsert("game", nil); err == nil {
		t.Fatalf("expected error for nil item")
	}
}

func TestBuildYouTubeBlock(t *testing.T) {
	cf := &CacheFile{Items: map[string]*CacheItem{}}
	block := BuildYouTubeBlock(cf, "missing")
	if block.Status != "unknown" {
		t.Fatalf("expected unknown")
	}
	cf.Items["game1"] = &CacheItem{Status: "none"}
	block = BuildYouTubeBlock(cf, "game1")
	if block.Status != "none" {
		t.Fatalf("expected none")
	}
	cf.Items["game3"] = &CacheItem{Status: StatusCandidate, Videos: []Video{{ID: "candidate"}}}
	block = BuildYouTubeBlock(cf, "game3")
	if block.Status != StatusCandidate {
		t.Fatalf("expected candidate status")
	}
	if len(block.Videos) != 0 {
		t.Fatalf("candidate videos must not be published")
	}
	embeddable := false
	cf.Items["game4"] = &CacheItem{Status: StatusApproved, Videos: []Video{{ID: "approved", Embeddable: &embeddable}}}
	block = BuildYouTubeBlock(cf, "game4")
	if block.Status != StatusApproved || len(block.Videos) != 1 {
		t.Fatalf("expected approved video published, got %+v", block)
	}
	if block.Videos[0].Embeddable == nil || *block.Videos[0].Embeddable {
		t.Fatalf("expected embeddable metadata to be preserved, got %+v", block.Videos[0])
	}
	cf.Items["game5"] = &CacheItem{
		Status: StatusSelected,
		Videos: []Video{{ID: "auto-selected"}},
		AutoScoring: &AutoScoringMeta{
			ScoreBreakdown: ScoreBreakdown{Total: 90},
			Source:         "youtube-data-api-v3",
		},
	}
	block = BuildYouTubeBlock(cf, "game5")
	if block.Status != StatusCandidate || len(block.Videos) != 0 {
		t.Fatalf("auto-scored selected item must be held for review, got %+v", block)
	}
	block = BuildYouTubeBlock(nil, "game1")
	if block.Status != "unknown" {
		t.Fatalf("expected unknown for nil cache")
	}
	cf.Items["game2"] = nil
	block = BuildYouTubeBlock(cf, "game2")
	if block.Status != "unknown" {
		t.Fatalf("expected unknown for nil item")
	}
}

func TestFileCacheLoadMigratesAutoScoredSelectedToCandidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "videos.json")
	data := []byte(`{
	  "schema":{"name":"mundodolphins.youtube.cache","version":"1.2.0"},
	  "items":{
	    "game1":{
	      "status":"selected",
	      "query":"q",
	      "updated_at":"2026-01-01T00:00:00Z",
	      "videos":[{"id":"v1","url":"u","title":"t","selected_at":"2026-01-01T00:00:00Z"}],
	      "auto_scoring":{"score_breakdown":{"total":90},"matched_teams":true,"matched_date_type":"year_only","source":"youtube-data-api-v3"}
	    }
	  }
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cf, err := NewFileCache(path).Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	item := cf.Items["game1"]
	if item.Status != StatusCandidate {
		t.Fatalf("status=%q want candidate", item.Status)
	}
	if !item.NeedsReview || item.AutoScoring == nil || !item.AutoScoring.NeedsReview {
		t.Fatalf("expected needs_review preserved on item and scoring: %+v", item)
	}
	if len(item.Videos) != 1 || item.Videos[0].ID != "v1" || item.Query != "q" {
		t.Fatalf("legacy data was not preserved: %+v", item)
	}
}
