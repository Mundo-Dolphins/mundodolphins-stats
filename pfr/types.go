package pfr

// SchemaInfo defines the schema name and version using SemVer
type SchemaInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// SourceInfo contains information about where the data was fetched from
type SourceInfo struct {
	Site      string `json:"site"`
	URL       string `json:"url"`
	CacheFile string `json:"cache_file,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

// TeamInfo contains team identification
type TeamInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LogoInfo represents a team logo found in page metadata.
type LogoInfo struct {
	Src string `json:"src"`
	Alt string `json:"alt,omitempty"`
}

// StaffRoleEntry represents a structured staff role extracted from div#meta.
type StaffRoleEntry struct {
	Role string    `json:"role"`
	Name LinkValue `json:"name"`
}

// PrincipalEntry represents ownership/chairman/founder leadership for summary rendering.
type PrincipalEntry struct {
	Label string    `json:"label"`
	Value LinkValue `json:"value"`
}

// Envelope is a generic wrapper for versioned JSON output
type Envelope[T any] struct {
	Schema      SchemaInfo `json:"schema"`
	GeneratedAt string     `json:"generated_at"`
	Source      SourceInfo `json:"source"`
	Team        TeamInfo   `json:"team"`
	Season      int        `json:"season"`
	Data        T          `json:"data"`
}

// SeasonDataV1 represents version 1.0.0 of the season data structure
type SeasonDataV1 struct {
	Info   map[string]string           `json:"info"`
	Tables map[string][]map[string]any `json:"tables"`
}

// TableResult represents a parsed table with title, legends, rows, and row count
type TableResult struct {
	Title      string            `json:"title"`
	Legends    map[string]string `json:"legends,omitempty"`
	Rows       []map[string]any  `json:"rows"`
	RowCount   int               `json:"row_count"`
	FooterRows []map[string]any  `json:"footer_rows,omitempty"`
}

// TradeItem represents a single trade entry from the trades page
type TradeItem struct {
	Date        string    `json:"date"`
	Description string    `json:"description"`
	Links       []LinkRef `json:"links,omitempty"`
}

// TradesPayload holds a list of trade items
type TradesPayload struct {
	Items []TradeItem `json:"items"`
}

// PageSource contains source information for a single page
type PageSource struct {
	URL       string `json:"url"`
	CacheFile string `json:"cache_file"`
}

// PagePayload represents the data for a single page type in v1.2.0
type PagePayload struct {
	Status       string                 `json:"status"` // "ok" | "not_found" | "error"
	Source       PageSource             `json:"source"`
	Info         map[string]string      `json:"info"`
	DivisionName string                 `json:"division_name,omitempty"`
	DivisionRank string                 `json:"division_rank,omitempty"`
	DivisionLink string                 `json:"division_link,omitempty"`
	Staff        []StaffRoleEntry       `json:"staff,omitempty"`
	Principal    *PrincipalEntry        `json:"principal,omitempty"`
	Logo         *LogoInfo              `json:"logo,omitempty"`
	Trades       *TradesPayload         `json:"trades,omitempty"`
	Tables       map[string]TableResult `json:"tables"`
	Derived      map[string]any         `json:"derived"`
}

// SeasonDataV1_1 represents version 1.2.0 of the season data structure
// It's a map where keys are page slugs (main, roster, draft, etc.)
type SeasonDataV1_1 map[string]PagePayload
