package videos

import (
	"context"
	"time"
)

type VideoDetails struct {
	Title       string
	Description string
	URL         string
	Duration    time.Duration
}

type WatchAvailabilityChecker interface {
	Exists(ctx context.Context, videoID string) (bool, error)
	Close() error
}

type WatchInspection struct {
	Exists  bool
	Details VideoDetails
}

type WatchPageInspector interface {
	WatchAvailabilityChecker
	Inspect(ctx context.Context, videoID string) (WatchInspection, error)
}
