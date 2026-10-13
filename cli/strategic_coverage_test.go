package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateJSONGamesRangePartialFailure tests range with some years succeeding and some failing
func TestGenerateJSONGamesRangePartialFailure(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games for 2024 (will succeed)
	gamesDir2024 := filepath.Join(tmpProcessed, "games", "2024")
	os.MkdirAll(gamesDir2024, 0755)

	gameData2024 := map[string]any{
		"schema": map[string]string{
			"name":    "mundodolphins.pfr.boxscore",
			"version": "1.1.0",
		},
		"game_id": "202409010buf",
		"data": map[string]any{
			"tables": map[string]any{},
		},
	}
	jsonData, _ := json.Marshal(gameData2024)
	os.WriteFile(filepath.Join(gamesDir2024, "202409010buf.json"), jsonData, 0644)

	// Create corrupted file in 2025 (will fail mid-range)
	gamesDir2025 := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir2025, 0755)
	os.WriteFile(filepath.Join(gamesDir2025, "202512280mia.json"), []byte("corrupt"), 0644)

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear
	defaultProc := jsonGamesProcessedDir
	defaultOut := jsonGamesOutputDir

	jsonGamesGameID = ""
	jsonGamesFromYear = 2024
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	// Should handle mixed success/failure
	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
	jsonGamesProcessedDir = defaultProc
	jsonGamesOutputDir = defaultOut
}

// TestGenerateJSONGamesRangeMultipleFilesPerSeason tests range with multiple games per season
func TestGenerateJSONGamesRangeMultipleFilesPerSeason(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create 3 games per season for 2 seasons
	for season := 2024; season <= 2025; season++ {
		gamesDir := filepath.Join(tmpProcessed, "games", fmt.Sprintf("%d", season))
		os.MkdirAll(gamesDir, 0755)

		for gameNum := 1; gameNum <= 3; gameNum++ {
			gameID := fmt.Sprintf("%d%02d%02d0buf", season, 9, gameNum)
			gameData := map[string]any{
				"schema": map[string]string{
					"name":    "mundodolphins.pfr.boxscore",
					"version": "1.1.0",
				},
				"game_id": gameID,
				"data": map[string]any{
					"tables": map[string]any{},
				},
			}
			jsonData, _ := json.Marshal(gameData)
			os.WriteFile(filepath.Join(gamesDir, gameID+".json"), jsonData, 0644)
		}
	}

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear
	defaultProc := jsonGamesProcessedDir
	defaultOut := jsonGamesOutputDir

	jsonGamesGameID = ""
	jsonGamesFromYear = 2024
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
	jsonGamesProcessedDir = defaultProc
	jsonGamesOutputDir = defaultOut
}

// TestGenerateJSONSeasonStatsRangePartialFailure tests range with some years succeeding and failing
func TestGenerateJSONSeasonStatsRangePartialFailure(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create valid file for 2024
	seasonJSON2024 := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2024,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"tables": {}}, "tables": {}}
	}`
	os.WriteFile(filepath.Join(tmpProcessed, "2024.json"), []byte(seasonJSON2024), 0644)

	// Create corrupted file for 2025
	os.WriteFile(filepath.Join(tmpProcessed, "2025.json"), []byte("invalid"), 0644)

	// Create valid file for 2026
	seasonJSON2026 := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2026,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"tables": {}}, "tables": {}}
	}`
	os.WriteFile(filepath.Join(tmpProcessed, "2026.json"), []byte(seasonJSON2026), 0644)

	cmd := jsonSeasonStatsCmd
	defaultFrom := jsonSeasonStatsFromYear
	defaultTo := jsonSeasonStatsToYear
	defaultProc := jsonSeasonStatsProcessedDir
	defaultOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 0
	jsonSeasonStatsFromYear = 2024
	jsonSeasonStatsToYear = 2026
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	// Should handle mixed success/failure gracefully
	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsFromYear = defaultFrom
	jsonSeasonStatsToYear = defaultTo
	jsonSeasonStatsProcessedDir = defaultProc
	jsonSeasonStatsOutputDir = defaultOut
}

// TestGenerateJSONSeasonStatsRangeWithMissingMiddleYear tests range where middle years are missing
func TestGenerateJSONSeasonStatsRangeWithMissingMiddleYear(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create files for 2023, 2025 (skip 2024)
	for year := 2023; year <= 2025; year += 2 {
		seasonJSON := fmt.Sprintf(`{
			"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
			"season": %d,
			"team": {"id": "mia", "name": "Miami Dolphins"},
			"data": {"main": {"tables": {}}, "tables": {}}
		}`, year)
		os.WriteFile(filepath.Join(tmpProcessed, fmt.Sprintf("%d.json", year)), []byte(seasonJSON), 0644)
	}

	cmd := jsonSeasonStatsCmd
	defaultFrom := jsonSeasonStatsFromYear
	defaultTo := jsonSeasonStatsToYear
	defaultProc := jsonSeasonStatsProcessedDir
	defaultOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 0
	jsonSeasonStatsFromYear = 2023
	jsonSeasonStatsToYear = 2025
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsFromYear = defaultFrom
	jsonSeasonStatsToYear = defaultTo
	jsonSeasonStatsProcessedDir = defaultProc
	jsonSeasonStatsOutputDir = defaultOut
}

// TestGenerateJSONGamesRangeAllYearsEmpty tests range where all years have no games
func TestGenerateJSONGamesRangeAllYearsEmpty(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create empty directories for each year
	for year := 2024; year <= 2026; year++ {
		os.MkdirAll(filepath.Join(tmpProcessed, "games", fmt.Sprintf("%d", year)), 0755)
	}

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear
	defaultProc := jsonGamesProcessedDir
	defaultOut := jsonGamesOutputDir

	jsonGamesGameID = ""
	jsonGamesFromYear = 2024
	jsonGamesToYear = 2026
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
	jsonGamesProcessedDir = defaultProc
	jsonGamesOutputDir = defaultOut
}

// TestGenerateJSONGamesWithNonJSONFiles tests range with non-JSON files mixed in
func TestGenerateJSONGamesWithNonJSONFiles(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	// Create valid JSON file
	gameData := map[string]any{
		"schema": map[string]string{
			"name":    "mundodolphins.pfr.boxscore",
			"version": "1.1.0",
		},
		"game_id": "202509010buf",
		"data": map[string]any{
			"tables": map[string]any{},
		},
	}
	jsonData, _ := json.Marshal(gameData)
	os.WriteFile(filepath.Join(gamesDir, "202509010buf.json"), jsonData, 0644)

	// Create non-JSON file (should be ignored)
	os.WriteFile(filepath.Join(gamesDir, "readme.txt"), []byte("not json"), 0644)

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear
	defaultProc := jsonGamesProcessedDir
	defaultOut := jsonGamesOutputDir

	jsonGamesGameID = ""
	jsonGamesFromYear = 2025
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
	jsonGamesProcessedDir = defaultProc
	jsonGamesOutputDir = defaultOut
}

// TestGenerateJSONGamesLargeRange tests a large year range (many seasons)
func TestGenerateJSONGamesLargeRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games for 1966-1970 (5 seasons)
	for season := 1966; season <= 1970; season++ {
		gamesDir := filepath.Join(tmpProcessed, "games", fmt.Sprintf("%d", season))
		os.MkdirAll(gamesDir, 0755)

		// Create 1 game per season
		gameID := fmt.Sprintf("%d0809xxx", season)
		gameData := map[string]any{
			"schema": map[string]string{
				"name":    "mundodolphins.pfr.boxscore",
				"version": "1.1.0",
			},
			"game_id": gameID,
			"data": map[string]any{
				"tables": map[string]any{},
			},
		}
		jsonData, _ := json.Marshal(gameData)
		os.WriteFile(filepath.Join(gamesDir, gameID+".json"), jsonData, 0644)
	}

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear
	defaultProc := jsonGamesProcessedDir
	defaultOut := jsonGamesOutputDir

	jsonGamesGameID = ""
	jsonGamesFromYear = 1966
	jsonGamesToYear = 1970
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
	jsonGamesProcessedDir = defaultProc
	jsonGamesOutputDir = defaultOut
}

// TestGenerateJSONSeasonStatsWithoutGamesDir tests season stats when games dir doesn't exist
func TestGenerateJSONSeasonStatsWithoutGamesDir(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season file but NO games directory
	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2024,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"tables": {}}, "tables": {}}
	}`
	os.WriteFile(filepath.Join(tmpProcessed, "2024.json"), []byte(seasonJSON), 0644)

	cmd := jsonSeasonStatsCmd
	defaultYear := jsonSeasonStatsYear
	defaultProc := jsonSeasonStatsProcessedDir
	defaultOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 2024
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	// Should handle missing games directory gracefully
	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsYear = defaultYear
	jsonSeasonStatsProcessedDir = defaultProc
	jsonSeasonStatsOutputDir = defaultOut
}

// TestGenerateJSONGamesRangeWritePermissionError tests handling of permission denied errors
func TestGenerateJSONGamesRangeWritePermissionError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	gameData := map[string]any{
		"schema": map[string]string{
			"name":    "mundodolphins.pfr.boxscore",
			"version": "1.1.0",
		},
		"game_id": "202509010buf",
		"data": map[string]any{
			"tables": map[string]any{},
		},
	}
	jsonData, _ := json.Marshal(gameData)
	os.WriteFile(filepath.Join(gamesDir, "202509010buf.json"), jsonData, 0644)

	// Make output directory read-only
	readOnlyOutput := filepath.Join(tmpOutput, "readonly")
	os.MkdirAll(readOnlyOutput, 0755)
	os.Chmod(readOnlyOutput, 0444)
	defer os.Chmod(readOnlyOutput, 0755)

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear
	defaultProc := jsonGamesProcessedDir
	defaultOut := jsonGamesOutputDir

	jsonGamesGameID = ""
	jsonGamesFromYear = 2025
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = readOnlyOutput

	// Should handle permission error
	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
	jsonGamesProcessedDir = defaultProc
	jsonGamesOutputDir = defaultOut
}

// TestGenerateJSONGamesSingleGameWithInvalidPath tests handling of invalid game paths
func TestGenerateJSONGamesSingleGameWithInvalidPath(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	cmd := jsonGamesCmd
	originalProc := jsonGamesProcessedDir
	originalOut := jsonGamesOutputDir
	originalGameID := jsonGamesGameID

	jsonGamesGameID = "invalidgameid"
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	// Should handle invalid game ID path
	_ = cmd.RunE(cmd, nil)

	jsonGamesGameID = originalGameID
	jsonGamesProcessedDir = originalProc
	jsonGamesOutputDir = originalOut
}
