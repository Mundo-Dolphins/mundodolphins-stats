package videos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PFRGameIndex struct {
	byDate map[string]GameRef
}

func BuildPFRGameIndexFromDir(gamesJSONDir string) (*PFRGameIndex, error) {
	if strings.TrimSpace(gamesJSONDir) == "" {
		return nil, fmt.Errorf("games json dir is empty")
	}
	pattern := filepath.Join(gamesJSONDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob games: %w", err)
	}
	idx := &PFRGameIndex{byDate: map[string]GameRef{}}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read game file: %w", err)
		}
		var gf gameIndexFile
		if err := json.Unmarshal(data, &gf); err != nil {
			return nil, fmt.Errorf("parse game file: %w", err)
		}
		if gf.GameID == "" {
			continue
		}
		date := dateFromGameID(gf.GameID)
		if date == "" {
			continue
		}
		if _, exists := idx.byDate[date]; exists {
			continue
		}

		away, home := gf.awayHomeNames()
		idx.byDate[date] = GameRef{
			GameID:   gf.GameID,
			Year:     parseYearFromGameID(gf.GameID),
			HomeTeam: home,
			AwayTeam: away,
			DateRaw:  gf.Data.Scorebox.Meta.Date,
		}
	}
	return idx, nil
}

func (i *PFRGameIndex) DateToGameID(date string) (string, bool) {
	if i == nil || i.byDate == nil {
		return "", false
	}
	ref, ok := i.byDate[date]
	return ref.GameID, ok
}

func (i *PFRGameIndex) GameInfo(date string) (GameRef, bool) {
	if i == nil || i.byDate == nil {
		return GameRef{}, false
	}
	ref, ok := i.byDate[date]
	return ref, ok
}

func dateFromGameID(gameID string) string {
	if len(gameID) < 8 {
		return ""
	}
	date := gameID[:8]
	if _, err := strconv.Atoi(date); err != nil {
		return ""
	}
	return fmt.Sprintf("%s-%s-%s", date[:4], date[4:6], date[6:8])
}

type gameIndexFile struct {
	GameID string `json:"game_id"`
	Data   struct {
		Scorebox struct {
			Teams []struct {
				Name struct {
					Content string `json:"content"`
				} `json:"name"`
			} `json:"teams"`
			Meta struct {
				Date string `json:"date"`
			} `json:"meta"`
		} `json:"scorebox"`
		Linescore struct {
			Teams []struct {
				Team struct {
					Content string `json:"content"`
				} `json:"team"`
			} `json:"teams"`
		} `json:"linescore"`
	} `json:"data"`
}

func (g gameIndexFile) awayHomeNames() (string, string) {
	if len(g.Data.Scorebox.Teams) >= 2 {
		away := strings.TrimSpace(g.Data.Scorebox.Teams[0].Name.Content)
		home := strings.TrimSpace(g.Data.Scorebox.Teams[1].Name.Content)
		if away != "" || home != "" {
			return away, home
		}
	}
	if len(g.Data.Linescore.Teams) >= 2 {
		away := strings.TrimSpace(g.Data.Linescore.Teams[0].Team.Content)
		home := strings.TrimSpace(g.Data.Linescore.Teams[1].Team.Content)
		return away, home
	}
	return "", ""
}
