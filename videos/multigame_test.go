package videos

import (
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// Helper to build a GameMetadata for the 1966-09-09 Jets @ Dolphins game.
func jetsAtDolphins1966() GameMetadata {
	return GameMetadata{
		GameID:   "196609090mia",
		DateISO:  "1966-09-09",
		GameDate: time.Date(1966, 9, 9, 0, 0, 0, 0, time.UTC),
		Year:     1966,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("New York Jets"),
	}
}

// Helper to build a GameMetadata for the 1967-09-17 Broncos @ Dolphins game.
func broncosAtDolphins1967() GameMetadata {
	return GameMetadata{
		GameID:   "196709170mia",
		DateISO:  "1967-09-17",
		GameDate: time.Date(1967, 9, 17, 0, 0, 0, 0, time.UTC),
		Year:     1967,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Denver Broncos"),
	}
}

// --- Case 1: Season compilation video with 10+ games listed ---

const seasonCompilationDescription = `1966 New York Jets Season - All Game Highlights

1966-09-09 New York Jets @ Miami Dolphins
1966-09-18 Houston Oilers @ New York Jets
1966-09-25 New York Jets @ Denver Broncos
1966-10-02 New York Jets @ Oakland Raiders
1966-10-09 San Diego Chargers @ New York Jets
1966-10-16 New York Jets @ Boston Patriots
1966-10-23 Buffalo Bills @ New York Jets
1966-10-30 New York Jets @ Kansas City Chiefs
1966-11-06 Denver Broncos @ New York Jets
1966-11-13 New York Jets @ Houston Oilers
1966-11-20 Miami Dolphins @ New York Jets
1966-11-27 New York Jets @ Buffalo Bills`

func TestDetectMultiGameVideo_SeasonCompilation(t *testing.T) {
	game := jetsAtDolphins1966()
	signal := DetectMultiGameVideo(
		"1966 New York Jets - Game Highlights",
		seasonCompilationDescription,
		game,
	)

	if !signal.IsMultiGame {
		t.Fatal("expected IsMultiGame=true for season compilation")
	}
	if signal.DistinctDates < 10 {
		t.Errorf("DistinctDates=%d, expected >=10", signal.DistinctDates)
	}
	if signal.DistinctOpponents < 5 {
		t.Errorf("DistinctOpponents=%d, expected >=5", signal.DistinctOpponents)
	}
	if signal.DetectedGames < 10 {
		t.Errorf("DetectedGames=%d, expected >=10", signal.DetectedGames)
	}
	if signal.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

func TestExtractGameMentions_SeasonCompilation(t *testing.T) {
	mentions := ExtractGameMentions(seasonCompilationDescription)
	if len(mentions) < 10 {
		t.Errorf("expected >=10 game mentions, got %d", len(mentions))
	}

	// Verify the Dolphins game is among them
	foundDolphins := false
	for _, m := range mentions {
		if m.Date == "1966-09-09" {
			foundDolphins = true
			break
		}
	}
	if !foundDolphins {
		t.Error("expected to find 1966-09-09 game mention")
	}
}

func TestComputeSingleGameConfidence_SeasonCompilation(t *testing.T) {
	game := jetsAtDolphins1966()
	sgc := ComputeSingleGameConfidence(
		"1966 New York Jets - Game Highlights",
		seasonCompilationDescription,
		game,
	)

	if sgc.Score >= 0 {
		t.Errorf("expected negative confidence for season compilation, got %d", sgc.Score)
	}
	if sgc.MentionContext != MentionInSchedule {
		t.Errorf("expected MentionInSchedule, got %s", sgc.MentionContext)
	}
}

func TestScoreCandidate_DiscardsSeasonCompilation(t *testing.T) {
	game := jetsAtDolphins1966()
	candidate := youtubeapi.VideoCandidate{
		ID:          "XhFyNtTfCOk",
		Title:       "1966 New York Jets - Game Highlights",
		Description: seasonCompilationDescription,
		Duration:    45 * time.Minute,
		Embeddable:  true,
		ViewCount:   50000,
	}

	result := ScoreCandidate(game, candidate, 300)
	if !result.Discarded {
		t.Errorf("expected season compilation to be discarded, got total=%d, reason=%q",
			result.Score.Total, result.DiscardReason)
	}
	if result.DiscardReason == "" {
		t.Error("expected discard reason to be set")
	}
}

// --- Case 2: Single-game video ---

func TestScoreCandidate_SingleGameVideo(t *testing.T) {
	game := GameMetadata{
		GameID:   "196609020mia",
		DateISO:  "1966-09-02",
		GameDate: time.Date(1966, 9, 2, 0, 0, 0, 0, time.UTC),
		Year:     1966,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Oakland Raiders"),
	}
	candidate := youtubeapi.VideoCandidate{
		ID:          "abc123",
		Title:       "1966 Raiders at Dolphins",
		Description: "Oakland Raiders visit the Miami Dolphins in the 1966 AFL season opener.",
		Duration:    30 * time.Minute,
		Embeddable:  true,
		ViewCount:   10000,
	}

	result := ScoreCandidate(game, candidate, 300)
	if result.Discarded {
		t.Errorf("expected single-game video to not be discarded, reason=%q", result.DiscardReason)
	}
	if result.Score.SingleGameBonus <= 0 {
		t.Errorf("expected positive SingleGameBonus, got %d", result.Score.SingleGameBonus)
	}
	if result.Score.MultiGamePenalty != 0 {
		t.Errorf("expected no multi-game penalty, got %d", result.Score.MultiGamePenalty)
	}
}

// --- Case 3: Bob Griese specific game ---

func TestScoreCandidate_BobGrieseSpecificGame(t *testing.T) {
	game := broncosAtDolphins1967()
	candidate := youtubeapi.VideoCandidate{
		ID:          "griese67",
		Title:       "Bob Griese's First Dolphins Game | Miami Dolphins vs Denver Broncos | September 17, 1967",
		Description: "Watch Bob Griese's NFL debut as the Miami Dolphins take on the Denver Broncos.",
		Duration:    1 * time.Hour,
		Embeddable:  true,
		ViewCount:   200000,
	}

	result := ScoreCandidate(game, candidate, 300)
	if result.Discarded {
		t.Errorf("expected specific game to not be discarded, reason=%q", result.DiscardReason)
	}
	if result.Score.SingleGameBonus <= 0 {
		t.Errorf("expected positive SingleGameBonus, got %d", result.Score.SingleGameBonus)
	}
	if result.Score.Total < 100 {
		t.Errorf("expected high score (>100), got %d", result.Score.Total)
	}
}

// --- Case 4: Title with both teams but description with multiple weeks/rivals ---

const multiWeekDescription = `Week 01: Miami Dolphins vs Denver Broncos
Week 02: Miami Dolphins vs Kansas City Chiefs
Week 03: Buffalo Bills vs Miami Dolphins
Week 04: Miami Dolphins vs Oakland Raiders
Week 05: New York Jets vs Miami Dolphins
Week 06: Boston Patriots vs Miami Dolphins`

func TestScoreCandidate_TitleMatchButMultiWeekDescription(t *testing.T) {
	game := broncosAtDolphins1967()
	candidate := youtubeapi.VideoCandidate{
		ID:          "multiweek",
		Title:       "Denver Broncos vs Miami Dolphins 1967",
		Description: multiWeekDescription,
		Duration:    2 * time.Hour,
		Embeddable:  true,
		ViewCount:   100000,
	}

	result := ScoreCandidate(game, candidate, 300)
	if !result.Discarded {
		t.Errorf("expected multi-week video to be discarded or heavily penalized, total=%d", result.Score.Total)
	}
}

// --- DetectMultiGameVideo unit tests ---

func TestDetectMultiGameVideo_NoMultiGame(t *testing.T) {
	game := jetsAtDolphins1966()
	signal := DetectMultiGameVideo(
		"New York Jets at Miami Dolphins",
		"Week 1 of the 1966 AFL season. The Jets visit the Orange Bowl.",
		game,
	)
	if signal.IsMultiGame {
		t.Errorf("expected single-game, got multi-game: %s", signal.Reason)
	}
}

func TestDetectMultiGameVideo_WeekMarkers(t *testing.T) {
	game := jetsAtDolphins1966()
	desc := `Week 01 - Jets vs Dolphins
Week 02 - Jets vs Oilers
Week 03 - Jets vs Broncos
Week 04 - Jets vs Raiders`
	signal := DetectMultiGameVideo("1966 Jets Highlights", desc, game)
	if !signal.IsMultiGame {
		t.Error("expected IsMultiGame=true with 4 week markers")
	}
	if !signal.HasWeekMarkers {
		t.Error("expected HasWeekMarkers=true")
	}
	if signal.WeekMarkerCount < 4 {
		t.Errorf("WeekMarkerCount=%d, expected >=4", signal.WeekMarkerCount)
	}
}

func TestDetectMultiGameVideo_SeasonKeywords(t *testing.T) {
	game := jetsAtDolphins1966()
	signal := DetectMultiGameVideo(
		"1966 New York Jets Season Highlights",
		"1966-09-09 Jets @ Dolphins\n1966-09-18 Oilers @ Jets",
		game,
	)
	if !signal.IsMultiGame {
		t.Error("expected IsMultiGame=true with season keywords + 2 dates")
	}
}

// --- ExtractGameMentions unit tests ---

func TestExtractGameMentions_Basic(t *testing.T) {
	text := `1966-09-09 New York Jets @ Miami Dolphins
1966-09-18 Houston Oilers @ New York Jets`

	mentions := ExtractGameMentions(text)
	if len(mentions) != 2 {
		t.Fatalf("expected 2 mentions, got %d", len(mentions))
	}
	if mentions[0].Date != "1966-09-09" {
		t.Errorf("mention[0].Date = %q", mentions[0].Date)
	}
	if mentions[1].Date != "1966-09-18" {
		t.Errorf("mention[1].Date = %q", mentions[1].Date)
	}
}

func TestExtractGameMentions_Empty(t *testing.T) {
	mentions := ExtractGameMentions("Just a regular description about football.")
	if len(mentions) != 0 {
		t.Errorf("expected 0 mentions, got %d", len(mentions))
	}
}

func TestExtractGameMentions_VsFormat(t *testing.T) {
	text := "1966-09-09 New York Jets vs Miami Dolphins"
	mentions := ExtractGameMentions(text)
	if len(mentions) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(mentions))
	}
}

// --- CountDistinctDates unit tests ---

func TestCountDistinctDates(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"no dates", "Hello world", 0},
		{"one date", "Game on 1966-09-09", 1},
		{"duplicate date", "1966-09-09 and again 1966-09-09", 1},
		{"multiple dates", "1966-09-09 and 1966-09-18 and 1966-10-02", 3},
		{"slash format", "1966/09/09 and 1966/10/02", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountDistinctDates(tt.text)
			if len(got) != tt.want {
				t.Errorf("CountDistinctDates() = %d, want %d", len(got), tt.want)
			}
		})
	}
}

// --- CountDistinctOpponents unit tests ---

func TestCountDistinctOpponents(t *testing.T) {
	game := jetsAtDolphins1966()

	tests := []struct {
		name    string
		text    string
		wantMin int
	}{
		{"no extra opponents", "New York Jets at Miami Dolphins", 0},
		{"one extra opponent", "New York Jets at Miami Dolphins. Also Buffalo Bills.", 1},
		{"many extra opponents",
			"New York Jets at Miami Dolphins. Also Houston Oilers, Denver Broncos, Oakland Raiders, San Diego Chargers, Buffalo Bills.",
			5},
		{"only game teams", "Jets vs Dolphins in Miami", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountDistinctOpponents(tt.text, game)
			if got < tt.wantMin {
				t.Errorf("CountDistinctOpponents() = %d, want >= %d", got, tt.wantMin)
			}
		})
	}
}

// --- LooksLikeScheduleList unit tests ---

func TestLooksLikeScheduleList(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want bool
	}{
		{"simple description", "A great game between two teams.", false},
		{"schedule with dates", "1966-09-09 Jets @ Dolphins\n1966-09-18 Oilers @ Jets\n1966-09-25 Jets @ Broncos", true},
		{"schedule with weeks", "Week 01 highlight\nWeek 02 highlight\nWeek 03 highlight", true},
		{"schedule with matchups", "Jets @ Dolphins\nJets @ Oilers\nJets @ Broncos", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LooksLikeScheduleList(tt.desc)
			if got != tt.want {
				t.Errorf("LooksLikeScheduleList() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- ComputeSingleGameConfidence unit tests ---

func TestComputeSingleGameConfidence_ExactGame(t *testing.T) {
	game := jetsAtDolphins1966()
	sgc := ComputeSingleGameConfidence(
		"New York Jets at Miami Dolphins 1966-09-09",
		"Classic AFL game at the Orange Bowl.",
		game,
	)
	if sgc.Score <= 0 {
		t.Errorf("expected positive score for exact game, got %d", sgc.Score)
	}
	if sgc.MentionContext != MentionExactGame {
		t.Errorf("expected MentionExactGame, got %s", sgc.MentionContext)
	}
}

func TestComputeSingleGameConfidence_InScheduleList(t *testing.T) {
	game := jetsAtDolphins1966()
	sgc := ComputeSingleGameConfidence(
		"1966 New York Jets Season",
		seasonCompilationDescription,
		game,
	)
	if sgc.Score >= 0 {
		t.Errorf("expected negative score for schedule-list mention, got %d", sgc.Score)
	}
	if sgc.MentionContext != MentionInSchedule {
		t.Errorf("expected MentionInSchedule, got %s", sgc.MentionContext)
	}
}

func TestComputeSingleGameConfidence_MultiGameNoMention(t *testing.T) {
	game := GameMetadata{
		GameID:   "196609180buf",
		DateISO:  "1966-09-18",
		GameDate: time.Date(1966, 9, 18, 0, 0, 0, 0, time.UTC),
		Year:     1966,
		Home:     LookupTeam("Buffalo Bills"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	// Description about Jets season (not about BUF @ MIA specifically),
	// but mentions both Dolphins and Bills elsewhere
	sgc := ComputeSingleGameConfidence(
		"1966 AFL Season Recap",
		"1966-09-09 New York Jets @ Miami Dolphins\n1966-09-25 Jets @ Broncos\n1966-10-23 Buffalo Bills @ New York Jets",
		game,
	)
	if sgc.Score >= 0 {
		t.Errorf("expected negative score, got %d", sgc.Score)
	}
}

// --- Integration: single-game beats compilation ---

func TestSingleGameBeatsCompilation(t *testing.T) {
	game := jetsAtDolphins1966()

	compilation := youtubeapi.VideoCandidate{
		ID:          "compilation",
		Title:       "1966 New York Jets - Game Highlights",
		Description: seasonCompilationDescription,
		Duration:    2 * time.Hour,
		Embeddable:  true,
		ViewCount:   500000,
	}

	singleGame := youtubeapi.VideoCandidate{
		ID:          "single",
		Title:       "New York Jets at Miami Dolphins 1966-09-09",
		Description: "The Jets visit the Dolphins at the Orange Bowl in 1966.",
		Duration:    15 * time.Minute,
		Embeddable:  true,
		ViewCount:   5000,
	}

	compilationResult := ScoreCandidate(game, compilation, 300)
	singleResult := ScoreCandidate(game, singleGame, 300)

	// Compilation should be discarded
	if !compilationResult.Discarded {
		t.Errorf("compilation should be discarded, total=%d", compilationResult.Score.Total)
	}

	// Single game should NOT be discarded
	if singleResult.Discarded {
		t.Errorf("single game should not be discarded, reason=%q", singleResult.DiscardReason)
	}

	// Even if we compare scores, single game should win
	if !compilationResult.Discarded && singleResult.Score.Total <= compilationResult.Score.Total {
		t.Errorf("single game (score=%d) should beat compilation (score=%d)",
			singleResult.Score.Total, compilationResult.Score.Total)
	}
}

func TestDetectMultiGameVideoDoubleheaderTitle(t *testing.T) {
	game := GameMetadata{
		GameID:   "202409300mia",
		DateISO:  "2024-09-30",
		GameDate: time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC),
		Year:     2024,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Tennessee Titans"),
	}
	signal := DetectMultiGameVideo("MNF Doubleheader! Titans @ Dolphins / Seahawks @ Lions: 9/30/24", "", game)
	if !signal.IsMultiGame {
		t.Fatalf("expected doubleheader title to be multi-game, got %+v", signal)
	}
}

// --- Regression: video with only compilation keywords but no actual multi-game signals ---

func TestScoreCandidate_GameHighlightsNotMultiGame(t *testing.T) {
	game := jetsAtDolphins1966()
	candidate := youtubeapi.VideoCandidate{
		ID:          "highlights_single",
		Title:       "New York Jets vs Miami Dolphins - Game Highlights",
		Description: "Full game highlights from the 1966 AFL season opener.",
		Duration:    20 * time.Minute,
		Embeddable:  true,
		ViewCount:   30000,
	}

	result := ScoreCandidate(game, candidate, 300)
	if result.Discarded {
		t.Errorf("single-game highlights should not be discarded, reason=%q", result.DiscardReason)
	}
	if result.Score.MultiGamePenalty != 0 {
		t.Errorf("no multi-game penalty expected, got %d", result.Score.MultiGamePenalty)
	}
}

// --- Edge case: "game highlights" with one week marker only ---

func TestDetectMultiGameVideo_SingleWeekMarker(t *testing.T) {
	game := jetsAtDolphins1966()
	signal := DetectMultiGameVideo(
		"New York Jets vs Miami Dolphins Week 1 Highlights",
		"AFL 1966 Week 1",
		game,
	)
	if signal.IsMultiGame {
		t.Error("single week marker should not trigger multi-game")
	}
}

// --- Edge case: no description at all ---

func TestDetectMultiGameVideo_EmptyDescription(t *testing.T) {
	game := jetsAtDolphins1966()
	signal := DetectMultiGameVideo("Jets vs Dolphins 1966", "", game)
	if signal.IsMultiGame {
		t.Error("empty description should not be multi-game")
	}
}

// --- Score comparison: same video penalized vs not ---

func TestScoreCandidateMultiGamePenalty_AppliedCorrectly(t *testing.T) {
	game := jetsAtDolphins1966()

	// Candidate with multi-game signals but below the hard-discard threshold (2 dates, 2 opponents)
	candidate := youtubeapi.VideoCandidate{
		ID:    "softmulti",
		Title: "New York Jets at Miami Dolphins",
		Description: "1966-09-09 New York Jets @ Miami Dolphins\n" +
			"1966-09-18 Houston Oilers @ New York Jets",
		Duration:   30 * time.Minute,
		Embeddable: true,
		ViewCount:  20000,
	}

	result := ScoreCandidate(game, candidate, 300)
	// Should have a penalty but might not be discarded (2 dates, 1 extra opponent)
	if result.Score.MultiGamePenalty > 0 {
		t.Errorf("expected non-positive MultiGamePenalty, got %d", result.Score.MultiGamePenalty)
	}
}

// --- isCurrentGameInMentions ---

func TestIsCurrentGameInMentions_Found(t *testing.T) {
	game := jetsAtDolphins1966()
	mentions := []GameMention{
		{Date: "1966-09-09", NormalizedTeamA: "new york jets", NormalizedTeamB: "miami dolphins"},
		{Date: "1966-09-18", NormalizedTeamA: "houston oilers", NormalizedTeamB: "new york jets"},
	}
	if !isCurrentGameInMentions(mentions, game) {
		t.Error("expected to find current game in mentions")
	}
}

func TestIsCurrentGameInMentions_NotFound(t *testing.T) {
	game := jetsAtDolphins1966()
	mentions := []GameMention{
		{Date: "1966-09-18", NormalizedTeamA: "houston oilers", NormalizedTeamB: "new york jets"},
		{Date: "1966-09-25", NormalizedTeamA: "new york jets", NormalizedTeamB: "denver broncos"},
	}
	if isCurrentGameInMentions(mentions, game) {
		t.Error("expected NOT to find current game in mentions")
	}
}

// --- Verify discard reason contains useful debug info ---

func TestScoreCandidate_DiscardReasonContainsStats(t *testing.T) {
	game := jetsAtDolphins1966()
	candidate := youtubeapi.VideoCandidate{
		ID:          "debug_test",
		Title:       "1966 New York Jets - Game Highlights",
		Description: seasonCompilationDescription,
		Duration:    45 * time.Minute,
		Embeddable:  true,
		ViewCount:   50000,
	}

	result := ScoreCandidate(game, candidate, 300)
	if !result.Discarded {
		t.Fatal("expected discarded")
	}
	// Check that reason contains diagnostic info
	if !strings.Contains(result.DiscardReason, "dates=") {
		t.Errorf("expected discard reason to contain 'dates=', got: %s", result.DiscardReason)
	}
	if !strings.Contains(result.DiscardReason, "opponents=") {
		t.Errorf("expected discard reason to contain 'opponents=', got: %s", result.DiscardReason)
	}
}
