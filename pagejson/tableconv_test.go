package pagejson

import (
	"testing"
)

func TestConvertTableToRows_SliceOfInterface(t *testing.T) {
	// Input: []interface{} with map entries
	tableData := []interface{}{
		map[string]interface{}{
			"player": "Tua Tagovailoa",
			"pos":    "QB",
			"age":    25,
			"yards":  4108.5,
		},
		map[string]interface{}{
			"player": "Jaylen Waddle",
			"pos":    "WR",
			"age":    24,
			"yards":  1146.0,
		},
	}

	headerMapping := map[string]string{
		"player": "Nombre",
		"pos":    "Posición",
		"age":    "Edad",
		"yards":  "Yardas",
	}

	rows, err := ConvertTableToRows(tableData, headerMapping)
	if err != nil {
		t.Fatalf("ConvertTableToRows failed: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}

	// Check first row
	if rows[0].Get("Nombre") != "Tua Tagovailoa" {
		t.Errorf("expected 'Tua Tagovailoa', got '%s'", rows[0].Get("Nombre"))
	}

	if rows[0].Get("Posición") != "QB" {
		t.Errorf("expected 'QB', got '%s'", rows[0].Get("Posición"))
	}

	if rows[0].Get("Edad") != "25" {
		t.Errorf("expected '25', got '%s'", rows[0].Get("Edad"))
	}

	// Check float conversion (should format with 2 decimals)
	if rows[0].Get("Yardas") != "4108.50" {
		t.Errorf("expected '4108.50', got '%s'", rows[0].Get("Yardas"))
	}
}

func TestConvertTableToRows_EmptyCells(t *testing.T) {
	tableData := []interface{}{
		map[string]interface{}{
			"player": "Player 1",
			"pos":    "RB",
			"yards":  nil, // Nil value
		},
		map[string]interface{}{
			"player": "Player 2",
			"pos":    "", // Empty string
			"yards":  100,
		},
	}

	rows, err := ConvertTableToRows(tableData, nil)
	if err != nil {
		t.Fatalf("ConvertTableToRows failed: %v", err)
	}

	// Check nil converts to empty string
	if rows[0].Get("yards") != "" {
		t.Errorf("expected empty string for nil, got '%s'", rows[0].Get("yards"))
	}

	// Check empty string remains empty
	if rows[1].Get("pos") != "" {
		t.Errorf("expected empty string, got '%s'", rows[1].Get("pos"))
	}
}

func TestConvertTableToRows_LinkValue(t *testing.T) {
	// Test LinkValue structure (map with "content" and "link")
	// Only "content" should be extracted; links are NOT included in output
	tableData := []interface{}{
		map[string]interface{}{
			"player": map[string]interface{}{
				"content": "Tua Tagovailoa",
				"link":    "/players/T/TagoTu00.htm",
			},
			"yards": 4108,
		},
	}

	rows, err := ConvertTableToRows(tableData, nil)
	if err != nil {
		t.Fatalf("ConvertTableToRows failed: %v", err)
	}

	// Should extract "content" from LinkValue
	if rows[0].Get("Nombre") != "Tua Tagovailoa" {
		t.Errorf("expected 'Tua Tagovailoa', got '%s'", rows[0].Get("Nombre"))
	}
	// _link fields should NOT be present
	if rows[0].Has("Nombre_link") {
		t.Errorf("expected no Nombre_link field, but got '%s'", rows[0].Get("Nombre_link"))
	}
}

func TestNormalizeHeaders_Duplicates(t *testing.T) {
	headers := []string{"Columna", "Nombre", "Columna", "Nombre", "Columna"}

	normalized := NormalizeHeaders(headers)

	expected := []string{"Columna", "Nombre", "Columna_2", "Nombre_2", "Columna_3"}

	if len(normalized) != len(expected) {
		t.Fatalf("expected %d headers, got %d", len(expected), len(normalized))
	}

	for i, exp := range expected {
		if normalized[i] != exp {
			t.Errorf("index %d: expected '%s', got '%s'", i, exp, normalized[i])
		}
	}
}

func TestNormalizeHeaders_EmptyHeaders(t *testing.T) {
	headers := []string{"Name", "", "Age", "", "Score"}

	normalized := NormalizeHeaders(headers)

	expected := []string{"Name", "Columna", "Age", "Columna_2", "Score"}

	for i, exp := range expected {
		if normalized[i] != exp {
			t.Errorf("index %d: expected '%s', got '%s'", i, exp, normalized[i])
		}
	}
}

func TestConvertTableWithHeaders_Basic(t *testing.T) {
	headers := []string{"Player", "Position", "Yards"}
	rows := [][]interface{}{
		{"Tua Tagovailoa", "QB", 4108},
		{"Jaylen Waddle", "WR", 1146},
	}

	result, err := ConvertTableWithHeaders(headers, rows)
	if err != nil {
		t.Fatalf("ConvertTableWithHeaders failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result))
	}

	if result[0].Get("Player") != "Tua Tagovailoa" {
		t.Errorf("expected 'Tua Tagovailoa', got '%s'", result[0].Get("Player"))
	}

	if result[0].Get("Yards") != "4108" {
		t.Errorf("expected '4108', got '%s'", result[0].Get("Yards"))
	}
}

func TestConvertTableWithHeaders_MismatchedColumns(t *testing.T) {
	headers := []string{"Player", "Position"}
	rows := [][]interface{}{
		{"Tua Tagovailoa", "QB", 4108}, // 3 cells but only 2 headers
	}

	_, err := ConvertTableWithHeaders(headers, rows)
	if err == nil {
		t.Error("expected error for mismatched columns, got nil")
	}
}

func TestConvertValueToString_Numbers(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{int(42), "42"},
		{int64(12345), "12345"},
		{float64(3.14), "3.14"},
		{float64(100.0), "100"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
		{"hello", "hello"},
	}

	for _, test := range tests {
		result := convertValueToString(test.input)
		if result != test.expected {
			t.Errorf("convertValueToString(%v): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestDisplayHeaderForKey_DefaultAliasesAndHumanize(t *testing.T) {
	if got := displayHeaderForKey("player", nil); got != "Nombre" {
		t.Fatalf("expected alias Nombre, got %q", got)
	}
	if got := displayHeaderForKey("pass_adj_yds_per_att", nil); got != "Yardas ajust./intento" {
		t.Fatalf("unexpected translated header: %q", got)
	}
	// Key not in any alias → falls through to humanizeHeaderKey
	if got := displayHeaderForKey("some_unknown_key_xyz", nil); got != "Some Unknown Key Xyz" {
		t.Fatalf("unexpected humanized header: %q", got)
	}
	if got := displayHeaderForKey("custom_key", map[string]string{"custom_key": "Custom"}); got != "Custom" {
		t.Fatalf("expected explicit mapping override, got %q", got)
	}
}
