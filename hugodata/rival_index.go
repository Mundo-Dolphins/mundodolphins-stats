package hugodata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/gamemd"
	"github.com/Mundo-Dolphins/mundodolphins-stats/pagejson"
	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
)

const (
	RivalIndexSchemaName    = "mundodolphins.hugo.rival-index"
	RivalIndexSchemaVersion = "1.0.0"
)

type RivalIndexBuilder struct {
	processedDir string
	now          time.Time
	teamID       string
	teamName     string
	latestLimit  int
}

type RivalIndexDocument struct {
	Schema      RivalIndexSchema `json:"schema"`
	GeneratedAt string           `json:"generated_at"`
	Data        RivalIndexData   `json:"data"`
}

type RivalIndexSchema struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RivalIndexData struct {
	Team   RivalTeamRef      `json:"team"`
	Range  RivalIndexRange   `json:"range"`
	Totals RivalAggregate    `json:"totals"`
	Rivals []RivalIndexRival `json:"rivals"`
}

type RivalTeamRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RivalIndexRange struct {
	FromSeason int `json:"from_season"`
	ToSeason   int `json:"to_season"`
}

type RivalAggregate struct {
	Opponents          int `json:"opponents,omitempty"`
	Games              int `json:"games"`
	Wins               int `json:"wins"`
	Losses             int `json:"losses"`
	Ties               int `json:"ties"`
	PointsFor          int `json:"points_for"`
	PointsAgainst      int `json:"points_against"`
	PointDifferential  int `json:"point_differential"`
	RegularSeasonGames int `json:"regular_season_games"`
	PlayoffGames       int `json:"playoff_games"`
	UnknownGames       int `json:"unknown_games,omitempty"`
}

type RivalIndexRival struct {
	Opponent    RivalTeamRef         `json:"opponent"`
	Summary     RivalSummary         `json:"summary"`
	BySeason    []RivalSeasonSummary `json:"by_season"`
	LatestGames []RivalGameSummary   `json:"latest_games"`
}

type RivalSummary struct {
	Games              int    `json:"games"`
	Wins               int    `json:"wins"`
	Losses             int    `json:"losses"`
	Ties               int    `json:"ties"`
	PointsFor          int    `json:"points_for"`
	PointsAgainst      int    `json:"points_against"`
	PointDifferential  int    `json:"point_differential"`
	RegularSeasonGames int    `json:"regular_season_games"`
	PlayoffGames       int    `json:"playoff_games"`
	UnknownGames       int    `json:"unknown_games,omitempty"`
	FirstGame          string `json:"first_game,omitempty"`
	LastGame           string `json:"last_game,omitempty"`
}

type RivalSeasonSummary struct {
	Season             int `json:"season"`
	Games              int `json:"games"`
	Wins               int `json:"wins"`
	Losses             int `json:"losses"`
	Ties               int `json:"ties"`
	PointsFor          int `json:"points_for"`
	PointsAgainst      int `json:"points_against"`
	PointDifferential  int `json:"point_differential"`
	RegularSeasonGames int `json:"regular_season_games"`
	PlayoffGames       int `json:"playoff_games"`
	UnknownGames       int `json:"unknown_games,omitempty"`
}

type RivalGameSummary struct {
	GameID        string `json:"game_id"`
	Season        int    `json:"season"`
	Date          string `json:"date"`
	Week          string `json:"week,omitempty"`
	GameType      string `json:"game_type"`
	HomeAway      string `json:"home_away"`
	OpponentID    string `json:"opponent_id"`
	OpponentName  string `json:"opponent_name"`
	PointsFor     int    `json:"points_for"`
	PointsAgainst int    `json:"points_against"`
	Result        string `json:"result"`
	Margin        int    `json:"margin"`
	Path          string `json:"path,omitempty"`
}

type rivalAccumulator struct {
	opponent       RivalTeamRef
	latestNameDate string
	summary        RivalSummary
	seasons        map[int]*RivalSeasonSummary
	games          []RivalGameSummary
}

func NewRivalIndexBuilder(processedDir string) *RivalIndexBuilder {
	return &RivalIndexBuilder{
		processedDir: processedDir,
		now:          time.Now().UTC(),
		teamID:       "mia",
		teamName:     "Miami Dolphins",
		latestLimit:  5,
	}
}

func (b *RivalIndexBuilder) SetTime(t time.Time) *RivalIndexBuilder {
	b.now = t
	return b
}

func (b *RivalIndexBuilder) SetLatestLimit(limit int) *RivalIndexBuilder {
	if limit > 0 {
		b.latestLimit = limit
	}
	return b
}

func (b *RivalIndexBuilder) BuildFromDir(fromSeason, toSeason int) (*RivalIndexDocument, error) {
	if fromSeason > toSeason {
		return nil, fmt.Errorf("invalid season range: from %d is greater than to %d", fromSeason, toSeason)
	}

	rivals := map[string]*rivalAccumulator{}
	total := RivalAggregate{}

	for season := fromSeason; season <= toSeason; season++ {
		seasonDir := filepath.Join(b.processedDir, "games", fmt.Sprintf("%d", season))
		entries, err := os.ReadDir(seasonDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read games directory %s: %w", seasonDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			env, err := LoadGameFile(filepath.Join(seasonDir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("load game %s: %w", entry.Name(), err)
			}
			game, opponent, ok := b.summarizeGame(*env)
			if !ok {
				continue
			}
			acc, exists := rivals[opponent.ID]
			if !exists {
				acc = &rivalAccumulator{
					opponent: opponent,
					seasons:  map[int]*RivalSeasonSummary{},
				}
				rivals[opponent.ID] = acc
			}
			updateLatestOpponentName(acc, opponent, game.Date)
			acc.games = append(acc.games, game)
			accumulateRivalSummary(&acc.summary, game)
			seasonSummary := acc.seasons[game.Season]
			if seasonSummary == nil {
				seasonSummary = &RivalSeasonSummary{Season: game.Season}
				acc.seasons[game.Season] = seasonSummary
			}
			accumulateSeasonSummary(seasonSummary, game)
			accumulateAggregate(&total, game)
		}
	}

	out := RivalIndexDocument{
		Schema: RivalIndexSchema{
			Name:    RivalIndexSchemaName,
			Version: RivalIndexSchemaVersion,
		},
		GeneratedAt: b.now.UTC().Format(time.RFC3339),
		Data: RivalIndexData{
			Team: RivalTeamRef{
				ID:   b.teamID,
				Name: b.teamName,
			},
			Range: RivalIndexRange{
				FromSeason: fromSeason,
				ToSeason:   toSeason,
			},
			Totals: total,
			Rivals: make([]RivalIndexRival, 0, len(rivals)),
		},
	}
	out.Data.Totals.Opponents = len(rivals)
	out.Data.Totals.PointDifferential = out.Data.Totals.PointsFor - out.Data.Totals.PointsAgainst

	for _, acc := range rivals {
		sort.Slice(acc.games, func(i, j int) bool {
			if acc.games[i].Date == acc.games[j].Date {
				return acc.games[i].GameID > acc.games[j].GameID
			}
			return acc.games[i].Date > acc.games[j].Date
		})
		latestCount := len(acc.games)
		if latestCount > b.latestLimit {
			latestCount = b.latestLimit
		}
		latest := append([]RivalGameSummary(nil), acc.games[:latestCount]...)

		seasons := make([]RivalSeasonSummary, 0, len(acc.seasons))
		for _, item := range acc.seasons {
			item.PointDifferential = item.PointsFor - item.PointsAgainst
			seasons = append(seasons, *item)
		}
		sort.Slice(seasons, func(i, j int) bool {
			return seasons[i].Season < seasons[j].Season
		})

		acc.summary.PointDifferential = acc.summary.PointsFor - acc.summary.PointsAgainst
		sort.Slice(acc.games, func(i, j int) bool {
			if acc.games[i].Date == acc.games[j].Date {
				return acc.games[i].GameID < acc.games[j].GameID
			}
			return acc.games[i].Date < acc.games[j].Date
		})
		if len(acc.games) > 0 {
			acc.summary.FirstGame = acc.games[0].Date
			acc.summary.LastGame = acc.games[len(acc.games)-1].Date
		}

		out.Data.Rivals = append(out.Data.Rivals, RivalIndexRival{
			Opponent:    acc.opponent,
			Summary:     acc.summary,
			BySeason:    seasons,
			LatestGames: latest,
		})
	}

	sort.Slice(out.Data.Rivals, func(i, j int) bool {
		left := strings.ToLower(out.Data.Rivals[i].Opponent.Name)
		right := strings.ToLower(out.Data.Rivals[j].Opponent.Name)
		if left == right {
			return out.Data.Rivals[i].Opponent.ID < out.Data.Rivals[j].Opponent.ID
		}
		return left < right
	})

	return &out, nil
}

func (b *RivalIndexBuilder) summarizeGame(env boxscore.BoxscoreEnvelope) (RivalGameSummary, RivalTeamRef, bool) {
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return RivalGameSummary{}, RivalTeamRef{}, false
	}

	teamIndex := -1
	for i, team := range env.Data.Scorebox.Teams[:2] {
		if isDolphinsTeam(team) {
			teamIndex = i
			break
		}
	}
	if teamIndex == -1 {
		return RivalGameSummary{}, RivalTeamRef{}, false
	}

	opponentIndex := 1
	homeAway := "away"
	if teamIndex == 1 {
		opponentIndex = 0
		homeAway = "home"
	}

	team := env.Data.Scorebox.Teams[teamIndex]
	opponent := env.Data.Scorebox.Teams[opponentIndex]
	season, err := util.NFLSeasonFromGameID(env.GameID)
	if err != nil {
		return RivalGameSummary{}, RivalTeamRef{}, false
	}
	date := normalizeRivalGameDate(env.Data.Scorebox.Meta.Date, env.GameID)
	opponentRef := RivalTeamRef{
		ID:   teamCode(opponent),
		Name: linkContent(opponent.Name),
	}
	if opponentRef.Name == "" {
		opponentRef.Name = opponentRef.ID
	}
	result := resultFor(team.Score, opponent.Score)
	game := RivalGameSummary{
		GameID:        env.GameID,
		Season:        season,
		Date:          date,
		Week:          strings.TrimSpace(env.Data.Week),
		GameType:      gameTypeFromWeek(env.Data.Week),
		HomeAway:      homeAway,
		OpponentID:    opponentRef.ID,
		OpponentName:  opponentRef.Name,
		PointsFor:     team.Score,
		PointsAgainst: opponent.Score,
		Result:        result,
		Margin:        team.Score - opponent.Score,
		Path:          gamePath(env.GameID, env.Data.Scorebox.Teams[0], env.Data.Scorebox.Teams[1], date),
	}
	return game, opponentRef, true
}

func updateLatestOpponentName(acc *rivalAccumulator, opponent RivalTeamRef, gameDate string) {
	if acc == nil || opponent.Name == "" {
		return
	}
	if acc.opponent.Name == "" || gameDate > acc.latestNameDate || (gameDate == acc.latestNameDate && opponent.Name > acc.opponent.Name) {
		acc.opponent = opponent
		acc.latestNameDate = gameDate
	}
}

func accumulateRivalSummary(summary *RivalSummary, game RivalGameSummary) {
	summary.Games++
	summary.PointsFor += game.PointsFor
	summary.PointsAgainst += game.PointsAgainst
	switch game.Result {
	case "W":
		summary.Wins++
	case "L":
		summary.Losses++
	case "T":
		summary.Ties++
	}
	accumulateGameType(&summary.RegularSeasonGames, &summary.PlayoffGames, &summary.UnknownGames, game.GameType)
}

func accumulateSeasonSummary(summary *RivalSeasonSummary, game RivalGameSummary) {
	summary.Games++
	summary.PointsFor += game.PointsFor
	summary.PointsAgainst += game.PointsAgainst
	switch game.Result {
	case "W":
		summary.Wins++
	case "L":
		summary.Losses++
	case "T":
		summary.Ties++
	}
	accumulateGameType(&summary.RegularSeasonGames, &summary.PlayoffGames, &summary.UnknownGames, game.GameType)
}

func accumulateAggregate(total *RivalAggregate, game RivalGameSummary) {
	total.Games++
	total.PointsFor += game.PointsFor
	total.PointsAgainst += game.PointsAgainst
	switch game.Result {
	case "W":
		total.Wins++
	case "L":
		total.Losses++
	case "T":
		total.Ties++
	}
	accumulateGameType(&total.RegularSeasonGames, &total.PlayoffGames, &total.UnknownGames, game.GameType)
}

func accumulateGameType(regular, playoffs, unknown *int, gameType string) {
	switch gameType {
	case "regular_season":
		(*regular)++
	case "playoffs":
		(*playoffs)++
	default:
		(*unknown)++
	}
}

func isDolphinsTeam(team boxscore.ScoreboxTeam) bool {
	return teamCode(team) == "mia" || strings.EqualFold(linkContent(team.Name), "Miami Dolphins")
}

func teamCode(team boxscore.ScoreboxTeam) string {
	if strings.TrimSpace(team.Abbr) != "" {
		return strings.ToLower(strings.TrimSpace(team.Abbr))
	}
	if team.Name != nil {
		parts := strings.Split(strings.Trim(team.Name.Link, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == "teams" && parts[i+1] != "" {
				return strings.ToLower(parts[i+1])
			}
		}
	}
	name := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(linkContent(team.Name), ""))
	if len(name) > 3 {
		return name[:3]
	}
	return name
}

func linkContent(v *boxscore.LinkContent) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(v.Content)
}

func resultFor(pointsFor, pointsAgainst int) string {
	switch {
	case pointsFor > pointsAgainst:
		return "W"
	case pointsFor < pointsAgainst:
		return "L"
	default:
		return "T"
	}
}

func gameTypeFromWeek(week string) string {
	value := strings.TrimSpace(strings.ToLower(week))
	if value == "" {
		return "unknown"
	}
	if _, err := strconv.Atoi(value); err == nil {
		return "regular_season"
	}
	playoffMarkers := []string{"playoff", "wild card", "division", "conference", "super bowl", "championship"}
	for _, marker := range playoffMarkers {
		if strings.Contains(value, marker) {
			return "playoffs"
		}
	}
	return "unknown"
}

func normalizeRivalGameDate(dateText, gameID string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(dateText, ",,", ","))
	cleaned = strings.TrimPrefix(cleaned, "Sunday ")
	cleaned = strings.TrimPrefix(cleaned, "Monday ")
	cleaned = strings.TrimPrefix(cleaned, "Tuesday ")
	cleaned = strings.TrimPrefix(cleaned, "Wednesday ")
	cleaned = strings.TrimPrefix(cleaned, "Thursday ")
	cleaned = strings.TrimPrefix(cleaned, "Friday ")
	cleaned = strings.TrimPrefix(cleaned, "Saturday ")
	cleaned = strings.TrimPrefix(cleaned, "Sun ")
	cleaned = strings.TrimPrefix(cleaned, "Mon ")
	cleaned = strings.TrimPrefix(cleaned, "Tue ")
	cleaned = strings.TrimPrefix(cleaned, "Wed ")
	cleaned = strings.TrimPrefix(cleaned, "Thu ")
	cleaned = strings.TrimPrefix(cleaned, "Fri ")
	cleaned = strings.TrimPrefix(cleaned, "Sat ")

	for _, layout := range []string{"January 2, 2006", "Jan 2, 2006", "2006-01-02"} {
		if t, err := time.Parse(layout, cleaned); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(gameID) >= 8 {
		if t, err := time.Parse("20060102", gameID[:8]); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func gamePath(gameID string, visitor, home boxscore.ScoreboxTeam, date string) string {
	visitorName := linkContent(visitor.Name)
	homeName := linkContent(home.Name)
	if date == "" || visitorName == "" || homeName == "" {
		return ""
	}
	return fmt.Sprintf("historia/partidos/%s", gamemd.BuildSlug(visitorName, homeName, date))
}

func EncodeRivalIndexToJSON(doc *RivalIndexDocument) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal rival index JSON: %w", err)
	}
	return data, nil
}

func GenerateRivalIndexData(ctx context.Context, processedDir, outputDir string, fromSeason, toSeason int) error {
	_ = ctx
	doc, err := NewRivalIndexBuilder(processedDir).BuildFromDir(fromSeason, toSeason)
	if err != nil {
		return err
	}
	data, err := EncodeRivalIndexToJSON(doc)
	if err != nil {
		return err
	}

	dir := pagejson.HistoricalDataPath(outputDir, "rivals")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create rival index directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, "index.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write rival index to %s: %w", path, err)
	}
	return nil
}
