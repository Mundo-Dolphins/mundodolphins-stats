package pagejson

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
)

// EncodeToJSON converts a SectionedPage to pretty-printed JSON bytes
func EncodeToJSON(page *SectionedPage) ([]byte, error) {
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return data, nil
}

// PagePaths generates output file paths for different page types
type PagePaths struct {
	baseDir string
}

// NewPagePaths creates a new path generator
func NewPagePaths(baseDir string) *PagePaths {
	return &PagePaths{baseDir: baseDir}
}

// HistoricalDataPath returns a path under the Hugo data/historical contract.
func HistoricalDataPath(baseDir string, parts ...string) string {
	allParts := append([]string{baseDir, "data", "historical"}, parts...)
	return filepath.Join(allParts...)
}

// SeasonOutputPath returns the output path for a season page
func (p *PagePaths) SeasonOutputPath(year int) string {
	return HistoricalDataPath(p.baseDir, "seasons", fmt.Sprintf("%d.json", year))
}

// SeasonStatsOutputPath returns the output path for a season-stats page
func (p *PagePaths) SeasonStatsOutputPath(year int) string {
	return HistoricalDataPath(p.baseDir, "season_stats", fmt.Sprintf("%d.json", year))
}

// GameOutputPath returns the output path for a game page
func (p *PagePaths) GameOutputPath(gameID string) string {
	if len(gameID) < 8 {
		return HistoricalDataPath(p.baseDir, "games", "0", fmt.Sprintf("%s.json", gameID))
	}
	season, err := util.NFLSeasonFromGameID(gameID)
	if err != nil {
		season = 0
	}
	return HistoricalDataPath(p.baseDir, "games", fmt.Sprintf("%d", season), fmt.Sprintf("%s.json", gameID))
}
