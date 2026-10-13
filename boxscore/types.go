package boxscore

import (
	"github.com/Mundo-Dolphins/mundodolphins-stats/nflcom"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

// Schema constants for boxscore envelope
const (
	BoxscoreSchemaName        = "mundodolphins.pfr.boxscore"
	BoxscoreSchemaVersion     = "1.3.0"
	BoxscoreSchemaVersionV1_0 = "1.0.0"
	BoxscoreSchemaVersionV1_1 = "1.1.0"
	BoxscoreSchemaVersionV1_2 = "1.2.0"
)

// SchemaInfo defines the schema name and version
type SchemaInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// BoxscoreSource contains source information
type BoxscoreSource struct {
	Site      string `json:"site"`
	URL       string `json:"url"`
	CacheFile string `json:"cache_file"`
}

// BoxscoreEnvelope wraps the parsed boxscore data
type BoxscoreEnvelope struct {
	Schema      SchemaInfo     `json:"schema"`
	GeneratedAt string         `json:"generated_at"`
	Source      BoxscoreSource `json:"source"`
	GameID      string         `json:"game_id"`
	Data        BoxscoreData   `json:"data"`
}

// BoxscoreData contains the parsed sections
type BoxscoreData struct {
	Scorebox  *Scorebox              `json:"scorebox,omitempty"`
	Linescore *Linescore             `json:"linescore,omitempty"`
	Week      string                 `json:"week,omitempty"`
	YouTube   videos.YouTubeBlock    `json:"youtube"`
	NFLCom    *nflcom.GameData       `json:"nflcom,omitempty"`
	Tables    map[string]TableResult `json:"tables"`
}

// LinkContent represents a link with content
type LinkContent struct {
	Content string `json:"content"`
	Link    string `json:"link"`
}

// ImageInfo represents an image
type ImageInfo struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
}

// PrevNextLinks represents previous/next links
type PrevNextLinks struct {
	Prev *LinkContent `json:"prev,omitempty"`
	Next *LinkContent `json:"next,omitempty"`
}

// ScoreboxTeam represents a team block in scorebox
type ScoreboxTeam struct {
	SBID     string         `json:"sb_id"`
	Name     *LinkContent   `json:"name,omitempty"`
	Abbr     string         `json:"abbr,omitempty"`
	Score    int            `json:"score"`
	Record   string         `json:"record,omitempty"`
	Coach    *LinkContent   `json:"coach,omitempty"`
	Logo     *ImageInfo     `json:"logo,omitempty"`
	PrevNext *PrevNextLinks `json:"prevnext,omitempty"`
}

// ScoreboxMeta represents metadata in scorebox
type ScoreboxMeta struct {
	Date       string       `json:"date,omitempty"`
	StartTime  string       `json:"start_time,omitempty"`
	Stadium    *LinkContent `json:"stadium,omitempty"`
	Attendance *LinkContent `json:"attendance,omitempty"`
	TimeOfGame string       `json:"time_of_game,omitempty"`
	Raw        []string     `json:"raw,omitempty"`
}

// Scorebox represents the scorebox section
type Scorebox struct {
	Teams []ScoreboxTeam `json:"teams"`
	Meta  ScoreboxMeta   `json:"meta"`
}

// LinescoreTeam represents a team row in linescore
type LinescoreTeam struct {
	Team     *LinkContent   `json:"team,omitempty"`
	Logo     *ImageInfo     `json:"logo,omitempty"`
	Quarters map[string]int `json:"quarters"`
	Final    int            `json:"final"`
}

// Linescore represents the linescore section
type Linescore struct {
	Title string          `json:"title"`
	Teams []LinescoreTeam `json:"teams"`
}

// TableResult mirrors pfr.TableResult to avoid extra imports in JSON output
type TableResult struct {
	Title      string            `json:"title"`
	Legends    map[string]string `json:"legends,omitempty"`
	RowCount   int               `json:"row_count"`
	Rows       []map[string]any  `json:"rows"`
	FooterRows []map[string]any  `json:"footer_rows,omitempty"`
}
