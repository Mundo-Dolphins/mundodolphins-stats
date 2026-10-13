package videos

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// Paths centralizes all filesystem paths for the videos pipeline.
// All paths are derived from a single base directory (typically "cache/videos").
type Paths struct {
	BaseDir               string // cache/videos
	VideosFile            string // cache/videos/videos.json
	StateFile             string // cache/videos/state.json
	ChannelsFile          string // cache/videos/channels.json
	BlacklistFile         string // cache/videos/channels_blacklist.json
	CacheDir              string // cache/videos/cache
	ChannelCache          string // cache/videos/cache/channels
	ChannelDiscoveryCache string // cache/videos/cache/channel_discovery
	SearchCache           string // cache/videos/cache/search
	VideoCache            string // cache/videos/cache/videos
}

// NewPaths builds all paths from a base directory.
func NewPaths(baseDir string) Paths {
	cacheDir := filepath.Join(baseDir, "cache")
	return Paths{
		BaseDir:               baseDir,
		VideosFile:            filepath.Join(baseDir, "videos.json"),
		StateFile:             filepath.Join(baseDir, "state.json"),
		ChannelsFile:          filepath.Join(baseDir, "channels.json"),
		BlacklistFile:         filepath.Join(baseDir, "channels_blacklist.json"),
		CacheDir:              cacheDir,
		ChannelCache:          filepath.Join(cacheDir, "channels"),
		ChannelDiscoveryCache: filepath.Join(cacheDir, "channel_discovery"),
		SearchCache:           filepath.Join(cacheDir, "search"),
		VideoCache:            filepath.Join(cacheDir, "videos"),
	}
}

// EnsureDirs creates all required directories.
func (p Paths) EnsureDirs() error {
	dirs := []string{
		p.BaseDir,
		p.ChannelCache,
		p.ChannelDiscoveryCache,
		p.SearchCache,
		p.VideoCache,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}
	return nil
}

// MigrateFromLegacy moves files from an old "videos/" root-level directory
// into the proper cache location (e.g. "cache/videos/"). This is a one-time
// migration that runs automatically when the old directory is detected.
func (p Paths) MigrateFromLegacy(legacyDir string) error {
	info, err := os.Stat(legacyDir)
	if err != nil || !info.IsDir() {
		return nil // nothing to migrate
	}

	// Don't migrate onto itself
	absLegacy, _ := filepath.Abs(legacyDir)
	absBase, _ := filepath.Abs(p.BaseDir)
	if absLegacy == absBase {
		return nil
	}

	log.Printf("migrating old %s/ folder to %s/", legacyDir, p.BaseDir)

	if err := p.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure dirs for migration: %w", err)
	}

	moved := 0
	err = filepath.WalkDir(legacyDir, func(srcPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(legacyDir, srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(p.BaseDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		// Only move if destination doesn't already exist
		if _, err := os.Stat(dstPath); err == nil {
			return nil // already exists at destination
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return fmt.Errorf("create parent %s: %w", filepath.Dir(dstPath), err)
		}
		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("move %s → %s: %w", srcPath, dstPath, err)
		}
		moved++
		return nil
	})
	if err != nil {
		return fmt.Errorf("migration walk: %w", err)
	}

	log.Printf("migration complete: %d files moved", moved)

	// Remove legacy dir tree (all files already moved)
	if moved > 0 {
		if err := os.RemoveAll(legacyDir); err != nil {
			log.Printf("WARN: could not remove legacy directory %s: %v", legacyDir, err)
		} else {
			log.Printf("removed legacy directory %s/", legacyDir)
		}
	} else if isEmpty, _ := isDirEmpty(legacyDir); isEmpty {
		os.RemoveAll(legacyDir)
		log.Printf("removed empty legacy directory %s/", legacyDir)
	}

	return nil
}

func isDirEmpty(name string) (bool, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
