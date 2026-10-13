package videos

import (
	"testing"
)

func TestNewMatchIndex(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
		{GameID: "196609090mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "New York Jets", DateRaw: "September 9, 1966"},
		{GameID: "202309100mia", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Los Angeles Chargers", DateRaw: "September 10, 2023"},
	}

	idx := NewMatchIndex(games)

	if idx.TotalGames() != 3 {
		t.Errorf("TotalGames() = %d, want 3", idx.TotalGames())
	}

	ids := idx.AllGameIDs()
	if len(ids) != 3 {
		t.Errorf("AllGameIDs() len = %d, want 3", len(ids))
	}
	// Should be sorted
	if ids[0] != "196609020mia" {
		t.Errorf("first ID = %q, want 196609020mia", ids[0])
	}
}

func TestMatchIndex_GameByID(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders"},
	}
	idx := NewMatchIndex(games)

	g, ok := idx.GameByID("196609020mia")
	if !ok {
		t.Fatal("should find game by ID")
	}
	if g.AwayTeam != "Oakland Raiders" {
		t.Errorf("AwayTeam = %q, want Oakland Raiders", g.AwayTeam)
	}

	_, ok = idx.GameByID("nonexistent")
	if ok {
		t.Error("should not find nonexistent game")
	}
}

func TestMatchIndex_GamesByDate(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders"},
		{GameID: "202309100mia", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Los Angeles Chargers"},
	}
	idx := NewMatchIndex(games)

	found := idx.GamesByDate("1966-09-02")
	if len(found) != 1 {
		t.Errorf("GamesByDate(1966-09-02) = %d games, want 1", len(found))
	}

	found = idx.GamesByDate("2025-01-01")
	if len(found) != 0 {
		t.Errorf("GamesByDate(2025-01-01) = %d games, want 0", len(found))
	}
}

func TestMatchIndex_GamesByYear(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders"},
		{GameID: "196609090mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "New York Jets"},
		{GameID: "202309100mia", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Los Angeles Chargers"},
	}
	idx := NewMatchIndex(games)

	y66 := idx.GamesByYear(1966)
	if len(y66) != 2 {
		t.Errorf("1966 games = %d, want 2", len(y66))
	}

	y23 := idx.GamesByYear(2023)
	if len(y23) != 1 {
		t.Errorf("2023 games = %d, want 1", len(y23))
	}

	y00 := idx.GamesByYear(2000)
	if len(y00) != 0 {
		t.Errorf("2000 games = %d, want 0", len(y00))
	}
}

func TestMatchIndex_GamesNeedingVideo(t *testing.T) {
	games := []GameRef{
		{GameID: "game1", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "New York Jets"},
		{GameID: "game2", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Buffalo Bills"},
		{GameID: "game3", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "New England Patriots"},
		{GameID: "game4", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Denver Broncos"},
		{GameID: "game5", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Dallas Cowboys"},
	}
	idx := NewMatchIndex(games)

	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"game1": {Status: StatusSelected, Videos: []Video{{ID: "v1"}}},
			"game2": {Status: StatusCandidate, Videos: []Video{{ID: "v2"}}},
			"game4": {Status: StatusApproved},
			"game5": {Status: StatusNone},
		},
	}

	need := idx.GamesNeedingVideo(cf)
	if len(need) != 2 {
		t.Errorf("GamesNeedingVideo = %d, want 2", len(need))
	}
	if len(need) == 2 && (need[0] != "game3" || need[1] != "game4") {
		t.Errorf("GamesNeedingVideo = %v, want [game3 game4]", need)
	}
}

func TestMatchIndex_EmptyGames(t *testing.T) {
	idx := NewMatchIndex(nil)
	if idx.TotalGames() != 0 {
		t.Errorf("TotalGames() = %d, want 0", idx.TotalGames())
	}
	if len(idx.AllGameIDs()) != 0 {
		t.Error("AllGameIDs should be empty")
	}
}

func TestScoreVideoMatchForGame(t *testing.T) {
	game := GameRef{
		GameID: "196609020mia", Year: 1966,
		HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders",
	}

	tests := []struct {
		name     string
		text     string
		wantMin  int
		wantZero bool
	}{
		{
			name:    "both teams mentioned",
			text:    "Miami Dolphins vs Oakland Raiders 1966",
			wantMin: 50,
		},
		{
			name:    "both teams with date",
			text:    "Oakland Raiders at Miami Dolphins 1966-09-02",
			wantMin: 70,
		},
		{
			name:     "no teams mentioned",
			text:     "random football video 1966",
			wantZero: true,
		},
		{
			name:     "only one team",
			text:     "Miami Dolphins highlights 1966",
			wantZero: false, // Single team gets 10 points
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreVideoMatchForGame(tt.text, game)
			if tt.wantZero && score != 0 {
				t.Errorf("score = %d, want 0", score)
			}
			if tt.wantMin > 0 && score < tt.wantMin {
				t.Errorf("score = %d, want >= %d", score, tt.wantMin)
			}
		})
	}
}

func TestMatchVideoToGame_SingleDateMatch(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
		{GameID: "196609090mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "New York Jets", DateRaw: "September 9, 1966"},
	}
	idx := NewMatchIndex(games)

	// Title contains the ISO date 1966-09-02 and both teams
	game, score, ok := idx.MatchVideoToGame(
		"Oakland Raiders vs Miami Dolphins 1966-09-02 Full Game",
		"Complete AFL game from September 2 1966", 10,
	)
	if !ok {
		t.Fatal("expected match")
	}
	if game.GameID != "196609020mia" {
		t.Errorf("matched game = %q, want 196609020mia", game.GameID)
	}
	if score < 50 { // both teams + date match
		t.Errorf("score = %d, want >= 50", score)
	}
}

func TestMatchVideoToGame_YearFallback(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
		{GameID: "202309100mia", Year: 2023, HomeTeam: "Miami Dolphins", AwayTeam: "Los Angeles Chargers", DateRaw: "September 10, 2023"},
	}
	idx := NewMatchIndex(games)

	// No ISO date but has year, should match via year-narrowed scan
	game, score, ok := idx.MatchVideoToGame(
		"Oakland Raiders vs Miami Dolphins 1966 AFL Highlights",
		"Full football action", 10,
	)
	if !ok {
		t.Fatal("expected match")
	}
	if game.GameID != "196609020mia" {
		t.Errorf("matched game = %q, want 196609020mia", game.GameID)
	}
	if score < 10 {
		t.Errorf("score = %d, want >= 10", score)
	}
}

func TestMatchVideoToGame_NoMatch(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders"},
	}
	idx := NewMatchIndex(games)

	_, _, ok := idx.MatchVideoToGame("Random cat video", "Nothing football related", 10)
	if ok {
		t.Error("expected no match for irrelevant video")
	}
}

func TestMatchVideoToGame_BelowMinScore(t *testing.T) {
	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders"},
	}
	idx := NewMatchIndex(games)

	// Only one team mentioned → score ~10-20, set minScore very high
	_, _, ok := idx.MatchVideoToGame("Miami Dolphins 1966 highlights", "Season overview", 100)
	if ok {
		t.Error("expected no match when score below minScore")
	}
}

func Test_teamYearKey(t *testing.T) {
	tests := []struct {
		team string
		year int
		want string
	}{
		{"Miami Dolphins", 1966, "miami dolphins-1966"},
		{"", 1966, ""},
		{"  ", 1966, ""},
	}
	for _, tt := range tests {
		got := teamYearKey(tt.team, tt.year)
		if got != tt.want {
			t.Errorf("teamYearKey(%q, %d) = %q, want %q", tt.team, tt.year, got, tt.want)
		}
	}
}
