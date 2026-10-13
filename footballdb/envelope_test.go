package footballdb

import (
	"testing"
)

func TestBuildSeasonEnvelope(t *testing.T) {
	data := SeasonData{
		"roster": PagePayload{
			Status: "ok",
			Tables: map[string]TableResult{
				"roster": {Title: "Roster", RowCount: 53},
			},
		},
	}
	env := BuildSeasonEnvelope(2024, data)

	if env.Schema.Name != SeasonSchemaName {
		t.Errorf("schema name = %q, want %q", env.Schema.Name, SeasonSchemaName)
	}
	if env.Schema.Version != SeasonSchemaVersion {
		t.Errorf("schema version = %q, want %q", env.Schema.Version, SeasonSchemaVersion)
	}
	if env.Season != 2024 {
		t.Errorf("season = %d, want 2024", env.Season)
	}
	if env.Team.ID != "mia" {
		t.Errorf("team id = %q, want 'mia'", env.Team.ID)
	}
	if env.Source.Site != SiteName {
		t.Errorf("source site = %q, want %q", env.Source.Site, SiteName)
	}
	if env.GeneratedAt == "" {
		t.Error("generated_at should not be empty")
	}
	if _, ok := env.Data["roster"]; !ok {
		t.Error("expected roster payload in data")
	}
}

func TestBuildGameEnvelope(t *testing.T) {
	data := GameData{
		GameID: "2025090703",
		Tables: map[string]TableResult{},
	}
	env := BuildGameEnvelope(2025, "2025090703", "https://example.com", "cache/file.html", fixedTime(), data)

	if env.Schema.Name != GameSchemaName {
		t.Errorf("schema name = %q, want %q", env.Schema.Name, GameSchemaName)
	}
	if env.Schema.Version != GameSchemaVersion {
		t.Errorf("schema version = %q, want %q", env.Schema.Version, GameSchemaVersion)
	}
	if env.Data.GameID != "2025090703" {
		t.Errorf("data.game_id = %q, want '2025090703'", env.Data.GameID)
	}
	if env.GameID != "2025090703" {
		t.Errorf("game_id = %q, want '2025090703'", env.GameID)
	}
	if env.Source.URL != "https://example.com" {
		t.Errorf("source url = %q", env.Source.URL)
	}
}

func TestBuildInjuryEnvelope(t *testing.T) {
	data := InjuryData{
		GID:    "2025113003",
		Tables: map[string]TableResult{},
	}
	env := BuildInjuryEnvelope(2025, "2025113003", "https://example.com", "cache/inj.html", fixedTime(), data)

	if env.Schema.Name != InjurySchemaName {
		t.Errorf("schema name = %q, want %q", env.Schema.Name, InjurySchemaName)
	}
	if env.Data.GID != "2025113003" {
		t.Errorf("gid = %q", env.Data.GID)
	}
}

func TestValidateSeasonYear(t *testing.T) {
	tests := []struct {
		year    int
		wantErr bool
	}{
		{1965, true},
		{1966, false},
		{2024, false},
		{2026, false},
		{3000, true},
	}
	for _, tt := range tests {
		err := ValidateSeasonYear(tt.year)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateSeasonYear(%d) error = %v, wantErr %v", tt.year, err, tt.wantErr)
		}
	}
}
