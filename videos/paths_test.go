package videos

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPaths_DerivesPaths(t *testing.T) {
	p := NewPaths("cache/videos")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"BaseDir", p.BaseDir, "cache/videos"},
		{"VideosFile", p.VideosFile, filepath.Join("cache/videos", "videos.json")},
		{"StateFile", p.StateFile, filepath.Join("cache/videos", "state.json")},
		{"ChannelsFile", p.ChannelsFile, filepath.Join("cache/videos", "channels.json")},
		{"BlacklistFile", p.BlacklistFile, filepath.Join("cache/videos", "channels_blacklist.json")},
		{"CacheDir", p.CacheDir, filepath.Join("cache/videos", "cache")},
		{"ChannelCache", p.ChannelCache, filepath.Join("cache/videos", "cache", "channels")},
		{"ChannelDiscoveryCache", p.ChannelDiscoveryCache, filepath.Join("cache/videos", "cache", "channel_discovery")},
		{"SearchCache", p.SearchCache, filepath.Join("cache/videos", "cache", "search")},
		{"VideoCache", p.VideoCache, filepath.Join("cache/videos", "cache", "videos")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestNewPaths_CustomBase(t *testing.T) {
	p := NewPaths("/tmp/custom")
	if p.VideosFile != filepath.Join("/tmp/custom", "videos.json") {
		t.Errorf("unexpected VideosFile: %s", p.VideosFile)
	}
	if p.CacheDir != filepath.Join("/tmp/custom", "cache") {
		t.Errorf("unexpected CacheDir: %s", p.CacheDir)
	}
}

func TestEnsureDirs_CreatesAll(t *testing.T) {
	base := filepath.Join(t.TempDir(), "cache", "videos")
	p := NewPaths(base)

	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	dirs := []string{p.BaseDir, p.ChannelCache, p.SearchCache, p.VideoCache}
	for _, d := range dirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %s not created: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}

func TestEnsureDirs_Idempotent(t *testing.T) {
	base := filepath.Join(t.TempDir(), "cache", "videos")
	p := NewPaths(base)

	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("first EnsureDirs: %v", err)
	}
	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("second EnsureDirs: %v", err)
	}
}

func TestMigrateFromLegacy_MovesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	legacyDir := filepath.Join(tmpDir, "videos")
	newBase := filepath.Join(tmpDir, "cache", "videos")

	// Create legacy structure with files
	os.MkdirAll(filepath.Join(legacyDir, "cache", "channels"), 0o755)
	os.WriteFile(filepath.Join(legacyDir, "state.json"), []byte(`{"test":true}`), 0o644)
	os.WriteFile(filepath.Join(legacyDir, "videos.json"), []byte(`{"items":{}}`), 0o644)
	os.WriteFile(filepath.Join(legacyDir, "cache", "channels", "ch1.json"), []byte(`{}`), 0o644)

	p := NewPaths(newBase)
	if err := p.MigrateFromLegacy(legacyDir); err != nil {
		t.Fatalf("MigrateFromLegacy: %v", err)
	}

	// Verify files exist at new location
	for _, rel := range []string{"state.json", "videos.json", filepath.Join("cache", "channels", "ch1.json")} {
		path := filepath.Join(newBase, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after migration", path)
		}
	}

	// Legacy directory should be removed (was empty after move)
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Error("legacy directory should have been removed")
	}
}

func TestMigrateFromLegacy_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	legacyDir := filepath.Join(tmpDir, "videos")
	newBase := filepath.Join(tmpDir, "cache", "videos")

	// Create legacy file
	os.MkdirAll(legacyDir, 0o755)
	os.WriteFile(filepath.Join(legacyDir, "state.json"), []byte(`{"old":true}`), 0o644)

	// Create file at destination already
	os.MkdirAll(newBase, 0o755)
	os.WriteFile(filepath.Join(newBase, "state.json"), []byte(`{"new":true}`), 0o644)

	p := NewPaths(newBase)
	if err := p.MigrateFromLegacy(legacyDir); err != nil {
		t.Fatalf("MigrateFromLegacy: %v", err)
	}

	// Existing file should NOT be overwritten
	data, _ := os.ReadFile(filepath.Join(newBase, "state.json"))
	if string(data) != `{"new":true}` {
		t.Errorf("existing file was overwritten: got %q", string(data))
	}
}

func TestMigrateFromLegacy_NoLegacyDir(t *testing.T) {
	p := NewPaths(filepath.Join(t.TempDir(), "cache", "videos"))
	// Non-existent legacy dir: should be a no-op
	if err := p.MigrateFromLegacy(filepath.Join(t.TempDir(), "nonexistent")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrateFromLegacy_SameDir(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "videos")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{}`), 0o644)

	p := NewPaths(dir)
	// Migrating from same dir should be a no-op
	if err := p.MigrateFromLegacy(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should still be there
	if _, err := os.Stat(filepath.Join(dir, "state.json")); os.IsNotExist(err) {
		t.Error("file should still exist after same-dir migration")
	}
}

func TestIsDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	empty := filepath.Join(tmpDir, "empty")
	os.MkdirAll(empty, 0o755)
	if ok, _ := isDirEmpty(empty); !ok {
		t.Error("expected empty dir")
	}

	nonEmpty := filepath.Join(tmpDir, "notempty")
	os.MkdirAll(nonEmpty, 0o755)
	os.WriteFile(filepath.Join(nonEmpty, "file.txt"), []byte("hi"), 0o644)
	if ok, _ := isDirEmpty(nonEmpty); ok {
		t.Error("expected non-empty dir")
	}
}
