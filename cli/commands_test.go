package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// minimal season JSON for testing
var minimalSeasonJSON = `{
	"schema": {
		"name": "mundodolphins.pfr.team-season",
		"version": "1.3.0"
	},
	"generated_at": "2026-02-19T16:00:00Z",
	"source": {"site": "pro-football-reference"},
	"team": {"id": "mia", "name": "Miami Dolphins"},
	"season": 2024,
	"data": {
		"main": {
			"status": "ok",
			"source": {"url": "test", "cache_file": "test"},
			"info": {"record": "10-7"},
			"tables": {}
		}
	}
}`

// minimal boxscore JSON for testing
var minimalBoxscoreJSON = `{
	"schema": {
		"name": "mundodolphins.pfr.boxscore",
		"version": "1.1.0"
	},
	"generated_at": "2026-02-19T16:00:00Z",
	"source": {"site": "pro-football-reference", "url": "test", "cache_file": "test"},
	"game_id": "202409010mia",
	"data": {
		"scorebox": {
			"meta": {
				"date": "September 1, 2024",
				"stadium": {"content": "Hard Rock Stadium", "link": "/stadiums/"}
			},
			"teams": [
				{
					"sb_id": "TAA",
					"name": {"content": "Team A", "link": "/teams/taa/"},
					"score": 24
				},
				{
					"sb_id": "MIA",
					"name": {"content": "Miami Dolphins", "link": "/teams/mia/"},
					"score": 31
				}
			]
		},
		"week": "1",
		"youtube": {
			"video_id": "",
			"title": "",
			"url": "",
			"duration_iso": "",
			"duration_seconds": 0
		},
		"tables": {}
	}
}`

func TestRunGenerateHugoDataSingleYear(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 2024
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "all"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Create processed/2024.json
	processedPath := filepath.Join(hugoProcessedDir, "2024.json")
	if err := os.WriteFile(processedPath, []byte(minimalSeasonJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify season data output exists
	seasonPath := filepath.Join(hugoOutputDir, "data", "historical", "seasons", "2024.json")
	if _, err := os.Stat(seasonPath); err != nil {
		t.Fatalf("expected season output file: %v", err)
	}

	// Verify season stats data output exists
	statsPath := filepath.Join(hugoOutputDir, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(statsPath); err != nil {
		t.Fatalf("expected season stats output file: %v", err)
	}
}

func TestRunGenerateHugoDataYearRange(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 0 // Use range
	hugoFromYear = 2023
	hugoToYear = 2024
	hugoDataType = "seasons"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Create processed/2023.json and processed/2024.json with correct season numbers
	for year := 2023; year <= 2024; year++ {
		seasonJSON := fmt.Sprintf(`{
	"schema": {
		"name": "mundodolphins.pfr.team-season",
		"version": "1.3.0"
	},
	"generated_at": "2026-02-19T16:00:00Z",
	"source": {"site": "pro-football-reference"},
	"team": {"id": "mia", "name": "Miami Dolphins"},
	"season": %d,
	"data": {
		"main": {
			"status": "ok",
			"source": {"url": "test", "cache_file": "test"},
			"info": {"record": "10-7"},
			"tables": {}
		}
	}
}`, year)
		processedPath := filepath.Join(hugoProcessedDir, fmt.Sprintf("%d.json", year))
		if err := os.WriteFile(processedPath, []byte(seasonJSON), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify both years have season output
	for year := 2023; year <= 2024; year++ {
		seasonPath := filepath.Join(hugoOutputDir, "data", "historical", "seasons", fmt.Sprintf("%d.json", year))
		if _, err := os.Stat(seasonPath); err != nil {
			t.Fatalf("expected season output file for %d: %v", year, err)
		}
	}
}

func TestRunGenerateHugoDataTypeFilter(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 2024
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "season_stats"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Create processed/2024.json
	processedPath := filepath.Join(hugoProcessedDir, "2024.json")
	if err := os.WriteFile(processedPath, []byte(minimalSeasonJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify only season_stats output exists
	statsPath := filepath.Join(hugoOutputDir, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(statsPath); err != nil {
		t.Fatalf("expected season stats output file: %v", err)
	}

	// Verify season output does NOT exist
	seasonPath := filepath.Join(hugoOutputDir, "data", "historical", "seasons", "2024.json")
	if _, err := os.Stat(seasonPath); err == nil {
		t.Fatalf("unexpected season output file (should only generate season_stats)")
	}
}

func TestRunGenerateHugoDataSingleGame(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 0
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "all"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = "202409010mia" // Sept 1, 2024 game

	// Create processed/games/2024/ directory with boxscore
	gamesDir := filepath.Join(hugoProcessedDir, "games", "2024")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	boxscorePath := filepath.Join(gamesDir, "202409010mia.json")
	if err := os.WriteFile(boxscorePath, []byte(minimalBoxscoreJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify game output exists
	gamePath := filepath.Join(hugoOutputDir, "data", "historical", "games", "2024", "202409010mia.json")
	if _, err := os.Stat(gamePath); err != nil {
		t.Fatalf("expected game output file: %v", err)
	}
}

func TestRunGenerateHugoDataGamesType(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 2024
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "games"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Create processed/games/2024/ directory with a boxscore
	gamesDir := filepath.Join(hugoProcessedDir, "games", "2024")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	boxscorePath := filepath.Join(gamesDir, "202409010mia.json")
	if err := os.WriteFile(boxscorePath, []byte(minimalBoxscoreJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify game output exists
	gamePath := filepath.Join(hugoOutputDir, "data", "historical", "games", "2024", "202409010mia.json")
	if _, err := os.Stat(gamePath); err != nil {
		t.Fatalf("expected game output file: %v", err)
	}
}

func TestRunGenerateHugoDataMissingProcessedFile(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 2025 // Year that doesn't exist
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "all"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Don't create any files - should continue gracefully with warnings
	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("expected no error for missing processed file (resilient mode): %v", err)
	}
}

func TestRunGenerateHugoDataInvalidGameID(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 0
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "all"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = "invalid_game_id"

	// Should return error for invalid game ID
	if err := runGenerateHugoData(generateHugoDataCmd, nil); err == nil {
		t.Fatalf("expected error for invalid game ID")
	}
}

func TestRunGenerateHugoDataSeasonStatsOnly(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 2024
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "season_stats"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Create processed/2024.json
	processedPath := filepath.Join(hugoProcessedDir, "2024.json")
	if err := os.WriteFile(processedPath, []byte(minimalSeasonJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify season stats output exists
	statsPath := filepath.Join(hugoOutputDir, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(statsPath); err != nil {
		t.Fatalf("expected season stats output file: %v", err)
	}

	// Verify season output does NOT exist
	seasonPath := filepath.Join(hugoOutputDir, "data", "historical", "seasons", "2024.json")
	if _, err := os.Stat(seasonPath); err == nil {
		t.Fatalf("unexpected season output file (should only generate season_stats)")
	}
}

func TestRunGenerateHugoDataAllTypes(t *testing.T) {
	prevYear := hugoYear
	prevFromYear := hugoFromYear
	prevToYear := hugoToYear
	prevDataType := hugoDataType
	prevProcessedDir := hugoProcessedDir
	prevOutputDir := hugoOutputDir
	prevGameID := hugoGameID
	defer func() {
		hugoYear = prevYear
		hugoFromYear = prevFromYear
		hugoToYear = prevToYear
		hugoDataType = prevDataType
		hugoProcessedDir = prevProcessedDir
		hugoOutputDir = prevOutputDir
		hugoGameID = prevGameID
	}()

	hugoYear = 2024
	hugoFromYear = 1966
	hugoToYear = 2024
	hugoDataType = "all"
	hugoProcessedDir = t.TempDir()
	hugoOutputDir = t.TempDir()
	hugoGameID = ""

	// Create processed/2024.json
	processedPath := filepath.Join(hugoProcessedDir, "2024.json")
	if err := os.WriteFile(processedPath, []byte(minimalSeasonJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Create processed/games/2024/ with a game
	gamesDir := filepath.Join(hugoProcessedDir, "games", "2024")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	boxscorePath := filepath.Join(gamesDir, "202409010mia.json")
	if err := os.WriteFile(boxscorePath, []byte(minimalBoxscoreJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := runGenerateHugoData(generateHugoDataCmd, nil); err != nil {
		t.Fatalf("runGenerateHugoData() error = %v", err)
	}

	// Verify all output types exist
	seasonPath := filepath.Join(hugoOutputDir, "data", "historical", "seasons", "2024.json")
	if _, err := os.Stat(seasonPath); err != nil {
		t.Fatalf("expected season output file: %v", err)
	}

	statsPath := filepath.Join(hugoOutputDir, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(statsPath); err != nil {
		t.Fatalf("expected season stats output file: %v", err)
	}

	gamePath := filepath.Join(hugoOutputDir, "data", "historical", "games", "2024", "202409010mia.json")
	if _, err := os.Stat(gamePath); err != nil {
		t.Fatalf("expected game output file: %v", err)
	}
}
