package youtubeapi

import "time"

type VideoCandidate struct {
	ID          string
	URL         string
	Title       string
	Description string
	Channel     string
	ChannelID   string
	Duration    time.Duration
	Embeddable  bool
	ViewCount   uint64
	PublishedAt string
	Score       int
}

type searchListResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
	} `json:"items"`
}

type videosListResponse struct {
	Items []videosListItem `json:"items"`
}

type videosListItem struct {
	ID      string `json:"id"`
	Snippet struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Channel     string `json:"channelTitle"`
		ChannelID   string `json:"channelId"`
		PublishedAt string `json:"publishedAt"`
	} `json:"snippet"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	Statistics struct {
		ViewCount string `json:"viewCount"`
	} `json:"statistics"`
	Status struct {
		Embeddable bool `json:"embeddable"`
	} `json:"status"`
}

type googleErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}
