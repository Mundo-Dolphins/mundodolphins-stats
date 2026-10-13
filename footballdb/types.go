package footballdb

import "time"

const (
	DownloadStatusDownloaded         = "downloaded"
	DownloadStatusCached             = "cached"
	DownloadStatusSkippedUnsupported = "skipped_unsupported"
	DownloadStatusNotFoundOptional   = "not_found_optional"
	DownloadStatusError              = "error"
)

// SchemaInfo describes a versioned schema.
type SchemaInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// SourceInfo describes where data was obtained.
type SourceInfo struct {
	Site      string `json:"site"`
	URL       string `json:"url,omitempty"`
	CacheFile string `json:"cache_file,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

// TeamInfo identifies a team.
type TeamInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LinkValue represents a cell value that contains a hyperlink.
type LinkValue struct {
	Content string `json:"content"`
	Link    string `json:"link"`
}

// TableResult represents a parsed HTML table.
type TableResult struct {
	Title    string            `json:"title"`
	Headers  []string          `json:"headers,omitempty"`
	Rows     []map[string]any  `json:"rows"`
	RowCount int               `json:"row_count"`
	Legends  map[string]string `json:"legends,omitempty"`
}

// Envelope is a generic versioned JSON wrapper.
type Envelope[T any] struct {
	Schema      SchemaInfo `json:"schema"`
	GeneratedAt string     `json:"generated_at"`
	Source      SourceInfo `json:"source"`
	Team        TeamInfo   `json:"team"`
	Season      int        `json:"season"`
	GameID      string     `json:"game_id,omitempty"`
	Data        T          `json:"data"`
}

// --- Season data ---

// SeasonData holds all parsed page payloads for a season keyed by page slug.
type SeasonData map[string]PagePayload

// PagePayload represents the parsed result of a single page type.
type PagePayload struct {
	Status string                 `json:"status"` // "ok", "not_found", "error"
	Source PageSource             `json:"source"`
	Tables map[string]TableResult `json:"tables"`
}

// PageSource records the origin of a specific page.
type PageSource struct {
	URL       string `json:"url"`
	CacheFile string `json:"cache_file,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

// --- Game boxscore data ---

// GameData holds parsed data from a single game boxscore page.
type GameData struct {
	GameID string                 `json:"game_id"`
	Tables map[string]TableResult `json:"tables"`
}

// --- Injury data ---

// InjuryData holds parsed injury report for a single game week.
type InjuryData struct {
	GID    string                 `json:"gid"`
	Tables map[string]TableResult `json:"tables"`
}

// --- Download result ---

// DownloadResult represents the outcome of a single download operation.
type DownloadResult struct {
	Status   string // See DownloadStatus* constants.
	PageType string
	Year     int
	URL      string
	Path     string
	Error    error
}

// SeasonDownloadSummary aggregates per-status results for a season download.
type SeasonDownloadSummary struct {
	Downloaded int
	Cached     int
	Skipped    int
	NotFound   int
	Errors     int
}

// --- Throttle configuration ---

// ThrottleConfig controls delays between downloads.
type ThrottleConfig struct {
	// BaseDelay is the sleep time between consecutive downloads.
	BaseDelay time.Duration
	// Jitter adds a random duration up to this value.
	Jitter time.Duration
	// NavigationTimeout is the max time to wait for a page to load.
	NavigationTimeout time.Duration
}

// DefaultThrottle returns conservative default throttle settings.
func DefaultThrottle() ThrottleConfig {
	return ThrottleConfig{
		BaseDelay:         3 * time.Second,
		Jitter:            2 * time.Second,
		NavigationTimeout: 45 * time.Second,
	}
}
