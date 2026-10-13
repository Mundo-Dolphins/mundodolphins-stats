package pagejson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageTargeted_FallbacksAndHelpers(t *testing.T) {
	// Trigger Build fallback -> buildLooseMap (string team names are invalid for typed boxscore structs).
	builder := NewGameBuilder("", "")
	page, err := builder.Build(map[string]interface{}{
		"game_id": "202401010mia",
		"data": map[string]interface{}{
			"scorebox": map[string]interface{}{
				"teams": []interface{}{
					map[string]interface{}{"name": "Team A"},
					map[string]interface{}{"name": "Team B"},
				},
			},
		},
	}, "202401010mia")
	if err != nil {
		t.Fatalf("Build fallback failed: %v", err)
	}
	if _, ok := page.Sections["Equipos"]; !ok {
		t.Fatal("fallback page missing Equipos")
	}

	// BuildFromGameID happy path
	tmp := t.TempDir()
	gamesDir := filepath.Join(tmp, "games", "2024")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"game_id":"202409080mia","data":{"scorebox":{"teams":[]},"tables":{}}}`
	if err := os.WriteFile(filepath.Join(gamesDir, "202409080mia.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGameBuilder(tmp, "").BuildFromGameID("202409080mia"); err != nil {
		t.Fatalf("BuildFromGameID should tolerate minimal fixture: %v", err)
	}

	// convertSliceOfMapInterface path
	rows, err := ConvertTableToRows([]map[string]interface{}{{"a": 1, "b": "x"}}, nil)
	if err != nil || len(rows) != 1 || rows[0].Get("A") != "1" {
		t.Fatalf("ConvertTableToRows []map[string]interface{} failed: rows=%v err=%v", rows, err)
	}

	// TableRow helpers Has/Keys
	r := NewTableRow()
	r.Set("A", "1")
	r.Set("B", "2")
	if !r.Has("A") || r.Has("C") {
		t.Fatal("TableRow.Has returned unexpected result")
	}
	if keys := r.Keys(); len(keys) != 2 || keys[0] != "A" || keys[1] != "B" {
		t.Fatalf("TableRow.Keys order mismatch: %#v", keys)
	}

	// extractString branch and GameOutputPath invalid gameID branch.
	if got := extractString(map[string]interface{}{"x": map[string]interface{}{"content": "v"}}, "x"); got != "v" {
		t.Fatalf("extractString link branch failed: %q", got)
	}
	p := NewPagePaths("out")
	_ = p.GameOutputPath("bad") // invalid gameID should not panic

	// SortSections non-priority branch
	sorted := SortSections(map[string]interface{}{"Z": nil, "A": nil})
	if len(sorted) != 2 {
		t.Fatalf("SortSections unexpected: %#v", sorted)
	}

	// encode path smoke to exercise MarshalJSON ordering path.
	buf, err := EncodeToJSON(page)
	if err != nil {
		t.Fatalf("EncodeToJSON failed: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
}

func TestCoverageTargeted_ExtractSeasonYearBranches(t *testing.T) {
	cases := []map[string]interface{}{
		{"season": 2024},
		{"season": int64(2024)},
		{"season": "2024"},
	}
	for _, c := range cases {
		if _, err := extractSeasonYear(c); err != nil {
			t.Fatalf("extractSeasonYear failed for %#v: %v", c, err)
		}
	}
	if _, err := extractSeasonYear(map[string]interface{}{"season": struct{}{}}); err == nil {
		t.Fatal("expected error for unsupported season type")
	}
}
