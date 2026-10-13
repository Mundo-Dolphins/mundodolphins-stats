package videos

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Details provides the title, description and duration of a video. The
// library ships no implementation that reads YouTube pages; use the YouTube
// Data API (package youtubeapi) or your own source.

func enrichCandidates(ctx context.Context, candidates []Candidate, limit int, fetcher func(context.Context, string) (VideoDetails, error)) []Candidate {
	if len(candidates) == 0 || fetcher == nil || limit == 0 {
		return candidates
	}
	if limit < 0 || limit > len(candidates) {
		limit = len(candidates)
	}
	for i := 0; i < limit; i++ {
		if candidates[i].VideoID == "" {
			continue
		}
		if candidates[i].DurationSec > 0 && candidates[i].DurationText != "" && candidates[i].Title != "" && candidates[i].Snippet != "" {
			continue
		}
		details, err := fetcher(ctx, candidates[i].VideoID)
		if err != nil {
			continue
		}
		if details.Title != "" {
			candidates[i].Title = details.Title
		}
		if details.Description != "" {
			candidates[i].Snippet = details.Description
		}
		if details.Duration > 0 {
			candidates[i].DurationSec = int(details.Duration.Seconds())
			candidates[i].DurationText = formatDuration(details.Duration)
		}
		if candidates[i].URL == "" && details.URL != "" {
			candidates[i].URL = details.URL
		}
		if candidates[i].URL == "" {
			candidates[i].URL = "https://www.youtube.com/watch?v=" + candidates[i].VideoID
		}
	}
	return candidates
}

func ParseDurationToSeconds(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ":")
	if len(parts) == 0 {
		return 0
	}
	seconds := 0
	for _, part := range parts {
		value := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				return 0
			}
			value = value*10 + int(r-'0')
		}
		seconds = seconds*60 + value
	}
	return seconds
}
func formatDuration(value time.Duration) string {
	total := int(value.Seconds())
	if total <= 0 {
		return ""
	}
	hours := total / 3600
	mins := (total % 3600) / 60
	secs := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%d:%02d", mins, secs)
}
func FormatDuration(value time.Duration) string {
	return formatDuration(value)
}
