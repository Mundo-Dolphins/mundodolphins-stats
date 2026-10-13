package audit

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestInspectSeason_1966(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join("testdata", "1966_minimal.json")

	inventory, err := InspectSeason(ctx, fixturePath)
	if err != nil {
		t.Fatalf("InspectSeason failed: %v", err)
	}

	if inventory.Year != 1966 {
		t.Errorf("Expected year 1966, got %d", inventory.Year)
	}

	if len(inventory.Tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(inventory.Tables))
	}

	expectedTables := []string{"passing", "rushing_and_receiving", "defense"}
	for _, tableName := range expectedTables {
		if _, exists := inventory.Tables[tableName]; !exists {
			t.Errorf("Expected table %q not found", tableName)
		}
	}
}

func TestInspectSeason_2024(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join("testdata", "2024_extended.json")

	inventory, err := InspectSeason(ctx, fixturePath)
	if err != nil {
		t.Fatalf("InspectSeason failed: %v", err)
	}

	if inventory.Year != 2024 {
		t.Errorf("Expected year 2024, got %d", inventory.Year)
	}

	if len(inventory.Tables) < 10 {
		t.Errorf("Expected at least 10 tables, got %d", len(inventory.Tables))
	}

	// Verify columns are sorted
	for tableName, tableInfo := range inventory.Tables {
		columns := tableInfo.Columns
		sortedColumns := make([]string, len(columns))
		copy(sortedColumns, columns)
		sort.Strings(sortedColumns)

		for i, col := range columns {
			if col != sortedColumns[i] {
				t.Errorf("Table %q columns not sorted: got %v, expected %v", tableName, columns, sortedColumns)
				break
			}
		}
	}
}

func TestInspectSeason_FileNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := InspectSeason(ctx, "nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestInspectSeason_InvalidJSON(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "invalid_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	tmpfile.WriteString("{invalid json")
	tmpfile.Close()

	ctx := context.Background()
	_, err = InspectSeason(ctx, tmpfile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestInspectSeason_EmptyJSON(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "empty_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	tmpfile.WriteString("{}")
	tmpfile.Close()

	ctx := context.Background()
	inventory, err := InspectSeason(ctx, tmpfile.Name())
	if err != nil {
		t.Errorf("Unexpected error for empty JSON: %v", err)
	}

	if len(inventory.Tables) != 0 {
		t.Errorf("Expected 0 tables for empty JSON, got %d", len(inventory.Tables))
	}
}

func TestInspectSeason_NestedTablesObjects(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "nested_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	jsonData := `{
		"data": {
			"gamelog": {
				"tables": {
					"game_logs": {
						"rows": [
							{"id": 1, "value": "test1"},
							{"id": 2, "value": "test2"}
						],
						"row_count": 2
					}
				}
			},
			"direct_array": [
				{"id": 3}
			]
		}
	}`

	tmpfile.WriteString(jsonData)
	tmpfile.Close()

	ctx := context.Background()
	inventory, err := InspectSeason(ctx, tmpfile.Name())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(inventory.Tables) < 2 {
		t.Errorf("Expected at least 2 tables, got %d", len(inventory.Tables))
	}

	if table, exists := inventory.Tables["game_logs"]; exists {
		expectedCols := []string{"id", "value"}
		sort.Strings(expectedCols)
		if len(table.Columns) != len(expectedCols) {
			t.Errorf("Expected %d columns for game_logs, got %d", len(expectedCols), len(table.Columns))
		}
	}
}

func TestBuildReport(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join("testdata", "2024_extended.json")

	inventory, err := InspectSeason(ctx, fixturePath)
	if err != nil {
		t.Fatalf("InspectSeason failed: %v", err)
	}

	report := BuildReport(inventory)

	if report.Year != 2024 {
		t.Errorf("Expected year 2024, got %d", report.Year)
	}

	if len(report.TablesFound) == 0 {
		t.Error("Expected some tables in report")
	}
}

func TestDiffSeasons(t *testing.T) {
	ctx := context.Background()
	fixture1966 := filepath.Join("testdata", "1966_minimal.json")
	fixture2024 := filepath.Join("testdata", "2024_extended.json")

	inv1966, err := InspectSeason(ctx, fixture1966)
	if err != nil {
		t.Fatalf("InspectSeason failed for 1966: %v", err)
	}

	inv2024, err := InspectSeason(ctx, fixture2024)
	if err != nil {
		t.Fatalf("InspectSeason failed for 2024: %v", err)
	}

	diff := DiffSeasons(inv1966, inv2024)

	if len(diff.TablesOnlyInB) == 0 {
		t.Error("Expected tables only in 2024 (B)")
	}

	if !sort.StringsAreSorted(diff.TablesOnlyInA) {
		t.Error("TablesOnlyInA not sorted")
	}
	if !sort.StringsAreSorted(diff.TablesOnlyInB) {
		t.Error("TablesOnlyInB not sorted")
	}
}

func TestDiffSeasons_Identical(t *testing.T) {
	ctx := context.Background()
	fixture := filepath.Join("testdata", "1966_minimal.json")

	inv1, err := InspectSeason(ctx, fixture)
	if err != nil {
		t.Fatalf("InspectSeason failed: %v", err)
	}

	inv2, err := InspectSeason(ctx, fixture)
	if err != nil {
		t.Fatalf("InspectSeason failed: %v", err)
	}

	diff := DiffSeasons(inv1, inv2)

	if len(diff.TablesOnlyInA) > 0 {
		t.Error("Expected no tables only in A for identical inventories")
	}

	if len(diff.TablesOnlyInB) > 0 {
		t.Error("Expected no tables only in B for identical inventories")
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		{"processed/2024.json", 2024},
		{"processed/1966.json", 1966},
		{"/path/to/2020.json", 2020},
		{"cache/2019.jsonl", 2019},
		{"data_2015.json", 2015},
	}

	for _, tt := range tests {
		result := extractYear(tt.path)
		if result != tt.expected {
			t.Errorf("extractYear(%q) = %d, want %d", tt.path, result, tt.expected)
		}
	}
}

func TestInspectTableInfo(t *testing.T) {
	ctx := context.Background()
	fixturePath := filepath.Join("testdata", "2024_extended.json")

	inventory, err := InspectSeason(ctx, fixturePath)
	if err != nil {
		t.Fatalf("InspectSeason failed: %v", err)
	}

	if table, exists := inventory.Tables["passing"]; exists {
		if table.Name != "passing" {
			t.Errorf("Expected table name 'passing', got %q", table.Name)
		}

		if len(table.Columns) == 0 {
			t.Error("Expected columns in passing table")
		}

		if table.RowCount <= 0 {
			t.Error("Expected positive row count")
		}
	}
}

func BenchmarkInspectSeason(b *testing.B) {
	ctx := context.Background()
	fixturePath := filepath.Join("testdata", "2024_extended.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InspectSeason(ctx, fixturePath)
	}
}

func TestRealData2024(t *testing.T) {
	realPath := "processed/2024.json"
	if _, err := os.Stat(realPath); os.IsNotExist(err) {
		t.Skip("Real 2024.json not found, skipping integration test")
	}

	ctx := context.Background()
	inventory, err := InspectSeason(ctx, realPath)
	if err != nil {
		t.Fatalf("InspectSeason failed on real data: %v", err)
	}

	if inventory.Year != 2024 {
		t.Errorf("Expected year 2024, got %d", inventory.Year)
	}

	if len(inventory.Tables) < 20 {
		t.Logf("Warning: Expected at least 20 tables in real 2024.json, got %d", len(inventory.Tables))
	}

	t.Logf("Real 2024.json inspection: Year=%d, Tables=%d", inventory.Year, len(inventory.Tables))
}
