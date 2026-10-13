package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestJSONSeasonSingleCommand(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	seasonJSON := `{
		"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
		"season": 2025,
		"team": {"id": "mia", "name": "Miami Dolphins"},
		"data": {"main": {"info": {"Record": "7-10-0"}}}
	}`
	if err := os.WriteFile(filepath.Join(tmpProcessed, "2025.json"), []byte(seasonJSON), 0644); err != nil {
		t.Fatalf("failed to write test season file: %v", err)
	}

	jsonSeasonYear = 2025
	jsonSeasonFromYear = 1966
	jsonSeasonToYear = 2026
	jsonSeasonOutputDir = tmpOutput
	jsonSeasonProcessedDir = tmpProcessed

	if err := runJSONSeason(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeason single failed: %v", err)
	}

	expected := filepath.Join(tmpOutput, "data", "historical", "seasons", "2025.json")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected output not found: %v", err)
	}
}

func TestJSONSeasonRangeCommand(t *testing.T) {
	tmpProcessed := t.TempDir()
	tmpOutput := t.TempDir()

	for year := 2024; year <= 2025; year++ {
		seasonJSON := `{
			"schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
			"season": ` + toString(year) + `,
			"team": {"id": "mia", "name": "Miami Dolphins"},
			"data": {"main": {"info": {"Record": "0-0"}}}
		}`
		if err := os.WriteFile(filepath.Join(tmpProcessed, toString(year)+".json"), []byte(seasonJSON), 0644); err != nil {
			t.Fatalf("failed to write test season file: %v", err)
		}
	}

	jsonSeasonYear = 0
	jsonSeasonFromYear = 2024
	jsonSeasonToYear = 2025
	jsonSeasonOutputDir = tmpOutput
	jsonSeasonProcessedDir = tmpProcessed

	if err := runJSONSeason(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runJSONSeason range failed: %v", err)
	}

	for year := 2024; year <= 2025; year++ {
		expected := filepath.Join(tmpOutput, "data", "historical", "seasons", toString(year)+".json")
		if _, err := os.Stat(expected); err != nil {
			t.Fatalf("expected output for %d not found: %v", year, err)
		}
	}
}

func TestGenerateJSONSeasonSingle_FileNotFound(t *testing.T) {
	jsonSeasonProcessedDir = t.TempDir()
	jsonSeasonOutputDir = t.TempDir()

	err := generateJSONSeasonSingle(2099)
	if err == nil {
		t.Fatal("expected error for missing season file")
	}
}
