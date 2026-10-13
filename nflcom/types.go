package nflcom

import "time"

const (
	DownloadStatusDownloaded = "downloaded"
	DownloadStatusCached     = "cached"
	DownloadStatusNotFound   = "not_found"
	DownloadStatusError      = "error"

	SeasonSchemaName    = "mundodolphins.nflcom.team-schedule"
	SeasonSchemaVersion = "1.0.0"
	GameSchemaName      = "mundodolphins.nflcom.game"
	GameSchemaVersion   = "1.2.0"
	SiteName            = "nfl.com"
)

type SchemaInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SourceInfo struct {
	Site      string `json:"site"`
	URL       string `json:"url,omitempty"`
	CacheFile string `json:"cache_file,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

type TeamInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Envelope[T any] struct {
	Schema      SchemaInfo `json:"schema"`
	GeneratedAt string     `json:"generated_at"`
	Source      SourceInfo `json:"source"`
	Team        TeamInfo   `json:"team"`
	Season      int        `json:"season"`
	Data        T          `json:"data"`
}

type ScheduleData struct {
	Games []ScheduleGame `json:"games"`
}

type ScheduleGame struct {
	SeasonType string `json:"season_type"`
	Week       string `json:"week"`
	Status     string `json:"status,omitempty"`
	Date       string `json:"date,omitempty"`
	URL        string `json:"url"`
	Slug       string `json:"slug"`
	Summary    string `json:"summary"`
	AwayTeam   string `json:"away_team,omitempty"`
	AwayScore  string `json:"away_score,omitempty"`
	HomeTeam   string `json:"home_team,omitempty"`
	HomeScore  string `json:"home_score,omitempty"`
}

type GameData struct {
	Slug         string        `json:"slug"`
	URL          string        `json:"url"`
	Gamebook     *GamebookLink `json:"gamebook,omitempty"`
	GamebookData *GamebookData `json:"gamebook_data,omitempty"`
	Stats        *GameStats    `json:"stats,omitempty"`
	Links        []PageLink    `json:"links,omitempty"`
	PageTitle    string        `json:"page_title,omitempty"`
	RawHeadings  []string      `json:"raw_headings,omitempty"`
}

type GameStats struct {
	AwayTeam map[string]any `json:"away_team,omitempty"`
	HomeTeam map[string]any `json:"home_team,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type GamebookLink struct {
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
}

type GamebookData struct {
	Metadata     GamebookMetadata      `json:"metadata,omitempty"`
	Officials    []GamebookOfficial    `json:"officials,omitempty"`
	ScoringPlays []GamebookScoringPlay `json:"scoring_plays,omitempty"`
	TeamStats    []GamebookTeamStat    `json:"team_stats,omitempty"`
	SnapCounts   []GamebookSnapCount   `json:"snap_counts,omitempty"`
}

type GamebookMetadata struct {
	Date        string `json:"date,omitempty"`
	StartTime   string `json:"start_time,omitempty"`
	VisitorTeam string `json:"visitor_team,omitempty"`
	HomeTeam    string `json:"home_team,omitempty"`
	Stadium     string `json:"stadium,omitempty"`
	Weather     string `json:"weather,omitempty"`
	Surface     string `json:"surface,omitempty"`
	Attendance  string `json:"attendance,omitempty"`
	TimeOfGame  string `json:"time_of_game,omitempty"`
}

type GamebookOfficial struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

type GamebookScoringPlay struct {
	Team         string `json:"team"`
	Quarter      string `json:"quarter"`
	Time         string `json:"time"`
	Description  string `json:"description"`
	VisitorScore string `json:"visitor_score"`
	HomeScore    string `json:"home_score"`
}

type GamebookTeamStat struct {
	Stat    string `json:"stat"`
	Visitor string `json:"visitor"`
	Home    string `json:"home"`
}

type GamebookSnapCount struct {
	Team            string `json:"team"`
	Player          string `json:"player"`
	Position        string `json:"pos,omitempty"`
	Offense         string `json:"offense,omitempty"`
	OffensePct      string `json:"off_pct,omitempty"`
	Defense         string `json:"defense,omitempty"`
	DefensePct      string `json:"def_pct,omitempty"`
	SpecialTeams    string `json:"special_teams,omitempty"`
	SpecialTeamsPct string `json:"st_pct,omitempty"`
}

type PageLink struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type DownloadResult struct {
	Status   string
	PageType string
	Year     int
	URL      string
	Path     string
	Error    error
}

type DownloadSummary struct {
	Downloaded int
	Cached     int
	NotFound   int
	Errors     int
}

type ThrottleConfig struct {
	BaseDelay time.Duration
	Jitter    time.Duration
}
