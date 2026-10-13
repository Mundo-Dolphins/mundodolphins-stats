package footballdb

// EnrichmentData holds all FootballDB enrichment data for a season.
// This is added to the PFR season JSON under data.footballdb.
type EnrichmentData struct {
	Status       string             `json:"status"`                     // "ok", "partial", "unavailable"
	Source       EnrichmentSource   `json:"source"`                     // traceability
	Coverage     EnrichmentCoverage `json:"coverage"`                   // what was integrated
	Transactions *TransactionsBlock `json:"transactions,omitempty"`     // roster transactions (2012+)
	GameIndex    *GameIndexBlock    `json:"game_index,omitempty"`       // game metadata + boxscore links
	InjByGame    *InjuriesByGame    `json:"injuries_by_game,omitempty"` // per-game injury reports
	Derived      *DerivedData       `json:"derived,omitempty"`          // computed summaries
}

// EnrichmentSource records the origin of enrichment data.
type EnrichmentSource struct {
	Site       string            `json:"site"`        // "footballdb.com"
	CacheFiles map[string]string `json:"cache_files"` // page_type → cache path
}

// EnrichmentCoverage describes what was integrated and what was skipped.
type EnrichmentCoverage struct {
	Transactions *CoverageEntry `json:"transactions,omitempty"`
	GameIndex    *CoverageEntry `json:"game_index,omitempty"`
	InjByGame    *CoverageEntry `json:"injuries_by_game,omitempty"`
}

// CoverageEntry records presence and stats about an enrichment section.
type CoverageEntry struct {
	Present bool   `json:"present"`
	Count   int    `json:"count,omitempty"`  // row/item count
	Reason  string `json:"reason,omitempty"` // why absent, e.g. "unsupported_before_2012"
}

// TransactionsBlock holds parsed season transactions.
type TransactionsBlock struct {
	Source SourceRef        `json:"source"`
	Rows   []TransactionRow `json:"rows"`
}

// TransactionRow represents a single transaction entry.
type TransactionRow struct {
	Date        string           `json:"date"`              // "March 8, 2024"
	Description string           `json:"description"`       // full text
	Players     []TransactionRef `json:"players,omitempty"` // extracted player references
}

// TransactionRef represents a player mentioned in a transaction.
type TransactionRef struct {
	Name     string `json:"name"`               // "Jonnu Smith"
	Position string `json:"position,omitempty"` // "TE"
	Link     string `json:"link,omitempty"`     // "/players/jonnu-smith-smithjo13"
}

// GameIndexBlock holds the game index extracted from the results page.
type GameIndexBlock struct {
	Source SourceRef        `json:"source"`
	Games  []GameIndexEntry `json:"games"`
}

// GameIndexEntry represents a single game with metadata from JSON-LD.
type GameIndexEntry struct {
	BoxscoreSlug string `json:"boxscore_slug"`        // "miami-dolphins-vs-kansas-city-chiefs-2024011302"
	BoxscoreURL  string `json:"boxscore_url"`         // full URL to boxscore page
	Name         string `json:"name"`                 // "Dolphins vs Chiefs - January 13, 2024"
	Date         string `json:"date,omitempty"`       // ISO date "2024-01-13"
	StartTime    string `json:"start_time,omitempty"` // "2024-01-13 20:10:00"
	AwayTeam     string `json:"away_team,omitempty"`  // "Miami Dolphins"
	HomeTeam     string `json:"home_team,omitempty"`  // "Kansas City Chiefs"
	Venue        string `json:"venue,omitempty"`      // "GEHA Field at Arrowhead Stadium"
	City         string `json:"city,omitempty"`       // "Kansas City"
	State        string `json:"state,omitempty"`      // "MO"
	Country      string `json:"country,omitempty"`    // "USA"
}

// InjuriesByGame holds per-game injury reports indexed by GID.
type InjuriesByGame struct {
	Source SourceRef                   `json:"source"`
	Games  map[string]GameInjuryReport `json:"games"` // gid → report
}

// GameInjuryReport holds the injury report for a single game.
type GameInjuryReport struct {
	GID     string                 `json:"gid"`
	Players []InjuredPlayer        `json:"players"`
	Tables  map[string]TableResult `json:"tables,omitempty"` // raw tables if valuable
}

// InjuredPlayer represents a player entry in an injury report.
type InjuredPlayer struct {
	Name       string `json:"name"`
	Position   string `json:"position,omitempty"`
	Injury     string `json:"injury,omitempty"`
	GameStatus string `json:"game_status,omitempty"` // "Out", "Doubtful", "Questionable", etc.
	Link       string `json:"link,omitempty"`
}

// DerivedData holds computed summaries from the enrichment data.
type DerivedData struct {
	TransactionsByType  map[string]int `json:"transactions_by_type,omitempty"` // "Signed" → 15
	GamesWithInjuries   int            `json:"games_with_injuries,omitempty"`
	TotalInjuredPlayers int            `json:"total_injured_players,omitempty"`
	BoxscoreCount       int            `json:"boxscore_count,omitempty"`
}

// SourceRef records the origin of a specific enrichment block.
type SourceRef struct {
	URL       string `json:"url"`
	CacheFile string `json:"cache_file"`
}
