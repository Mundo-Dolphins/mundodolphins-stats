package footballdb

import (
	"fmt"
	"time"
)

// Schema constants for FootballDB data.
const (
	// Season-level data
	SeasonSchemaName    = "mundodolphins.footballdb.team-season"
	SeasonSchemaVersion = "1.0.0"

	// Game boxscore data
	GameSchemaName    = "mundodolphins.footballdb.game"
	GameSchemaVersion = "1.0.0"

	// Injury report data
	InjurySchemaName    = "mundodolphins.footballdb.injuries"
	InjurySchemaVersion = "1.0.0"

	// Data source identifier
	SiteName = "footballdb.com"
)

// BuildSeasonEnvelope creates a versioned JSON envelope for FootballDB season data.
func BuildSeasonEnvelope(season int, data SeasonData) Envelope[SeasonData] {
	return Envelope[SeasonData]{
		Schema: SchemaInfo{
			Name:    SeasonSchemaName,
			Version: SeasonSchemaVersion,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source: SourceInfo{
			Site: SiteName,
		},
		Team: TeamInfo{
			ID:   "mia",
			Name: "Miami Dolphins",
		},
		Season: season,
		Data:   data,
	}
}

// BuildGameEnvelope creates a versioned JSON envelope for a FootballDB game boxscore.
func BuildGameEnvelope(season int, gameID string, url string, cacheFile string, fetchedAt time.Time, data GameData) Envelope[GameData] {
	source := SourceInfo{
		Site:      SiteName,
		URL:       url,
		CacheFile: cacheFile,
	}
	if !fetchedAt.IsZero() {
		source.FetchedAt = fetchedAt.Format(time.RFC3339)
	}

	return Envelope[GameData]{
		Schema: SchemaInfo{
			Name:    GameSchemaName,
			Version: GameSchemaVersion,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source:      source,
		Team: TeamInfo{
			ID:   "mia",
			Name: "Miami Dolphins",
		},
		Season: season,
		GameID: gameID,
		Data:   data,
	}
}

// BuildInjuryEnvelope creates a versioned JSON envelope for a FootballDB injury report.
func BuildInjuryEnvelope(season int, gid string, url string, cacheFile string, fetchedAt time.Time, data InjuryData) Envelope[InjuryData] {
	return Envelope[InjuryData]{
		Schema: SchemaInfo{
			Name:    InjurySchemaName,
			Version: InjurySchemaVersion,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source: SourceInfo{
			Site:      SiteName,
			URL:       url,
			CacheFile: cacheFile,
			FetchedAt: fetchedAt.Format(time.RFC3339),
		},
		Team: TeamInfo{
			ID:   "mia",
			Name: "Miami Dolphins",
		},
		Season: season,
		Data:   data,
	}
}

// ValidateSeasonYear checks if the season year is within valid range for FootballDB.
// The Dolphins entered the AFL in 1966.
func ValidateSeasonYear(year int) error {
	maxYear := time.Now().Year()
	if year < 1966 || year > maxYear {
		return fmt.Errorf("season year %d is out of valid range (1966-%d)", year, maxYear)
	}
	return nil
}
