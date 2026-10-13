package videos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SeasonAwareGameLoader loads games from season-organized directories:
// processed/games/<season>/<game_id>.json
type SeasonAwareGameLoader struct {
	BaseDir string
}

// NewSeasonAwareGameLoader creates a loader for season-organized game directories.
func NewSeasonAwareGameLoader(baseDir string) *SeasonAwareGameLoader {
	return &SeasonAwareGameLoader{BaseDir: baseDir}
}

// LoadAllGames walks all season subdirectories and loads every game.
func (l *SeasonAwareGameLoader) LoadAllGames() ([]GameRef, error) {
	if l.BaseDir == "" {
		return nil, fmt.Errorf("base dir is empty")
	}

	entries, err := os.ReadDir(l.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("read games base dir: %w", err)
	}

	var games []GameRef
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		seasonDir := filepath.Join(l.BaseDir, entry.Name())
		seasonGames, err := loadGamesFromDir(seasonDir)
		if err != nil {
			return nil, fmt.Errorf("load games from %s: %w", seasonDir, err)
		}
		games = append(games, seasonGames...)
	}
	return games, nil
}

func loadGamesFromDir(dir string) ([]GameRef, error) {
	pattern := filepath.Join(dir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob games: %w", err)
	}

	games := make([]GameRef, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read game file %s: %w", path, err)
		}
		var gf gameFile
		if err := json.Unmarshal(data, &gf); err != nil {
			return nil, fmt.Errorf("parse game file %s: %w", path, err)
		}
		if gf.GameID == "" {
			continue
		}
		ref := GameRef{
			GameID:   gf.GameID,
			Year:     parseYearFromGameID(gf.GameID),
			HomeTeam: gf.Data.Scorebox.Teams.homeName(),
			AwayTeam: gf.Data.Scorebox.Teams.awayName(),
			DateRaw:  strings.TrimSpace(gf.Data.Scorebox.Meta.Date),
			Week:     strings.TrimSpace(gf.Data.Week),
		}
		games = append(games, ref)
	}
	return games, nil
}
