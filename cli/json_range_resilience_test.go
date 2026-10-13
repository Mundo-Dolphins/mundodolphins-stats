package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSONSeasonRangeWritesValidYearsAndSkipsMissing(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withJSONSeasonGlobals(t, processedDir, outputDir)

	writeFile(t, filepath.Join(processedDir, "2024.json"), `{"season":2024,"data":{"main":{"tables":{}}}}`)

	if err := generateJSONSeasonRange(2024, 2025); err != nil {
		t.Fatalf("generateJSONSeasonRange failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "seasons", "2024.json")); err != nil {
		t.Fatalf("expected 2024 season JSON output: %v", err)
	}
}

func TestJSONSeasonStatsRangeWritesValidYearsAndSkipsMissing(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withJSONSeasonStatsGlobals(t, processedDir, outputDir)

	writeFile(t, filepath.Join(processedDir, "2024.json"), `{"season":2024,"data":{"main":{"tables":{"passing":{"rows":[{"player":"QB1","pass_yds":3000}]}}}}}`)

	if err := generateJSONSeasonStatsRange(2024, 2025); err != nil {
		t.Fatalf("generateJSONSeasonStatsRange failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "season_stats", "2024.json")); err != nil {
		t.Fatalf("expected 2024 season stats JSON output: %v", err)
	}
}

func TestJSONGamesRangeWritesValidGamesAndSkipsBadSiblings(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withJSONGamesGlobals(t, processedDir, outputDir)

	gamesDir := filepath.Join(processedDir, "games", "2024")
	writeFile(t, filepath.Join(gamesDir, "202409080mia.json"), `{"game_id":"202409080mia","data":{"scorebox":{"teams":[]},"tables":{}}}`)
	writeFile(t, filepath.Join(gamesDir, "bad.json"), `{bad json`)

	if err := generateJSONGamesRange(2024, 2025); err != nil {
		t.Fatalf("generateJSONGamesRange failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "games", "2024", "202409080mia.json")); err != nil {
		t.Fatalf("expected game JSON output: %v", err)
	}
}

func TestJSONGamesSingleRejectsInvalidGameID(t *testing.T) {
	withJSONGamesGlobals(t, t.TempDir(), t.TempDir())

	if err := generateJSONGamesSingle("bad"); err == nil {
		t.Fatal("expected invalid game ID error")
	}
}

func TestJSONGamesSingleUsesNFLSeasonDirectoryForJanuaryGame(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	withJSONGamesGlobals(t, processedDir, outputDir)

	gameID := "202601100mia"
	writeFile(t, filepath.Join(processedDir, "games", "2025", gameID+".json"), `{"game_id":"202601100mia","data":{"scorebox":{"teams":[]},"tables":{}}}`)

	if err := generateJSONGamesSingle(gameID); err != nil {
		t.Fatalf("generateJSONGamesSingle failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "data", "historical", "games", "2025", gameID+".json")); err != nil {
		t.Fatalf("expected January game in 2025 season output: %v", err)
	}
}

func withJSONSeasonGlobals(t *testing.T, processedDir, outputDir string) {
	t.Helper()
	prevProcessed := jsonSeasonProcessedDir
	prevOutput := jsonSeasonOutputDir
	t.Cleanup(func() {
		jsonSeasonProcessedDir = prevProcessed
		jsonSeasonOutputDir = prevOutput
	})
	jsonSeasonProcessedDir = processedDir
	jsonSeasonOutputDir = outputDir
}

func withJSONSeasonStatsGlobals(t *testing.T, processedDir, outputDir string) {
	t.Helper()
	prevProcessed := jsonSeasonStatsProcessedDir
	prevOutput := jsonSeasonStatsOutputDir
	t.Cleanup(func() {
		jsonSeasonStatsProcessedDir = prevProcessed
		jsonSeasonStatsOutputDir = prevOutput
	})
	jsonSeasonStatsProcessedDir = processedDir
	jsonSeasonStatsOutputDir = outputDir
}

func withJSONGamesGlobals(t *testing.T, processedDir, outputDir string) {
	t.Helper()
	prevProcessed := jsonGamesProcessedDir
	prevOutput := jsonGamesOutputDir
	prevVideosCache := jsonGamesVideosCache
	t.Cleanup(func() {
		jsonGamesProcessedDir = prevProcessed
		jsonGamesOutputDir = prevOutput
		jsonGamesVideosCache = prevVideosCache
	})
	jsonGamesProcessedDir = processedDir
	jsonGamesOutputDir = outputDir
	jsonGamesVideosCache = filepath.Join(t.TempDir(), "videos.json")
}
