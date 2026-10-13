package videos

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Selector struct {
	In  io.Reader
	Out io.Writer
}

type SelectResult struct {
	Action string
	Picks  []int
}

func (s Selector) Prompt(game GameRef, candidates []Candidate) (SelectResult, error) {
	if s.In == nil || s.Out == nil {
		return SelectResult{Action: "skip"}, fmt.Errorf("selector requires input and output")
	}
	PrintCandidates(s.Out, game, candidates)
	fmt.Fprint(s.Out, "Select videos (e.g. 1,3 | a=all | n=none | skip): ")

	scanner := bufio.NewScanner(s.In)
	if !scanner.Scan() {
		return SelectResult{Action: "skip"}, scanner.Err()
	}
	input := scanner.Text()
	return ParseSelectionInput(input, len(candidates))
}

func PrintCandidates(out io.Writer, game GameRef, candidates []Candidate) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "\n**%s vs %s (%s)**\n", game.AwayTeam, game.HomeTeam, gameDateISO(game))
	if len(candidates) == 0 {
		fmt.Fprintln(out, "No candidates found or none met the minimum criteria (duration/relevance).")
		return
	}
	for i, c := range candidates {
		url := c.URL
		if url == "" && c.VideoID != "" {
			url = "https://www.youtube.com/watch?v=" + c.VideoID
		}
		duration := c.DurationText
		if duration == "" {
			duration = "unknown"
		}
		fmt.Fprintf(out, "[%d] %s (%s)\n", i+1, c.Title, duration)
		fmt.Fprintf(out, "    URL: %s\n", url)
		fmt.Fprintf(out, "    Desc: %s\n", SanitizeSnippet(c.Snippet))
		fmt.Fprintf(out, "    Score: %d | Channel: %s\n", c.Score, nonEmpty(c.Channel, "unknown"))
	}
}

func gameDateISO(game GameRef) string {
	if len(game.GameID) < 8 {
		if game.Year > 0 {
			return strconv.Itoa(game.Year)
		}
		return "unknown"
	}
	raw := game.GameID[:8]
	for _, r := range raw {
		if r < '0' || r > '9' {
			if game.Year > 0 {
				return strconv.Itoa(game.Year)
			}
			return "unknown"
		}
	}
	return fmt.Sprintf("%s-%s-%s", raw[:4], raw[4:6], raw[6:8])
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func ParseSelectionInput(input string, max int) (SelectResult, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" || input == "n" || input == "0" {
		return SelectResult{Action: "none"}, nil
	}
	if input == "skip" {
		return SelectResult{Action: "skip"}, nil
	}
	if input == "a" || input == "all" {
		picks := make([]int, 0, max)
		for i := 1; i <= max; i++ {
			picks = append(picks, i)
		}
		return SelectResult{Action: "selected", Picks: picks}, nil
	}
	parts := strings.Split(input, ",")
	picked := make(map[int]struct{})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx, err := strconv.Atoi(part)
		if err != nil {
			return SelectResult{}, fmt.Errorf("invalid selection: %s", part)
		}
		if idx < 1 || idx > max {
			return SelectResult{}, fmt.Errorf("selection out of range: %d", idx)
		}
		picked[idx] = struct{}{}
	}
	if len(picked) == 0 {
		return SelectResult{Action: "none"}, nil
	}
	picks := make([]int, 0, len(picked))
	for idx := range picked {
		picks = append(picks, idx)
	}
	sort.Ints(picks)
	return SelectResult{Action: "selected", Picks: picks}, nil
}

func sortCandidatesByDuration(candidates []Candidate) []Candidate {
	if len(candidates) == 0 {
		return candidates
	}
	sorted := make([]Candidate, len(candidates))
	copy(sorted, candidates)
	for i := range sorted {
		if sorted[i].DurationSec == 0 && sorted[i].DurationText != "" && sorted[i].DurationText != "unknown" {
			sorted[i].DurationSec = ParseDurationToSeconds(sorted[i].DurationText)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		di := sorted[i].DurationSec
		dj := sorted[j].DurationSec
		if di == 0 && dj == 0 {
			return sorted[i].Title < sorted[j].Title
		}
		if di == 0 {
			return false
		}
		if dj == 0 {
			return true
		}
		if di == dj {
			return sorted[i].Title < sorted[j].Title
		}
		return di > dj
	})
	return sorted
}
