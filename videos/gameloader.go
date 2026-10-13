package videos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type JSONGameLoader struct {
	Dir string
}

func NewJSONGameLoader(dir string) *JSONGameLoader {
	return &JSONGameLoader{Dir: dir}
}

func (l *JSONGameLoader) LoadGames(yearFrom, yearTo int) ([]GameRef, error) {
	if l == nil || l.Dir == "" {
		return nil, fmt.Errorf("games dir is empty")
	}
	files, err := findGameJSONFiles(l.Dir)
	if err != nil {
		return nil, err
	}
	games := make([]GameRef, 0, len(files))
	seasonFallbacks := map[int]map[string]GameRef{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read game file: %w", err)
		}
		var gf gameFile
		if err := json.Unmarshal(data, &gf); err != nil {
			return nil, fmt.Errorf("parse game file: %w", err)
		}
		if gf.GameID == "" {
			continue
		}
		season := parseSeasonFromGamePath(path, gf.GameID)
		if yearFrom > 0 && season < yearFrom {
			continue
		}
		if yearTo > 0 && season > yearTo {
			continue
		}
		ref := GameRef{
			GameID:   gf.GameID,
			Year:     season,
			HomeTeam: gf.Data.homeName(),
			AwayTeam: gf.Data.awayName(),
			DateRaw:  gf.Data.Scorebox.Meta.Date,
			Week:     gf.Data.Week,
		}
		if ref.HomeTeam == "" || ref.AwayTeam == "" || ref.Week == "" || ref.DateRaw == "" {
			fallbacks := seasonFallbacks[season]
			if fallbacks == nil {
				fallbacks = l.loadSeasonFallbacks(season)
				seasonFallbacks[season] = fallbacks
			}
			if fallback, ok := fallbacks[gf.GameID]; ok {
				ref = mergeGameRef(ref, fallback)
			}
		}
		ref = applyGameIDFallback(ref)
		games = append(games, ref)
	}
	return games, nil
}

func findGameJSONFiles(dir string) ([]string, error) {
	patterns := []string{
		filepath.Join(dir, "*.json"),
		filepath.Join(dir, "*", "*.json"),
	}
	seen := make(map[string]bool)
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob games %s: %w", pattern, err)
		}
		for _, match := range matches {
			if seen[match] {
				continue
			}
			seen[match] = true
			files = append(files, match)
		}
	}
	return files, nil
}

func parseYearFromGameID(gameID string) int {
	if len(gameID) < 4 {
		return 0
	}
	year, _ := strconv.Atoi(gameID[:4])
	return year
}

func parseSeasonFromGamePath(path string, gameID string) int {
	season, err := strconv.Atoi(filepath.Base(filepath.Dir(path)))
	if err == nil && season >= 1900 && season <= 2200 {
		return season
	}
	return parseYearFromGameID(gameID)
}

func (l *JSONGameLoader) loadSeasonFallbacks(season int) map[string]GameRef {
	out := map[string]GameRef{}
	if l == nil || season <= 0 {
		return out
	}
	path := filepath.Join(filepath.Dir(l.Dir), fmt.Sprintf("%d.json", season))
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var sf seasonFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return out
	}
	games := sf.Data.FootballDB.GameIndex.Games
	sort.SliceStable(games, func(i, j int) bool {
		return games[i].Date < games[j].Date
	})
	firstRegular := firstRegularSeasonDate(games)
	preseasonWeek := 0
	for _, game := range games {
		gameID := gameIDFromSeasonGame(game)
		if gameID == "" {
			continue
		}
		week := ""
		date, err := time.Parse("2006-01-02", strings.TrimSpace(game.Date))
		if err == nil {
			if !firstRegular.IsZero() && !date.Before(firstRegular) {
				week = strconv.Itoa(1 + int(date.Sub(firstRegular).Hours()/(24*7)))
			} else {
				preseasonWeek++
				week = "P" + strconv.Itoa(preseasonWeek)
			}
		}
		out[gameID] = GameRef{
			GameID:   gameID,
			Year:     season,
			HomeTeam: strings.TrimSpace(game.HomeTeam),
			AwayTeam: strings.TrimSpace(game.AwayTeam),
			DateRaw:  strings.TrimSpace(game.Date),
			Week:     week,
		}
	}
	return out
}

func firstRegularSeasonDate(games []seasonGame) time.Time {
	var first time.Time
	for _, game := range games {
		date, err := time.Parse("2006-01-02", strings.TrimSpace(game.Date))
		if err != nil || date.Month() < time.September {
			continue
		}
		if first.IsZero() || date.Before(first) {
			first = date
		}
	}
	return first
}

func gameIDFromSeasonGame(game seasonGame) string {
	date := strings.ReplaceAll(strings.TrimSpace(game.Date), "-", "")
	if len(date) != 8 {
		return ""
	}
	abbr := strings.ToLower(strings.TrimSpace(LookupTeam(game.HomeTeam).Abbr))
	if abbr == "" {
		return ""
	}
	return date + "0" + abbr
}

func mergeGameRef(ref GameRef, fallback GameRef) GameRef {
	if ref.HomeTeam == "" {
		ref.HomeTeam = fallback.HomeTeam
	}
	if ref.AwayTeam == "" {
		ref.AwayTeam = fallback.AwayTeam
	}
	if ref.DateRaw == "" {
		ref.DateRaw = fallback.DateRaw
	}
	if ref.Week == "" {
		ref.Week = fallback.Week
	}
	return ref
}

func applyGameIDFallback(ref GameRef) GameRef {
	if ref.GameID == "" || len(ref.GameID) < 3 {
		return ref
	}
	homeCode := strings.ToUpper(ref.GameID[len(ref.GameID)-3:])
	if ref.HomeTeam == "" {
		if name := teamNameByAbbr(homeCode); name != "" {
			ref.HomeTeam = name
		}
	}
	if ref.AwayTeam == "" {
		ref.AwayTeam = "Opponent"
	}
	return ref
}

func teamNameByAbbr(abbr string) string {
	abbr = strings.ToUpper(strings.TrimSpace(abbr))
	for name, aliases := range nflTeams {
		if strings.EqualFold(aliases.Abbr, abbr) {
			return name
		}
	}
	return ""
}

type gameFile struct {
	GameID string   `json:"game_id"`
	Data   gameData `json:"data"`
}

type gameData struct {
	Week     string `json:"week"`
	Scorebox struct {
		Teams gameTeams `json:"teams"`
		Meta  struct {
			Date string `json:"date"`
		} `json:"meta"`
	} `json:"scorebox"`
	Linescore struct {
		Teams gameLineTeams `json:"teams"`
	} `json:"linescore"`
}

func (d gameData) awayName() string {
	if name := d.Scorebox.Teams.awayName(); name != "" {
		return name
	}
	return d.Linescore.Teams.awayName()
}

func (d gameData) homeName() string {
	if name := d.Scorebox.Teams.homeName(); name != "" {
		return name
	}
	return d.Linescore.Teams.homeName()
}

type gameTeams []struct {
	Name struct {
		Content string `json:"content"`
	} `json:"name"`
}

type seasonFile struct {
	Data struct {
		FootballDB struct {
			GameIndex struct {
				Games []seasonGame `json:"games"`
			} `json:"game_index"`
		} `json:"footballdb"`
	} `json:"data"`
}

type seasonGame struct {
	Date     string `json:"date"`
	AwayTeam string `json:"away_team"`
	HomeTeam string `json:"home_team"`
}

func (t gameTeams) awayName() string {
	if len(t) < 1 {
		return ""
	}
	return strings.TrimSpace(t[0].Name.Content)
}

func (t gameTeams) homeName() string {
	if len(t) < 2 {
		return ""
	}
	return strings.TrimSpace(t[1].Name.Content)
}

type gameLineTeams []struct {
	Team struct {
		Content string `json:"content"`
	} `json:"team"`
}

func (t gameLineTeams) awayName() string {
	if len(t) < 1 {
		return ""
	}
	return strings.TrimSpace(t[0].Team.Content)
}

func (t gameLineTeams) homeName() string {
	if len(t) < 2 {
		return ""
	}
	return strings.TrimSpace(t[1].Team.Content)
}
