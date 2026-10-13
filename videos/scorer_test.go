package videos

import (
	"strings"
	"testing"
)

func TestScorerScore(t *testing.T) {
	game := GameRef{GameID: "196612180mia", AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins", Year: 1966}
	scorer := NewScorer(3000, 60)
	c := Candidate{Title: "Houston Oilers vs Miami Dolphins 1966 Full Game", DurationSec: 4000}
	if scorer.IsDisallowed(c) {
		t.Fatalf("unexpected disallowed")
	}
	score := scorer.Score(game, c)
	if score < 60 {
		t.Fatalf("expected score >= 60, got %d", score)
	}

	bad := Candidate{Title: "Highlights: Oilers vs Dolphins", DurationSec: 500}
	if scorer.IsDisallowed(bad) {
		t.Fatalf("expected highlights to remain reviewable, not hard-disallowed")
	}
	eval := scorer.Evaluate(game, bad)
	if eval.Decision != DecisionReject || eval.Reason != "duration" {
		t.Fatalf("expected short highlight rejected by duration, got %+v", eval)
	}
}

func TestScorerScorePenalizesWrongYearInDescription(t *testing.T) {
	game := GameRef{
		GameID:   "196710080kan",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "Kansas City Chiefs",
		Year:     1967,
	}
	scorer := NewScorer(300, 60)

	candidate := Candidate{
		Title:       "1991 Chiefs vs Dolphins - Week 7 - Full Game",
		Snippet:     "The Chiefs look for revenge against Dan Marino and the Dolphins in a rematch of the 1990 playoff loss.",
		DurationSec: 7378,
	}

	score := scorer.Score(game, candidate)
	if score >= 0 {
		t.Fatalf("expected wrong years in title/description to dominate positive signals, got %d", score)
	}
}

func TestScorerScoreRewardsExactDateOverWrongYear(t *testing.T) {
	game := GameRef{
		GameID:   "196710080kan",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "Kansas City Chiefs",
		Year:     1967,
	}
	scorer := NewScorer(300, 60)
	exact := Candidate{
		Title:       "Miami Dolphins vs Kansas City Chiefs 1967-10-08 Full Game",
		Snippet:     "Broadcast copy of the October 8, 1967 AFL game.",
		DurationSec: 7200,
	}
	wrong := Candidate{
		Title:       "Miami Dolphins vs Kansas City Chiefs 2024 AFC Wild Card",
		Snippet:     "Tua Tagovailoa and the Dolphins take on Patrick Mahomes and the Chiefs.",
		DurationSec: 5227,
	}

	exactScore := scorer.Score(game, exact)
	wrongScore := scorer.Score(game, wrong)
	if exactScore <= wrongScore {
		t.Fatalf("expected exact date score %d to beat wrong year score %d", exactScore, wrongScore)
	}
	if wrongScore >= 0 {
		t.Fatalf("expected wrong year score to be negative, got %d", wrongScore)
	}
}

func TestScorerRejectsShortYearDateFromDifferentGameDespiteMatchingWeek(t *testing.T) {
	game := GameRef{
		GameID:   "199612160mia",
		AwayTeam: "Buffalo Bills",
		HomeTeam: "Miami Dolphins",
		Year:     1996,
		Week:     "16",
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "12-24-16 Dolphins@Bills 2of2 Week 16",
		DurationSec: 3600,
	}

	eval := scorer.Evaluate(game, candidate)
	if eval.Decision != DecisionReject || eval.Reason != "wrong_date" {
		t.Fatalf("expected short-year date mismatch to be rejected as wrong_date, got %+v", eval)
	}
	if eval.DateMatch != DateMatchWrong || eval.WeekMatch != "" {
		t.Fatalf("expected rejection before week can rescue candidate, got %+v", eval)
	}

	filtered := filterByLegacyRelevance(game, []Candidate{candidate}, scorer)
	if len(filtered) != 0 {
		t.Fatalf("expected wrong-date candidate filtered out, got %+v", filtered)
	}
}

func TestScorerRejectsWrongOpponentTitleEvenWhenSnippetMatchesQuery(t *testing.T) {
	game := GameRef{
		GameID:   "200208150mia",
		AwayTeam: "New Orleans Saints",
		HomeTeam: "Miami Dolphins",
		Year:     2002,
		Week:     "Preseason 2",
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "2002-03 W09 Miami Dolphins 10 @ Green Bay Packers 24",
		Snippet:     "New Orleans Saints at Miami Dolphins preseason 2002",
		DurationSec: 9020,
	}

	eval := scorer.Evaluate(game, candidate)
	if eval.Decision != DecisionReject || eval.Reason != "wrong_opponent" {
		t.Fatalf("expected explicit Packers title to be rejected as wrong_opponent, got %+v", eval)
	}
	if got := legacyWeekMatch(game, strings.ToLower(candidate.Title)); got != DateMatchWrong {
		t.Fatalf("legacyWeekMatch()=%s want=%s", got, DateMatchWrong)
	}

	filtered := filterByLegacyRelevance(game, []Candidate{candidate}, scorer)
	if len(filtered) != 0 {
		t.Fatalf("expected wrong-opponent candidate filtered out, got %+v", filtered)
	}
}

func TestScorerScoreTreatsTeamAbbreviationsAsTeamMatch(t *testing.T) {
	game := GameRef{
		GameID:   "196611130kan",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "Kansas City Chiefs",
		Year:     1966,
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:   "1966-11-13 MIA at KC",
		Snippet: "A second quarter video with summaries for the other three quarters added along with all new narration and other content.",
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected exact date plus MIA/KC aliases to reach threshold, got %d", score)
	}
}

func TestScorerScoreTreatsRaidAsRaidersAlias(t *testing.T) {
	game := GameRef{
		GameID:   "196609020mia",
		AwayTeam: "Oakland Raiders",
		HomeTeam: "Miami Dolphins",
		Year:     1966,
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "1966-09-02 RAID at MIA",
		Channel:     "Vintage Pro Football",
		DurationSec: 2668,
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected exact date plus RAID/MIA aliases to reach threshold, got %d", score)
	}
}

func TestScorerScoreRecognizesLongMonthDate(t *testing.T) {
	game := GameRef{
		GameID:   "196608060sdg",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "San Diego Chargers",
		Year:     1966,
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "Miami Dolphins vs San Diego Chargers | August 6, 1966 | Dolphins first game ever",
		DurationSec: 1134,
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected full team names and long month date to reach threshold, got %d", score)
	}
}

func TestScorerScoreRecognizesNumericBroadcastPartDate(t *testing.T) {
	game := GameRef{
		GameID:   "198009280mia",
		AwayTeam: "New Orleans Saints",
		HomeTeam: "Miami Dolphins",
		Year:     1980,
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "NFL 1980 09 28 80 New Orleans Saints at Miami Dolphins pt 1",
		DurationSec: 3586,
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected numeric broadcast title to reach threshold, got %d", score)
	}
}

func TestScorerScoreRecognizesPlayoffBroadcastSignals(t *testing.T) {
	game := GameRef{
		GameID:   "198412290mia",
		AwayTeam: "Seattle Seahawks",
		HomeTeam: "Miami Dolphins",
		Year:     1984,
		Week:     "Playoffs",
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "Seattle Seahawks vs Miami Dolphins 1984 AFC Div. Playoffs",
		DurationSec: 9231,
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected playoff broadcast signals to reach threshold, got %d", score)
	}
}

func TestLegacyWeekMatchRecognizesPlayoffRoundTerms(t *testing.T) {
	game := GameRef{Week: "Playoffs"}
	tests := []string{
		"Miami Dolphins vs Kansas City Chiefs AFC Divisional playoff",
		"Miami Dolphins vs New England Patriots AFC Wild Card Full Game",
		"1984 AFC CHAMPIONSHIP GAME",
		"Super Bowl VII Dolphins vs Redskins",
	}
	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			if got := legacyWeekMatch(game, text); got != DateMatchFull {
				t.Fatalf("legacyWeekMatch(%q) = %q, want %q", text, got, DateMatchFull)
			}
		})
	}
}

func TestLegacyWeekMatchRejectsRegularWeekForPlayoffGame(t *testing.T) {
	game := GameRef{Week: "Playoffs"}
	got := legacyWeekMatch(game, "Miami Dolphins vs Buffalo Bills 2024 Week 9")
	if got != DateMatchWrong {
		t.Fatalf("legacyWeekMatch() = %q, want %q", got, DateMatchWrong)
	}
}

func TestScorerScoreAllowsSeasonYearForJanuaryPlayoffGame(t *testing.T) {
	game := GameRef{
		GameID:   "198301080mia",
		AwayTeam: "New England Patriots",
		HomeTeam: "Miami Dolphins",
		Year:     1982,
		Week:     "Playoffs",
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		VideoID:     "1ji3BXj4jeY",
		Title:       "New England Patriots vs Miami Dolphins 1982 Wildcard Playoffs",
		DurationSec: 7345,
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected 1982 wildcard title to be viable for January 1983 playoff game, got %d", score)
	}
	if got := legacyDateMatch(game, strings.ToLower(candidate.Title)); got != DateMatchYearOnly {
		t.Fatalf("legacyDateMatch() = %q, want %q", got, DateMatchYearOnly)
	}
}

func TestFilterByLegacyRelevanceDropsPlayoffCandidateForRegularWeek(t *testing.T) {
	game := GameRef{
		GameID:   "198212120nwe",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "New England Patriots",
		Year:     1982,
		Week:     "14",
	}
	scorer := NewScorer(300, 60)
	candidates := []Candidate{
		{
			Title:       "New England Patriots vs Miami Dolphins 1982 Wildcard Playoffs",
			DurationSec: 7345,
		},
		{
			Title:       "Miami Dolphins at New England Patriots 1982 Week 14 Full Game",
			DurationSec: 7200,
		},
	}

	filtered := filterByLegacyRelevance(game, candidates, scorer)
	if len(filtered) != 1 {
		t.Fatalf("expected only regular-season candidate, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].Title != "Miami Dolphins at New England Patriots 1982 Week 14 Full Game" {
		t.Fatalf("unexpected candidate kept: %s", filtered[0].Title)
	}
}

func TestFilterByLegacyRelevanceDropsMNFForSundayGame(t *testing.T) {
	game := GameRef{
		GameID:   "198312040mia",
		AwayTeam: "New York Jets",
		HomeTeam: "Miami Dolphins",
		Year:     1983,
		Week:     "14",
	}
	scorer := NewScorer(300, 60)
	candidates := []Candidate{
		{
			Title:       "New York Jets at Miami Dolphins 1983 Monday Night Football",
			DurationSec: 7200,
		},
		{
			Title:       "New York Jets at Miami Dolphins 1983 Week 14 Full Game",
			DurationSec: 7200,
		},
	}

	filtered := filterByLegacyRelevance(game, candidates, scorer)
	if len(filtered) != 1 {
		t.Fatalf("expected only non-MNF candidate for Sunday game, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].Title != "New York Jets at Miami Dolphins 1983 Week 14 Full Game" {
		t.Fatalf("unexpected candidate kept: %s", filtered[0].Title)
	}
}

func TestFilterByLegacyRelevanceKeepsMNFForMondayGame(t *testing.T) {
	game := GameRef{
		GameID:   "201309300nor",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "New Orleans Saints",
		Year:     2013,
		Week:     "4",
	}
	scorer := NewScorer(300, 60)
	candidates := []Candidate{
		{
			Title:       "Miami Dolphins at New Orleans Saints 2013 MNF",
			DurationSec: 7200,
		},
	}

	filtered := filterByLegacyRelevance(game, candidates, scorer)
	if len(filtered) != 1 {
		t.Fatalf("expected MNF candidate for Monday game to remain viable, got %d: %+v", len(filtered), filtered)
	}
}

func TestRankCandidatesPrefersFullGameDurationOverShortMatchingClip(t *testing.T) {
	game := GameRef{
		GameID:   "198709130nwe",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "New England Patriots",
		Year:     1987,
		Week:     "1",
	}
	scorer := NewScorer(300, 60)
	candidates := []Candidate{
		{
			VideoID:      "p2jA_31fkdk",
			Title:        "Dolphins vs Patriots 1987 Week 1",
			DurationText: "5:02",
			Snippet:      "The First Ever NFL Primetime Highlight",
		},
		{
			VideoID:      "i_6kojza9Ak",
			Title:        "1987 week 1 Dolphins at Patriots",
			DurationText: "2:16:52",
			Snippet:      "Disfruta de los videos y la musica que te gustan.",
		},
	}

	ranked := rankCandidates(game, candidates, scorer)
	if len(ranked) != 2 {
		t.Fatalf("unexpected ranked length: %d", len(ranked))
	}
	if ranked[0].VideoID != "i_6kojza9Ak" {
		t.Fatalf("expected full game first, got %+v", ranked)
	}
	if ranked[0].Score != 75 || ranked[1].Score != 68 {
		t.Fatalf("unexpected scores: full game=%d short clip=%d", ranked[0].Score, ranked[1].Score)
	}
}

func TestScorerScoreRejectsDifferentLongMonthDateInSameYear(t *testing.T) {
	game := GameRef{
		GameID:   "196610020sdg",
		AwayTeam: "Miami Dolphins",
		HomeTeam: "San Diego Chargers",
		Year:     1966,
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "Miami Dolphins vs San Diego Chargers | August 6, 1966 | Dolphins first game ever",
		DurationSec: 1134,
	}

	score := scorer.Score(game, candidate)
	if score >= 0 {
		t.Fatalf("expected same-year date mismatch to be rejected, got %d", score)
	}
	exact := Candidate{
		Title:       "Miami Dolphins vs San Diego Chargers | October 2, 1966",
		DurationSec: 1134,
	}
	exactScore := scorer.Score(game, exact)
	if score >= exactScore {
		t.Fatalf("expected exact date score %d to beat same-year mismatch score %d", exactScore, score)
	}
}

func TestScorerScoreRecognizesMatchingWeek(t *testing.T) {
	game := GameRef{
		GameID:   "196811170mia",
		AwayTeam: "Cincinnati Bengals",
		HomeTeam: "Miami Dolphins",
		Year:     1968,
		Week:     "11",
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "Cincinnati Bengals 1968 Season Week 11 vs Dolphins",
		DurationSec: 301,
	}

	score := scorer.Score(game, candidate)
	if score < 60 {
		t.Fatalf("expected teams, year, and matching week to reach threshold, got %d", score)
	}
}

func TestScorerScorePenalizesWrongWeek(t *testing.T) {
	game := GameRef{
		GameID:   "196811170mia",
		AwayTeam: "Cincinnati Bengals",
		HomeTeam: "Miami Dolphins",
		Year:     1968,
		Week:     "11",
	}
	scorer := NewScorer(300, 60)
	candidate := Candidate{
		Title:       "Cincinnati Bengals 1968 Season Week 10 vs Dolphins",
		DurationSec: 301,
	}

	score := scorer.Score(game, candidate)
	if score >= 0 {
		t.Fatalf("expected wrong explicit week to dominate positive signals, got %d", score)
	}
}

func TestContainsTeam(t *testing.T) {
	if !containsTeam("miami dolphins", "Miami Dolphins") {
		t.Fatalf("expected match on full name")
	}
	if !containsTeam("dolphins vs oilers", "Miami Dolphins") {
		t.Fatalf("expected match on token")
	}
	if !containsTeam("1966-11-13 MIA at KC", "Miami Dolphins") {
		t.Fatalf("expected match on abbreviation")
	}
	if !containsTeam("1966-11-13 MIA at KC", "Kansas City Chiefs") {
		t.Fatalf("expected match on common city abbreviation")
	}
	if !containsTeam("1985 Week 7 Bucs at Dolphins Game of the Week", "Tampa Bay Buccaneers") {
		t.Fatalf("expected match on Bucs alias")
	}
	if !containsTeam("1974-11-10 MIA at NO", "New Orleans Saints") {
		t.Fatalf("expected match on New Orleans NO alias")
	}
	if !containsTeam("1968-11-03 MIA at SD", "San Diego Chargers") {
		t.Fatalf("expected match on San Diego SD alias")
	}
	if containsTeam("random", "Miami Dolphins") {
		t.Fatalf("expected no match")
	}
}

func TestStrconvItoa(t *testing.T) {
	if strconvItoa(1966) != "1966" {
		t.Fatalf("unexpected itoa")
	}
	if strconvItoa(-2) != "-2" {
		t.Fatalf("unexpected negative itoa")
	}
}
