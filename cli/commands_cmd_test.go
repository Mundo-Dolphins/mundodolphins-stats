package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

func TestExecuteNoExit(t *testing.T) {
	prev := rootCmd
	rootCmd = &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	t.Cleanup(func() { rootCmd = prev })

	Execute()
}

type cmdFakeIndex struct{ m map[string]string }

func (f cmdFakeIndex) DateToGameID(d string) (string, bool) {
	v, ok := f.m[d]
	return v, ok
}

type cmdFakeIndexWithInfo struct {
	cmdFakeIndex
	info map[string]videos.GameRef
}

func (f cmdFakeIndexWithInfo) GameInfo(d string) (videos.GameRef, bool) {
	v, ok := f.info[d]
	return v, ok
}

func TestRunVideosImportLegacyDryRun(t *testing.T) {
	prevLegacyFile := videosLegacyFile
	prevCacheFile := videosLegacyCacheFile
	prevDryRun := videosLegacyDryRun
	prevOnlyMissing := videosLegacyOnlyMissing
	prevOverwrite := videosLegacyOverwrite
	prevGamesDir := videosLegacyGamesDir
	prevFactory := videosLegacyImporterFactory
	prevIndexBuilder := videosLegacyIndexBuilder
	defer func() {
		videosLegacyFile = prevLegacyFile
		videosLegacyCacheFile = prevCacheFile
		videosLegacyDryRun = prevDryRun
		videosLegacyOnlyMissing = prevOnlyMissing
		videosLegacyOverwrite = prevOverwrite
		videosLegacyGamesDir = prevGamesDir
		videosLegacyImporterFactory = prevFactory
		videosLegacyIndexBuilder = prevIndexBuilder
	}()

	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	legacyData := []byte("[{\"date\":\"1966-09-02\",\"videos\":[{\"youtube_id\":\"AAA\",\"duration\":1000000000}]}]")
	if err := os.WriteFile(legacyPath, legacyData, 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cachePath := filepath.Join(t.TempDir(), "cache.json")

	idx := cmdFakeIndexWithInfo{
		cmdFakeIndex: cmdFakeIndex{m: map[string]string{"1966-09-02": "196609020mia"}},
		info:         map[string]videos.GameRef{"1966-09-02": {AwayTeam: "Oakland Raiders", HomeTeam: "Miami Dolphins", Year: 1966}},
	}

	videosLegacyIndexBuilder = func(dir string) (videos.GameIndex, error) { return idx, nil }
	videosLegacyImporterFactory = func() *videos.Importer {
		return &videos.Importer{Now: func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }}
	}

	videosLegacyFile = legacyPath
	videosLegacyCacheFile = cachePath
	videosLegacyDryRun = true
	videosLegacyOnlyMissing = true
	videosLegacyOverwrite = ""
	videosLegacyGamesDir = t.TempDir()

	cmd := &cobra.Command{}
	buf := &strings.Builder{}
	cmd.SetOut(buf)

	if err := runVideosImportLegacy(cmd, nil); err != nil {
		t.Fatalf("runVideosImportLegacy: %v", err)
	}
	if _, err := os.Stat(cachePath); err == nil {
		t.Fatalf("expected no cache file on dry-run")
	}
	if !strings.Contains(buf.String(), "Added:") {
		t.Fatalf("expected summary output")
	}
}

func TestRunVideosImportLegacyWrites(t *testing.T) {
	prevLegacyFile := videosLegacyFile
	prevCacheFile := videosLegacyCacheFile
	prevDryRun := videosLegacyDryRun
	prevOnlyMissing := videosLegacyOnlyMissing
	prevOverwrite := videosLegacyOverwrite
	prevGamesDir := videosLegacyGamesDir
	prevFactory := videosLegacyImporterFactory
	prevIndexBuilder := videosLegacyIndexBuilder
	defer func() {
		videosLegacyFile = prevLegacyFile
		videosLegacyCacheFile = prevCacheFile
		videosLegacyDryRun = prevDryRun
		videosLegacyOnlyMissing = prevOnlyMissing
		videosLegacyOverwrite = prevOverwrite
		videosLegacyGamesDir = prevGamesDir
		videosLegacyImporterFactory = prevFactory
		videosLegacyIndexBuilder = prevIndexBuilder
	}()

	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	legacyData := []byte("[{\"date\":\"1966-09-09\",\"videos\":[]}]")
	if err := os.WriteFile(legacyPath, legacyData, 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cachePath := filepath.Join(t.TempDir(), "cache.json")

	idx := cmdFakeIndexWithInfo{
		cmdFakeIndex: cmdFakeIndex{m: map[string]string{"1966-09-09": "196609090mia"}},
		info:         map[string]videos.GameRef{"1966-09-09": {AwayTeam: "New York Jets", HomeTeam: "Miami Dolphins", Year: 1966}},
	}

	videosLegacyIndexBuilder = func(dir string) (videos.GameIndex, error) { return idx, nil }
	videosLegacyImporterFactory = func() *videos.Importer {
		return &videos.Importer{Now: func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }}
	}

	videosLegacyFile = legacyPath
	videosLegacyCacheFile = cachePath
	videosLegacyDryRun = false
	videosLegacyOnlyMissing = true
	videosLegacyOverwrite = ""
	videosLegacyGamesDir = t.TempDir()

	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})

	if err := runVideosImportLegacy(cmd, nil); err != nil {
		t.Fatalf("runVideosImportLegacy: %v", err)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cache file to be written")
	}
}
