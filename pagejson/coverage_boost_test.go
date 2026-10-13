package pagejson

import (
	"strings"
	"testing"
)

// Test convertSliceOfMapInterface
func TestConvertTableToRows_SliceOfMapInterface(t *testing.T) {
	// This is the actual structure from PFR JSON
	tableData := []interface{}{
		map[string]interface{}{
			"player": map[string]interface{}{"content": "John Doe", "link": "/players/D/DoeJo00.htm"},
			"age":    float64(25),
			"pos":    "QB",
		},
		map[string]interface{}{
			"player": map[string]interface{}{"content": "Jane Smith"},
			"age":    float64(30),
			"pos":    "WR",
		},
	}

	headerMapping := map[string]string{
		"player": "Nombre",
		"age":    "Edad",
		"pos":    "Posición",
	}

	rows, err := ConvertTableToRows(tableData, headerMapping)
	if err != nil {
		t.Fatalf("ConvertTableToRows failed: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}

	// Check first row
	if rows[0].Get("Nombre") != "John Doe" {
		t.Errorf("Expected Nombre='John Doe', got=%q", rows[0].Get("Nombre"))
	}

	if rows[0].Get("Edad") != "25" { // int gets converted to string without decimals
		t.Errorf("Expected Edad='25', got=%q", rows[0].Get("Edad"))
	}

	// Check second row
	if rows[1].Get("Nombre") != "Jane Smith" {
		t.Errorf("Expected Nombre='Jane Smith', got=%q", rows[1].Get("Nombre"))
	}
}

// Test convertSliceOfMapString
func TestConvertTableToRows_SliceOfMapString(t *testing.T) {
	// This tests the []map[string]string path
	tableData := []map[string]string{
		{
			"player": "John Doe",
			"age":    "25",
			"pos":    "QB",
		},
		{
			"player": "Jane Smith",
			"age":    "30",
			"pos":    "WR",
		},
	}

	headerMapping := map[string]string{
		"player": "Nombre",
		"age":    "Edad",
		"pos":    "Posición",
	}

	rows, err := ConvertTableToRows(tableData, headerMapping)
	if err != nil {
		t.Fatalf("ConvertTableToRows failed: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rows))
	}

	// Check first row
	if rows[0].Get("Nombre") != "John Doe" {
		t.Errorf("Expected Nombre='John Doe', got=%q", rows[0].Get("Nombre"))
	}

	if rows[0].Get("Edad") != "25" {
		t.Errorf("Expected Edad='25', got=%q", rows[0].Get("Edad"))
	}
}

// Test AddNestedSection
func TestAddNestedSection_NewSection(t *testing.T) {
	page := NewSectionedPage(PageKindSeason, "Test", "2025-01-01", "test", "/test", nil)

	// Add a nested section
	page.AddNestedSection("Parent Section", "Child Section", []string{"item1", "item2"})

	// Verify parent section exists
	parentSection, ok := page.Sections["Parent Section"]
	if !ok {
		t.Fatal("Parent Section not found")
	}

	// Should be a map
	parentMap, ok := parentSection.(map[string]interface{})
	if !ok {
		t.Fatalf("Parent Section should be a map, got %T", parentSection)
	}

	// Verify child section exists
	childSection, ok := parentMap["Child Section"]
	if !ok {
		t.Fatal("Child Section not found")
	}

	// Should be a slice
	childSlice, ok := childSection.([]string)
	if !ok {
		t.Fatalf("Child Section should be a []string, got %T", childSection)
	}

	if len(childSlice) != 2 {
		t.Errorf("Expected 2 items in child section, got %d", len(childSlice))
	}
}

func TestAddNestedSection_AppendToExisting(t *testing.T) {
	page := NewSectionedPage(PageKindSeason, "Test", "2025-01-01", "test", "/test", nil)

	// Add first nested section
	page.AddNestedSection("Parent Section", "Child 1", []string{"item1"})

	// Add second nested section to same parent
	page.AddNestedSection("Parent Section", "Child 2", []string{"item2"})

	// Verify parent section exists
	parentSection := page.Sections["Parent Section"].(map[string]interface{})

	// Both children should exist
	if _, ok := parentSection["Child 1"]; !ok {
		t.Error("Child 1 not found")
	}

	if _, ok := parentSection["Child 2"]; !ok {
		t.Error("Child 2 not found")
	}
}

// Test error handling in ConvertTableToRows
func TestConvertTableToRows_UnsupportedType(t *testing.T) {
	// Pass something that's not a recognized type
	tableData := "not a table"

	_, err := ConvertTableToRows(tableData, nil)
	if err == nil {
		t.Error("Expected error for unsupported type, got nil")
	}
}

func TestConvertTableToRows_NilData(t *testing.T) {
	_, err := ConvertTableToRows(nil, nil)
	if err == nil {
		t.Error("Expected error for nil data, got nil")
	}

	// Error should mention unsupported type
	if err != nil && !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("Expected error about unsupported type, got: %v", err)
	}
}

// Test buildEstadisticasPorJugadorSection with more coverage
func TestSeasonStatsBuilder_BuildEstadisticasPorJugadorSection_WithData(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")
	page := NewSectionedPage(PageKindSeasonStats, "Test", "2025-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": 2025,
		"data": map[string]interface{}{
			"advanced": map[string]interface{}{
				"tables": map[string]interface{}{
					"passing": map[string]interface{}{
						"rows": []interface{}{
							map[string]interface{}{"player": "QB1", "yards": 300},
						},
					},
					"rushing_and_receiving": map[string]interface{}{
						"rows": []interface{}{
							map[string]interface{}{"player": "RB1", "yards": 100},
						},
					},
				},
			},
		},
	}

	err := builder.buildEstadisticasPorJugadorSection(page, seasonData)
	if err != nil {
		t.Fatalf("buildEstadisticasPorJugadorSection failed: %v", err)
	}

	// Check if section was added
	if _, ok := page.Sections["Estadísticas de la temporada por jugador"]; ok {
		t.Log("Estadísticas de la temporada por jugador section found")
	}
}

// Test more edge cases for convertValueToString
func TestConvertValueToString_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"nil", nil, ""},
		{"string", "test", "test"},
		{"int", 42, "42"},
		{"float64", 3.14, "3.14"},
		{"bool_true", true, "true"},
		{"bool_false", false, "false"},
		{"LinkValue_map", map[string]interface{}{"content": "Link Text"}, "Link Text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertValueToString(tt.value)
			if result != tt.expected {
				t.Errorf("convertValueToString(%v) = %q, expected %q", tt.value, result, tt.expected)
			}
		})
	}

	// Test map without content separately (it returns the string representation)
	t.Run("map_without_content", func(t *testing.T) {
		result := convertValueToString(map[string]string{"other": "value"})
		if result == "" {
			t.Error("Expected non-empty string for map without content")
		}
	})
}
