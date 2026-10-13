package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateHugoDataWritesHistoricalJSONAndNoDataMarkdown(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withHugoDataGlobals(t, processedDir, outputDir)

	writeCommandTestSeason(t, processedDir, 2025)
	writeCommandTestGame(t, processedDir, "2025", "202509070mia")

	hugoYear = 2025
	hugoDataType = "all"
	hugoIncludeArticles = false

	if err := runGenerateHugoData(nil, nil); err != nil {
		t.Fatalf("runGenerateHugoData failed: %v", err)
	}

	for _, path := range []string{
		filepath.Join(outputDir, "data", "historical", "seasons", "2025.json"),
		filepath.Join(outputDir, "data", "historical", "season_stats", "2025.json"),
		filepath.Join(outputDir, "data", "historical", "games", "2025", "202509070mia.json"),
		filepath.Join(outputDir, "data", "historical", "rivals", "index.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected JSON output %s: %v", path, err)
		}
	}

	assertNoMarkdownFiles(t, filepath.Join(outputDir, "data", "historical"))
}

func TestGenerateHugoDataRefreshesGameVideosFromCache(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withHugoDataGlobals(t, processedDir, outputDir)

	writeCommandTestGame(t, processedDir, "2025", "202509070mia")
	videosCache := filepath.Join(t.TempDir(), "videos.json")
	if err := os.WriteFile(videosCache, []byte(`{
		"schema": {"name": "mundodolphins.youtube.cache", "version": "1.4.0"},
		"generated_at": "2026-08-01T00:00:00Z",
		"items": {
			"202509070mia": {
				"status": "selected",
				"query": "Patriots at Dolphins 2025",
				"updated_at": "2026-08-01T00:00:00Z",
				"videos": [{
					"id": "yt-selected",
					"url": "https://www.youtube.com/watch?v=yt-selected",
					"title": "Patriots at Dolphins full game",
					"duration_seconds": 7200,
					"selected_at": "2026-08-01T00:00:00Z"
				}]
			}
		}
	}`), 0644); err != nil {
		t.Fatalf("write videos cache: %v", err)
	}

	hugoYear = 2025
	hugoDataType = "games"
	hugoVideosCache = videosCache

	if err := runGenerateHugoData(nil, nil); err != nil {
		t.Fatalf("runGenerateHugoData failed: %v", err)
	}

	outputPath := filepath.Join(outputDir, "data", "historical", "games", "2025", "202509070mia.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated game JSON: %v", err)
	}
	if !strings.Contains(string(data), `"youtube_id": "yt-selected"`) {
		t.Fatalf("generated game JSON did not include cached video:\n%s", data)
	}
}

func TestGenerateHugoDataPreservesEditorialArticleMarkdown(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	articlesDir := t.TempDir()
	articlesOut := filepath.Join(outputDir, "historia", "articulos")
	withHugoDataGlobals(t, processedDir, outputDir)

	writeCommandTestSeason(t, processedDir, 2025)
	if err := os.WriteFile(filepath.Join(articlesDir, "2025.md"), []byte("# Articulo 2025\n\nTexto editorial.\n"), 0644); err != nil {
		t.Fatalf("write article source: %v", err)
	}

	hugoYear = 2025
	hugoDataType = "seasons"
	hugoIncludeArticles = true
	hugoArticlesDir = articlesDir
	hugoArticlesOut = articlesOut

	if err := runGenerateHugoData(nil, nil); err != nil {
		t.Fatalf("runGenerateHugoData failed: %v", err)
	}

	articlePath := filepath.Join(articlesOut, "2025.md")
	data, err := os.ReadFile(articlePath)
	if err != nil {
		t.Fatalf("expected transformed editorial article markdown: %v", err)
	}
	if !strings.Contains(string(data), "Texto editorial.") || !strings.Contains(string(data), "[Volver](/historia/2025)") {
		t.Fatalf("article markdown was not transformed as expected:\n%s", data)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "seasons", "2025.json")); err != nil {
		t.Fatalf("expected season JSON alongside editorial article: %v", err)
	}
	assertNoMarkdownFiles(t, filepath.Join(outputDir, "data", "historical"))
}

func assertNoMarkdownFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			t.Fatalf("data command wrote Markdown file under JSON data output: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func writeCommandTestSeason(t *testing.T, processedDir string, year int) {
	t.Helper()
	data := []byte(fmt.Sprintf(`{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"generated_at": "2026-08-01T00:00:00Z",
		"season": %d,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"tables": {}}, "advanced": {"tables": {}}}
	}`, year))
	path := filepath.Join(processedDir, fmt.Sprintf("%d.json", year))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write season JSON: %v", err)
	}
}

func writeCommandTestGame(t *testing.T, processedDir, season, gameID string) {
	t.Helper()
	dir := filepath.Join(processedDir, "games", season)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir games dir: %v", err)
	}
	data := []byte(`{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.2.0"},
		"generated_at": "2026-08-01T00:00:00Z",
		"game_id": "` + gameID + `",
		"data": {
			"scorebox": {
				"meta": {"date": "September 7, 2025"},
				"teams": [
					{"name": {"content": "New England Patriots"}, "abbr": "nwe", "score": 17},
					{"name": {"content": "Miami Dolphins"}, "abbr": "mia", "score": 24}
				]
			},
			"week": "1",
			"youtube": {"status": "unknown"},
			"tables": {}
		}
	}`)
	if err := os.WriteFile(filepath.Join(dir, gameID+".json"), data, 0644); err != nil {
		t.Fatalf("write game JSON: %v", err)
	}
}
