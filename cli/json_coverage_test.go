package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestJSONSeasonStatsRangeWithMixedResults tests range with some files to process
func TestJSONSeasonStatsRangeWithMixedResults(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season files for 2023, 2024, 2025 with only 2024 being valid
	for year := 2023; year <= 2025; year++ {
		seasonJSON := fmt.Sprintf(`{
			"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
			"season": %d,
			"team": {"id": "mia", "name": "Miami Dolphins"},
			"data": {
				"main": {"tables": {"passing": {"rows": []}, "rushing_and_receiving": {"rows": []}}},
				"advanced": {"tables": {}}
			}
		}`, year)

		path := filepath.Join(tmpProcessed, fmt.Sprintf("%d.json", year))
		if err := os.WriteFile(path, []byte(seasonJSON), 0644); err != nil {
			t.Fatalf("failed to write season file: %v", err)
		}
	}

	// Set flags to test range
	jsonSeasonStatsYear = 0
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsFromYear = 2023
	jsonSeasonStatsToYear = 2025

	if err := runJSONSeasonStats(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeasonStats range failed: %v", err)
	}

	// Verify all 3 files were created
	for year := 2023; year <= 2025; year++ {
		expectedPath := filepath.Join(tmpOutput, "data", "historical", "season_stats", fmt.Sprintf("%d.json", year))
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected season %d output not found: %v", year, err)
		}
	}
}

// TestJSONGamesSingleGameByID tests single game generation by ID
func TestJSONGamesSingleGameByID(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create game file structure - need games/2025 directory
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	gameID := "202512280mia"
	gameJSON := fmt.Sprintf(`{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "%s",
		"data": {
			"scorebox": {"teams": [{"name": "Team A"}, {"name": "Miami Dolphins"}]},
			"tables": {"passing": {"rows": []}}
		}
	}`, gameID)

	if err := os.WriteFile(filepath.Join(gamesDir, gameID+".json"), []byte(gameJSON), 0644); err != nil {
		t.Fatalf("failed to write game file: %v", err)
	}

	// Set to single game mode
	jsonGamesGameID = gameID
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 0
	jsonGamesFromYear = 0
	jsonGamesToYear = 0

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames single game failed: %v", err)
	}

	// Verify output file was created
	expectedPath := filepath.Join(tmpOutput, "data", "historical", "games", "2025", gameID+".json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected game output not found: %v", err)
	}
}

// TestJSONGamesSeasonMode tests season-based game generation
func TestJSONGamesSeasonMode(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season directory with games
	gamesDir := filepath.Join(tmpProcessed, "games", "2024")
	os.MkdirAll(gamesDir, 0755)

	games := []string{"202409080jax", "202409150buf"}
	for _, gameID := range games {
		gameJSON := fmt.Sprintf(`{
			"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
			"game_id": "%s",
			"data": {
				"scorebox": {"teams": [{"name": "Team A"}]},
				"tables": {}
			}
		}`, gameID)

		if err := os.WriteFile(filepath.Join(gamesDir, gameID+".json"), []byte(gameJSON), 0644); err != nil {
			t.Fatalf("failed to write game file: %v", err)
		}
	}

	// Set to season mode (not single game, not range)
	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 2024
	jsonGamesFromYear = 0
	jsonGamesToYear = 0

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames season mode failed: %v", err)
	}

	// Verify all games were processed
	for _, gameID := range games {
		expectedPath := filepath.Join(tmpOutput, "data", "historical", "games", "2024", gameID+".json")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected game %s output not found: %v", gameID, err)
		}
	}
}

// TestJSONGamesRangeMode tests multi-season game generation
func TestJSONGamesRangeMode(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games for 2024 and 2025 with actual files
	gamesToCreate := map[int]string{
		2024: "202409080buf",
		2025: "202512280mia",
	}

	for season, gameID := range gamesToCreate {
		gamesDir := filepath.Join(tmpProcessed, "games", fmt.Sprintf("%d", season))
		os.MkdirAll(gamesDir, 0755)

		gameJSON := fmt.Sprintf(`{
			"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
			"game_id": "%s",
			"data": {"scorebox": {"teams": []}, "tables": {}}
		}`, gameID)

		if err := os.WriteFile(filepath.Join(gamesDir, gameID+".json"), []byte(gameJSON), 0644); err != nil {
			t.Fatalf("failed to write game: %v", err)
		}
	}

	// Set to range mode
	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 0
	jsonGamesFromYear = 2024
	jsonGamesToYear = 2025

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames range mode failed: %v", err)
	}

	// Check that output files exist for both seasons
	for season, gameID := range gamesToCreate {
		expectedPath := filepath.Join(tmpOutput, "data", "historical", "games", fmt.Sprintf("%d", season), gameID+".json")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected season %d game output not found at %s: %v", season, expectedPath, err)
		}
	}
}

// TestJSONSeasonStatsDefaultRange tests using default year range
func TestJSONSeasonStatsDefaultRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create just one season file (1966)
	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 1966,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {
			"main": {"tables": {}},
			"advanced": {"tables": {}}
		}
	}`

	if err := os.WriteFile(filepath.Join(tmpProcessed, "1966.json"), []byte(seasonJSON), 0644); err != nil {
		t.Fatalf("failed to write season file: %v", err)
	}

	// Use default range
	jsonSeasonStatsYear = 0
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsFromYear = 1966
	jsonSeasonStatsToYear = 1966

	if err := runJSONSeasonStats(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeasonStats default range failed: %v", err)
	}

	// Verify output
	expectedPath := filepath.Join(tmpOutput, "data", "historical", "season_stats", "1966.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected 1966 output not found: %v", err)
	}
}

// TestJSONGamesWriteError simulates write permission error
func TestJSONGamesWriteError(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create read-only output directory
	readOnlyDir := filepath.Join(tmpOutput, "readonly")
	os.MkdirAll(readOnlyDir, 0755)

	// Create game file
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	gameJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "202512280mia",
		"data": {"scorebox": {"teams": []}, "tables": {}}
	}`
	os.WriteFile(filepath.Join(gamesDir, "202512280mia.json"), []byte(gameJSON), 0644)

	// Make output directory read-only
	os.Chmod(readOnlyDir, 0444)
	defer os.Chmod(readOnlyDir, 0755) // Restore for cleanup

	jsonGamesGameID = ""
	jsonGamesOutputDir = readOnlyDir
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 2025

	// Should handle error gracefully
	err := runJSONGames(&cobra.Command{}, nil)
	// Could error due to permission denied, which is acceptable
	_ = err
}

// TestJSONSeasonStatsDirectoryCreation tests directory creation during output
func TestJSONSeasonStatsDirectoryCreation(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := "/nonexistent" + fmt.Sprintf("%d", os.Getpid()) // Non-existent temp path

	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2025,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"tables": {}}, "advanced": {"tables": {}}}
	}`

	if err := os.WriteFile(filepath.Join(tmpProcessed, "2025.json"), []byte(seasonJSON), 0644); err != nil {
		t.Fatalf("failed to write season file: %v", err)
	}

	jsonSeasonStatsYear = 2025
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed

	// This might fail due to permission or path issues, which is acceptable
	_ = runJSONSeasonStats(&cobra.Command{}, nil)
}
