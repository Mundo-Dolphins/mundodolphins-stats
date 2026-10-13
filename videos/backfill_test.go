package videos

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestBackfillVideoMetadataFromDiskCache(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "videos.json")
	videosDir := filepath.Join(dir, "cache")
	selectedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	cf := newCacheFile(selectedAt)
	cf.Items["196609020mia"] = &CacheItem{
		Status: StatusApproved,
		Videos: []Video{{
			ID:         "vid1",
			URL:        "https://www.youtube.com/watch?v=vid1",
			Title:      "Oakland Raiders at Miami Dolphins 1966-09-02",
			SelectedAt: selectedAt,
		}},
		AutoScoring: &AutoScoringMeta{},
	}
	if err := NewFileCache(cachePath).Save(cf); err != nil {
		t.Fatalf("seed video cache: %v", err)
	}
	disk := NewDiskCacheManager(NewPaths(videosDir).CacheDir)
	if err := disk.SaveMultiVideoCache([]youtubeapi.VideoCandidate{{
		ID:          "vid1",
		URL:         "https://www.youtube.com/watch?v=vid1",
		Title:       "Oakland Raiders at Miami Dolphins 1966-09-02",
		Description: "Full game broadcast.",
		Channel:     "Vintage Pro Football",
		ChannelID:   "UCclassic",
		Duration:    2 * time.Hour,
		Embeddable:  true,
		ViewCount:   12345,
	}}); err != nil {
		t.Fatalf("seed disk cache: %v", err)
	}

	stats, err := BackfillVideoMetadata(context.Background(), nil, MetadataBackfillConfig{
		CacheFile: cachePath,
		VideosDir: videosDir,
	})
	if err != nil {
		t.Fatalf("BackfillVideoMetadata error: %v", err)
	}
	if stats.Scanned != 1 || stats.Missing != 1 || stats.CacheHits != 1 || stats.UpdatedVideos != 1 || stats.UpdatedItems != 1 || stats.ChannelsSynced != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	got, err := NewFileCache(cachePath).Load()
	if err != nil {
		t.Fatalf("load updated cache: %v", err)
	}
	video := got.Items["196609020mia"].Videos[0]
	if video.ChannelID != "UCclassic" || video.Channel != "Vintage Pro Football" || video.DurationSeconds != 7200 {
		t.Fatalf("metadata not applied: %+v", video)
	}
	if video.Embeddable == nil || !*video.Embeddable || video.Role != string(ContentRoleFullGame) {
		t.Fatalf("expected embeddable full-game role, got %+v", video)
	}
	auto := got.Items["196609020mia"].AutoScoring
	if auto == nil || auto.ViewCount != 12345 || auto.DurationISO != "PT2H0M0S" || auto.ContentRole != string(ContentRoleFullGame) {
		t.Fatalf("auto-scoring metadata not synced: %+v", auto)
	}

	reg, err := NewChannelRegistry(NewPaths(videosDir).ChannelsFile).Load()
	if err != nil {
		t.Fatalf("load channels registry: %v", err)
	}
	if len(reg.Channels) != 1 || reg.Channels[0].ID != "UCclassic" || reg.Channels[0].VideosCount != 1 {
		t.Fatalf("registry not synced: %+v", reg.Channels)
	}
}

func TestBackfillVideoMetadataDryRunAndErrors(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "videos.json")
	videosDir := filepath.Join(dir, "cache")
	cf := newCacheFile(time.Now())
	cf.Items["game1"] = &CacheItem{Status: StatusApproved, Videos: []Video{{ID: "vid1"}}}
	if err := NewFileCache(cachePath).Save(cf); err != nil {
		t.Fatalf("seed video cache: %v", err)
	}
	disk := NewDiskCacheManager(NewPaths(videosDir).CacheDir)
	if err := disk.SaveMultiVideoCache([]youtubeapi.VideoCandidate{{ID: "vid1", Title: "Cached", Duration: time.Hour}}); err != nil {
		t.Fatalf("seed disk cache: %v", err)
	}

	stats, err := BackfillVideoMetadata(context.Background(), nil, MetadataBackfillConfig{
		CacheFile: cachePath,
		VideosDir: videosDir,
		DryRun:    true,
		MaxVideos: 1,
	})
	if err != nil {
		t.Fatalf("dry-run backfill error: %v", err)
	}
	if stats.Scanned != 1 || stats.Missing != 1 || stats.CacheHits != 1 || stats.UpdatedVideos != 0 {
		t.Fatalf("unexpected dry-run stats: %+v", stats)
	}

	if _, err := BackfillVideoMetadata(context.Background(), nil, MetadataBackfillConfig{}); err == nil {
		t.Fatalf("expected empty cache file error")
	}

	missingPath := filepath.Join(dir, "missing.json")
	if err := NewFileCache(missingPath).Save(newCacheFile(time.Now())); err != nil {
		t.Fatalf("seed missing client cache: %v", err)
	}
	cfMissing, err := NewFileCache(missingPath).Load()
	if err != nil {
		t.Fatalf("load missing client cache: %v", err)
	}
	cfMissing.Items["game2"] = &CacheItem{Status: StatusApproved, Videos: []Video{{ID: "missing"}}}
	if err := NewFileCache(missingPath).Save(cfMissing); err != nil {
		t.Fatalf("save missing client cache: %v", err)
	}
	if _, err := BackfillVideoMetadata(context.Background(), nil, MetadataBackfillConfig{CacheFile: missingPath, VideosDir: filepath.Join(dir, "empty")}); err == nil {
		t.Fatalf("expected missing API client error")
	}
}

func TestApplyCandidateMetadataNoops(t *testing.T) {
	if applyCandidateMetadata(nil, youtubeapi.VideoCandidate{}) {
		t.Fatalf("nil video should not change")
	}
	video := Video{
		ID:              "vid1",
		URL:             "u",
		Title:           "Title",
		Channel:         "Channel",
		ChannelID:       "UC",
		Role:            string(ContentRoleHighlight),
		DurationSeconds: 600,
	}
	embeddable := true
	video.Embeddable = &embeddable
	if applyCandidateMetadata(&video, youtubeapi.VideoCandidate{ID: "vid1"}) {
		t.Fatalf("complete video should not change: %+v", video)
	}
	if valueOr("primary", "fallback") != "primary" || valueOr("", "fallback") != "fallback" {
		t.Fatalf("unexpected valueOr result")
	}
}

func TestSyncChannelRegistryFromCacheUpdatesExistingAndNoops(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "channels.json")
	registry := NewChannelRegistry(path)
	if err := registry.Save(&ChannelRegistryFile{Channels: []ChannelRecord{{
		ID:             "UCclassic",
		Name:           "",
		VideosCount:    0,
		AnalyzedVideos: 0,
		Score:          0,
	}}}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	cf := newCacheFile(time.Now())
	cf.Items["g1"] = &CacheItem{
		Status: StatusApproved,
		Videos: []Video{{ID: "v1", ChannelID: "UCclassic", Channel: "Classic NFL"}},
	}
	cf.Items["g2"] = &CacheItem{
		Status: StatusCandidate,
		Videos: []Video{{ID: "v2", ChannelID: "UCignored", Channel: "Ignored"}},
	}
	cf.Items["g3"] = &CacheItem{
		Status: StatusApproved,
		Videos: []Video{{ID: "v3", ChannelID: "UCDVYQ4Zhbm3S2dlz7P1GBDg", Channel: "NFL"}},
	}

	changed, err := syncChannelRegistryFromCache(path, cf)
	if err != nil {
		t.Fatalf("syncChannelRegistryFromCache error: %v", err)
	}
	if changed != 1 {
		t.Fatalf("changed=%d want=1", changed)
	}
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if len(reg.Channels) != 1 || reg.Channels[0].Name != "Classic NFL" || reg.Channels[0].VideosCount != 1 || reg.Channels[0].Score != 1 {
		t.Fatalf("unexpected registry: %+v", reg.Channels)
	}

	changed, err = syncChannelRegistryFromCache(filepath.Join(dir, "empty-channels.json"), newCacheFile(time.Now()))
	if err != nil {
		t.Fatalf("empty sync error: %v", err)
	}
	if changed != 0 {
		t.Fatalf("empty changed=%d want=0", changed)
	}
}
