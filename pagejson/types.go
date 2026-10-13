package pagejson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Schema constants for mundodolphins.hugo.sectioned-page
const (
	SchemaName    = "mundodolphins.hugo.sectioned-page"
	SchemaVersion = "1.0.0"
)

// PageKind represents the type of page
type PageKind string

const (
	PageKindSeason      PageKind = "season"
	PageKindSeasonStats PageKind = "season-stats"
	PageKindGame        PageKind = "game"
)

// SectionedPage represents the new JSON structure for Hugo pages
type SectionedPage struct {
	Schema      SchemaInfo             `json:"schema"`
	GeneratedAt string                 `json:"generated_at"` // RFC3339
	Page        PageInfo               `json:"page"`
	Sections    map[string]interface{} `json:"sections"`
}

// SchemaInfo contains schema metadata
type SchemaInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PageInfo contains page metadata
type PageInfo struct {
	Kind   PageKind               `json:"kind"`
	Title  string                 `json:"title"`
	Date   string                 `json:"date"` // YYYY-MM-DD
	Slug   string                 `json:"slug"`
	Path   string                 `json:"path"`
	Params map[string]interface{} `json:"params"`
}

// TableRow represents an ordered row in a table section.
// JSON objects are unordered semantically, but Hugo consumers benefit from stable key order,
// so we preserve insertion order and emit keys in that order during marshaling.
type TableRow struct {
	order  []string
	values map[string]string
}

func NewTableRow() TableRow {
	return TableRow{
		order:  make([]string, 0),
		values: make(map[string]string),
	}
}

func (r *TableRow) Set(key, value string) {
	if r.values == nil {
		r.values = make(map[string]string)
	}
	if _, exists := r.values[key]; !exists {
		r.order = append(r.order, key)
	}
	r.values[key] = value
}

func (r TableRow) Get(key string) string {
	if r.values == nil {
		return ""
	}
	return r.values[key]
}

func (r TableRow) Has(key string) bool {
	if r.values == nil {
		return false
	}
	_, ok := r.values[key]
	return ok
}

func (r TableRow) Keys() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

func (r TableRow) MarshalJSON() ([]byte, error) {
	if r.values == nil {
		return []byte(`{}`), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range r.order {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("marshal table row key %q: %w", key, err)
		}
		v, err := json.Marshal(r.values[key])
		if err != nil {
			return nil, fmt.Errorf("marshal table row value for %q: %w", key, err)
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// GameLink represents a structured link to a game
type GameLink struct {
	Title    string `json:"title"`
	ID       string `json:"id"`
	GameID   string `json:"game_id,omitempty"`
	Fecha    string `json:"fecha"` // DD-MM-YYYY format
	Date     string `json:"date,omitempty"`
	URL      string `json:"url"`
	Stage    string `json:"stage,omitempty"`
	Week     string `json:"week,omitempty"`
	GameType string `json:"game_type,omitempty"`
	Status   string `json:"status,omitempty"`
}

// VideoInfo represents a video in a game page
type VideoInfo struct {
	YouTubeID       string `json:"youtube_id"`
	Title           string `json:"title"`
	URL             string `json:"url"`
	DurationSeconds int    `json:"duration_seconds"`
	Embeddable      *bool  `json:"embeddable,omitempty"`
}

// TeamStats represents stats for a team in a game
type TeamStats struct {
	Coach                  string                 `json:"coach,omitempty"`
	Puntos                 interface{}            `json:"puntos,omitempty"` // Can be int or string
	Record                 string                 `json:"record,omitempty"`
	Estadisticas           interface{}            `json:"estadisticas,omitempty"` // []string or map
	EstadisticasPorJugador map[string]interface{} `json:"estadisticas_por_jugador,omitempty"`
}

// NewSectionedPage creates a new SectionedPage with default schema info
func NewSectionedPage(kind PageKind, title, date, slug, path string, params map[string]interface{}) *SectionedPage {
	return &SectionedPage{
		Schema: SchemaInfo{
			Name:    SchemaName,
			Version: SchemaVersion,
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Page: PageInfo{
			Kind:   kind,
			Title:  title,
			Date:   date,
			Slug:   slug,
			Path:   path,
			Params: params,
		},
		Sections: make(map[string]interface{}),
	}
}

// NewSectionedPageWithTime creates a new SectionedPage with a specific time (for testing)
func NewSectionedPageWithTime(kind PageKind, title, date, slug, path string, params map[string]interface{}, generatedAt time.Time) *SectionedPage {
	return &SectionedPage{
		Schema: SchemaInfo{
			Name:    SchemaName,
			Version: SchemaVersion,
		},
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Page: PageInfo{
			Kind:   kind,
			Title:  title,
			Date:   date,
			Slug:   slug,
			Path:   path,
			Params: params,
		},
		Sections: make(map[string]interface{}),
	}
}

// AddSection adds a section to the page
func (p *SectionedPage) AddSection(name string, content interface{}) {
	p.Sections[name] = content
}

// AddNestedSection adds a nested section (H3 within H2)
func (p *SectionedPage) AddNestedSection(h2Name, h3Name string, content interface{}) {
	// Get or create H2 section as map
	h2Section, exists := p.Sections[h2Name]
	if !exists {
		h2Section = make(map[string]interface{})
		p.Sections[h2Name] = h2Section
	}

	// Ensure h2Section is a map
	h2Map, ok := h2Section.(map[string]interface{})
	if !ok {
		// If it's not a map, convert it to one with _content key
		h2Map = map[string]interface{}{
			"_content": h2Section,
		}
		p.Sections[h2Name] = h2Map
	}

	// Add nested section
	h2Map[h3Name] = content
}
