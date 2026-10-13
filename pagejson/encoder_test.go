package pagejson

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEncodeToJSON_ValidPage(t *testing.T) {
	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	page := NewSectionedPageWithTime(
		PageKindSeason,
		"Test Season",
		"2025-01-01",
		"2025",
		"historia/2025",
		map[string]interface{}{"year": 2025},
		fixedTime,
	)
	row := NewTableRow()
	row.Set("col1", "value1")
	row.Set("col2", "value2")
	page.AddSection("Test Section", []TableRow{row})

	data, err := EncodeToJSON(page)
	if err != nil {
		t.Fatalf("EncodeToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty JSON data")
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Generated invalid JSON: %v", err)
	}

	// Verify schema is present
	schema, ok := result["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected schema in JSON")
	}

	if schema["name"] != "mundodolphins.hugo.sectioned-page" {
		t.Errorf("Expected schema name mundodolphins.hugo.sectioned-page, got %v", schema["name"])
	}
}

func TestEncodeToJSON_PrettyPrint(t *testing.T) {
	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	page := NewSectionedPageWithTime(
		PageKindSeason,
		"Test",
		"2025-01-01",
		"2025",
		"historia/2025",
		nil,
		fixedTime,
	)
	data, err := EncodeToJSON(page)
	if err != nil {
		t.Fatalf("EncodeToJSON failed: %v", err)
	}

	// Verify it's pretty-printed (has newlines and indentation)
	jsonStr := string(data)
	if !strings.Contains(jsonStr, "\n") {
		t.Error("Expected pretty-printed JSON with newlines")
	}

	if !strings.Contains(jsonStr, "  ") {
		t.Error("Expected indented JSON")
	}
}

func TestNewPagePaths(t *testing.T) {
	paths := NewPagePaths("/output/base")

	if paths.baseDir != "/output/base" {
		t.Errorf("Expected baseDir /output/base, got %s", paths.baseDir)
	}
}

func TestPagePaths_SeasonOutputPath(t *testing.T) {
	paths := NewPagePaths("output")

	path := paths.SeasonOutputPath(2025)
	expected := "output/data/historical/seasons/2025.json"

	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

func TestPagePaths_SeasonStatsOutputPath(t *testing.T) {
	paths := NewPagePaths("output")

	path := paths.SeasonStatsOutputPath(2023)
	expected := "output/data/historical/season_stats/2023.json"

	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

func TestPagePaths_GameOutputPath(t *testing.T) {
	paths := NewPagePaths("output")

	path := paths.GameOutputPath("202309100sdg")
	expected := "output/data/historical/games/2023/202309100sdg.json"

	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

func TestPagePaths_GameOutputPath_DifferentSeasons(t *testing.T) {
	paths := NewPagePaths("output")

	tests := []struct {
		gameID   string
		expected string
	}{
		{"202309100sdg", "output/data/historical/games/2023/202309100sdg.json"},
		{"202412250mia", "output/data/historical/games/2024/202412250mia.json"},
		// Jan 2, 1972 belongs to NFL season 1971.
		{"197201020mia", "output/data/historical/games/1971/197201020mia.json"},
	}

	for _, tt := range tests {
		t.Run(tt.gameID, func(t *testing.T) {
			path := paths.GameOutputPath(tt.gameID)
			if path != tt.expected {
				t.Errorf("GameOutputPath(%s) = %s, want %s", tt.gameID, path, tt.expected)
			}
		})
	}
}
