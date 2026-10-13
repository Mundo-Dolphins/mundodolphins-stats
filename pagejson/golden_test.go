package pagejson

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestGolden_GenerateSeasonJSON generates the expected season JSON (run once to create/update)
func TestGolden_GenerateSeasonJSON(t *testing.T) {
	if os.Getenv("GENERATE_GOLDEN") != "1" {
		t.Skip("Skipping golden file generation (set GENERATE_GOLDEN=1 to run)")
	}

	fixedTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	builder := NewSeasonBuilder("testdata").SetTime(fixedTime)

	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	if err := os.WriteFile("testdata/expected_season_2025.json", data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Log("✓ Generated: testdata/expected_season_2025.json")
}

// TestGolden_GenerateSeasonStatsJSON generates the expected season stats JSON
func TestGolden_GenerateSeasonStatsJSON(t *testing.T) {
	if os.Getenv("GENERATE_GOLDEN") != "1" {
		t.Skip("Skipping golden file generation (set GENERATE_GOLDEN=1 to run)")
	}

	fixedTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	builder := NewSeasonStatsBuilder("testdata").SetTime(fixedTime)

	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	if err := os.WriteFile("testdata/expected_season_stats_2025.json", data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Log("✓ Generated: testdata/expected_season_stats_2025.json")
}

// TestGolden_GenerateGameJSON generates the expected game JSON
func TestGolden_GenerateGameJSON(t *testing.T) {
	if os.Getenv("GENERATE_GOLDEN") != "1" {
		t.Skip("Skipping golden file generation (set GENERATE_GOLDEN=1 to run)")
	}

	fixedTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	builder := NewGameBuilder("testdata", "testdata/videos.json").SetTime(fixedTime)

	page, err := builder.BuildFromFile("testdata/201309300nor.json", "201309300nor")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	if err := os.WriteFile("testdata/expected_game_201309300nor.json", data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Log("✓ Generated: testdata/expected_game_201309300nor.json")
}

// TestGolden_SeasonMatchesExpected compares generated season JSON with golden file
func TestGolden_SeasonMatchesExpected(t *testing.T) {
	fixedTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	builder := NewSeasonBuilder("testdata").SetTime(fixedTime)

	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	generatedData, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	expectedData, err := os.ReadFile("testdata/expected_season_2025.json")
	if err != nil {
		t.Skip("Golden file not found (run with GENERATE_GOLDEN=1 first)")
	}

	// Normalize JSON for comparison
	var generated, expected map[string]interface{}
	if err := json.Unmarshal(generatedData, &generated); err != nil {
		t.Fatalf("Unmarshal generated failed: %v", err)
	}
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("Unmarshal expected failed: %v", err)
	}

	// Compare as JSON objects (ignore formatting differences)
	generatedJSON, _ := json.Marshal(generated)
	expectedJSON, _ := json.Marshal(expected)

	if string(generatedJSON) != string(expectedJSON) {
		t.Errorf("Generated season JSON does not match expected golden file")
		t.Logf("Generated: %d bytes", len(generatedJSON))
		t.Logf("Expected: %d bytes", len(expectedJSON))
	}
}

// TestGolden_SeasonStatsMatchesExpected compares generated season stats JSON with golden file
func TestGolden_SeasonStatsMatchesExpected(t *testing.T) {
	fixedTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	builder := NewSeasonStatsBuilder("testdata").SetTime(fixedTime)

	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	generatedData, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	expectedData, err := os.ReadFile("testdata/expected_season_stats_2025.json")
	if err != nil {
		t.Skip("Golden file not found (run with GENERATE_GOLDEN=1 first)")
	}

	var generated, expected map[string]interface{}
	if err := json.Unmarshal(generatedData, &generated); err != nil {
		t.Fatalf("Unmarshal generated failed: %v", err)
	}
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("Unmarshal expected failed: %v", err)
	}

	generatedJSON, _ := json.Marshal(generated)
	expectedJSON, _ := json.Marshal(expected)

	if string(generatedJSON) != string(expectedJSON) {
		t.Errorf("Generated season stats JSON does not match expected golden file")
	}
}

// TestGolden_GameMatchesExpected compares generated game JSON with golden file
func TestGolden_GameMatchesExpected(t *testing.T) {
	fixedTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	builder := NewGameBuilder("testdata", "testdata/videos.json").SetTime(fixedTime)

	page, err := builder.BuildFromFile("testdata/201309300nor.json", "201309300nor")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}

	generatedData, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	expectedData, err := os.ReadFile("testdata/expected_game_201309300nor.json")
	if err != nil {
		t.Skip("Golden file not found (run with GENERATE_GOLDEN=1 first)")
	}

	var generated, expected map[string]interface{}
	if err := json.Unmarshal(generatedData, &generated); err != nil {
		t.Fatalf("Unmarshal generated failed: %v", err)
	}
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("Unmarshal expected failed: %v", err)
	}

	generatedJSON, _ := json.Marshal(generated)
	expectedJSON, _ := json.Marshal(expected)

	if string(generatedJSON) != string(expectedJSON) {
		t.Errorf("Generated game JSON does not match expected golden file")
	}
}
