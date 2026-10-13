package videos

import (
	"strings"
	"time"
)

type Scorer struct {
	MinDurationSec int
	ScoreThreshold int
	BlacklistWords []string
}

type LegacyVideoKind string

const (
	LegacyVideoKindFullGame  LegacyVideoKind = "full_game"
	LegacyVideoKindRadio     LegacyVideoKind = "radio"
	LegacyVideoKindPartial   LegacyVideoKind = "partial"
	LegacyVideoKindHighlight LegacyVideoKind = "highlight"
	LegacyVideoKindStory     LegacyVideoKind = "story"
	LegacyVideoKindUnknown   LegacyVideoKind = "unknown"
)

type LegacyScoreBreakdown struct {
	Teams            int `json:"teams,omitempty"`
	Date             int `json:"date,omitempty"`
	Week             int `json:"week,omitempty"`
	BroadcastWeekday int `json:"broadcast_weekday,omitempty"`
	ContentType      int `json:"content_type,omitempty"`
	Duration         int `json:"duration,omitempty"`
	Rejected         int `json:"rejected,omitempty"`
	Total            int `json:"total"`
}

type LegacyCandidateEvaluation struct {
	Decision       ClassificationDecision `json:"decision"`
	Reason         string                 `json:"reason"`
	VideoKind      LegacyVideoKind        `json:"video_kind"`
	Score          int                    `json:"score"`
	Breakdown      LegacyScoreBreakdown   `json:"breakdown"`
	DateMatch      DateMatchType          `json:"date_match,omitempty"`
	WeekMatch      DateMatchType          `json:"week_match,omitempty"`
	DurationSec    int                    `json:"duration_seconds,omitempty"`
	StrongTemporal bool                   `json:"strong_temporal,omitempty"`
}

func NewScorer(minDurSec, threshold int) Scorer {
	return Scorer{
		MinDurationSec: minDurSec,
		ScoreThreshold: threshold,
		BlacklistWords: []string{"preview", "reaction", "madden", "simulation", "betting", "prediction", "predictions", "podcast", "press conference"},
	}
}

func (s Scorer) IsDisallowed(c Candidate) bool {
	title := strings.ToLower(c.Title)
	for _, word := range s.BlacklistWords {
		if strings.Contains(title, word) {
			return true
		}
	}
	return false
}

func (s Scorer) Score(game GameRef, c Candidate) int {
	return s.Evaluate(game, c).Score
}

func (s Scorer) Evaluate(game GameRef, c Candidate) LegacyCandidateEvaluation {
	text := strings.ToLower(strings.TrimSpace(c.Title + " " + c.Snippet))
	durationSec := candidateDurationSec(c)
	if s.MinDurationSec <= 0 {
		s.MinDurationSec = 300
	}
	if s.ScoreThreshold <= 0 {
		s.ScoreThreshold = 60
	}
	eval := LegacyCandidateEvaluation{
		Decision:    DecisionReview,
		VideoKind:   classifyLegacyVideoKind(text, durationSec),
		DurationSec: durationSec,
	}
	if durationSec > 0 && durationSec < s.MinDurationSec {
		return legacyReject(eval, "duration")
	}
	if s.IsDisallowed(c) {
		return legacyReject(eval, "disallowed")
	}
	if titleHasConflictingOpponent(game, c.Title) {
		return legacyReject(eval, "wrong_opponent")
	}
	if containsTeam(text, game.AwayTeam) && containsTeam(text, game.HomeTeam) {
		eval.Breakdown.Teams = 30
	} else {
		return legacyReject(eval, "missing_teams")
	}
	if isLegacyMultiGameCandidate(text) {
		return legacyReject(eval, "multi_game")
	}
	switch legacyDateMatch(game, text) {
	case DateMatchFull:
		eval.DateMatch = DateMatchFull
		eval.Breakdown.Date = 30
	case DateMatchYearOnly:
		eval.DateMatch = DateMatchYearOnly
		eval.Breakdown.Date = 15
	case DateMatchWrong:
		eval.DateMatch = DateMatchWrong
		eval.Breakdown.Date = -120
		return legacyReject(eval, "wrong_date")
	default:
		eval.DateMatch = DateMatchNone
	}
	switch legacyWeekMatch(game, text) {
	case DateMatchFull:
		eval.WeekMatch = DateMatchFull
		eval.Breakdown.Week = 15
	case DateMatchWrong:
		eval.WeekMatch = DateMatchWrong
		eval.Breakdown.Week = -60
		return legacyReject(eval, "wrong_week")
	default:
		eval.WeekMatch = DateMatchNone
	}
	if broadcastWeekdayMismatch(ExtractGameMetadata(game), text) {
		eval.Breakdown.BroadcastWeekday = -80
		return legacyReject(eval, "wrong_broadcast_weekday")
	}
	if contentScore := realGameContentScore(text); contentScore > 0 {
		eval.Breakdown.ContentType = contentScore
	}
	if s.MinDurationSec > 0 && durationSec >= s.MinDurationSec {
		eval.Breakdown.Duration = legacyDurationScore(durationSec)
	}
	eval.Score = legacyScoreTotal(eval.Breakdown)
	eval.Breakdown.Total = eval.Score
	eval.StrongTemporal = eval.DateMatch == DateMatchFull || eval.WeekMatch == DateMatchFull
	if !hasLegacyTemporalEvidence(game, eval.DateMatch, eval.WeekMatch) {
		return legacyReject(eval, "missing_temporal")
	}
	if eval.Score <= 0 {
		return legacyReject(eval, "score_non_positive")
	}
	if shouldReviewLegacyKind(eval.VideoKind) {
		eval.Decision = DecisionReview
		eval.Reason = "review_" + string(eval.VideoKind)
		return eval
	}
	if eval.Score >= s.ScoreThreshold && (eval.StrongTemporal || legacyAllowsYearOnlyAutoAccept(game)) {
		eval.Decision = DecisionAccept
		eval.Reason = "auto_accept"
		return eval
	}
	eval.Decision = DecisionReview
	eval.Reason = "review_score"
	return eval
}

func realGameContentScore(text string) int {
	terms := findRegexTerms(NormalizeText(text), realGameTermPatterns())
	if len(terms) == 0 {
		return 0
	}
	if containsAnyExact(terms, []string{"full game", "complete game", "broadcast", "game film"}) {
		return 10
	}
	return 8
}

func legacyDateMatch(game GameRef, text string) DateMatchType {
	meta := ExtractGameMetadata(game)
	if !meta.GameDate.IsZero() {
		return DetectGameDateMatch(text, meta)
	}
	if game.Year > 0 && strings.Contains(text, strconvItoa(game.Year)) {
		return DateMatchYearOnly
	}
	return DateMatchNone
}

func legacyWeekMatch(game GameRef, text string) DateMatchType {
	want := strings.TrimSpace(game.Week)
	if want == "" {
		return DateMatchNone
	}
	if isPreseasonWeek(want) {
		return DetectGameWeekMatch(text, GameMetadata{Week: want})
	}
	if isPlayoffWeek(want) {
		if containsPlayoffTemporalTerm(text) {
			return DateMatchFull
		}
		if len(countWeekMarkers(text)) > 0 {
			return DateMatchWrong
		}
		return DateMatchNone
	}
	wantInt, ok := parseWeekNumber(want)
	if !ok {
		return DateMatchNone
	}
	if containsPlayoffTemporalTerm(text) {
		return DateMatchWrong
	}
	weeks := countWeekMarkers(text)
	if len(weeks) == 0 {
		return DateMatchNone
	}
	for _, week := range weeks {
		got, ok := parseWeekNumber(week)
		if ok && got == wantInt {
			return DateMatchFull
		}
	}
	return DateMatchWrong
}

func isPlayoffWeek(week string) bool {
	return containsNormalizedTerm(NormalizeText(week), "playoffs")
}

func containsPlayoffTemporalTerm(text string) bool {
	norm := NormalizeText(text)
	for _, term := range []string{
		"playoff",
		"playoffs",
		"wild card",
		"wildcard",
		"divisional",
		"championship",
		"super bowl",
	} {
		if containsNormalizedTerm(norm, term) {
			return true
		}
	}
	return false
}

func broadcastWeekdayMismatch(game GameMetadata, text string) bool {
	if game.GameDate.IsZero() {
		return false
	}
	days := detectBroadcastWeekdays(text)
	if len(days) == 0 {
		return false
	}
	return !days[game.GameDate.Weekday()]
}

func detectBroadcastWeekdays(text string) map[time.Weekday]bool {
	norm := NormalizeText(text)
	days := map[time.Weekday]bool{}
	for _, signal := range []struct {
		weekday time.Weekday
		terms   []string
	}{
		{time.Monday, []string{"mnf", "monday night football", "monday night"}},
		{time.Thursday, []string{"tnf", "thursday night football", "thursday night"}},
		{time.Sunday, []string{"snf", "sunday night football", "sunday night"}},
		{time.Saturday, []string{"saturday night football", "saturday night"}},
		{time.Friday, []string{"friday night football", "friday night"}},
	} {
		for _, term := range signal.terms {
			if containsNormalizedTerm(norm, term) {
				days[signal.weekday] = true
				break
			}
		}
	}
	return days
}

func candidateDurationSec(c Candidate) int {
	if c.DurationSec > 0 {
		return c.DurationSec
	}
	return ParseDurationToSeconds(c.DurationText)
}

func classifyLegacyVideoKind(text string, durationSec int) LegacyVideoKind {
	norm := NormalizeText(text)
	if containsNormalizedTerm(norm, "radio broadcast") || containsNormalizedTerm(norm, "radio edit") || containsNormalizedTerm(norm, "radio merged") || containsNormalizedTerm(norm, "radio") {
		return LegacyVideoKindRadio
	}
	if containsNormalizedTerm(norm, "full game") || containsNormalizedTerm(norm, "complete game") || containsNormalizedTerm(norm, "broadcast") {
		return LegacyVideoKindFullGame
	}
	if containsNormalizedTerm(norm, "partial") || containsNormalizedTerm(norm, "part") || containsNormalizedTerm(norm, "pt") || strings.Contains(norm, "1st half") || strings.Contains(norm, "2nd half") {
		return LegacyVideoKindPartial
	}
	if containsNormalizedTerm(norm, "highlight") || containsNormalizedTerm(norm, "highlights") || containsNormalizedTerm(norm, "nfl primetime") || containsNormalizedTerm(norm, "game of the week") {
		return LegacyVideoKindHighlight
	}
	if containsNormalizedTerm(norm, "story") || containsNormalizedTerm(norm, "controversy") || containsNormalizedTerm(norm, "worst") || strings.Contains(norm, "the time") {
		return LegacyVideoKindStory
	}
	if durationSec >= 5400 {
		return LegacyVideoKindFullGame
	}
	return LegacyVideoKindUnknown
}

func isLegacyMultiGameCandidate(text string) bool {
	norm := NormalizeText(text)
	for _, term := range []string{"season film", "team film", "yearbook", "all games", "every game"} {
		if containsNormalizedTerm(norm, term) {
			return true
		}
	}
	uniqueDates := map[detectedDate]bool{}
	for _, date := range extractNumericDates(text) {
		uniqueDates[date] = true
	}
	for _, date := range extractMonthNameDates(text) {
		uniqueDates[date] = true
	}
	if len(uniqueDates) > 1 {
		return true
	}
	return len(countWeekMarkers(text)) > 1
}

func shouldReviewLegacyKind(kind LegacyVideoKind) bool {
	switch kind {
	case LegacyVideoKindPartial, LegacyVideoKindHighlight, LegacyVideoKindStory, LegacyVideoKindUnknown:
		return true
	default:
		return false
	}
}

func legacyAllowsYearOnlyAutoAccept(game GameRef) bool {
	return strings.TrimSpace(game.GameID) == "" && strings.TrimSpace(game.DateRaw) == ""
}

func legacyReject(eval LegacyCandidateEvaluation, reason string) LegacyCandidateEvaluation {
	eval.Decision = DecisionReject
	eval.Reason = reason
	eval.Breakdown.Rejected = -100
	eval.Score = legacyScoreTotal(eval.Breakdown)
	eval.Breakdown.Total = eval.Score
	return eval
}

func legacyScoreTotal(b LegacyScoreBreakdown) int {
	return b.Teams + b.Date + b.Week + b.BroadcastWeekday + b.ContentType + b.Duration + b.Rejected
}

func legacyDurationScore(seconds int) int {
	score := scoreDuration(seconds)
	if seconds >= 900 && score < 10 {
		return 10
	}
	return score
}

func parseWeekNumber(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, n > 0
}

func containsTeam(title string, team string) bool {
	return ContainsTeamReference(title, LookupTeam(team))
}

func titleHasConflictingOpponent(game GameRef, title string) bool {
	return CountDistinctOpponents(title, ExtractGameMetadata(game)) > 0
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	neg := false
	if value < 0 {
		neg = true
		value = -value
	}
	buf := make([]byte, 0, 12)
	for value > 0 {
		d := value % 10
		buf = append([]byte{byte('0' + d)}, buf...)
		value /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
