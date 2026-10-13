package youtube

import (
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestMatchesCriteria(t *testing.T) {
	criteria := Criteria{HomeTeam: "Miami Dolphins", VisitorTeam: "Oakland Raiders", DateISO: "1966-09-02"}

	ok := youtubeapi.VideoCandidate{
		Title:       "Oakland Raiders vs Miami Dolphins 1966-09-02",
		Description: "full game broadcast",
		Duration:    44 * time.Minute,
		Embeddable:  true,
	}
	if !MatchesCriteria(ok, criteria, 5*time.Minute) {
		t.Fatalf("expected candidate to match")
	}

	notEmbed := ok
	notEmbed.Embeddable = false
	if MatchesCriteria(notEmbed, criteria, 5*time.Minute) {
		t.Fatalf("expected non embeddable candidate to fail")
	}

	tooShort := ok
	tooShort.Duration = 2 * time.Minute
	if MatchesCriteria(tooShort, criteria, 5*time.Minute) {
		t.Fatalf("expected short candidate to fail")
	}
}

func TestScoreVideoPenalties(t *testing.T) {
	criteria := Criteria{HomeTeam: "Miami Dolphins", VisitorTeam: "Oakland Raiders", DateISO: "1966-09-02"}
	full := youtubeapi.VideoCandidate{
		Title:       "Oakland Raiders vs Miami Dolphins 1966-09-02",
		Description: "full game",
		Duration:    44 * time.Minute,
		Embeddable:  true,
	}
	highlights := youtubeapi.VideoCandidate{
		Title:       "Oakland Raiders vs Miami Dolphins highlights",
		Description: "short clip 1966",
		Duration:    44 * time.Minute,
		Embeddable:  true,
	}

	sFull := ScoreVideo(full, criteria)
	sHighlights := ScoreVideo(highlights, criteria)
	if sHighlights >= sFull {
		t.Fatalf("expected highlights score lower: highlights=%d full=%d", sHighlights, sFull)
	}
}

func TestSortCandidates(t *testing.T) {
	items := []youtubeapi.VideoCandidate{
		{Title: "B", Duration: 10 * time.Minute, Score: 8},
		{Title: "A", Duration: 20 * time.Minute, Score: 8},
		{Title: "C", Duration: 30 * time.Minute, Score: 9},
	}
	SortCandidates(items)
	if items[0].Title != "C" || items[1].Title != "A" || items[2].Title != "B" {
		t.Fatalf("unexpected sort order: %s %s %s", items[0].Title, items[1].Title, items[2].Title)
	}
}

func TestScoringHelperEdgeBranches(t *testing.T) {
	if containsAnyToken(youtubeapi.VideoCandidate{Title: "Miami Dolphins"}, nil) {
		t.Fatalf("empty token list should not match")
	}
	if got := minInt(9, 5); got != 5 {
		t.Fatalf("minInt = %d, want 5", got)
	}
	if got := atoiYear("20x4"); got != 0 {
		t.Fatalf("atoiYear invalid = %d, want 0", got)
	}
}
