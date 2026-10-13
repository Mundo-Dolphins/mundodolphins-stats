package videos

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type ClassificationDecision string

const (
	DecisionAccept ClassificationDecision = "accept"
	DecisionReview ClassificationDecision = "review"
	DecisionReject ClassificationDecision = "reject"
)

type ClassificationPolicy struct {
	AcceptThreshold                int
	ReviewThreshold                int
	MinDurationSec                 int
	FullGameMinDurationSec         int
	HighlightsMinDurationSec       int
	AllowHighlights                bool
	RequireBothTeams               bool
	RequireDateForAccept           bool
	RequireRealGameSignalForAccept bool
	RejectTerms                    []string
	ReviewTerms                    []string
}

type ClassificationEvidence struct {
	Signal string `json:"signal"`
	Value  string `json:"value"`
	Source string `json:"source,omitempty"`
	Score  int    `json:"score,omitempty"`
}

type ClassificationSignals struct {
	TeamsMatched    bool          `json:"teams_matched,omitempty"`
	TitleTeams      bool          `json:"title_teams,omitempty"`
	TeamOrder       string        `json:"team_order,omitempty"`
	DateMatch       string        `json:"date_match,omitempty"`
	WeekMatch       string        `json:"week_match,omitempty"`
	DurationSeconds int           `json:"duration_seconds,omitempty"`
	ContentRole     string        `json:"content_role,omitempty"`
	RealGameTerms   []string      `json:"real_game_terms,omitempty"`
	HighlightTerms  []string      `json:"highlight_terms,omitempty"`
	RejectTerms     []string      `json:"reject_terms,omitempty"`
	ReviewTerms     []string      `json:"review_terms,omitempty"`
	MixedSignals    []string      `json:"mixed_signals,omitempty"`
	MultiGame       bool          `json:"multi_game,omitempty"`
	MultiGameReason string        `json:"multi_game_reason,omitempty"`
	Embeddable      bool          `json:"embeddable,omitempty"`
	Channel         string        `json:"channel,omitempty"`
	Duration        time.Duration `json:"-"`
}

type ClassificationResult struct {
	Decision ClassificationDecision   `json:"decision"`
	Score    int                      `json:"score"`
	Reasons  []string                 `json:"reasons,omitempty"`
	Evidence []ClassificationEvidence `json:"evidence,omitempty"`
	Signals  ClassificationSignals    `json:"signals,omitempty"`
}

func DefaultClassificationPolicy() ClassificationPolicy {
	return ClassificationPolicy{
		AcceptThreshold:                95,
		ReviewThreshold:                55,
		MinDurationSec:                 600,
		FullGameMinDurationSec:         1800,
		HighlightsMinDurationSec:       300,
		AllowHighlights:                false,
		RequireBothTeams:               true,
		RequireDateForAccept:           false,
		RequireRealGameSignalForAccept: true,
		RejectTerms: []string{
			"preview",
			"prediction",
			"predictions",
			"betting",
			"odds",
			"madden",
			"simulation",
			"simulated",
			"pro strategy football",
			"apba football",
			"second and ten football",
			"action pc sports football",
			"test game",
			"mock draft",
			"fantasy",
			"reaction",
			"podcast",
			"press conference",
			"interview",
		},
		ReviewTerms: []string{
			"analysis",
			"breakdown",
			"recap",
			"pregame",
			"postgame",
			"postgame show",
			"greatest games",
			"g mix",
			"shorts",
			"clip",
			"clips",
			"top plays",
			"top 10",
			"every team",
			"every teams",
			"all teams",
			"best plays",
			"live",
			"stream",
			"streams",
		},
	}
}

func (p ClassificationPolicy) withDefaults() ClassificationPolicy {
	defaults := DefaultClassificationPolicy()
	if p.isZero() {
		return defaults
	}
	if p.AcceptThreshold == 0 {
		p.AcceptThreshold = defaults.AcceptThreshold
	}
	if p.ReviewThreshold == 0 {
		p.ReviewThreshold = defaults.ReviewThreshold
	}
	if p.MinDurationSec == 0 {
		p.MinDurationSec = defaults.MinDurationSec
	}
	if p.FullGameMinDurationSec == 0 {
		p.FullGameMinDurationSec = defaults.FullGameMinDurationSec
	}
	if p.HighlightsMinDurationSec == 0 {
		p.HighlightsMinDurationSec = defaults.HighlightsMinDurationSec
	}
	if len(p.RejectTerms) == 0 {
		p.RejectTerms = defaults.RejectTerms
	}
	if len(p.ReviewTerms) == 0 {
		p.ReviewTerms = defaults.ReviewTerms
	}
	return p
}

func (p ClassificationPolicy) isZero() bool {
	return p.AcceptThreshold == 0 &&
		p.ReviewThreshold == 0 &&
		p.MinDurationSec == 0 &&
		p.FullGameMinDurationSec == 0 &&
		p.HighlightsMinDurationSec == 0 &&
		!p.AllowHighlights &&
		!p.RequireBothTeams &&
		!p.RequireDateForAccept &&
		!p.RequireRealGameSignalForAccept &&
		len(p.RejectTerms) == 0 &&
		len(p.ReviewTerms) == 0
}

func ClassifyVideoForGame(game GameMetadata, c youtubeapi.VideoCandidate, policy ClassificationPolicy) ClassificationResult {
	policy = policy.withDefaults()
	text := c.Title + " " + c.Description
	norm := NormalizeText(text)
	durationSec := int(c.Duration.Seconds())
	result := ClassificationResult{
		Decision: DecisionReview,
		Signals: ClassificationSignals{
			DurationSeconds: durationSec,
			Duration:        c.Duration,
			Embeddable:      c.Embeddable,
			Channel:         c.Channel,
		},
	}

	teamsMatched := ContainsBothTeams(c.Title, c.Description, game.Home, game.Away)
	titleTeamsMatched := ContainsBothTeams(c.Title, "", game.Home, game.Away)
	result.Signals.TeamsMatched = teamsMatched
	result.Signals.TitleTeams = titleTeamsMatched
	if teamsMatched {
		result.Score += 35
		result.Evidence = append(result.Evidence, evidence("teams", "both teams referenced", "title_description", 35))
		if titleTeamsMatched {
			result.Score += 12
			result.Evidence = append(result.Evidence, evidence("title_teams", "both teams referenced", "title", 12))
		}
	} else if policy.RequireBothTeams {
		return reject(result, "missing both team references", evidence("teams", "missing", "title_description", -100))
	}

	order := DetectTeamOrder(text, game.Home, game.Away)
	result.Signals.TeamOrder = string(order)
	switch order {
	case TeamOrderCorrect:
		result.Score += 10
		result.Evidence = append(result.Evidence, evidence("team_order", string(order), "title_description", 10))
	case TeamOrderReverse:
		result.Score += 5
		result.Evidence = append(result.Evidence, evidence("team_order", string(order), "title_description", 5))
	}

	dateMatch := DateMatchNone
	if !game.GameDate.IsZero() {
		dateMatch = DetectGameDateMatch(text, game)
	}
	result.Signals.DateMatch = string(dateMatch)
	switch dateMatch {
	case DateMatchFull:
		result.Score += 30
		result.Evidence = append(result.Evidence, evidence("date", string(dateMatch), "title_description", 30))
	case DateMatchYearOnly:
		result.Score += 8
		result.Evidence = append(result.Evidence, evidence("date", string(dateMatch), "title_description", 8))
	case DateMatchWrong:
		return reject(result, "explicit wrong year", evidence("date", string(dateMatch), "title_description", -100))
	}
	if broadcastWeekdayMismatch(game, text) {
		return reject(result, "broadcast weekday does not match game date", evidence("broadcast_weekday", "mismatch", "title_description", -100))
	}
	weekMatch := DetectGameWeekMatch(text, game)
	result.Signals.WeekMatch = string(weekMatch)
	switch weekMatch {
	case DateMatchFull:
		result.Score += 18
		result.Evidence = append(result.Evidence, evidence("week", string(weekMatch), "title_description", 18))
	case DateMatchWrong:
		return reject(result, "explicit wrong week", evidence("week", string(weekMatch), "title_description", -100))
	}

	result.Signals.RejectTerms = findTerms(norm, policy.RejectTerms)
	if len(result.Signals.RejectTerms) > 0 {
		sort.Strings(result.Signals.RejectTerms)
		return reject(result, "rejected term: "+strings.Join(result.Signals.RejectTerms, ", "), evidence("reject_terms", strings.Join(result.Signals.RejectTerms, ", "), "title_description", -100))
	}

	result.Signals.ReviewTerms = findTerms(norm, policy.ReviewTerms)
	if len(result.Signals.ReviewTerms) > 0 {
		result.Score -= 20
		sort.Strings(result.Signals.ReviewTerms)
		result.Evidence = append(result.Evidence, evidence("review_terms", strings.Join(result.Signals.ReviewTerms, ", "), "title_description", -20))
	}

	result.Signals.RealGameTerms = findRegexTerms(norm, realGameTermPatterns())
	if len(result.Signals.RealGameTerms) > 0 {
		points := 20
		if containsAnyExact(result.Signals.RealGameTerms, []string{"full game", "complete game", "broadcast", "game film"}) {
			points = 25
		}
		result.Score += points
		result.Evidence = append(result.Evidence, evidence("real_game_terms", strings.Join(result.Signals.RealGameTerms, ", "), "title_description", points))
	}

	result.Signals.HighlightTerms = findRegexTerms(norm, highlightTermPatterns())
	if len(result.Signals.HighlightTerms) > 0 {
		if policy.AllowHighlights {
			result.Score += 8
			result.Evidence = append(result.Evidence, evidence("highlight_terms", strings.Join(result.Signals.HighlightTerms, ", "), "title_description", 8))
		} else {
			result.Signals.MixedSignals = append(result.Signals.MixedSignals, "highlights require manual review")
			result.Score -= 10
			result.Evidence = append(result.Evidence, evidence("highlight_terms", strings.Join(result.Signals.HighlightTerms, ", "), "title_description", -10))
		}
	}

	if durationSec > 0 {
		durationScore := classifyDurationScore(durationSec)
		result.Score += durationScore
		result.Evidence = append(result.Evidence, evidence("duration", fmt.Sprintf("%ds", durationSec), "metadata", durationScore))
		if durationSec < policy.MinDurationSec {
			return reject(result, fmt.Sprintf("duration %ds below minimum %ds", durationSec, policy.MinDurationSec), evidence("duration", fmt.Sprintf("%ds", durationSec), "metadata", -100))
		}
	}

	if c.Embeddable {
		result.Score += 5
		result.Evidence = append(result.Evidence, evidence("embeddable", "true", "metadata", 5))
	}
	if scoreTrustedChannel(c.Channel) > 0 {
		result.Score += 5
		result.Evidence = append(result.Evidence, evidence("trusted_channel", c.Channel, "metadata", 5))
	}
	result.Signals.ContentRole = string(ClassifyContentRole(c.Title, c.Description, c.Channel, durationSec))
	if result.Signals.ContentRole != "" {
		result.Evidence = append(result.Evidence, evidence("content_role", result.Signals.ContentRole, "title_description", 0))
	}

	multiSignal := DetectMultiGameVideo(c.Title, c.Description, game)
	if multiSignal.IsMultiGame {
		result.Signals.MultiGame = true
		result.Signals.MultiGameReason = multiSignal.Reason
		return reject(result, "multi-game video: "+multiSignal.Reason, evidence("multi_game", multiSignal.Reason, "title_description", -100))
	}

	strongGameIdentity := titleTeamsMatched &&
		dateMatch == DateMatchFull &&
		durationSec >= policy.MinDurationSec
	if policy.RequireRealGameSignalForAccept && len(result.Signals.RealGameTerms) == 0 && len(result.Signals.HighlightTerms) == 0 && !strongGameIdentity {
		result.Signals.MixedSignals = append(result.Signals.MixedSignals, "no real-game content term")
	}
	if policy.RequireDateForAccept && dateMatch != DateMatchFull {
		result.Signals.MixedSignals = append(result.Signals.MixedSignals, "missing exact game date")
	}
	if len(result.Signals.HighlightTerms) > 0 && !policy.AllowHighlights {
		result.Signals.MixedSignals = append(result.Signals.MixedSignals, "highlights disabled by policy")
	}
	if !titleTeamsMatched && len(result.Signals.HighlightTerms) > 0 {
		result.Signals.MixedSignals = append(result.Signals.MixedSignals, "highlights need both teams in title")
	}

	switch {
	case canAccept(result, policy):
		result.Decision = DecisionAccept
		result.Reasons = append(result.Reasons, "accepted by deterministic policy")
	case result.Score >= policy.ReviewThreshold:
		result.Decision = DecisionReview
		result.Reasons = append(result.Reasons, "requires manual review")
	default:
		result.Decision = DecisionReject
		result.Reasons = append(result.Reasons, "score below review threshold")
	}
	return result
}

func ClassifyLegacyCandidate(game GameRef, c Candidate, policy ClassificationPolicy) ClassificationResult {
	meta := ExtractGameMetadata(game)
	return ClassifyVideoForGame(meta, youtubeapi.VideoCandidate{
		ID:          c.VideoID,
		URL:         c.URL,
		Title:       c.Title,
		Description: c.Snippet,
		Channel:     c.Channel,
		Duration:    time.Duration(c.DurationSec) * time.Second,
		Embeddable:  true,
		Score:       c.Score,
	}, policy)
}

func canAccept(result ClassificationResult, policy ClassificationPolicy) bool {
	if result.Score < policy.AcceptThreshold {
		return false
	}
	if len(result.Signals.ReviewTerms) > 0 {
		return false
	}
	if policy.RequireBothTeams && !result.Signals.TeamsMatched {
		return false
	}
	if policy.RequireDateForAccept && result.Signals.DateMatch != string(DateMatchFull) {
		return false
	}
	strongGameIdentity := result.Signals.TitleTeams &&
		result.Signals.DateMatch == string(DateMatchFull) &&
		result.Signals.DurationSeconds >= policy.MinDurationSec
	if policy.RequireRealGameSignalForAccept && len(result.Signals.RealGameTerms) == 0 && len(result.Signals.HighlightTerms) == 0 && !strongGameIdentity {
		return false
	}
	if len(result.Signals.HighlightTerms) > 0 && !policy.AllowHighlights {
		return false
	}
	if result.Signals.DurationSeconds > 0 && len(result.Signals.HighlightTerms) > 0 && result.Signals.DurationSeconds < policy.HighlightsMinDurationSec {
		return false
	}
	if result.Signals.DurationSeconds > 0 && len(result.Signals.RealGameTerms) > 0 && result.Signals.DurationSeconds < policy.FullGameMinDurationSec && len(result.Signals.HighlightTerms) == 0 {
		return false
	}
	if len(result.Signals.MixedSignals) > 0 {
		return false
	}
	return true
}

func reject(result ClassificationResult, reason string, ev ClassificationEvidence) ClassificationResult {
	result.Decision = DecisionReject
	result.Reasons = append(result.Reasons, reason)
	result.Evidence = append(result.Evidence, ev)
	return result
}

func evidence(signal, value, source string, score int) ClassificationEvidence {
	return ClassificationEvidence{Signal: signal, Value: value, Source: source, Score: score}
}

func classifyDurationScore(seconds int) int {
	switch {
	case seconds >= 7200:
		return 15
	case seconds >= 3600:
		return 12
	case seconds >= 1800:
		return 8
	case seconds >= 900:
		return 4
	default:
		return 0
	}
}

func findTerms(norm string, terms []string) []string {
	found := make([]string, 0)
	seen := map[string]bool{}
	for _, term := range terms {
		term = NormalizeText(term)
		if term == "" || seen[term] {
			continue
		}
		if containsNormalizedTerm(norm, term) {
			seen[term] = true
			found = append(found, term)
		}
	}
	return found
}

func containsNormalizedTerm(norm, term string) bool {
	return regexp.MustCompile(`(?:^|\s)`+regexp.QuoteMeta(term)+`(?:\s|$)`).FindStringIndex(norm) != nil
}

func findRegexTerms(norm string, patterns map[string]*regexp.Regexp) []string {
	found := make([]string, 0)
	for term, pattern := range patterns {
		if pattern.MatchString(norm) {
			found = append(found, term)
		}
	}
	sort.Strings(found)
	return found
}

func realGameTermPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"abc":                   regexp.MustCompile(`(?:^|\s)abc(?:\s|$)`),
		"broadcast":             regexp.MustCompile(`(?:^|\s)broadcast(?:\s|$)`),
		"cbs":                   regexp.MustCompile(`(?:^|\s)cbs(?:\s|$)`),
		"championship":          regexp.MustCompile(`(?:^|\s)championship(?:\s|$)`),
		"complete game":         regexp.MustCompile(`(?:^|\s)complete(?:\s+\w+){0,3}\s+game(?:\s|$)`),
		"condensed":             regexp.MustCompile(`(?:^|\s)condensed(?:\s|$)`),
		"divisional":            regexp.MustCompile(`(?:^|\s)div(?:isional)?(?:\s|$)`),
		"full game":             regexp.MustCompile(`(?:^|\s)full(?:\s+\w+){0,3}\s+game(?:\s|$)`),
		"game film":             regexp.MustCompile(`(?:^|\s)game\s+film(?:\s|$)`),
		"game of the week":      regexp.MustCompile(`(?:^|\s)game\s+of\s+the\s+week(?:\s|$)`),
		"half":                  regexp.MustCompile(`(?:^|\s)(?:1st|2nd|first|second)\s+half(?:\s|$)`),
		"hosts":                 regexp.MustCompile(`(?:^|\s)hosts(?:\s|$)`),
		"mnf":                   regexp.MustCompile(`(?:^|\s)mnf(?:\s|$)`),
		"monday night football": regexp.MustCompile(`(?:^|\s)monday\s+night\s+football(?:\s|$)`),
		"nbc":                   regexp.MustCompile(`(?:^|\s)nbc(?:\s|$)`),
		"nfl":                   regexp.MustCompile(`(?:^|\s)nfl(?:\s|$)`),
		"part":                  regexp.MustCompile(`(?:^|\s)(?:pt|part)\s+\d+(?:\s|$)`),
		"playoffs":              regexp.MustCompile(`(?:^|\s)playoffs?(?:\s|$)`),
		"radio":                 regexp.MustCompile(`(?:^|\s)radio(?:\s|$)`),
		"radio broadcast":       regexp.MustCompile(`(?:^|\s)radio\s+broadcast(?:\s|$)`),
		"super bowl":            regexp.MustCompile(`(?:^|\s)super\s+bowl(?:\s|$)`),
		"vhs":                   regexp.MustCompile(`(?:^|\s)vhs(?:\s|$)`),
		"wild card":             regexp.MustCompile(`(?:^|\s)wild\s+card(?:\s|$)`),
		"wildcard":              regexp.MustCompile(`(?:^|\s)wildcard(?:\s|$)`),
		"visit":                 regexp.MustCompile(`(?:^|\s)visit(?:s)?(?:\s|$)`),
	}
}

func highlightTermPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"highlight":  regexp.MustCompile(`(?:^|\s)highlight(?:s)?(?:\s|$)`),
		"highlights": regexp.MustCompile(`(?:^|\s)highlights(?:\s|$)`),
	}
}

func containsAnyExact(values []string, wanted []string) bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	for _, value := range wanted {
		if set[value] {
			return true
		}
	}
	return false
}
