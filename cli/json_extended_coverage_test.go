package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateJSONGamesSingleGame tests single game JSON generation
func TestGenerateJSONGamesSingleGame(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games/2025 directory
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	// Create single game file
	gameData := map[string]any{
		"schema": map[string]string{
			"name":    "mundodolphins.pfr.boxscore",
			"version": "1.1.0",
		},
		"game_id": "202509010buf",
		"data": map[string]any{
			"tables": map[string]any{
				"scoring": []any{},
			},
		},
	}
	jsonData, _ := json.Marshal(gameData)
	os.WriteFile(filepath.Join(gamesDir, "202509010buf.json"), jsonData, 0644)

	// Test single game by ID
	cmd := jsonGamesCmd
	original := [3]string{jsonGamesGameID, jsonGamesProcessedDir, jsonGamesOutputDir}

	jsonGamesGameID = "202509010buf"
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesGameID = original[0]
	jsonGamesProcessedDir = original[1]
	jsonGamesOutputDir = original[2]
}

// TestGenerateJSONGamesRange tests JSON generation for game range
func TestGenerateJSONGamesRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games for multiple years
	for year := 2024; year <= 2025; year++ {
		gamesDir := filepath.Join(tmpProcessed, "games", fmt.Sprintf("%d", year))
		os.MkdirAll(gamesDir, 0755)

		for i := 1; i <= 2; i++ {
			gameID := fmt.Sprintf("%d%02d%02d0buf", year, 9, i)
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
	original := [3]string{jsonGamesGameID, jsonGamesProcessedDir, jsonGamesOutputDir}
	originalYears := [2]int64{jsonGamesFromYear, jsonGamesToYear}

	jsonGamesGameID = ""
	jsonGamesFromYear = 2024
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesGameID = original[0]
	jsonGamesProcessedDir = original[1]
	jsonGamesOutputDir = original[2]
	jsonGamesFromYear = originalYears[0]
	jsonGamesToYear = originalYears[1]
}

// TestGenerateJSONGamesSeasonMode tests JSON generation for entire season
func TestGenerateJSONGamesSeasonMode(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games for single year
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	for i := 1; i <= 3; i++ {
		gameID := fmt.Sprintf("202509%02d0buf", i)
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

	jsonGamesGameID = ""
	jsonGamesFromYear = 2025
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
}

// TestGenerateJSONGamesEmptyRange tests JSON generation with no games
func TestGenerateJSONGamesEmptyRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear

	jsonGamesGameID = ""
	jsonGamesFromYear = 2035
	jsonGamesToYear = 2036
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
}

// TestGenerateJSONGamesInvalidGame tests handling of invalid game files
func TestGenerateJSONGamesInvalidGame(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games directory with corrupted file
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	// Create invalid JSON file
	os.WriteFile(filepath.Join(gamesDir, "202509010buf.json"), []byte("invalid {"), 0644)

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear

	jsonGamesGameID = ""
	jsonGamesFromYear = 2025
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
}

// TestGenerateJSONGamesMissingFile tests handling of missing games
func TestGenerateJSONGamesMissingFile(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create only the directory, no games
	os.MkdirAll(filepath.Join(tmpProcessed, "games", "2025"), 0755)

	cmd := jsonGamesCmd
	defaultFrom := jsonGamesFromYear
	defaultTo := jsonGamesToYear

	jsonGamesGameID = ""
	jsonGamesFromYear = 2025
	jsonGamesToYear = 2025
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonGamesFromYear = defaultFrom
	jsonGamesToYear = defaultTo
}

// TestGenerateJSONSeasonStatsRangeMultiple tests season stats for multiple years
func TestGenerateJSONSeasonStatsRangeMultiple(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season files for multiple years
	for year := 2023; year <= 2025; year++ {
		seasonJSON := fmt.Sprintf(`{
			"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
			"season": %d,
			"team": {"id": "mia", "name": "Miami Dolphins"},
			"data": {
				"main": {"tables": {}},
				"tables": {}
			}
		}`, year)
		os.WriteFile(filepath.Join(tmpProcessed, fmt.Sprintf("%d.json", year)), []byte(seasonJSON), 0644)
	}

	cmd := jsonSeasonStatsCmd
	originalFrom := jsonSeasonStatsFromYear
	originalTo := jsonSeasonStatsToYear
	originalProc := jsonSeasonStatsProcessedDir
	originalOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 0
	jsonSeasonStatsFromYear = 2023
	jsonSeasonStatsToYear = 2025
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsFromYear = originalFrom
	jsonSeasonStatsToYear = originalTo
	jsonSeasonStatsProcessedDir = originalProc
	jsonSeasonStatsOutputDir = originalOut
}

// TestGenerateJSONSeasonStatsSingleYear tests season stats for single year
func TestGenerateJSONSeasonStatsSingleYear(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 1966,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"tables": {}}, "tables": {}}
	}`
	os.WriteFile(filepath.Join(tmpProcessed, "1966.json"), []byte(seasonJSON), 0644)

	cmd := jsonSeasonStatsCmd
	originalYear := jsonSeasonStatsYear
	originalProc := jsonSeasonStatsProcessedDir
	originalOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 1966
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsYear = originalYear
	jsonSeasonStatsProcessedDir = originalProc
	jsonSeasonStatsOutputDir = originalOut
}

// TestGenerateJSONSeasonStatsEmptyRange tests season stats with no files
func TestGenerateJSONSeasonStatsEmptyRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	cmd := jsonSeasonStatsCmd
	originalFrom := jsonSeasonStatsFromYear
	originalTo := jsonSeasonStatsToYear
	originalProc := jsonSeasonStatsProcessedDir
	originalOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 0
	jsonSeasonStatsFromYear = 2050
	jsonSeasonStatsToYear = 2055
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsFromYear = originalFrom
	jsonSeasonStatsToYear = originalTo
	jsonSeasonStatsProcessedDir = originalProc
	jsonSeasonStatsOutputDir = originalOut
}

// TestGenerateJSONSeasonStatsInvalidJSON tests season stats with corrupted file
func TestGenerateJSONSeasonStatsInvalidJSON(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create corrupted JSON file
	os.WriteFile(filepath.Join(tmpProcessed, "2024.json"), []byte("not json"), 0644)

	cmd := jsonSeasonStatsCmd
	originalYear := jsonSeasonStatsYear
	originalProc := jsonSeasonStatsProcessedDir
	originalOut := jsonSeasonStatsOutputDir

	jsonSeasonStatsYear = 2024
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsOutputDir = tmpOutput

	_ = cmd.RunE(cmd, nil)

	jsonSeasonStatsYear = originalYear
	jsonSeasonStatsProcessedDir = originalProc
	jsonSeasonStatsOutputDir = originalOut
}
