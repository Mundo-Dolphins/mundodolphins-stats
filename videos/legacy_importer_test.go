package videos

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeIndex struct{ m map[string]string }

func (f fakeIndex) DateToGameID(d string) (string, bool) {
	v, ok := f.m[d]
	return v, ok
}

type fakeIndexWithInfo struct {
	fakeIndex
	info map[string]GameRef
}

func (f fakeIndexWithInfo) GameInfo(d string) (GameRef, bool) {
	v, ok := f.info[d]
	return v, ok
}

func TestImportLegacyOnlyMissing(t *testing.T) {
	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")

	legacyData, err := os.ReadFile(filepath.Join("testdata", "legacy_videos.json"))
	if err != nil {
		t.Fatalf("read legacy testdata: %v", err)
	}
	if err := os.WriteFile(legacyPath, legacyData, 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cacheData, err := os.ReadFile(filepath.Join("testdata", "cache_videos.json"))
	if err != nil {
		t.Fatalf("read cache testdata: %v", err)
	}
	if err := os.WriteFile(cachePath, cacheData, 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	idx := fakeIndexWithInfo{
		fakeIndex: fakeIndex{m: map[string]string{
			"1966-09-02": "196609020mia",
			"1966-09-09": "196609090mia",
		}},
		info: map[string]GameRef{
			"1966-09-09": {AwayTeam: "New York Jets", HomeTeam: "Miami Dolphins", Year: 1966},
		},
	}

	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	importer := &Importer{Now: func() time.Time { return now }}

	summary, err := importer.ImportLegacy(context.TODO(), legacyPath, cachePath, idx, ImportOptions{OnlyMissing: true})
	if err != nil {
		t.Fatalf("import legacy: %v", err)
	}
	if summary.Added != 1 || summary.SkippedExisting != 1 || summary.UnmatchedDates != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	cache := NewFileCache(cachePath)
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["196609090mia"]
	if item == nil {
		t.Fatalf("expected new item for 196609090mia")
	}
	if item.Status != "none" {
		t.Fatalf("expected status none, got %s", item.Status)
	}
	if item.Videos != nil {
		t.Fatalf("expected no videos for empty legacy")
	}
	if item.Query == "" {
		t.Fatalf("expected query to be set")
	}
}

func TestImportLegacyMergeOverwriteAndDuration(t *testing.T) {
	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	cachePath := filepath.Join(t.TempDir(), "cache.json")

	legacy := LegacyVideosFile{
		{
			Date: "1966-09-02",
			Videos: []LegacyVideo{
				{YoutubeID: "EXISTING", Duration: "600000000000"},
				{YoutubeID: "NEW", Duration: 1200000000000},
			},
		},
	}
	legacyBytes, _ := json.Marshal(legacy)
	if err := os.WriteFile(legacyPath, legacyBytes, 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cache := CacheFile{
		Schema:      CacheSchema{Name: cacheSchemaName, Version: cacheSchemaVersion},
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Items: map[string]*CacheItem{
			"196609020mia": {
				Status:    "none",
				Query:     "",
				UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Videos: []Video{{
					ID:              "EXISTING",
					URL:             "https://www.youtube.com/watch?v=EXISTING",
					Title:           "x",
					Channel:         "y",
					DurationSeconds: 10,
					SelectedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				}},
			},
		},
	}
	cacheBytes, _ := json.Marshal(cache)
	if err := os.WriteFile(cachePath, cacheBytes, 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	idx := fakeIndexWithInfo{
		fakeIndex: fakeIndex{m: map[string]string{"1966-09-02": "196609020mia"}},
		info:      map[string]GameRef{"1966-09-02": {AwayTeam: "Oakland Raiders", HomeTeam: "Miami Dolphins", Year: 1966}},
	}

	now := time.Date(2026, 1, 3, 9, 0, 0, 0, time.UTC)
	importer := &Importer{Now: func() time.Time { return now }}

	summary, err := importer.ImportLegacy(context.TODO(), legacyPath, cachePath, idx, ImportOptions{OnlyMissing: false, OverwriteStatus: "none"})
	if err != nil {
		t.Fatalf("import legacy: %v", err)
	}
	if summary.Duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", summary.Duplicates)
	}

	cf, err := NewFileCache(cachePath).Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil {
		t.Fatalf("missing item after import")
	}
	if item.Status != "selected" {
		t.Fatalf("expected status selected, got %s", item.Status)
	}
	if len(item.Videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(item.Videos))
	}
	var newVideo *Video
	for i := range item.Videos {
		if item.Videos[i].ID == "NEW" {
			newVideo = &item.Videos[i]
		}
	}
	if newVideo == nil {
		t.Fatalf("expected NEW video")
	}
	if newVideo.DurationSeconds != 1200 {
		t.Fatalf("expected duration 1200, got %d", newVideo.DurationSeconds)
	}
	if newVideo.SelectedAt.IsZero() {
		t.Fatalf("expected selected_at to be set")
	}
}

func TestLegacyDurationSecondsVariants(t *testing.T) {
	if legacyDurationSeconds(nil) != 0 {
		t.Fatalf("expected 0 for nil")
	}
	if legacyDurationSeconds(float64(2*time.Second)) != 2 {
		t.Fatalf("expected 2 for float64 nanos")
	}
	if legacyDurationSeconds(int64(3*time.Second)) != 3 {
		t.Fatalf("expected 3 for int64 nanos")
	}
	if legacyDurationSeconds(int(4*time.Second)) != 4 {
		t.Fatalf("expected 4 for int nanos")
	}
	if legacyDurationSeconds(json.Number("5000000000")) != 5 {
		t.Fatalf("expected 5 for json.Number nanos")
	}
	if legacyDurationSeconds("6000000000") != 6 {
		t.Fatalf("expected 6 for numeric string nanos")
	}
	if legacyDurationSeconds("2m") != 120 {
		t.Fatalf("expected 120 for duration string")
	}
}

func TestLegacyStatusFromMapped(t *testing.T) {
	if legacyStatusFromMapped(nil) != "none" {
		t.Fatalf("expected none for empty")
	}
	if legacyStatusFromMapped([]Video{}) != "none" {
		t.Fatalf("expected none for empty slice")
	}
	if legacyStatusFromMapped([]Video{{ID: "X"}}) != "selected" {
		t.Fatalf("expected selected for non-empty")
	}
}
