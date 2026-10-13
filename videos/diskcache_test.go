package videos

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestDiskCacheManager_ChannelCache(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	channelID := "UC_test_channel"

	if mgr.HasChannelCache(channelID) {
		t.Error("should not have channel cache initially")
	}

	entry := &ChannelCacheEntry{
		ChannelID:  channelID,
		Title:      "Test Channel",
		FetchedAt:  time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		TotalItems: 2,
		Videos: []youtubeapi.PlaylistItem{
			{VideoID: "v1", Title: "Video 1"},
			{VideoID: "v2", Title: "Video 2"},
		},
	}

	if err := mgr.SaveChannelCache(entry); err != nil {
		t.Fatalf("SaveChannelCache: %v", err)
	}

	if !mgr.HasChannelCache(channelID) {
		t.Error("should have channel cache after save")
	}

	loaded, err := mgr.LoadChannelCache(channelID)
	if err != nil {
		t.Fatalf("LoadChannelCache: %v", err)
	}
	if loaded.ChannelID != channelID {
		t.Errorf("ChannelID = %q, want %q", loaded.ChannelID, channelID)
	}
	if len(loaded.Videos) != 2 {
		t.Errorf("Videos count = %d, want 2", len(loaded.Videos))
	}
}

func TestDiskCacheManager_SearchCache(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	gameID := "196609020mia"

	if mgr.HasSearchCache(gameID) {
		t.Error("should not have search cache initially")
	}

	entry := &SearchCacheEntry{
		GameID:   gameID,
		Query:    "dolphins vs raiders 1966",
		VideoIDs: []string{"vid1", "vid2"},
	}

	if err := mgr.SaveSearchCache(entry); err != nil {
		t.Fatalf("SaveSearchCache: %v", err)
	}

	if !mgr.HasSearchCache(gameID) {
		t.Error("should have search cache after save")
	}

	loaded, err := mgr.LoadSearchCache(gameID)
	if err != nil {
		t.Fatalf("LoadSearchCache: %v", err)
	}
	if loaded.GameID != gameID {
		t.Errorf("GameID = %q, want %q", loaded.GameID, gameID)
	}
	if len(loaded.VideoIDs) != 2 {
		t.Errorf("VideoIDs count = %d, want 2", len(loaded.VideoIDs))
	}
}

func TestDiskCacheManager_VideoCache(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	videoID := "abc123"

	if mgr.HasVideoCache(videoID) {
		t.Error("should not have video cache initially")
	}

	entry := &VideoCacheEntry{
		VideoID:   videoID,
		FetchedAt: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		Video: youtubeapi.VideoCandidate{
			ID:    videoID,
			Title: "Test Video",
			URL:   "https://www.youtube.com/watch?v=abc123",
		},
	}

	if err := mgr.SaveVideoCache(entry); err != nil {
		t.Fatalf("SaveVideoCache: %v", err)
	}

	if !mgr.HasVideoCache(videoID) {
		t.Error("should have video cache after save")
	}

	loaded, err := mgr.LoadVideoCache(videoID)
	if err != nil {
		t.Fatalf("LoadVideoCache: %v", err)
	}
	if loaded.Video.Title != "Test Video" {
		t.Errorf("Video.Title = %q, want %q", loaded.Video.Title, "Test Video")
	}
}

func TestDiskCacheManager_LoadMultiVideoCache(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)
	mgr.nowFunc = func() time.Time { return time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC) }

	// Save one video
	if err := mgr.SaveVideoCache(&VideoCacheEntry{
		VideoID: "v1",
		Video:   youtubeapi.VideoCandidate{ID: "v1", Title: "Cached"},
	}); err != nil {
		t.Fatal(err)
	}

	found, missing := mgr.LoadMultiVideoCache([]string{"v1", "v2", "v3"})
	if len(found) != 1 {
		t.Errorf("found = %d, want 1", len(found))
	}
	if len(missing) != 2 {
		t.Errorf("missing = %d, want 2", len(missing))
	}
}

func TestDiskCacheManager_SaveMultiVideoCache(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)
	mgr.nowFunc = func() time.Time { return time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC) }

	candidates := []youtubeapi.VideoCandidate{
		{ID: "v1", Title: "Video 1"},
		{ID: "v2", Title: "Video 2"},
	}

	if err := mgr.SaveMultiVideoCache(candidates); err != nil {
		t.Fatalf("SaveMultiVideoCache: %v", err)
	}

	if !mgr.HasVideoCache("v1") || !mgr.HasVideoCache("v2") {
		t.Error("both videos should be cached")
	}
}

func TestDiskCacheManager_NilEntries(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	if err := mgr.SaveChannelCache(nil); err == nil {
		t.Error("SaveChannelCache(nil) should return error")
	}
	if err := mgr.SaveSearchCache(nil); err == nil {
		t.Error("SaveSearchCache(nil) should return error")
	}
	if err := mgr.SaveVideoCache(nil); err == nil {
		t.Error("SaveVideoCache(nil) should return error")
	}
}

func TestDiskCacheManager_LoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	entry, err := mgr.LoadChannelCache("nonexistent")
	if err != nil {
		t.Errorf("LoadChannelCache should return nil, nil for non-existent: %v", err)
	}
	if entry != nil {
		t.Error("should return nil for non-existent channel cache")
	}

	sEntry, err := mgr.LoadSearchCache("nonexistent")
	if err != nil {
		t.Errorf("LoadSearchCache should return nil, nil for non-existent: %v", err)
	}
	if sEntry != nil {
		t.Error("should return nil for non-existent search cache")
	}

	vEntry, err := mgr.LoadVideoCache("nonexistent")
	if err != nil {
		t.Errorf("LoadVideoCache should return nil, nil for non-existent: %v", err)
	}
	if vEntry != nil {
		t.Error("should return nil for non-existent video cache")
	}
}

func TestDiskCacheManager_LoadInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	for _, tc := range []struct {
		name string
		path string
		load func() error
	}{
		{
			name: "channel",
			path: filepath.Join(dir, "channels", "bad-channel.json"),
			load: func() error {
				_, err := mgr.LoadChannelCache("bad-channel")
				return err
			},
		},
		{
			name: "search",
			path: filepath.Join(dir, "search", "bad-game.json"),
			load: func() error {
				_, err := mgr.LoadSearchCache("bad-game")
				return err
			},
		},
		{
			name: "video",
			path: filepath.Join(dir, "videos", "bad-video.json"),
			load: func() error {
				_, err := mgr.LoadVideoCache("bad-video")
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Dir(tc.path), 0755); err != nil {
				t.Fatalf("MkdirAll failed: %v", err)
			}
			if err := os.WriteFile(tc.path, []byte("{bad json"), 0644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if err := tc.load(); err == nil {
				t.Fatal("expected invalid JSON error")
			}
		})
	}
}

func TestDiskCacheManager_SaveReturnsErrorWhenCacheSubdirIsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "videos"), []byte("not a dir"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	mgr := NewDiskCacheManager(dir)
	err := mgr.SaveVideoCache(&VideoCacheEntry{VideoID: "v1"})
	if err == nil {
		t.Fatal("expected error when videos cache directory is a file")
	}
}

func TestDiskCacheManager_DirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	mgr := NewDiskCacheManager(dir)

	// Save one of each
	mgr.SaveChannelCache(&ChannelCacheEntry{ChannelID: "ch1"})
	mgr.SaveSearchCache(&SearchCacheEntry{GameID: "g1"})
	mgr.SaveVideoCache(&VideoCacheEntry{VideoID: "v1"})

	// Verify directory structure
	if _, err := os.Stat(filepath.Join(dir, "channels", "ch1.json")); err != nil {
		t.Errorf("channel cache file not at expected path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "search", "g1.json")); err != nil {
		t.Errorf("search cache file not at expected path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "videos", "v1.json")); err != nil {
		t.Errorf("video cache file not at expected path: %v", err)
	}
}
