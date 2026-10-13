package pfr

import (
	"testing"
	"time"
)

func TestBuildSeasonEnvelope(t *testing.T) {
	fetchedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	data := SeasonDataV1{
		Info:   map[string]string{"Coach": "Test"},
		Tables: map[string][]map[string]any{"T": {{"A": 1}}},
	}

	env := BuildSeasonEnvelope(2024, "http://example.com", "cache/2024.html", fetchedAt, data)
	if env.Schema.Name != SchemaName || env.Schema.Version != SchemaVersion {
		t.Fatalf("unexpected schema info: %+v", env.Schema)
	}
	if env.Season != 2024 {
		t.Fatalf("unexpected season: %d", env.Season)
	}
	if env.Source.URL != "http://example.com" {
		t.Fatalf("unexpected source url: %s", env.Source.URL)
	}
	if env.Source.CacheFile != "cache/2024.html" {
		t.Fatalf("unexpected cache file: %s", env.Source.CacheFile)
	}
	if env.Source.FetchedAt != fetchedAt.Format(time.RFC3339) {
		t.Fatalf("unexpected fetched_at: %s", env.Source.FetchedAt)
	}
	if env.Team.ID != "mia" || env.Team.Name != "Miami Dolphins" {
		t.Fatalf("unexpected team info: %+v", env.Team)
	}
}

func TestValidateSeasonYear(t *testing.T) {
	maxYear := time.Now().Year()
	if err := ValidateSeasonYear(1966); err != nil {
		t.Fatalf("unexpected error for 1966: %v", err)
	}
	if err := ValidateSeasonYear(maxYear); err != nil {
		t.Fatalf("unexpected error for %d: %v", maxYear, err)
	}
	if err := ValidateSeasonYear(1965); err == nil {
		t.Fatalf("expected error for 1965")
	}
	if err := ValidateSeasonYear(maxYear + 1); err == nil {
		t.Fatalf("expected error for %d", maxYear+1)
	}
}

func TestGetSeasonURL(t *testing.T) {
	url := GetSeasonURL(2024)
	if url != "https://www.pro-football-reference.com/teams/mia/2024.htm" {
		t.Fatalf("unexpected url: %s", url)
	}
}
