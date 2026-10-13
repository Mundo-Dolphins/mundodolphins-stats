package videos

import (
	"strings"
	"testing"
)

func TestParseSelectionInput(t *testing.T) {
	result, err := ParseSelectionInput("1,3", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "selected" || len(result.Picks) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}

	result, err = ParseSelectionInput("a", 3)
	if err != nil || len(result.Picks) != 3 {
		t.Fatalf("expected all picks")
	}

	result, err = ParseSelectionInput("n", 3)
	if err != nil || result.Action != "none" {
		t.Fatalf("expected none")
	}

	result, err = ParseSelectionInput("skip", 3)
	if err != nil || result.Action != "skip" {
		t.Fatalf("expected skip")
	}

	if _, err := ParseSelectionInput("x", 3); err == nil {
		t.Fatalf("expected error for invalid input")
	}
	if _, err := ParseSelectionInput("5", 3); err == nil {
		t.Fatalf("expected error for out of range")
	}
}

func TestSelectorPrompt(t *testing.T) {
	in := strings.NewReader("1\n")
	out := &strings.Builder{}
	selector := Selector{In: in, Out: out}
	game := GameRef{GameID: "196612180mia", AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins", Year: 1966}
	candidates := []Candidate{{VideoID: "abc", Title: "Game 1", DurationText: "1:00:00", Snippet: "Desc", Score: 17, Channel: "Vintage Pro Football"}}
	result, err := selector.Prompt(game, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != "selected" || len(result.Picks) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(out.String(), "https://www.youtube.com/watch?v=abc") {
		t.Fatalf("expected url in output")
	}
	if !strings.Contains(out.String(), "Desc") {
		t.Fatalf("expected description in output")
	}
	if !strings.Contains(out.String(), "Score: 17") {
		t.Fatalf("expected score in output")
	}
	if !strings.Contains(out.String(), "Channel: Vintage Pro Football") {
		t.Fatalf("expected channel in output")
	}
}

func TestPrintCandidatesNoResultsMessage(t *testing.T) {
	out := &strings.Builder{}
	game := GameRef{GameID: "196610230oti", AwayTeam: "Miami Dolphins", HomeTeam: "Houston Oilers", Year: 1966}
	PrintCandidates(out, game, nil)
	if !strings.Contains(out.String(), "No candidates found or none met the minimum criteria") {
		t.Fatalf("expected empty-candidates message in output")
	}
}

func TestSortCandidatesByDuration(t *testing.T) {
	candidates := []Candidate{
		{Title: "Short", DurationSec: 60},
		{Title: "Unknown", DurationSec: 0},
		{Title: "Long", DurationSec: 120},
	}
	sorted := sortCandidatesByDuration(candidates)
	if sorted[0].Title != "Long" || sorted[1].Title != "Short" || sorted[2].Title != "Unknown" {
		t.Fatalf("unexpected order: %s, %s, %s", sorted[0].Title, sorted[1].Title, sorted[2].Title)
	}
}

func TestSortCandidatesByDurationParsesText(t *testing.T) {
	candidates := []Candidate{
		{Title: "Short", DurationText: "0:37"},
		{Title: "Long", DurationText: "2:47:59"},
		{Title: "Mid", DurationText: "44:28"},
	}
	sorted := sortCandidatesByDuration(candidates)
	if sorted[0].Title != "Long" || sorted[1].Title != "Mid" || sorted[2].Title != "Short" {
		t.Fatalf("unexpected parsed order: %s, %s, %s", sorted[0].Title, sorted[1].Title, sorted[2].Title)
	}
}
