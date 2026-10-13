package youtube

import (
	"context"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type Discoverer interface {
	DiscoverForGame(ctx context.Context, gameID string, criteria Criteria) ([]youtubeapi.VideoCandidate, error)
}

func DiscoverViaAPI(ctx context.Context, yt *youtubeapi.Client, criteria Criteria, max int, minDur time.Duration) ([]youtubeapi.VideoCandidate, error) {
	if yt == nil {
		return nil, nil
	}
	query := BuildQuery(criteria)
	ids, err := yt.SearchVideoIDs(ctx, query, max)
	if err != nil {
		return nil, err
	}
	items, err := yt.HydrateVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]youtubeapi.VideoCandidate, 0, len(items))
	for _, item := range items {
		if !MatchesCriteria(item, criteria, minDur) {
			continue
		}
		item.Score = ScoreVideo(item, criteria)
		out = append(out, item)
	}
	SortCandidates(out)
	return out, nil
}
