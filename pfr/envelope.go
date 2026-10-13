package pfr

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	SchemaName        = "mundodolphins.pfr.team-season"
	SchemaVersion     = "1.0.0"
	SchemaVersionV1_1 = "1.2.0"
	SchemaVersionV1_2 = "1.3.0"
	SchemaVersionV1_3 = "1.4.0"
	SchemaVersionV1_4 = "1.5.0" // Added FootballDB enrichment (transactions, game_index, injuries_by_game)
)

// BuildSeasonEnvelope creates a versioned JSON envelope for season data
func BuildSeasonEnvelope(
	season int,
	url string,
	cacheFile string,
	fetchedAt time.Time,
	data SeasonDataV1,
) Envelope[SeasonDataV1] {
	return Envelope[SeasonDataV1]{
		Schema: SchemaInfo{
			Name:    SchemaName,
			Version: SchemaVersion,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source: SourceInfo{
			Site:      "pro-football-reference",
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

// ValidateSeasonYear checks if the season year is within valid range
func ValidateSeasonYear(year int) error {
	maxYear := time.Now().Year()
	if year < 1966 || year > maxYear {
		return fmt.Errorf("season year %d is out of valid range (1966-%d)", year, maxYear)
	}
	return nil
}

// GetSeasonURL returns the pro-football-reference URL for a given season
func GetSeasonURL(year int) string {
	return fmt.Sprintf("https://www.pro-football-reference.com/teams/mia/%d.htm", year)
}

// BuildSeasonEnvelopeV1_1 creates a versioned JSON envelope for season data v1.4.0
func BuildSeasonEnvelopeV1_1(
	season int,
	data SeasonDataV1_1,
) Envelope[SeasonDataV1_1] {
	return Envelope[SeasonDataV1_1]{
		Schema: SchemaInfo{
			Name:    SchemaName,
			Version: SchemaVersionV1_3,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source: SourceInfo{
			Site: "pro-football-reference",
		},
		Team: TeamInfo{
			ID:   "mia",
			Name: "Miami Dolphins",
		},
		Season: season,
		Data:   data,
	}
}

// EnrichedSeasonData wraps PFR page data with optional FootballDB enrichment.
// It serializes to a flat JSON map where page slugs are keys
// and "footballdb" holds the enrichment data.
type EnrichedSeasonData struct {
	Pages      SeasonDataV1_1 `json:"-"`
	FootballDB any            `json:"-"` // *footballdb.EnrichmentData or nil
}

// MarshalJSON produces a flat JSON object with PFR page slug keys
// plus an optional "footballdb" key for enrichment data.
func (d EnrichedSeasonData) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(d.Pages)+1)
	for k, v := range d.Pages {
		m[k] = v
	}
	if d.FootballDB != nil {
		m["footballdb"] = d.FootballDB
	}
	return json.Marshal(m)
}

// BuildEnrichedSeasonEnvelope creates a v1.5.0 envelope with FootballDB enrichment.
func BuildEnrichedSeasonEnvelope(
	season int,
	pfrData SeasonDataV1_1,
	footballDB any,
) Envelope[EnrichedSeasonData] {
	data := EnrichedSeasonData{
		Pages:      pfrData,
		FootballDB: footballDB,
	}
	return Envelope[EnrichedSeasonData]{
		Schema: SchemaInfo{
			Name:    SchemaName,
			Version: SchemaVersionV1_4,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source: SourceInfo{
			Site: "pro-football-reference",
		},
		Team: TeamInfo{
			ID:   "mia",
			Name: "Miami Dolphins",
		},
		Season: season,
		Data:   data,
	}
}

// WriteEnrichedSeasonJSON writes the enriched season envelope to a JSON file.
func WriteEnrichedSeasonJSON(envelope Envelope[EnrichedSeasonData], outputPath string) error {
	jsonData, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}
