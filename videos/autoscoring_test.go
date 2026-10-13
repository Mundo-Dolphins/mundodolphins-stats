package videos

import (
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestBuildPrimaryQuery(t *testing.T) {
	meta := GameMetadata{
		GameID:  "196909140cin",
		DateISO: "1969-09-14",
		Year:    1969,
		Home:    LookupTeam("Cincinnati Bengals"),
		Away:    LookupTeam("Miami Dolphins"),
	}
	got := BuildPrimaryQuery(meta)
	want := "Miami Dolphins - Cincinnati Bengals 1969-09-14"
	if got != want {
		t.Errorf("BuildPrimaryQuery() = %q, want %q", got, want)
	}
}

func TestBuildFallbackQueries(t *testing.T) {
	meta := GameMetadata{
		GameID:  "196909140cin",
		DateISO: "1969-09-14",
		Year:    1969,
		Home:    LookupTeam("Cincinnati Bengals"),
		Away:    LookupTeam("Miami Dolphins"),
	}
	queries := BuildFallbackQueries(meta)
	if len(queries) != 3 {
		t.Fatalf("expected 3 fallback queries, got %d", len(queries))
	}
	if queries[0] != "Miami Dolphins vs Cincinnati Bengals 1969-09-14" {
		t.Errorf("fallback[0] = %q", queries[0])
	}
	if queries[1] != "Dolphins vs Bengals 1969" {
		t.Errorf("fallback[1] = %q", queries[1])
	}
	if queries[2] != "Miami vs Cincinnati 1969" {
		t.Errorf("fallback[2] = %q", queries[2])
	}
}

func TestContainsTeamReference(t *testing.T) {
	dolphins := LookupTeam("Miami Dolphins")
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"full name", "Miami Dolphins game 1969", true},
		{"city only", "Miami plays today", true},
		{"mascot only", "Dolphins win big", true},
		{"abbr only", "1966-11-13 MIA at KC", true},
		{"case insensitive", "MIAMI DOLPHINS", true},
		{"no match", "New England Patriots game", false},
		{"abbr not inside word", "Miasma around the stadium", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsTeamReference(tt.text, dolphins)
			if got != tt.want {
				t.Errorf("ContainsTeamReference(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestContainsBothTeams(t *testing.T) {
	dolphins := LookupTeam("Miami Dolphins")
	bengals := LookupTeam("Cincinnati Bengals")
	tests := []struct {
		name  string
		title string
		desc  string
		want  bool
	}{
		{"both in title", "Miami Dolphins vs Cincinnati Bengals", "", true},
		{"split title+desc", "Dolphins game", "at Cincinnati", true},
		{"only one team", "Miami Dolphins highlights", "great game", false},
		{"neither", "NFL Game 1969", "", false},
		{"city refs", "Miami at Cincinnati", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsBothTeams(tt.title, tt.desc, bengals, dolphins)
			if got != tt.want {
				t.Errorf("ContainsBothTeams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsBothTeamsWithCommonAbbreviations(t *testing.T) {
	dolphins := LookupTeam("Miami Dolphins")
	chiefs := LookupTeam("Kansas City Chiefs")
	raiders := LookupTeam("Oakland Raiders")
	if !ContainsBothTeams("1966-11-13 MIA at KC", "", chiefs, dolphins) {
		t.Fatalf("expected MIA/KC to match Dolphins/Chiefs")
	}
	if !ContainsBothTeams("1966-09-02 RAID at MIA", "", dolphins, raiders) {
		t.Fatalf("expected RAID/MIA to match Raiders/Dolphins")
	}
	if !ContainsBothTeams("Miami vs Kansas City", "", chiefs, dolphins) {
		t.Fatalf("expected city names to match Dolphins/Chiefs")
	}
	if !ContainsBothTeams("Dolphins vs Chiefs", "", chiefs, dolphins) {
		t.Fatalf("expected mascots to match Dolphins/Chiefs")
	}
}

func TestDetectTeamOrder(t *testing.T) {
	dolphins := LookupTeam("Miami Dolphins")
	bengals := LookupTeam("Cincinnati Bengals")
	tests := []struct {
		name string
		text string
		want TeamOrderMatch
	}{
		{"correct order", "Miami Dolphins vs Cincinnati Bengals", TeamOrderCorrect},
		{"reversed order", "Cincinnati Bengals vs Miami Dolphins", TeamOrderReverse},
		{"partial away first", "Dolphins at Bengals", TeamOrderCorrect},
		{"only one team", "Miami Dolphins highlight reel", TeamOrderNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTeamOrder(tt.text, bengals, dolphins)
			if got != tt.want {
				t.Errorf("DetectTeamOrder(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectDateMatch(t *testing.T) {
	gameDate := time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		text string
		want DateMatchType
	}{
		{"iso date", "Game from 1969-09-14 was great", DateMatchFull},
		{"slash date", "Game 1969/09/14 classic", DateMatchFull},
		{"month name full", "September 14, 1969 game", DateMatchFull},
		{"month name short", "Sep 14, 1969 classic", DateMatchFull},
		{"day month year", "14 sep 1969 football", DateMatchFull},
		{"long month date", "Miami Dolphins vs San Diego Chargers | August 6, 1966", DateMatchWrong},
		{"year only", "Great game from 1969 season", DateMatchYearOnly},
		{"wrong month day same year", "Game from October 2, 1969", DateMatchWrong},
		{"wrong year", "This happened in 1970", DateMatchWrong},
		{"no date", "football game dolphins bengals", DateMatchNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDateMatch(tt.text, gameDate)
			if got != tt.want {
				t.Errorf("DetectDateMatch(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectDateMatchLongMonthDateExact(t *testing.T) {
	gameDate := time.Date(1966, 8, 6, 0, 0, 0, 0, time.UTC)
	got := DetectDateMatch("Miami Dolphins vs San Diego Chargers | August 6, 1966 | Dolphins first game ever", gameDate)
	if got != DateMatchFull {
		t.Fatalf("expected long month date to match exactly, got %q", got)
	}
}

func TestDetectDateMatchNumericBroadcastDates(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		gameDate time.Time
		want     DateMatchType
	}{
		{
			name:     "spaced year month day with trailing broadcast year",
			text:     "NFL 1980 09 28 80 New Orleans Saints at Miami Dolphins pt 1",
			gameDate: time.Date(1980, 9, 28, 0, 0, 0, 0, time.UTC),
			want:     DateMatchFull,
		},
		{
			name:     "hyphenated year month day short year",
			text:     "NFL 1980 12-08-80 New England Patriots at Miami Dolphins pt 1 of 2",
			gameDate: time.Date(1980, 12, 8, 0, 0, 0, 0, time.UTC),
			want:     DateMatchFull,
		},
		{
			name:     "month day short year",
			text:     "1-4-86 Cleveland Browns at Miami Dolphins AFC Divisional",
			gameDate: time.Date(1986, 1, 4, 0, 0, 0, 0, time.UTC),
			want:     DateMatchFull,
		},
		{
			name:     "month day full year with spaces",
			text:     "11 24 1977 Miami Dolphins at St Louis Cardinals",
			gameDate: time.Date(1977, 11, 24, 0, 0, 0, 0, time.UTC),
			want:     DateMatchFull,
		},
		{
			name:     "modern month day short year",
			text:     "09-12-24 Buffalo Bills at Miami Dolphins TNF",
			gameDate: time.Date(2024, 9, 12, 0, 0, 0, 0, time.UTC),
			want:     DateMatchFull,
		},
		{
			name:     "wrong numeric month day same year",
			text:     "NFL 1980 09 29 80 New Orleans Saints at Miami Dolphins pt 1",
			gameDate: time.Date(1980, 9, 28, 0, 0, 0, 0, time.UTC),
			want:     DateMatchWrong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectDateMatch(tt.text, tt.gameDate)
			if got != tt.want {
				t.Fatalf("DetectDateMatch(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectGameDateMatchAllowsSeasonYearForJanuaryRegularSeason(t *testing.T) {
	game := GameMetadata{
		GameID:   "202601040mia",
		DateISO:  "2026-01-04",
		GameDate: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
		Year:     2025,
	}
	got := DetectGameDateMatch("Miami Dolphins vs New England Patriots 2025 Week 18", game)
	if got != DateMatchYearOnly {
		t.Fatalf("DetectGameDateMatch() = %q, want %q", got, DateMatchYearOnly)
	}
}

func TestDetectGameDateMatchKeepsWrongFullDateWrong(t *testing.T) {
	game := GameMetadata{
		GameID:   "202601040mia",
		DateISO:  "2026-01-04",
		GameDate: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
		Year:     2025,
	}
	got := DetectGameDateMatch("Miami Dolphins vs New England Patriots 2025-12-28", game)
	if got != DateMatchWrong {
		t.Fatalf("DetectGameDateMatch() = %q, want %q", got, DateMatchWrong)
	}
}

func TestScoreCandidate_ValidCandidate(t *testing.T) {
	meta := GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "abc123",
		Title:       "Miami Dolphins vs Cincinnati Bengals Full Game 1969-09-14",
		Description: "Classic NFL game from 1969",
		Duration:    2*time.Hour + 30*time.Minute,
		Embeddable:  true,
		ViewCount:   150000,
	}
	result := ScoreCandidate(meta, candidate, 300)

	if result.Discarded {
		t.Fatalf("expected not discarded, got discarded: %s", result.DiscardReason)
	}
	if !result.MatchedTeams {
		t.Error("expected MatchedTeams=true")
	}
	if result.Score.TeamsMatch != 50 {
		t.Errorf("TeamsMatch = %d, want 50", result.Score.TeamsMatch)
	}
	if result.Score.DateFull != 30 {
		t.Errorf("DateFull = %d, want 30", result.Score.DateFull)
	}
	if result.Score.Duration != 15 {
		t.Errorf("Duration = %d, want 15 (>=2h)", result.Score.Duration)
	}
	if result.Score.Embeddable != 5 {
		t.Errorf("Embeddable = %d, want 5", result.Score.Embeddable)
	}
	if result.Score.Views == 0 {
		t.Error("Views should be > 0 for 150K views")
	}
	if result.Score.ContentType == 0 {
		t.Error("ContentType should be > 0 for 'full game'")
	}
	if result.Score.Total == 0 {
		t.Error("Total score should not be 0")
	}
}

func TestScoreCandidateRejectsBroadcastWeekdayMismatch(t *testing.T) {
	meta := GameMetadata{
		GameID:   "198312040mia",
		DateISO:  "1983-12-04",
		GameDate: time.Date(1983, 12, 4, 0, 0, 0, 0, time.UTC),
		Year:     1983,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("New York Jets"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "mnf-wrong-day",
		Title:       "New York Jets at Miami Dolphins 1983 Monday Night Football",
		Description: "Full broadcast.",
		Duration:    2 * time.Hour,
	}

	got := ScoreCandidate(meta, candidate, 300)
	if !got.Discarded || got.DiscardReason != "broadcast weekday does not match game date" {
		t.Fatalf("expected broadcast weekday mismatch discard, got discarded=%v reason=%q", got.Discarded, got.DiscardReason)
	}
}

func TestScoreCandidate_PreviewFalsePositiveIsRejectedByHardFilter(t *testing.T) {
	meta := GameMetadata{
		GameID:   "202409120mia",
		DateISO:  "2024-09-12",
		GameDate: time.Date(2024, 9, 12, 0, 0, 0, 0, time.UTC),
		Year:     2024,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Buffalo Bills"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "preview1",
		Title:       "Buffalo Bills vs Miami Dolphins Week Preview and Predictions",
		Description: "Betting odds and matchup preview for Thursday night",
		Duration:    30 * time.Minute,
		Embeddable:  true,
		ViewCount:   100000,
	}
	filter := IsValidCandidate(candidate, meta, 300)
	if filter.Accepted {
		t.Fatalf("expected preview false positive rejected")
	}
	if !strings.Contains(filter.Reason, "preview") {
		t.Fatalf("expected preview reason, got %q", filter.Reason)
	}
	score := scoreRiskyText(candidate.Title + " " + candidate.Description)
	if score > -80 {
		t.Fatalf("expected strong risky text penalty, got %d", score)
	}
}

func TestScoreCandidate_ClearFullGameHighConfidence(t *testing.T) {
	meta := GameMetadata{
		GameID:   "202409120mia",
		DateISO:  "2024-09-12",
		GameDate: time.Date(2024, 9, 12, 0, 0, 0, 0, time.UTC),
		Year:     2024,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Buffalo Bills"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "fullgame1",
		Title:       "Buffalo Bills vs Miami Dolphins Full Game 2024-09-12",
		Description: "Complete NFL game broadcast from September 12, 2024",
		Channel:     "NFL Throwback",
		Duration:    2*time.Hour + 20*time.Minute,
		Embeddable:  true,
		ViewCount:   900000,
	}
	result := ScoreCandidate(meta, candidate, 300)
	if result.Discarded {
		t.Fatalf("expected good full game accepted: %s", result.DiscardReason)
	}
	if result.Score.TrustedChannel != 10 {
		t.Fatalf("trusted channel bonus=%d want 10", result.Score.TrustedChannel)
	}
	if result.Score.Total < 130 {
		t.Fatalf("score=%d want >=130", result.Score.Total)
	}
	if confidenceFromScore(result.Score.Total) != "high" {
		t.Fatalf("expected high confidence for score=%d", result.Score.Total)
	}
}

func TestScoreCandidate_DiscardSingleTeam(t *testing.T) {
	meta := GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "single1",
		Title:       "Miami Dolphins Season Highlights 1969",
		Description: "All games from the season",
		Duration:    10 * time.Minute,
		Embeddable:  true,
		ViewCount:   50000,
	}
	result := ScoreCandidate(meta, candidate, 300)
	if !result.Discarded {
		t.Error("expected discarded when only one team is referenced")
	}
	if result.DiscardReason != "missing team reference" {
		t.Errorf("unexpected discard reason: %s", result.DiscardReason)
	}
}

func TestScoreCandidate_DiscardWrongYear(t *testing.T) {
	meta := GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "wrong_year",
		Title:       "Dolphins vs Bengals 1970 Classic",
		Description: "Cincinnati hosts Miami in 1970",
		Duration:    1 * time.Hour,
		Embeddable:  true,
		ViewCount:   100000,
	}
	result := ScoreCandidate(meta, candidate, 300)
	if !result.Discarded {
		t.Error("expected discarded when year is wrong")
	}
}

func TestScoreCandidate_DiscardTooShort(t *testing.T) {
	meta := GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "tooshort",
		Title:       "Dolphins at Bengals 1969-09-14",
		Description: "",
		Duration:    2 * time.Minute,
		Embeddable:  true,
		ViewCount:   5000,
	}
	result := ScoreCandidate(meta, candidate, 300)
	if !result.Discarded {
		t.Error("expected discarded when duration < minimum")
	}
}

func TestScoreCandidate_NotEmbeddable(t *testing.T) {
	meta := GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "noembed",
		Title:       "Dolphins vs Bengals 1969",
		Description: "Cincinnati",
		Duration:    1 * time.Hour,
		Embeddable:  false,
		ViewCount:   10000,
	}
	result := ScoreCandidate(meta, candidate, 300)
	if result.Discarded {
		t.Error("non-embeddable should not be auto-discarded")
	}
	if result.Score.Embeddable != 0 {
		t.Errorf("Embeddable score should be 0 for non-embeddable, got %d", result.Score.Embeddable)
	}
}

func TestScoreCandidate_HighViewsWrongDate(t *testing.T) {
	meta := GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "highviews",
		Title:       "Dolphins vs Bengals 1975 Epic Game",
		Description: "Cincinnati Bengal vs Miami",
		Duration:    3 * time.Hour,
		Embeddable:  true,
		ViewCount:   5000000,
	}
	result := ScoreCandidate(meta, candidate, 300)
	if !result.Discarded {
		t.Error("expected discarded: high views but wrong year should still be discarded")
	}
}

func TestSelectBestCandidate(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Candidate: youtubeapi.VideoCandidate{ID: "low", Duration: 10 * time.Minute, Embeddable: true},
			Score:     ScoreBreakdown{Total: 50},
		},
		{
			Candidate: youtubeapi.VideoCandidate{ID: "high", Duration: 2 * time.Hour, Embeddable: true, ViewCount: 100000},
			Score:     ScoreBreakdown{Total: 120},
		},
		{
			Candidate:     youtubeapi.VideoCandidate{ID: "discarded"},
			Score:         ScoreBreakdown{Total: 200},
			Discarded:     true,
			DiscardReason: "wrong year",
		},
	}
	best := SelectBestCandidate(candidates)
	if best == nil {
		t.Fatal("expected non-nil best candidate")
	}
	if best.Candidate.ID != "high" {
		t.Errorf("expected best=high, got %s", best.Candidate.ID)
	}
}

func TestSelectBestCandidate_TiebreakEmbeddable(t *testing.T) {
	candidates := []ScoredCandidate{
		{
			Candidate: youtubeapi.VideoCandidate{ID: "noembed", Duration: 1 * time.Hour, Embeddable: false},
			Score:     ScoreBreakdown{Total: 100},
		},
		{
			Candidate: youtubeapi.VideoCandidate{ID: "embed", Duration: 1 * time.Hour, Embeddable: true},
			Score:     ScoreBreakdown{Total: 100},
		},
	}
	best := SelectBestCandidate(candidates)
	if best == nil {
		t.Fatal("expected non-nil")
	}
	if best.Candidate.ID != "embed" {
		t.Errorf("expected embeddable to win tiebreak, got %s", best.Candidate.ID)
	}
}

func TestSelectBestCandidate_AllDiscarded(t *testing.T) {
	candidates := []ScoredCandidate{
		{Discarded: true, DiscardReason: "reason1"},
		{Discarded: true, DiscardReason: "reason2"},
	}
	best := SelectBestCandidate(candidates)
	if best != nil {
		t.Error("expected nil when all discarded")
	}
}

func TestSelectBestCandidate_Empty(t *testing.T) {
	best := SelectBestCandidate(nil)
	if best != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Miami Dolphins (1969)", "miami dolphins 1969"},
		{"  HELLO World! ", "hello world"},
		{"no-special_chars.here", "nospecialcharshere"},
	}
	for _, tt := range tests {
		got := NormalizeText(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractGameMetadata(t *testing.T) {
	game := GameRef{
		GameID:   "196909140cin",
		Year:     1969,
		HomeTeam: "Cincinnati Bengals",
		AwayTeam: "Miami Dolphins",
	}
	meta := ExtractGameMetadata(game)
	if meta.Home.FullName != "Cincinnati Bengals" {
		t.Errorf("Home = %q", meta.Home.FullName)
	}
	if meta.Away.FullName != "Miami Dolphins" {
		t.Errorf("Away = %q", meta.Away.FullName)
	}
	if meta.DateISO != "1969-09-14" {
		t.Errorf("DateISO = %q", meta.DateISO)
	}
	if meta.Home.Mascot != "Bengals" {
		t.Errorf("Home.Mascot = %q", meta.Home.Mascot)
	}
}

func TestScoreDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    int
	}{
		{7200, 15},
		{3600, 12},
		{1800, 9},
		{900, 6},
		{600, 3},
		{300, 0},
		{100, 0},
	}
	for _, tt := range tests {
		got := scoreDuration(tt.seconds)
		if got != tt.want {
			t.Errorf("scoreDuration(%d) = %d, want %d", tt.seconds, got, tt.want)
		}
	}
}

func TestScoreViews(t *testing.T) {
	tests := []struct {
		views uint64
		want  int
	}{
		{0, 0},
		{50, 0},
		{100, 1},
		{1000, 2},
		{10000, 3},
		{100000, 4},
		{1000000, 5},
	}
	for _, tt := range tests {
		got := scoreViews(tt.views)
		if got != tt.want {
			t.Errorf("scoreViews(%d) = %d, want %d", tt.views, got, tt.want)
		}
	}
}

func TestLookupTeam(t *testing.T) {
	tests := []struct {
		name   string
		mascot string
		city   string
	}{
		{"Miami Dolphins", "Dolphins", "Miami"},
		{"Oakland Raiders", "Raiders", "Oakland"},
		{"Houston Oilers", "Oilers", "Houston"},
		{"Boston Patriots", "Patriots", "Boston"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := LookupTeam(tt.name)
			if a.Mascot != tt.mascot {
				t.Errorf("Mascot = %q, want %q", a.Mascot, tt.mascot)
			}
			if a.City != tt.city {
				t.Errorf("City = %q, want %q", a.City, tt.city)
			}
		})
	}
}

func TestLookupTeam_Unknown(t *testing.T) {
	a := LookupTeam("Gotham Rogues")
	if a.FullName != "Gotham Rogues" {
		t.Errorf("FullName = %q", a.FullName)
	}
	if a.City != "Gotham" {
		t.Errorf("City = %q, want Gotham", a.City)
	}
	if a.Mascot != "Rogues" {
		t.Errorf("Mascot = %q, want Rogues", a.Mascot)
	}
}

func TestScoreContentType(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"full game keyword", "Full Game Highlights", 15},
		{"full match keyword", "Full Match Miami vs Buffalo", 15},
		{"condensed keyword", "Condensed Game Miami Dolphins", 10},
		{"game film keyword", "Game Film Week 1", 10},
		{"broadcast keyword", "NFL Broadcast Week 1", 10},
		{"broadcast part marker", "NFL 1980 09 28 80 Saints at Dolphins pt 1", 10},
		{"playoff broadcast marker", "1984 AFC Div. Playoffs Seahawks at Dolphins", 10},
		{"multiple keywords capped at 15", "Full Game Condensed Version", 15},
		{"all keywords capped at 15", "Full Game Game Film Broadcast", 15},
		{"no matching keywords", "Some Random Video Title", 0},
		{"case insensitive", "FULL GAME HIGHLIGHTS", 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreContentType(tt.text)
			if got != tt.want {
				t.Errorf("scoreContentType(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}
