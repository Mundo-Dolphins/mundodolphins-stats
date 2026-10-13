package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJSONSeasonStatsCommand(t *testing.T) {
	// Create temporary directories for testing
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create a minimal test season JSON file
	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2025,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {
			"main": {
				"tables": {
					"passing": {"title": "Passing", "rows": []},
					"rushing_and_receiving": {"title": "Rushing/Rec", "rows": []},
					"defense": {"title": "Defense", "rows": []}
				}
			},
			"advanced": {"tables": {}}
		}
	}`

	season2025Path := filepath.Join(tmpProcessed, "2025.json")
	if err := os.WriteFile(season2025Path, []byte(seasonJSON), 0644); err != nil {
		t.Fatalf("failed to write test season file: %v", err)
	}

	// Test single year generation
	jsonSeasonStatsYear = 2025
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsFromYear = 1966
	jsonSeasonStatsToYear = 2026

	if err := runJSONSeasonStats(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeasonStats single year failed: %v", err)
	}

	// Verify output file was created
	expectedPath := filepath.Join(tmpOutput, "data", "historical", "season_stats", "2025.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected output file not found at %s: %v", expectedPath, err)
	}

	// Verify JSON is valid
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("output file is empty")
	}
	if !strings.Contains(string(data), "mundodolphins.hugo.sectioned-page") {
		t.Fatal("output does not contain expected schema")
	}
}

func TestJSONSeasonStatsRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create two test season files
	for season := 2024; season <= 2025; season++ {
		seasonJSON := `{
			"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
			"season": ` + toString(season) + `,
			"team": {"id": "mia", "name": "Miami Dolphins"},
			"data": {
				"main": {"tables": {}},
				"advanced": {"tables": {}}
			}
		}`
		path := filepath.Join(tmpProcessed, toString(season)+".json")
		if err := os.WriteFile(path, []byte(seasonJSON), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	// Test range generation
	jsonSeasonStatsYear = 0 // 0 means use range
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsFromYear = 2024
	jsonSeasonStatsToYear = 2025

	if err := runJSONSeasonStats(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeasonStats range failed: %v", err)
	}

	// Verify both files were created
	for season := 2024; season <= 2025; season++ {
		expectedPath := filepath.Join(tmpOutput, "data", "historical", "season_stats", toString(season)+".json")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected output file not found for season %d: %v", season, err)
		}
	}
}

func TestJSONGamesCommandSingle(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create processed games directory structure
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("failed to create games directory: %v", err)
	}

	// Create a minimal test game JSON
	gameJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "202512280mia",
		"data": {
			"scorebox": {
				"meta": {"date": "December 28, 2025", "stadium": "Hard Rock Stadium"},
				"teams": [
					{"name": "Tampa Bay", "abbr": "TB", "score": 17},
					{"name": "Miami Dolphins", "abbr": "MIA", "score": 20}
				]
			},
			"tables": {}
		}
	}`

	gameFilePath := filepath.Join(gamesDir, "202512280mia.json")
	if err := os.WriteFile(gameFilePath, []byte(gameJSON), 0644); err != nil {
		t.Fatalf("failed to write test game file: %v", err)
	}

	// Test single game generation
	jsonGamesGameID = "202512280mia"
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 0
	jsonGamesFromYear = 2023
	jsonGamesToYear = 2025

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames single game failed: %v", err)
	}

	// Verify output file was created
	expectedPath := filepath.Join(tmpOutput, "data", "historical", "games", "2025", "202512280mia.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected output file not found at %s: %v", expectedPath, err)
	}

	// Verify JSON is valid
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("output file is empty")
	}
	if !strings.Contains(string(data), "mundodolphins.hugo.sectioned-page") {
		t.Fatal("output does not contain expected schema")
	}
}

func TestJSONRivalsCommand(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()
	gamesDir := filepath.Join(tmpProcessed, "games", "2024")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("failed to create games directory: %v", err)
	}

	gameJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.2.0"},
		"game_id": "202409120mia",
		"data": {
			"scorebox": {
				"meta": {"date": "September 12, 2024"},
				"teams": [
					{"name": {"content": "Buffalo Bills", "link": "https://www.pro-football-reference.com/teams/buf/2024.htm"}, "abbr": "buf", "score": 24},
					{"name": {"content": "Miami Dolphins", "link": "https://www.pro-football-reference.com/teams/mia/2024.htm"}, "abbr": "mia", "score": 17}
				]
			},
			"week": "2",
			"youtube": {"status": "unknown"},
			"tables": {}
		}
	}`
	if err := os.WriteFile(filepath.Join(gamesDir, "202409120mia.json"), []byte(gameJSON), 0644); err != nil {
		t.Fatalf("failed to write game file: %v", err)
	}

	prevFrom := jsonRivalsFromYear
	prevTo := jsonRivalsToYear
	prevProcessed := jsonRivalsProcessedDir
	prevOutput := jsonRivalsOutputDir
	defer func() {
		jsonRivalsFromYear = prevFrom
		jsonRivalsToYear = prevTo
		jsonRivalsProcessedDir = prevProcessed
		jsonRivalsOutputDir = prevOutput
	}()

	jsonRivalsFromYear = 2024
	jsonRivalsToYear = 2024
	jsonRivalsProcessedDir = tmpProcessed
	jsonRivalsOutputDir = tmpOutput

	if err := runJSONRivals(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONRivals failed: %v", err)
	}

	expectedPath := filepath.Join(tmpOutput, "data", "historical", "rivals", "index.json")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("expected rival index file: %v", err)
	}
	if !strings.Contains(string(data), "mundodolphins.hugo.rival-index") {
		t.Fatalf("output does not contain rival schema: %s", data)
	}
}

func TestJSONGamesCommandSeason(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season games directory
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("failed to create games directory: %v", err)
	}

	// Create two test games
	for _, gameID := range []string{"202512210mia", "202512280mia"} {
		gameJSON := `{
			"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
			"game_id": "` + gameID + `",
			"data": {
				"scorebox": {"meta": {}, "teams": []},
				"tables": {}
			}
		}`
		path := filepath.Join(gamesDir, gameID+".json")
		if err := os.WriteFile(path, []byte(gameJSON), 0644); err != nil {
			t.Fatalf("failed to write test game: %v", err)
		}
	}

	// Test season generation
	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 2025
	jsonGamesFromYear = 2023
	jsonGamesToYear = 2025

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames season failed: %v", err)
	}

	// Verify both files were created
	for _, gameID := range []string{"202512210mia", "202512280mia"} {
		expectedPath := filepath.Join(tmpOutput, "data", "historical", "games", "2025", gameID+".json")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected output file not found for %s: %v", gameID, err)
		}
	}
}

func TestJSONGamesCommandRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create games for two seasons
	// Season 2024 has games in 2024 (mostly) and some in 2025 (Jan-Feb playoff games)
	// Season 2025 has games in 2025 (summer) and 2026 (Jan-Feb playoff games)

	// Create 2024 season games (starting with Sept 2024)
	season2024Dir := filepath.Join(tmpProcessed, "games", "2024")
	if err := os.MkdirAll(season2024Dir, 0755); err != nil {
		t.Fatalf("failed to create 2024 directory: %v", err)
	}

	game202409 := "202409080jax"
	gameJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "` + game202409 + `",
		"data": {"scorebox": {"meta": {}, "teams": []}, "tables": {}}
	}`
	if err := os.WriteFile(filepath.Join(season2024Dir, game202409+".json"), []byte(gameJSON), 0644); err != nil {
		t.Fatalf("failed to write 2024 game: %v", err)
	}

	// Create 2025 season games (starting with Sept 2025)
	season2025Dir := filepath.Join(tmpProcessed, "games", "2025")
	if err := os.MkdirAll(season2025Dir, 0755); err != nil {
		t.Fatalf("failed to create 2025 directory: %v", err)
	}

	game202509 := "202509070clt"
	gameJSON = `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "` + game202509 + `",
		"data": {"scorebox": {"meta": {}, "teams": []}, "tables": {}}
	}`
	if err := os.WriteFile(filepath.Join(season2025Dir, game202509+".json"), []byte(gameJSON), 0644); err != nil {
		t.Fatalf("failed to write 2025 game: %v", err)
	}

	// Test range generation
	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 0
	jsonGamesFromYear = 2024
	jsonGamesToYear = 2025

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames range failed: %v", err)
	}

	// Verify files were created for both seasons
	expectedPath2024 := filepath.Join(tmpOutput, "data", "historical", "games", "2024", game202409+".json")
	if _, err := os.Stat(expectedPath2024); err != nil {
		t.Fatalf("expected 2024 game output not found: %v", err)
	}

	expectedPath2025 := filepath.Join(tmpOutput, "data", "historical", "games", "2025", game202509+".json")
	if _, err := os.Stat(expectedPath2025); err != nil {
		t.Fatalf("expected 2025 game output not found: %v", err)
	}
}

// TestJSONSeasonStatsMissingFile tests graceful handling of missing season files
func TestJSONSeasonStatsMissingFile(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Don't create any files - should handle gracefully
	jsonSeasonStatsYear = 0
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsFromYear = 2024
	jsonSeasonStatsToYear = 2025

	// Should not error, just log warnings
	if err := runJSONSeasonStats(&cobra.Command{}, nil); err != nil {
		t.Fatalf("unexpected error with missing files: %v", err)
	}

	// No output files should be created
	historyDir := filepath.Join(tmpOutput, "data", "historical", "season_stats")
	if _, err := os.Stat(historyDir); err == nil {
		files, _ := os.ReadDir(historyDir)
		if len(files) > 0 {
			t.Errorf("expected no output files, got %d", len(files))
		}
	}
}

// TestJSONGamesMissingGameFiles tests graceful handling of missing game files
func TestJSONGamesMissingGameFiles(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season directory but no games
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 2025
	jsonGamesFromYear = 0
	jsonGamesToYear = 0

	// Should not error
	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("unexpected error with missing games: %v", err)
	}
}

// TestJSONGamesInvalidJSON tests handling of corrupted game files
func TestJSONGamesInvalidJSON(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season directory with invalid JSON
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	invalidJSON := "not valid json {"
	if err := os.WriteFile(filepath.Join(gamesDir, "202512280mia.json"), []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 2025
	jsonGamesFromYear = 0
	jsonGamesToYear = 0

	// Should handle error gracefully
	err := runJSONGames(&cobra.Command{}, nil)
	// Error is acceptable since JSON is invalid
	if err == nil {
		t.Logf("Note: runJSONGames handled invalid JSON gracefully")
	}
}

// TestJSONSeasonStatsPartialRange tests range with mix of valid and missing files
func TestJSONSeasonStatsPartialRange(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create only 2024 file, not 2025
	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2024,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {
			"main": {"tables": {"passing": {"rows": []}, "rushing_and_receiving": {"rows": []}, "defense": {"rows": []}}},
			"advanced": {"tables": {}}
		}
	}`

	if err := os.WriteFile(filepath.Join(tmpProcessed, "2024.json"), []byte(seasonJSON), 0644); err != nil {
		t.Fatalf("failed to write 2024 season: %v", err)
	}

	jsonSeasonStatsYear = 0
	jsonSeasonStatsOutputDir = tmpOutput
	jsonSeasonStatsProcessedDir = tmpProcessed
	jsonSeasonStatsFromYear = 2023
	jsonSeasonStatsToYear = 2025

	if err := runJSONSeasonStats(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeasonStats partial range failed: %v", err)
	}

	// Verify 2024 file was created
	expectedPath := filepath.Join(tmpOutput, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected 2024 output not found: %v", err)
	}
}

// TestJSONGamesMultipleGamesInSeason tests processing multiple games
func TestJSONGamesMultipleGamesInSeason(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	// Create season directory with multiple games
	gamesDir := filepath.Join(tmpProcessed, "games", "2025")
	os.MkdirAll(gamesDir, 0755)

	gameIDs := []string{"202512070buf", "202512140nyj", "202512210mia"}
	gameJSON := `{
		"schema": {"name": "mundodolphins.pfr.boxscore", "version": "1.1.0"},
		"game_id": "%s",
		"data": {"scorebox": {"meta": {}, "teams": []}, "tables": {}}
	}`

	for _, gameID := range gameIDs {
		content := fmt.Sprintf(gameJSON, gameID)
		if err := os.WriteFile(filepath.Join(gamesDir, gameID+".json"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write game file: %v", err)
		}
	}

	jsonGamesGameID = ""
	jsonGamesOutputDir = tmpOutput
	jsonGamesProcessedDir = tmpProcessed
	jsonGamesYear = 2025
	jsonGamesFromYear = 0
	jsonGamesToYear = 0

	if err := runJSONGames(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONGames multiple games failed: %v", err)
	}

	// Verify all files were created
	outputDir := filepath.Join(tmpOutput, "data", "historical", "games", "2025")
	for _, gameID := range gameIDs {
		expectedPath := filepath.Join(outputDir, gameID+".json")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected game output %s not found: %v", gameID, err)
		}
	}
}

// Helper function to convert int to string
func toString(i int) string {
	return fmt.Sprintf("%d", i)
}
