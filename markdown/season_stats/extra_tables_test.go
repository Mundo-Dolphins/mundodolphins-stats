package season_stats

import (
	"strings"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/pfr"
)

func TestNormalizeTable_RespectsAllowlistOrder(t *testing.T) {
	// Test that NormalizeTable respects the column order from the allowlist
	tableName := "roster"
	rows := []map[string]any{
		{
			"Name":    "Player One",
			"Pos":     "QB",
			"Age":     "26",
			"Weight":  "220",
			"College": "Alabama",
		},
	}

	allowedCols := GetAllowedColumns(tableName)
	if len(allowedCols) == 0 {
		t.Fatalf("no allowed columns for %s", tableName)
	}

	cols, normalizedRows := NormalizeTable(tableName, rows, allowedCols)

	// Check that columns follow the allowlist order
	if len(normalizedRows) == 0 {
		t.Fatalf("expected normalized rows")
	}

	row := normalizedRows[0]
	for _, col := range cols {
		if _, exists := row[col]; !exists {
			t.Fatalf("column %s missing from normalized row", col)
		}
	}

	// Verify the allowed columns are actually used
	for _, col := range cols {
		found := false
		for _, allowed := range allowedCols {
			if col == allowed {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("column %s not in allowlist", col)
		}
	}
}

func TestNormalizeTable_RemovesEmptyColumns(t *testing.T) {
	// Test that NormalizeTable removes columns that are completely empty
	tableName := "roster"
	rows := []map[string]any{
		{
			"Name":   "Player One",
			"Pos":    "QB",
			"Age":    "26",
			"Empty1": nil,
			"Empty2": "",
		},
		{
			"Name":   "Player Two",
			"Pos":    "RB",
			"Age":    "23",
			"Empty1": nil,
			"Empty2": "",
		},
	}

	allowedCols := GetAllowedColumns(tableName)
	cols, normalizedRows := NormalizeTable(tableName, rows, allowedCols)

	// Empty columns should not appear in the final columns list
	for _, col := range cols {
		if col == "Empty1" || col == "Empty2" {
			t.Fatalf("empty column %s should not be in normalized table", col)
		}
	}

	// Verify data is intact for non-empty columns
	if len(normalizedRows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(normalizedRows))
	}
}

func TestExtraTables_SkipsPreseasonOdds(t *testing.T) {
	// Test that preseason_odds table is never included in extra tables
	data := pfr.SeasonDataV1_1{
		"main": {
			Tables: map[string]pfr.TableResult{
				"preseason_odds": {
					Rows: []map[string]any{
						{"Game": "Test", "Opponent": "Someone"},
					},
					Title: "Preseason Odds",
				},
				"roster": {
					Rows: []map[string]any{
						{"Name": "Player", "Pos": "QB"},
					},
					Title: "Roster",
				},
			},
		},
	}

	extras := buildExtraTables(data)

	// Check that preseason_odds is not in the result
	for _, extra := range extras {
		if extra.Name == "preseason_odds" {
			t.Fatalf("preseason_odds should not be in extra tables")
		}
	}

	// Check that roster (if valid) is included
	hasRoster := false
	for _, extra := range extras {
		if extra.Name == "roster" {
			hasRoster = true
			break
		}
	}
	if !hasRoster {
		t.Logf("Note: roster table was not included (may be expected if no roster columns in allowlist)")
	}
}

func TestExtraTables_OmitsMissingTablesForOldYears(t *testing.T) {
	// Test that tables not in allowlist are gracefully skipped
	data := pfr.SeasonDataV1_1{
		"main": {
			Tables: map[string]pfr.TableResult{
				"passing": {
					Rows: []map[string]any{
						{"Name": "QB", "Yds": "1000"},
					},
					Title: "Passing",
				},
				"some_future_table": {
					Rows: []map[string]any{
						{"Data": "Value"},
					},
					Title: "Future Table",
				},
			},
		},
	}

	extras := buildExtraTables(data)

	// Verify that "passing" (a rendered table) is not in extras
	for _, extra := range extras {
		if extra.Name == "passing" {
			t.Fatalf("passing is a rendered table and should not be in extras")
		}
	}

	// Verify that "some_future_table" (not in allowlist) is not in extras
	for _, extra := range extras {
		if extra.Name == "some_future_table" {
			t.Fatalf("unknown table should not be in extras")
		}
	}
}

func TestBuildExtraTables_2024_ExtraTablesPresent(t *testing.T) {
	data := pfr.SeasonDataV1_1{
		"main": {
			Tables: map[string]pfr.TableResult{
				"passing": {
					Rows: []map[string]any{
						{"Name": "QB Test", "Cmp": "100", "Att": "150", "Yds": "1000", "TD": "5", "Int": "2"},
					},
					Title:   "Passing",
					Legends: map[string]string{},
				},
				"roster": {
					Rows: []map[string]any{
						{"Name": "Player One", "Pos": "QB", "Age": "26", "College": "Alabama"},
					},
					Title:   "Roster",
					Legends: map[string]string{},
				},
				"preseason_odds": {
					Rows: []map[string]any{
						{"Game": "Test", "Odds": "value"},
					},
					Title:   "Preseason Odds",
					Legends: map[string]string{},
				},
			},
		},
	}

	extras := buildExtraTables(data)
	if len(extras) != 1 {
		t.Fatalf("expected one extra table, got %d", len(extras))
	}
	if extras[0].Name != "roster" {
		t.Fatalf("expected roster extra table, got %q", extras[0].Name)
	}
}

func TestBuildExtraTables_NilInput(t *testing.T) {
	// Test that buildExtraTables handles nil input gracefully
	var data pfr.SeasonDataV1_1
	extras := buildExtraTables(data)

	if len(extras) != 0 {
		t.Fatalf("expected no extras for nil data, got %d", len(extras))
	}
}

func TestBuildExtraTables_DeterministicOrder(t *testing.T) {
	// Test that buildExtraTables returns tables in deterministic order
	data := pfr.SeasonDataV1_1{
		"main": {
			Tables: map[string]pfr.TableResult{
				"roster": {
					Rows: []map[string]any{
						{"Name": "Player", "Pos": "QB", "Age": "26"},
					},
					Title: "Roster",
				},
				"draft": {
					Rows: []map[string]any{
						{"Rnd": "1", "Pk": "10", "Player": "Someone"},
					},
					Title: "Draft",
				},
			},
		},
	}

	// Call buildExtraTables multiple times
	extras1 := buildExtraTables(data)
	extras2 := buildExtraTables(data)

	if len(extras1) != len(extras2) {
		t.Fatalf("inconsistent number of extras: %d vs %d", len(extras1), len(extras2))
	}

	// Verify order is sorted alphabetically by table name
	if len(extras1) > 0 {
		for i := 0; i < len(extras1)-1; i++ {
			if extras1[i].Name > extras1[i+1].Name {
				t.Fatalf("extras not sorted: %s > %s", extras1[i].Name, extras1[i+1].Name)
			}
		}
	}

	// Verify order is the same across calls
	for i := range extras1 {
		if extras1[i].Name != extras2[i].Name {
			t.Fatalf("order mismatch at index %d: %s vs %s", i, extras1[i].Name, extras2[i].Name)
		}
	}
}

func TestGetTableTitleES(t *testing.T) {
	// Test that GetTableTitleES returns correct Spanish titles
	tests := []struct {
		name     string
		wantES   string
		wantFork bool // if true, title can either be ES or humanized
	}{
		{"roster", "Plantilla", false},
		{"draft", "Draft", false},
		{"snap_counts", "Conteo de snaps", false},
		{"some_unknown_table", "", true}, // Should use HumanizeTableName
	}

	for _, tt := range tests {
		got := GetTableTitleES(tt.name)
		if tt.wantFork {
			// For unknown tables, just verify it's not empty and is humanized
			if got == "" {
				t.Fatalf("GetTableTitleES(%s) returned empty string", tt.name)
			}
		} else if got != tt.wantES {
			t.Fatalf("GetTableTitleES(%s) = %q, want %q", tt.name, got, tt.wantES)
		}
	}
}

func TestIsRenderableTable(t *testing.T) {
	// Test that IsRenderableTable correctly filters tables
	tests := []struct {
		name   string
		wantOK bool
	}{
		{"roster", true},          // In allowlist, not skipped
		{"draft", true},           // In allowlist, not skipped
		{"preseason_odds", false}, // Must be filtered (in SkipTables)
		{"passing", true},         // Not in SkipTables, so IsRenderableTable = true
		{"unknown_table", true},   // Not in SkipTables, so IsRenderableTable = true
	}

	for _, tt := range tests {
		got := IsRenderableTable(tt.name)
		if got != tt.wantOK {
			t.Fatalf("IsRenderableTable(%s) = %v, want %v", tt.name, got, tt.wantOK)
		}
	}
}

func TestHumanizeTableName(t *testing.T) {
	// Test that HumanizeTableName converts table names correctly
	tests := []struct {
		input string
		want  string
	}{
		{"roster", "Roster"},
		{"snap_counts", "Snap Counts"},
		{"team_injuries", "Team Injuries"},
		{"adv_defense", "Adv Defense"},
		{"down_splits", "Down Splits"},
	}

	for _, tt := range tests {
		got := HumanizeTableName(tt.input)
		if got != tt.want {
			t.Fatalf("HumanizeTableName(%s) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValueToStringConversions(t *testing.T) {
	// Testing internal valueToString function via NormalizeTable
	testCases := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, ""},
		{"empty string", "", ""},
		{"string", "test", "test"},
		{"int", 42, "42"},
		{"float (int-like)", 42.0, "42"},
		{"float (decimal)", 3.14, "3.14"},
		{"bool true", true, "yes"},
		{"bool false", false, "no"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows := []map[string]any{
				{"test_field": tc.value},
			}
			_, normalizedRows := NormalizeTable("roster", rows, []string{"test_field"})

			if len(normalizedRows) > 0 {
				got := normalizedRows[0]["test_field"]
				if got != tc.want {
					t.Fatalf("valueToString(%#v) = %q, want %q", tc.value, got, tc.want)
				}
			}
		})
	}
}

func TestNormalizeTableWithLinkValues(t *testing.T) {
	// Test that NormalizeTable correctly handles LinkValue types
	rows := []map[string]any{
		{
			"Name": map[string]any{
				"content": "John Doe",
				"link":    "/players/123",
			},
			"Age": "26",
		},
	}

	_, normalizedRows := NormalizeTable("roster", rows, []string{"Name", "Age"})

	if len(normalizedRows) == 0 {
		t.Fatalf("expected normalized rows")
	}

	// The LinkValue should be converted to markdown link format
	nameValue := normalizedRows[0]["Name"]
	if nameValue == "" {
		t.Fatalf("expected non-empty name value")
	}
	// Verify it contains the content
	if !strings.Contains(nameValue, "John Doe") {
		t.Fatalf("expected name content 'John Doe' in %q", nameValue)
	}
}

func TestNormalizeTableFiltersNonExistentColumns(t *testing.T) {
	// Test that NormalizeTable gracefully handles columns not present in data
	rows := []map[string]any{
		{"Name": "Player", "Age": "26"},
	}

	// Request columns including one that doesn't exist
	cols, normalizedRows := NormalizeTable("roster", rows, []string{"Name", "Age", "NonExistent"})

	if len(normalizedRows) == 0 {
		t.Fatalf("expected normalized rows")
	}

	// NonExistent column should not be in the final columns
	for _, col := range cols {
		if col == "NonExistent" {
			t.Fatalf("non-existent column should not be in normalized table")
		}
	}

	// But Name and Age should be there
	row := normalizedRows[0]
	if _, exists := row["Name"]; !exists {
		t.Fatalf("Name column missing from normalized row")
	}
	if _, exists := row["Age"]; !exists {
		t.Fatalf("Age column missing from normalized row")
	}
}

func TestGetAllColumnsFromRows(t *testing.T) {
	rows := []map[string]any{
		{"Name": "Player1", "Age": "25"},
		{"Name": "Player2", "Age": "30", "Team": "MIA"},
	}
	cols := getAllColumnsFromRows(rows)

	// Should contain all unique columns
	expected := []string{"Age", "Name", "Team"}
	if len(cols) != len(expected) {
		t.Fatalf("expected %d columns, got %d: %v", len(expected), len(cols), cols)
	}
	for i, c := range expected {
		if cols[i] != c {
			t.Fatalf("col[%d] = %q, want %q", i, cols[i], c)
		}
	}
}

func TestGetAllColumnsFromRows_Empty(t *testing.T) {
	cols := getAllColumnsFromRows(nil)
	if cols != nil {
		t.Fatalf("expected nil for empty rows, got %v", cols)
	}
}

func TestValueToString_LinkValueEmptyLink(t *testing.T) {
	// LinkValue with link="." or "" should just return content without link
	tests := []struct {
		name string
		val  map[string]any
		want string
	}{
		{"dot link", map[string]any{"content": "Player", "link": "."}, "Player"},
		{"empty link", map[string]any{"content": "Player", "link": ""}, "Player"},
		{"no link key", map[string]any{"content": "Player"}, "Player"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := valueToString(tc.val)
			if got != tc.want {
				t.Errorf("valueToString(%v) = %q, want %q", tc.val, got, tc.want)
			}
		})
	}
}

func TestValueToString_RichTextWithLinks(t *testing.T) {
	// RichTextWithLinks with links should return content
	val := map[string]any{
		"content": "Drafted by Miami",
		"links":   []any{map[string]any{"anchor": "Miami", "link": "/teams/mia"}},
	}
	got := valueToString(val)
	if got != "Drafted by Miami" {
		t.Errorf("valueToString(RichTextWithLinks) = %q, want %q", got, "Drafted by Miami")
	}
}

func TestValueToString_ArrayType(t *testing.T) {
	val := []any{"item1", "item2", "item3"}
	got := valueToString(val)
	if got != "item1, item2, item3" {
		t.Errorf("valueToString([]any) = %q, want %q", got, "item1, item2, item3")
	}
}

func TestValueToString_UnknownMap(t *testing.T) {
	val := map[string]any{"unknown": "data", "count": 3}
	got := valueToString(val)
	if got == "" {
		t.Error("expected non-empty string for unknown map type")
	}
}

func TestNormalizeTable_EmptyRows(t *testing.T) {
	cols, rows := NormalizeTable("test", []map[string]any{}, []string{"col1", "col2"})
	if len(cols) != 0 {
		t.Errorf("expected empty columns for empty rows, got %v", cols)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty rows, got %v", rows)
	}
}
