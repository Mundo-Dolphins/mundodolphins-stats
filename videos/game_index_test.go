package videos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPFRGameIndexFromDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "196609020mia.json")
	payload := map[string]any{
		"game_id": "196609020mia",
		"data": map[string]any{
			"linescore": map[string]any{
				"teams": []any{
					map[string]any{"team": map[string]any{"content": "Oakland Raiders"}},
					map[string]any{"team": map[string]any{"content": "Miami Dolphins"}},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatalf("write game file: %v", err)
	}

	idx, err := BuildPFRGameIndexFromDir(dir)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	id, ok := idx.DateToGameID("1966-09-02")
	if !ok || id != "196609020mia" {
		t.Fatalf("expected game id mapping")
	}
	ref, ok := idx.GameInfo("1966-09-02")
	if !ok || ref.AwayTeam != "Oakland Raiders" || ref.HomeTeam != "Miami Dolphins" {
		t.Fatalf("expected team names from linescore")
	}
}
