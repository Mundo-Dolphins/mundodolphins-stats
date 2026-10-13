package videos

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// bannedTitleWords are words that cause immediate discard when found in the title.
var bannedTitleWords = []string{
	"preview",
	"recap",
	"reaction",
	"analysis",
	"breakdown",
	"interview",
	"press conference",
	"podcast",
	"gmfb",
	"good morning football",
	"top 10",
	"top plays",
	"top 5",
	"best plays",
	"free agency",
	"draft pick",
	"mock draft",
	"fantasy",
	"betting",
	"prediction",
	"madden",
	"simulation",
}

// bannedDescriptionWords cause discard when found in the description combined with title.
var bannedDescriptionWords = []string{
	"press conference",
	"podcast",
	"gmfb",
	"good morning football",
	"fantasy football",
}

// multiMatchupRe detects multiple "vs" / "@" patterns.
var multiMatchupRe = regexp.MustCompile(`(?i)\b(?:vs\.?|v\.?|@)\b`)

// multiWeekRe detects Week references.
var multiWeekRe = regexp.MustCompile(`(?i)\bweek\s*\d+`)

// HardFilterResult explains why a video was accepted or rejected.
type HardFilterResult struct {
	Accepted bool
	Reason   string
}

// IsValidCandidate applies all hard filters BEFORE scoring.
// Returns false with a reason if the video should be discarded.
func IsValidCandidate(c youtubeapi.VideoCandidate, game GameMetadata, minDurationSec int) HardFilterResult {
	titleLower := strings.ToLower(c.Title)
	descLower := strings.ToLower(c.Description)
	combined := titleLower + " " + descLower

	// 1. Banned words in title
	for _, word := range bannedTitleWords {
		if strings.Contains(titleLower, word) {
			return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("banned word in title: %q", word)}
		}
	}

	// 2. Banned words in description
	for _, word := range bannedDescriptionWords {
		if strings.Contains(descLower, word) {
			return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("banned word in description: %q", word)}
		}
	}

	// 3. Minimum duration (hard: 600 seconds = 10 minutes)
	durationSec := int(c.Duration.Seconds())
	effectiveMin := minDurationSec
	if effectiveMin < 600 {
		effectiveMin = 600
	}
	if durationSec > 0 && durationSec < effectiveMin {
		return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("duration %ds < minimum %ds", durationSec, effectiveMin)}
	}

	// 4. Both teams must appear in title+description
	if !ContainsBothTeams(c.Title, c.Description, game.Home, game.Away) {
		return HardFilterResult{Accepted: false, Reason: "both teams not mentioned"}
	}

	// 5. Wrong year
	dateMatch := DetectDateMatch(combined, game.GameDate)
	if dateMatch == DateMatchWrong {
		return HardFilterResult{Accepted: false, Reason: "wrong year in video"}
	}

	// 6. Multi-game detection: too many "vs"/"@" patterns
	vsMatches := multiMatchupRe.FindAllString(combined, -1)
	if len(vsMatches) > 2 {
		return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("multi-game: %d vs/@ patterns", len(vsMatches))}
	}

	// 7. Multi-game: too many Week references
	weekMatches := multiWeekRe.FindAllString(combined, -1)
	if len(weekMatches) > 2 {
		return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("multi-game: %d week references", len(weekMatches))}
	}

	// 8. Too many distinct NFL teams (>2 unique teams = compilation)
	teamCount := countDistinctNFLTeams(combined, game)
	if teamCount > 2 {
		return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("too many teams: %d distinct teams found", teamCount)}
	}

	// 9. Full multi-game signal from existing detector
	multiSignal := DetectMultiGameVideo(c.Title, c.Description, game)
	if multiSignal.IsMultiGame {
		return HardFilterResult{Accepted: false, Reason: fmt.Sprintf("multi-game: %s", multiSignal.Reason)}
	}

	return HardFilterResult{Accepted: true}
}

// countDistinctNFLTeams counts how many different NFL teams appear in text,
// including the two teams from the current game.
func countDistinctNFLTeams(text string, game GameMetadata) int {
	norm := NormalizeText(text)
	found := make(map[string]bool)

	// Check each team by full name (reliable, avoids city ambiguity)
	for _, team := range nflTeams {
		if team.FullName == "" {
			continue
		}
		if strings.Contains(norm, strings.ToLower(team.FullName)) {
			found[team.Abbr] = true
		}
	}

	return len(found)
}

// filterByMinDuration filters Candidates (legacy type) by minimum duration.
func filterByMinDuration(candidates []Candidate, minDurationSec int) []Candidate {
	if minDurationSec <= 0 || len(candidates) == 0 {
		return candidates
	}
	filtered := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		c.DurationSec = candidateDurationSec(c)
		if c.DurationSec <= 0 {
			continue
		}
		if c.DurationSec < minDurationSec {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}
