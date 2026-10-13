package videos

import (
	"fmt"
	"regexp"
	"strings"
)

// MultiGameSignal holds detection results for whether a video covers multiple games.
type MultiGameSignal struct {
	IsMultiGame       bool
	DetectedGames     int
	DistinctOpponents int
	DistinctDates     int
	HasWeekMarkers    bool
	WeekMarkerCount   int
	Reason            string
}

// GameMention represents a single game reference found in text.
type GameMention struct {
	Date            string // "1966-09-09" or "" if no date
	TeamA           string // first team mentioned
	TeamB           string // second team mentioned
	RawLine         string
	NormalizedTeamA string
	NormalizedTeamB string
}

// MentionContext describes how the current game is referenced.
type MentionContext string

const (
	MentionExactGame  MentionContext = "exact_game"
	MentionInSchedule MentionContext = "mention_in_schedule"
	MentionLooseTeam  MentionContext = "loose_team"
	MentionNotFound   MentionContext = "not_found"
)

// SingleGameConfidence quantifies how likely a video is about exactly one game.
type SingleGameConfidence struct {
	Score          int
	MentionContext MentionContext
	TotalMentions  int
	Reason         string
}

// --- Regex patterns ---

// matchupPattern matches lines like "Team A @ Team B", "Team A vs Team B", "Team A vs. Team B"
var matchupPattern = regexp.MustCompile(`(?i)([A-Z][A-Za-z. ]+(?:49ers|[A-Za-z]+))\s+(?:@|vs\.?|v\.?|at)\s+([A-Z][A-Za-z. ]+(?:49ers|[A-Za-z]+))`)

// datePattern matches ISO dates like 1966-09-09 or similar
var datePattern = regexp.MustCompile(`\b(19[5-9]\d|20[0-4]\d)[-/](0[1-9]|1[0-2])[-/](0[1-9]|[12]\d|3[01])\b`)

// weekPattern matches "Week 01", "Week 1", "week 02", "Wk 3", "W09", etc.
var weekPattern = regexp.MustCompile(`(?i)\b(?:week|wk|w)\s*0*(\d{1,2})\b`)

// seasonCompilationPattern matches season-level phrases.
var seasonCompilationPattern = regexp.MustCompile(`(?i)\b(?:season highlights?|full season|all highlights?|all games?|all teams?|every teams?|top\s*10\s+plays|season recap|season review|every game|complete season)\b`)

// datedMatchupPattern matches "DATE Team @ Team" or "Team @ Team DATE"
var datedMatchupPattern = regexp.MustCompile(
	`(?i)(?:` +
		`((?:19|20)\d{2}[-/]\d{2}[-/]\d{2})\s+(.+?)\s+(?:@|vs\.?|v\.?|at)\s+(.+?)(?:\s*$|\s*\n)` +
		`|` +
		`(.+?)\s+(?:@|vs\.?|v\.?|at)\s+(.+?)\s+((?:19|20)\d{2}[-/]\d{2}[-/]\d{2})` +
		`)`,
)

// --- Core detection functions ---

// DetectMultiGameVideo analyzes title and description for multi-game indicators.
func DetectMultiGameVideo(title, description string, game GameMetadata) MultiGameSignal {
	combined := title + "\n" + description
	signal := MultiGameSignal{}

	// Count distinct dates in description (not title)
	dates := CountDistinctDates(description)
	signal.DistinctDates = len(dates)

	// Count distinct opponents beyond the current game
	signal.DistinctOpponents = CountDistinctOpponents(combined, game)

	// Count week markers
	weeks := countWeekMarkers(combined)
	signal.WeekMarkerCount = len(weeks)
	signal.HasWeekMarkers = len(weeks) >= 2

	// Extract game mentions from description
	mentions := ExtractGameMentions(description)
	signal.DetectedGames = len(mentions)

	// Also count matchups without dates in the description
	allMatchups := matchupPattern.FindAllStringSubmatch(description, -1)
	if len(allMatchups) > signal.DetectedGames {
		signal.DetectedGames = len(allMatchups)
	}
	titleMatchups := matchupPattern.FindAllStringSubmatch(title, -1)
	if len(titleMatchups) > signal.DetectedGames {
		signal.DetectedGames = len(titleMatchups)
	}

	// Season compilation keywords
	hasSeasonKeywords := seasonCompilationPattern.MatchString(combined)
	hasDoubleheader := containsNormalizedTerm(NormalizeText(combined), "doubleheader")

	// Determine if multi-game
	reasons := make([]string, 0, 4)

	if signal.DistinctDates >= 3 {
		signal.IsMultiGame = true
		reasons = append(reasons, fmt.Sprintf("dates=%d", signal.DistinctDates))
	}

	if signal.DistinctOpponents >= 3 {
		signal.IsMultiGame = true
		reasons = append(reasons, fmt.Sprintf("opponents=%d", signal.DistinctOpponents))
	}

	if signal.WeekMarkerCount >= 3 {
		signal.IsMultiGame = true
		reasons = append(reasons, fmt.Sprintf("weeks=%d", signal.WeekMarkerCount))
	}

	if signal.DetectedGames >= 3 {
		signal.IsMultiGame = true
		reasons = append(reasons, fmt.Sprintf("matchups=%d", signal.DetectedGames))
	}

	if hasSeasonKeywords && (signal.DistinctDates >= 2 || signal.DistinctOpponents >= 2 || signal.DetectedGames >= 2) {
		signal.IsMultiGame = true
		reasons = append(reasons, "season_keywords")
	}
	if hasDoubleheader && (signal.DistinctOpponents >= 2 || signal.DetectedGames >= 2) {
		signal.IsMultiGame = true
		reasons = append(reasons, "doubleheader")
	}

	// Softer threshold: 2 dates + 2 opponents is already suspicious
	if signal.DistinctDates >= 2 && signal.DistinctOpponents >= 2 {
		signal.IsMultiGame = true
		if len(reasons) == 0 {
			reasons = append(reasons, fmt.Sprintf("dates=%d+opponents=%d", signal.DistinctDates, signal.DistinctOpponents))
		}
	}

	if len(reasons) > 0 {
		signal.Reason = strings.Join(reasons, ", ")
	}

	return signal
}

// ExtractGameMentions finds all game references (with dates) in text.
func ExtractGameMentions(text string) []GameMention {
	lines := strings.Split(text, "\n")
	var mentions []GameMention
	seen := make(map[string]bool) // dedup by "date-teamA-teamB"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := datedMatchupPattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			var date, teamA, teamB string
			if m[1] != "" {
				// DATE TeamA @ TeamB
				date = m[1]
				teamA = strings.TrimSpace(m[2])
				teamB = strings.TrimSpace(m[3])
			} else if m[6] != "" {
				// TeamA @ TeamB DATE
				teamA = strings.TrimSpace(m[4])
				teamB = strings.TrimSpace(m[5])
				date = m[6]
			}

			if teamA == "" || teamB == "" {
				continue
			}

			key := fmt.Sprintf("%s-%s-%s", date, NormalizeText(teamA), NormalizeText(teamB))
			if seen[key] {
				continue
			}
			seen[key] = true

			mentions = append(mentions, GameMention{
				Date:            date,
				TeamA:           teamA,
				TeamB:           teamB,
				RawLine:         line,
				NormalizedTeamA: NormalizeText(teamA),
				NormalizedTeamB: NormalizeText(teamB),
			})
		}
	}

	return mentions
}

// CountDistinctDates returns unique ISO-style dates found in text.
func CountDistinctDates(text string) []string {
	matches := datePattern.FindAllString(text, -1)
	unique := make(map[string]bool, len(matches))
	for _, m := range matches {
		unique[m] = true
	}
	result := make([]string, 0, len(unique))
	for d := range unique {
		result = append(result, d)
	}
	return result
}

// countWeekMarkers returns distinct week numbers found in text.
func countWeekMarkers(text string) []string {
	matches := weekPattern.FindAllStringSubmatch(text, -1)
	unique := make(map[string]bool, len(matches))
	for _, m := range matches {
		unique[m[1]] = true
	}
	result := make([]string, 0, len(unique))
	for w := range unique {
		result = append(result, w)
	}
	return result
}

// CountDistinctOpponents counts how many NFL teams (other than the two in the current game)
// are mentioned in the text.
func CountDistinctOpponents(text string, game GameMetadata) int {
	norm := NormalizeText(text)
	currentTeams := make(map[string]bool)
	// Mark both game teams (and their aliases) as "current" so they don't count
	for _, token := range game.Home.AllTokens() {
		currentTeams[NormalizeText(token)] = true
	}
	for _, token := range game.Away.AllTokens() {
		currentTeams[NormalizeText(token)] = true
	}

	// Also mark the abbreviations
	currentTeams[strings.ToLower(game.Home.Abbr)] = true
	currentTeams[strings.ToLower(game.Away.Abbr)] = true

	// Track which extra teams we find by full name (most reliable)
	foundTeams := make(map[string]bool)
	for name, team := range nflTeams {
		lowerName := strings.ToLower(name)
		// Skip the two game teams
		if currentTeams[lowerName] {
			continue
		}
		// Check by full name only (city/mascot alone is too ambiguous for opponent counting)
		if strings.Contains(norm, lowerName) {
			foundTeams[team.Abbr] = true
		}
	}

	return len(foundTeams)
}

// LooksLikeScheduleList checks if description appears to be a schedule or game index.
func LooksLikeScheduleList(description string) bool {
	dates := CountDistinctDates(description)
	if len(dates) >= 3 {
		return true
	}
	matchups := matchupPattern.FindAllStringSubmatch(description, -1)
	if len(matchups) >= 3 {
		return true
	}
	weeks := countWeekMarkers(description)
	return len(weeks) >= 3
}

// ComputeSingleGameConfidence evaluates how confidently a video is about exactly one game.
func ComputeSingleGameConfidence(title, description string, game GameMetadata) SingleGameConfidence {
	sgc := SingleGameConfidence{}

	multiSignal := DetectMultiGameVideo(title, description, game)

	// Extract mentions from description to see if current game is in a list
	mentions := ExtractGameMentions(description)
	allMatchups := matchupPattern.FindAllStringSubmatch(description, -1)
	totalMatchups := len(allMatchups)
	if len(mentions) > totalMatchups {
		totalMatchups = len(mentions)
	}
	sgc.TotalMentions = totalMatchups

	// Check if current game is specifically mentioned
	currentGameFound := isCurrentGameInMentions(mentions, game)

	// Check title for current game specificity
	titleHasBothTeams := ContainsBothTeams(title, "", game.Home, game.Away)

	switch {
	// Case: schedule list with current game buried inside
	case multiSignal.IsMultiGame && currentGameFound:
		sgc.Score = -60
		sgc.MentionContext = MentionInSchedule
		sgc.Reason = fmt.Sprintf("current game found in schedule with %d other games", sgc.TotalMentions-1)

	// Case: multi-game without even specifically mentioning current game
	case multiSignal.IsMultiGame && !currentGameFound:
		sgc.Score = -80
		sgc.MentionContext = MentionLooseTeam
		sgc.Reason = fmt.Sprintf("multi-game video (%s) without specific game mention", multiSignal.Reason)

	// Case: title specifically about current game, no multi-game signals
	case titleHasBothTeams && !multiSignal.IsMultiGame && totalMatchups <= 1:
		sgc.Score = 40
		sgc.MentionContext = MentionExactGame
		sgc.Reason = "title contains both teams, single-game video"

	// Case: description mentions the game but no strong multi-game signals
	case currentGameFound && !multiSignal.IsMultiGame:
		sgc.Score = 30
		sgc.MentionContext = MentionExactGame
		sgc.Reason = "specific game mention without multi-game signals"

	// Default: no specific mention either way
	default:
		sgc.Score = 0
		sgc.MentionContext = MentionNotFound
		sgc.Reason = "no specific game mentions detected"
	}

	return sgc
}

// isCurrentGameInMentions checks if any extracted mention matches the current game.
func isCurrentGameInMentions(mentions []GameMention, game GameMetadata) bool {
	for _, m := range mentions {
		// Check date match
		dateMatch := m.Date == game.DateISO || strings.ReplaceAll(m.Date, "/", "-") == game.DateISO

		if !dateMatch {
			continue
		}

		// Check team match: either order
		teamAIsHome := isTeamMention(m.NormalizedTeamA, game.Home)
		teamAIsAway := isTeamMention(m.NormalizedTeamA, game.Away)
		teamBIsHome := isTeamMention(m.NormalizedTeamB, game.Home)
		teamBIsAway := isTeamMention(m.NormalizedTeamB, game.Away)

		if (teamAIsHome && teamBIsAway) || (teamAIsAway && teamBIsHome) {
			return true
		}
	}
	return false
}

// isTeamMention checks if a normalized mention string refers to the given team.
func isTeamMention(mention string, team TeamAliases) bool {
	for _, token := range team.AllTokens() {
		if strings.Contains(mention, NormalizeText(token)) {
			return true
		}
		if strings.Contains(NormalizeText(token), mention) && mention != "" {
			return true
		}
	}
	return false
}
