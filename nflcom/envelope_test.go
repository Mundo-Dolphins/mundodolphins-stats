package nflcom

import "testing"

func TestBuildEnvelopes(t *testing.T) {
	sched := BuildScheduleEnvelope(2025, "https://example.com/s", "cache/s.html", ScheduleData{})
	if sched.Schema.Name != SeasonSchemaName || sched.Season != 2025 || sched.Source.URL != "https://example.com/s" || sched.Team.ID != "mia" {
		t.Fatalf("unexpected schedule envelope: %+v", sched)
	}
	game := BuildGameEnvelope(2025, "https://example.com/g", "cache/g.html", GameData{})
	if game.Schema.Name != GameSchemaName || game.Source.CacheFile != "cache/g.html" || game.GeneratedAt == "" {
		t.Fatalf("unexpected game envelope: %+v", game)
	}
}
