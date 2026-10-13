package videos

// LegacyVideosFile represents the legacy videos.json format.
type LegacyVideosFile []LegacyDay

type LegacyDay struct {
	Date   string        `json:"date"`
	Videos []LegacyVideo `json:"videos"`
}

type LegacyVideo struct {
	YoutubeID string `json:"youtube_id"`
	Duration  any    `json:"duration"`
}
