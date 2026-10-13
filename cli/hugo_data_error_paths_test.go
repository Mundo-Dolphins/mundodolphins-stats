package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func withHugoDataGlobals(t *testing.T, processedDir, outputDir string) {
	t.Helper()
	prevProcessed := hugoProcessedDir
	prevOutput := hugoOutputDir
	prevType := hugoDataType
	prevGameID := hugoGameID
	prevYear := hugoYear
	prevFrom := hugoFromYear
	prevTo := hugoToYear
	prevIncludeArticles := hugoIncludeArticles
	prevArticlesDir := hugoArticlesDir
	prevArticlesOut := hugoArticlesOut
	prevGamesDir := hugoGamesDir
	prevEnsureAssets := hugoEnsureAssets
	prevAssetsDir := hugoAssetsDir
	prevVideosCache := hugoVideosCache

	hugoProcessedDir = processedDir
	hugoOutputDir = outputDir
	hugoIncludeArticles = false
	hugoGameID = ""
	hugoYear = 0
	hugoFromYear = 1966
	hugoToYear = 2026
	hugoDataType = "all"
	hugoArticlesDir = filepath.Join(t.TempDir(), "articles")
	hugoArticlesOut = filepath.Join(t.TempDir(), "articles-out")
	hugoGamesDir = filepath.Join(processedDir, "games")
	hugoEnsureAssets = false
	hugoAssetsDir = ""
	hugoVideosCache = filepath.Join(t.TempDir(), "videos.json")

	t.Cleanup(func() {
		hugoProcessedDir = prevProcessed
		hugoOutputDir = prevOutput
		hugoDataType = prevType
		hugoGameID = prevGameID
		hugoYear = prevYear
		hugoFromYear = prevFrom
		hugoToYear = prevTo
		hugoIncludeArticles = prevIncludeArticles
		hugoArticlesDir = prevArticlesDir
		hugoArticlesOut = prevArticlesOut
		hugoGamesDir = prevGamesDir
		hugoEnsureAssets = prevEnsureAssets
		hugoAssetsDir = prevAssetsDir
		hugoVideosCache = prevVideosCache
	})
}

func TestGenerateHugoDataForGameRejectsInvalidGameID(t *testing.T) {
	withHugoDataGlobals(t, t.TempDir(), t.TempDir())

	err := generateHugoDataForGame(context.Background(), "not-a-game")
	if err == nil {
		t.Fatal("expected invalid game id error")
	}
	if !strings.Contains(err.Error(), "determine season") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateHugoDataForGameLoadsJanuaryGameFromNFLSeasonDir(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withHugoDataGlobals(t, processedDir, outputDir)

	gameID := "202601100mia"
	gamesDir := filepath.Join(processedDir, "games", "2025")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	gameJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "202601100mia",
		"data": {"scorebox": {"teams": []}, "youtube": {"status": "unknown"}, "tables": {}}
	}`
	if err := os.WriteFile(filepath.Join(gamesDir, gameID+".json"), []byte(gameJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := generateHugoDataForGame(context.Background(), gameID); err != nil {
		t.Fatalf("generateHugoDataForGame failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "games", "2025", gameID+".json")); err != nil {
		t.Fatalf("expected generated January game in 2025 season directory: %v", err)
	}
}

func TestGenerateHugoDataForSeasonGamesSkipsBadFilesAndWritesGoodOnes(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withHugoDataGlobals(t, processedDir, outputDir)

	gamesDir := filepath.Join(processedDir, "games", "2024")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gamesDir, "bad.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatalf("WriteFile bad failed: %v", err)
	}
	goodJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "202409080mia",
		"data": {"scorebox": {"teams": []}, "youtube": {"status": "unknown"}, "tables": {}}
	}`
	if err := os.WriteFile(filepath.Join(gamesDir, "202409080mia.json"), []byte(goodJSON), 0644); err != nil {
		t.Fatalf("WriteFile good failed: %v", err)
	}

	generateHugoDataForSeasonGames(context.Background(), 2024)

	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "games", "2024", "202409080mia.json")); err != nil {
		t.Fatalf("expected valid game output despite bad sibling file: %v", err)
	}
}

func TestRunGenerateHugoDataRivalsReturnsRangeErrorForExplicitRivalsType(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withHugoDataGlobals(t, processedDir, outputDir)

	hugoDataType = "rivals"
	hugoFromYear = 2025
	hugoToYear = 2024

	if err := runGenerateHugoData(&cobra.Command{}, nil); err == nil {
		t.Fatal("expected invalid rival range error")
	}
}
