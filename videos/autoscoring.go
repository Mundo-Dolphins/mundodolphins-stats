package videos

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// GameMetadata holds extracted game info for query building and scoring.
type GameMetadata struct {
	GameID   string
	DateISO  string    // "1969-09-14"
	GameDate time.Time // parsed date
	Year     int
	Week     string
	Home     TeamAliases
	Away     TeamAliases
}

// ScoreBreakdown details how a candidate's score was calculated.
type ScoreBreakdown struct {
	TeamsMatch       int `json:"teams_match"`
	TeamsOrder       int `json:"teams_order"`
	DateFull         int `json:"date_full"`
	DateYear         int `json:"date_year"`
	Week             int `json:"week,omitempty"`
	Duration         int `json:"duration"`
	Views            int `json:"views"`
	Embeddable       int `json:"embeddable"`
	ContentType      int `json:"content_type"`
	TrustedChannel   int `json:"trusted_channel"`
	MultiGamePenalty int `json:"multi_game_penalty"`
	RiskyTextPenalty int `json:"risky_text_penalty"`
	SingleGameBonus  int `json:"single_game_bonus"`
	Total            int `json:"total"`
}

// DateMatchType describes how well a video matches the game date.
type DateMatchType string

const (
	DateMatchFull     DateMatchType = "full_date"
	DateMatchYearOnly DateMatchType = "year_only"
	DateMatchNone     DateMatchType = "none"
	DateMatchWrong    DateMatchType = "wrong_year"
)

// TeamOrderMatch describes whether teams appear in correct order.
type TeamOrderMatch string

const (
	TeamOrderCorrect TeamOrderMatch = "correct"
	TeamOrderReverse TeamOrderMatch = "reverse"
	TeamOrderNone    TeamOrderMatch = "none"
)

// ScoredCandidate pairs a video candidate with its scoring details.
type ScoredCandidate struct {
	Candidate      youtubeapi.VideoCandidate
	Score          ScoreBreakdown
	MatchedTeams   bool
	MatchedDate    DateMatchType
	MatchedWeek    DateMatchType
	Classification ClassificationResult
	Discarded      bool
	DiscardReason  string
}

// --- Text normalization ---

// NormalizeText lowercases and removes non-alphanumeric chars (except spaces).
func NormalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- Team matching ---

// ContainsTeamReference checks if text contains any alias of the team.
func ContainsTeamReference(text string, team TeamAliases) bool {
	norm := NormalizeText(text)
	for _, token := range team.AllTokens() {
		token = NormalizeText(token)
		if token == "" {
			continue
		}
		if len(token) <= 3 {
			if containsNormalizedTerm(norm, token) {
				return true
			}
			continue
		}
		if strings.Contains(norm, token) {
			return true
		}
	}
	return false
}

// ContainsBothTeams checks if title or description references both teams.
func ContainsBothTeams(title, description string, home, away TeamAliases) bool {
	combined := title + " " + description
	return ContainsTeamReference(combined, home) && ContainsTeamReference(combined, away)
}

// DetectTeamOrder checks if teams appear in the expected vs order.
// For a game where away plays at home, we expect patterns like:
//   - "Away vs Home", "Away at Home", "Away - Home"

func DetectTeamOrder(text string, home, away TeamAliases) TeamOrderMatch {
	norm := NormalizeText(text)

	// Find positions of teams in text
	awayPos := findTeamPosition(norm, away)
	homePos := findTeamPosition(norm, home)

	if awayPos < 0 || homePos < 0 {
		return TeamOrderNone
	}

	// Standard order: away team first, home team second (Away vs Home)
	if awayPos < homePos {
		return TeamOrderCorrect
	}
	return TeamOrderReverse
}

func findTeamPosition(text string, team TeamAliases) int {
	best := -1
	for _, token := range team.AllTokens() {
		nt := NormalizeText(token)
		idx := strings.Index(text, nt)
		if idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

// --- Date matching ---

// dateVariantPatterns lists regex patterns for matching dates in text.
var monthNames = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4,
	"may": 5, "june": 6, "july": 7, "august": 8,
	"september": 9, "october": 10, "november": 11, "december": 12,
	"jan": 1, "feb": 2, "mar": 3, "apr": 4,
	"jun": 6, "jul": 7, "aug": 8, "sep": 9, "sept": 9,
	"oct": 10, "nov": 11, "dec": 12,
}

var (
	monthNameAlternation  = `january|february|march|april|may|june|july|august|september|jan|feb|mar|apr|jun|jul|aug|sep|sept|october|november|december|oct|nov|dec`
	monthDayYearRe        = regexp.MustCompile(`(?i)\b(` + monthNameAlternation + `)\.?\s+(\d{1,2})(?:st|nd|rd|th)?[,]?\s+(19[5-9]\d|20[0-4]\d)\b`)
	dayMonthYearRe        = regexp.MustCompile(`(?i)\b(\d{1,2})(?:st|nd|rd|th)?\s+(` + monthNameAlternation + `)\.?,?\s+(19[5-9]\d|20[0-4]\d)\b`)
	yearMonthDayRe        = regexp.MustCompile(`(?i)\b(19[5-9]\d|20[0-4]\d)[-/\s]+(0?[1-9]|1[0-2])[-/\s]+(0?[1-9]|[12]\d|3[01])(?:[-/\s]+\d{2})?\b`)
	monthDayYearNumericRe = regexp.MustCompile(`(?i)\b(0?[1-9]|1[0-2])[-/\s]+(0?[1-9]|[12]\d|3[01])[-/\s]+(19[5-9]\d|20[0-4]\d)\b`)
	monthDayShortYearRe   = regexp.MustCompile(`(?i)\b(0?[1-9]|1[0-2])[-/](0?[1-9]|[12]\d|3[01])[-/](\d{2})\b`)
)

// DetectDateMatch checks if text contains the game date.
func DetectDateMatch(text string, gameDate time.Time) DateMatchType {
	return detectDateMatch(text, gameDate, nil)
}

func detectDateMatch(text string, gameDate time.Time, alternateYearOnly []int) DateMatchType {
	norm := strings.ToLower(text)
	year := gameDate.Year()
	month := int(gameDate.Month())
	day := gameDate.Day()

	yearStr := fmt.Sprintf("%d", year)
	allowedYearOnly := map[string]bool{yearStr: true}
	for _, alternateYear := range alternateYearOnly {
		if alternateYear > 0 && alternateYear != year {
			allowedYearOnly[fmt.Sprintf("%d", alternateYear)] = true
		}
	}

	// Check exact ISO date: 1969-09-14 or 1969/09/14
	isoDate := fmt.Sprintf("%d-%02d-%02d", year, month, day)
	isoDateSlash := fmt.Sprintf("%d/%02d/%02d", year, month, day)
	if strings.Contains(norm, isoDate) || strings.Contains(norm, isoDateSlash) {
		return DateMatchFull
	}

	// Check numeric broadcast-title dates: "1980 09 28 80", "1980 12-08-80", "1-4-86".
	if dates := extractNumericDates(norm); len(dates) > 0 {
		// The first date found decides.
		found := dates[0]
		if found.Year == year && found.Month == month && found.Day == day {
			return DateMatchFull
		}
		return DateMatchWrong
	}

	// Check month name patterns: "Sep 14, 1969", "September 14, 1969", "14 Sep 1969".
	// A complete date from the same year but with a different month/day is a conflicting date.
	if dates := extractMonthNameDates(norm); len(dates) > 0 {
		// The first date found decides.
		found := dates[0]
		if found.Year == year && found.Month == month && found.Day == day {
			return DateMatchFull
		}
		return DateMatchWrong
	}

	// Check year only
	for allowedYear := range allowedYearOnly {
		if strings.Contains(norm, allowedYear) {
			return DateMatchYearOnly
		}
	}

	// Check for an explicit different year (4-digit number that's a plausible year)
	yearRe := regexp.MustCompile(`\b(19[5-9]\d|20[0-4]\d)\b`)
	matches := yearRe.FindAllString(norm, -1)
	for _, m := range matches {
		if !allowedYearOnly[m] {
			return DateMatchWrong
		}
	}

	return DateMatchNone
}

func DetectGameDateMatch(text string, game GameMetadata) DateMatchType {
	if game.GameDate.IsZero() {
		return DateMatchNone
	}
	alternateYears := []int(nil)
	if !game.GameDate.IsZero() && game.Year > 0 && game.Year != game.GameDate.Year() && game.GameDate.Month() <= time.February {
		alternateYears = []int{game.Year}
	}
	return detectDateMatch(text, game.GameDate, alternateYears)
}

type detectedDate struct {
	Year  int
	Month int
	Day   int
}

func extractMonthNameDates(text string) []detectedDate {
	var dates []detectedDate
	for _, match := range monthDayYearRe.FindAllStringSubmatch(text, -1) {
		dates = appendDetectedDate(dates, match[3], match[1], match[2])
	}
	for _, match := range dayMonthYearRe.FindAllStringSubmatch(text, -1) {
		dates = appendDetectedDate(dates, match[3], match[2], match[1])
	}
	return dates
}

func extractNumericDates(text string) []detectedDate {
	var dates []detectedDate
	for _, match := range yearMonthDayRe.FindAllStringSubmatch(text, -1) {
		dates = appendDetectedNumericDate(dates, match[1], match[2], match[3])
	}
	for _, match := range monthDayYearNumericRe.FindAllStringSubmatch(text, -1) {
		dates = appendDetectedNumericDate(dates, match[3], match[1], match[2])
	}
	for _, match := range monthDayShortYearRe.FindAllStringSubmatch(text, -1) {
		dates = appendDetectedNumericDate(dates, expandShortYear(match[3]), match[1], match[2])
	}
	return dates
}

func expandShortYear(yearText string) string {
	year, err := strconv.Atoi(yearText)
	if err != nil {
		return yearText
	}
	if year <= 49 {
		return fmt.Sprintf("20%02d", year)
	}
	return fmt.Sprintf("19%02d", year)
}

func appendDetectedDate(dates []detectedDate, yearText, monthText, dayText string) []detectedDate {
	year, yearErr := strconv.Atoi(yearText)
	day, dayErr := strconv.Atoi(dayText)
	month, ok := monthNames[strings.ToLower(strings.TrimSuffix(monthText, "."))]
	if yearErr != nil || dayErr != nil || !ok || day < 1 || day > 31 {
		return dates
	}
	return append(dates, detectedDate{Year: year, Month: month, Day: day})
}

func appendDetectedNumericDate(dates []detectedDate, yearText, monthText, dayText string) []detectedDate {
	year, yearErr := strconv.Atoi(yearText)
	month, monthErr := strconv.Atoi(monthText)
	day, dayErr := strconv.Atoi(dayText)
	if yearErr != nil || monthErr != nil || dayErr != nil || month < 1 || month > 12 || day < 1 || day > 31 {
		return dates
	}
	return append(dates, detectedDate{Year: year, Month: month, Day: day})
}

// --- Query building ---

// BuildPrimaryQuery builds the canonical search query: "{away} - {home} {date}".
// NFL convention: away team listed first.
func BuildPrimaryQuery(game GameMetadata) string {
	return fmt.Sprintf("%s - %s %s", game.Away.FullName, game.Home.FullName, game.DateISO)
}

// BuildFallbackQueries returns secondary query variants.
// NFL convention: away team listed first.
func BuildFallbackQueries(game GameMetadata) []string {
	yearStr := fmt.Sprintf("%d", game.Year)
	return []string{
		fmt.Sprintf("%s vs %s %s", game.Away.FullName, game.Home.FullName, game.DateISO),
		fmt.Sprintf("%s vs %s %s", game.Away.Mascot, game.Home.Mascot, yearStr),
		fmt.Sprintf("%s vs %s %s", game.Away.City, game.Home.City, yearStr),
	}
}

// ExtractGameMetadata derives game metadata from a GameRef.
func ExtractGameMetadata(game GameRef) GameMetadata {
	dateISO := gameDateISO(game)
	home := LookupTeam(game.HomeTeam)
	away := LookupTeam(game.AwayTeam)

	var gameDate time.Time
	if len(dateISO) == 10 {
		gameDate, _ = time.Parse("2006-01-02", dateISO)
	}

	return GameMetadata{
		GameID:   game.GameID,
		DateISO:  dateISO,
		GameDate: gameDate,
		Year:     game.Year,
		Week:     game.Week,
		Home:     home,
		Away:     away,
	}
}

// --- Scoring ---

// ScoreCandidate applies the scoring rules and returns a detailed result.
func ScoreCandidate(game GameMetadata, c youtubeapi.VideoCandidate, minDurationSec int) ScoredCandidate {
	title := c.Title
	desc := c.Description
	combined := title + " " + desc

	policy := classificationPolicyForCandidate(c, minDurationSec)
	if minDurationSec > 0 {
		policy.MinDurationSec = minDurationSec
	}
	classification := ClassifyVideoForGame(game, c, policy)
	result := ScoredCandidate{Candidate: c, Classification: classification}

	// Rule 1: Both teams must appear
	if !ContainsBothTeams(title, desc, game.Home, game.Away) {
		result.Discarded = true
		result.DiscardReason = "missing team reference"
		result.MatchedTeams = false
		return result
	}
	result.MatchedTeams = true
	result.Score.TeamsMatch = 50

	// Rule 2: Team order bonus
	order := DetectTeamOrder(combined, game.Home, game.Away)
	switch order {
	case TeamOrderCorrect:
		result.Score.TeamsOrder = 20
	case TeamOrderReverse:
		result.Score.TeamsOrder = 10
	}

	// Rule 3: Date matching
	dateMatch := DateMatchNone
	if !game.GameDate.IsZero() {
		dateMatch = DetectGameDateMatch(combined, game)
	}
	result.MatchedDate = dateMatch
	switch dateMatch {
	case DateMatchFull:
		result.Score.DateFull = 30
	case DateMatchYearOnly:
		result.Score.DateYear = 10
	case DateMatchWrong:
		result.Discarded = true
		result.DiscardReason = "wrong year in video"
		return result
	}
	if broadcastWeekdayMismatch(game, combined) {
		result.Discarded = true
		result.DiscardReason = "broadcast weekday does not match game date"
		return result
	}
	weekMatch := DetectGameWeekMatch(combined, game)
	result.MatchedWeek = weekMatch
	switch weekMatch {
	case DateMatchFull:
		result.Score.Week = 20
	case DateMatchWrong:
		result.Discarded = true
		result.DiscardReason = "wrong week in video"
		return result
	}

	// Rule 4: Duration scoring (gradual, favoring longer)
	durationSec := int(c.Duration.Seconds())
	if minDurationSec > 0 && durationSec < minDurationSec {
		result.Discarded = true
		result.DiscardReason = fmt.Sprintf("duration %ds below minimum %ds", durationSec, minDurationSec)
		return result
	}
	result.Score.Duration = scoreDuration(durationSec)

	// Rule 5: View count (logarithmic scale)
	result.Score.Views = scoreViews(c.ViewCount)

	// Rule 6: Embeddable bonus
	if c.Embeddable {
		result.Score.Embeddable = 5
	}

	// Rule 7: Content type bonus
	result.Score.ContentType = scoreContentType(combined)

	// Rule 8: trusted channel bonus
	result.Score.TrustedChannel = scoreTrustedChannel(c.Channel)

	// Rule 9: risky editorial terms
	result.Score.RiskyTextPenalty = scoreRiskyText(combined)
	if result.Score.RiskyTextPenalty <= -80 {
		result.Discarded = true
		result.DiscardReason = "risky non-game video language"
		return result
	}

	// Rule 10: Multi-game detection (hard filter and penalty)
	multiSignal := DetectMultiGameVideo(title, desc, game)
	if multiSignal.IsMultiGame {
		// Hard discard: 3+ distinct dates OR 3+ distinct opponents OR 3+ week markers
		if multiSignal.DistinctDates >= 3 || multiSignal.DistinctOpponents >= 3 || multiSignal.WeekMarkerCount >= 3 {
			result.Discarded = true
			result.DiscardReason = fmt.Sprintf(
				"multi-game compilation: dates=%d opponents=%d weeks=%d matchups=%d (%s)",
				multiSignal.DistinctDates, multiSignal.DistinctOpponents,
				multiSignal.WeekMarkerCount, multiSignal.DetectedGames, multiSignal.Reason)
			return result
		}
		// Soft multi-game: strong penalty but not auto-discard
		result.Score.MultiGamePenalty = -80
	}

	// Rule 11: Single-game confidence
	sgc := ComputeSingleGameConfidence(title, desc, game)
	result.Score.SingleGameBonus = sgc.Score

	// Calculate total
	result.Score.Total = result.Score.TeamsMatch +
		result.Score.TeamsOrder +
		result.Score.DateFull +
		result.Score.DateYear +
		result.Score.Week +
		result.Score.Duration +
		result.Score.Views +
		result.Score.Embeddable +
		result.Score.ContentType +
		result.Score.TrustedChannel +
		result.Score.MultiGamePenalty +
		result.Score.RiskyTextPenalty +
		result.Score.SingleGameBonus

	// Floor: if total is negative, discard
	if result.Score.Total < 0 {
		result.Discarded = true
		result.DiscardReason = fmt.Sprintf("negative score %d after multi-game penalty", result.Score.Total)
	}

	return result
}

func classificationPolicyForCandidate(c youtubeapi.VideoCandidate, minDurationSec int) ClassificationPolicy {
	policy := DefaultClassificationPolicy()
	if minDurationSec > 0 {
		policy.MinDurationSec = minDurationSec
	}
	role := ClassifyContentRole(c.Title, c.Description, c.Channel, int(c.Duration.Seconds()))
	if role == ContentRoleOfficialHighlight || role == ContentRoleHighlight {
		policy.AllowHighlights = true
		policy.HighlightsMinDurationSec = 300
		if policy.MinDurationSec > policy.HighlightsMinDurationSec {
			policy.MinDurationSec = policy.HighlightsMinDurationSec
		}
		if policy.AcceptThreshold > 95 {
			policy.AcceptThreshold = 95
		}
		if role == ContentRoleHighlight && policy.AcceptThreshold > 80 {
			policy.AcceptThreshold = 80
		}
	}
	return policy
}

// scoreDuration returns points based on video duration.
// Intentionally low: duration must NOT compensate matching errors.
//
//	>=7200s (2h)  → 15 (full game broadcast)
//	>=3600s (1h)  → 12 (condensed game)
//	>=1800s (30m) → 9  (extended highlights)
//	>=900s  (15m) → 6  (highlights)
//	>=600s  (10m) → 3  (short clip)
//	<600s         → 0
func scoreDuration(seconds int) int {
	switch {
	case seconds >= 7200:
		return 15
	case seconds >= 3600:
		return 12
	case seconds >= 1800:
		return 9
	case seconds >= 900:
		return 6
	case seconds >= 600:
		return 3
	default:
		return 0
	}
}

// scoreViews returns points using a logarithmic scale.
// Intentionally low: views must NOT compensate matching errors.
//
//	>=1M   → 5
//	>=100K → 4
//	>=10K  → 3
//	>=1K   → 2
//	>=100  → 1
//	<100   → 0
func scoreViews(views uint64) int {
	if views == 0 {
		return 0
	}
	logVal := math.Log10(float64(views))
	switch {
	case logVal >= 6: // 1M+
		return 5
	case logVal >= 5: // 100K+
		return 4
	case logVal >= 4: // 10K+
		return 3
	case logVal >= 3: // 1K+
		return 2
	case logVal >= 2: // 100+
		return 1
	default:
		return 0
	}
}

// scoreContentType gives bonus for indicative keywords.
func scoreContentType(text string) int {
	norm := NormalizeText(text)
	bonus := 0
	if strings.Contains(norm, "full game") || strings.Contains(norm, "full match") || strings.Contains(norm, "complete game") {
		bonus += 15
	}
	if strings.Contains(norm, "highlights") || strings.Contains(norm, "highlight") {
		bonus += 8
	}
	if strings.Contains(norm, "condensed") {
		bonus += 10
	}
	if strings.Contains(norm, "game film") || strings.Contains(norm, "broadcast") {
		bonus += 10
	}
	if bonus == 0 && len(findRegexTerms(norm, realGameTermPatterns())) > 0 {
		bonus += 10
	}
	// Cap content type bonus
	if bonus > 15 {
		bonus = 15
	}
	return bonus
}

func scoreTrustedChannel(channel string) int {
	norm := NormalizeText(channel)
	if norm == "" {
		return 0
	}
	for _, token := range []string{"nfl", "nfl throwback", "miami dolphins", "dolphins", "official"} {
		if strings.Contains(norm, NormalizeText(token)) {
			return 10
		}
	}
	return 0
}

func scoreRiskyText(text string) int {
	norm := NormalizeText(text)
	penalty := 0
	for _, token := range []string{
		"all teams",
		"every team",
		"every teams",
		"live",
		"preview",
		"week preview",
		"stream",
		"streams",
		"top 10",
		"prediction",
		"predictions",
		"betting",
		"odds",
		"simulation",
		"madden",
		"reaction",
		"podcast",
		"press conference",
	} {
		if strings.Contains(norm, NormalizeText(token)) {
			penalty -= 40
		}
	}
	if penalty < -120 {
		return -120
	}
	return penalty
}

func confidenceFromScore(score int) string {
	switch {
	case score >= 130:
		return "high"
	case score >= 95:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "unknown"
	}
}

// SelectBestCandidate picks the highest-scored non-discarded candidate.
// Tiebreaking: embeddable > duration > views > title alphabetical.
func SelectBestCandidate(candidates []ScoredCandidate) *ScoredCandidate {
	valid := make([]ScoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !c.Discarded {
			valid = append(valid, c)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	sort.Slice(valid, func(i, j int) bool {
		si, sj := valid[i].Score.Total, valid[j].Score.Total
		if si != sj {
			return si > sj
		}
		// Tiebreak 1: embeddable
		ei, ej := boolToInt(valid[i].Candidate.Embeddable), boolToInt(valid[j].Candidate.Embeddable)
		if ei != ej {
			return ei > ej
		}
		// Tiebreak 2: duration
		di, dj := valid[i].Candidate.Duration, valid[j].Candidate.Duration
		if di != dj {
			return di > dj
		}
		// Tiebreak 3: views
		vi, vj := valid[i].Candidate.ViewCount, valid[j].Candidate.ViewCount
		if vi != vj {
			return vi > vj
		}
		// Tiebreak 4: title alphabetical
		return strings.ToLower(valid[i].Candidate.Title) < strings.ToLower(valid[j].Candidate.Title)
	})
	return &valid[0]
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
