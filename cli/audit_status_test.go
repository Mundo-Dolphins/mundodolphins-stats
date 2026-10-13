package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/audit"
)

func TestRunAuditStatusWritesManifest(t *testing.T) {
	prevProcessed := auditStatusProcessedDir
	prevGames := auditStatusGamesDir
	prevArticles := auditStatusArticlesDir
	prevVideos := auditStatusVideosCache
	prevOutput := auditStatusOutputDir
	prevGeneratedAt := auditStatusIncludeGeneratedAt
	defer func() {
		auditStatusProcessedDir = prevProcessed
		auditStatusGamesDir = prevGames
		auditStatusArticlesDir = prevArticles
		auditStatusVideosCache = prevVideos
		auditStatusOutputDir = prevOutput
		auditStatusIncludeGeneratedAt = prevGeneratedAt
	}()

	root := t.TempDir()
	processedDir := filepath.Join(root, "processed")
	gamesDir := filepath.Join(processedDir, "games")
	articlesDir := filepath.Join(root, "articles")
	videosCache := filepath.Join(root, "cache", "videos", "videos.json")
	outputDir := filepath.Join(root, "output", "data")

	writeCmdTestFile(t, filepath.Join(processedDir, "2025.json"), `{"schema":{"name":"mundodolphins.pfr.team-season","version":"1.5.0"},"season":2025}`)
	writeCmdTestFile(t, filepath.Join(gamesDir, "2025", "202509070mia.json"), `{"schema":{"name":"mundodolphins.pfr.boxscore","version":"1.2.0"},"game_id":"202509070mia"}`)
	writeCmdTestFile(t, filepath.Join(articlesDir, "2025.md"), "# 2025\n")
	writeCmdTestFile(t, videosCache, `{"schema":{"name":"mundodolphins.youtube.cache","version":"1.2.0"},"items":{}}`)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}

	auditStatusProcessedDir = processedDir
	auditStatusGamesDir = gamesDir
	auditStatusArticlesDir = articlesDir
	auditStatusVideosCache = videosCache
	auditStatusOutputDir = outputDir
	auditStatusIncludeGeneratedAt = false

	var out bytes.Buffer
	auditStatusCmd.SetOut(&out)
	defer auditStatusCmd.SetOut(os.Stdout)

	if err := runAuditStatus(auditStatusCmd, nil); err != nil {
		t.Fatalf("runAuditStatus error: %v", err)
	}

	var manifest audit.StatusManifest
	if err := json.Unmarshal(out.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest is not valid JSON: %v\n%s", err, out.String())
	}
	if manifest.Schema.Name != audit.StatusSchemaName {
		t.Fatalf("schema name = %q", manifest.Schema.Name)
	}
	if manifest.GeneratedAt != "" {
		t.Fatalf("expected deterministic default without generated_at, got %q", manifest.GeneratedAt)
	}
	if manifest.Games.Total != 1 {
		t.Fatalf("games total = %d, want 1", manifest.Games.Total)
	}
}

func writeCmdTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
