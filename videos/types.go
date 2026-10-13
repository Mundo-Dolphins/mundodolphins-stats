package videos

import "time"

const (
	StatusCandidate = "candidate"
	StatusApproved  = "approved"
	StatusSelected  = "selected"
	StatusRejected  = "rejected"
	StatusNone      = "none"
	StatusPending   = "pending"
	StatusUnknown   = "unknown"
	StatusError     = "error"
)

type Candidate struct {
	VideoID      string
	URL          string
	Title        string
	Channel      string
	DurationText string
	DurationSec  int
	Snippet      string
	Score        int
	Embeddable   *bool
}

type Video struct {
	ID              string    `json:"id"`
	URL             string    `json:"url"`
	Title           string    `json:"title"`
	Channel         string    `json:"channel,omitempty"`
	ChannelID       string    `json:"channel_id,omitempty"`
	Role            string    `json:"role,omitempty"`
	Part            int       `json:"part,omitempty"`
	PartTotal       int       `json:"part_total,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
	Embeddable      *bool     `json:"embeddable,omitempty"`
	SelectedAt      time.Time `json:"selected_at"`
}

type GameRef struct {
	GameID   string
	Year     int
	HomeTeam string
	AwayTeam string
	DateRaw  string
	Week     string
}

type CacheSchema struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CacheFile struct {
	Schema      CacheSchema           `json:"schema"`
	GeneratedAt time.Time             `json:"generated_at"`
	Items       map[string]*CacheItem `json:"items"`
}

type CacheItem struct {
	Status      string           `json:"status"`
	Query       string           `json:"query"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Videos      []Video          `json:"videos"`
	NeedsReview bool             `json:"needs_review,omitempty"`
	Error       string           `json:"error,omitempty"`
	AutoScoring *AutoScoringMeta `json:"auto_scoring,omitempty"`
}

// AutoScoringMeta stores detailed scoring info from automatic selection.
type AutoScoringMeta struct {
	ScoreBreakdown ScoreBreakdown        `json:"score_breakdown"`
	MatchedTeams   bool                  `json:"matched_teams"`
	MatchedDate    string                `json:"matched_date_type"`
	MatchedWeek    string                `json:"matched_week_type,omitempty"`
	ContentRole    string                `json:"content_role,omitempty"`
	ViewCount      uint64                `json:"view_count"`
	Embeddable     bool                  `json:"embeddable"`
	DurationISO    string                `json:"duration_iso"`
	Source         string                `json:"source"`
	Confidence     string                `json:"confidence,omitempty"`
	NeedsReview    bool                  `json:"needs_review,omitempty"`
	Classification *ClassificationResult `json:"classification,omitempty"`
}

// ScoreBreakdown is forward-declared in autoscoring.go, re-exported here for JSON.

type YouTubeBlock struct {
	Status string  `json:"status"`
	Videos []Video `json:"videos"`
}

func IsPublishableStatus(status string) bool {
	return status == StatusApproved || status == StatusSelected
}

func IsCandidateStatus(status string) bool {
	return status == StatusCandidate || status == StatusPending
}

func IsDiscoveryCompleteStatus(status string) bool {
	switch status {
	case StatusCandidate, StatusApproved, StatusSelected, StatusRejected, StatusNone:
		return true
	default:
		return false
	}
}

func IsPublishedCacheItem(item *CacheItem) bool {
	return item != nil && IsPublishableStatus(item.Status) && len(item.Videos) > 0
}

func IsReviewableCacheItem(item *CacheItem) bool {
	return item != nil && (item.Status == StatusCandidate || item.NeedsReview)
}
