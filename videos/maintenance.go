package videos

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// ValidateExistingVideos checks all video entries in the cache and removes
// any whose YouTube videos no longer exist (deleted/private/unavailable).
func ValidateExistingVideos(ctx context.Context, cf *CacheFile, client *youtubeapi.Client) (removed []string, err error) {
	if cf == nil || client == nil {
		return nil, fmt.Errorf("cache file and youtube client are required")
	}

	// Collect all video IDs → game mapping
	videoToGames := make(map[string][]string)
	for gameID, item := range cf.Items {
		if item == nil || (!IsPublishableStatus(item.Status) && item.Status != StatusCandidate) {
			continue
		}
		for _, v := range item.Videos {
			if v.ID != "" {
				videoToGames[v.ID] = append(videoToGames[v.ID], gameID)
			}
		}
	}

	if len(videoToGames) == 0 {
		return nil, nil
	}

	// Batch check existence
	ids := make([]string, 0, len(videoToGames))
	for id := range videoToGames {
		ids = append(ids, id)
	}

	existing, err := client.CheckExistence(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("check video existence: %w", err)
	}

	// Remove entries for videos that no longer exist
	for videoID, gameIDs := range videoToGames {
		if existing[videoID] {
			continue
		}
		for _, gameID := range gameIDs {
			item := cf.Items[gameID]
			if item == nil {
				continue
			}
			// Remove the specific video from the item
			newVideos := make([]Video, 0, len(item.Videos))
			for _, v := range item.Videos {
				if v.ID != videoID {
					newVideos = append(newVideos, v)
				}
			}
			item.Videos = newVideos
			if len(item.Videos) == 0 {
				item.Status = StatusNone
				item.Error = fmt.Sprintf("video %s no longer exists on YouTube", videoID)
			}
			entryMsg := fmt.Sprintf("removed video for game %s: youtube video %s no longer exists", gameID, videoID)
			log.Println(entryMsg)
			removed = append(removed, entryMsg)
		}
	}

	return removed, nil
}

func ValidateExistingVideosWithChecker(ctx context.Context, cf *CacheFile, checker WatchAvailabilityChecker, now func() time.Time) (removed []string, err error) {
	return ValidateAndScoreExistingVideosWithChecker(ctx, cf, nil, checker, Scorer{}, 0, now)
}

func ValidateAndScoreExistingVideosWithChecker(ctx context.Context, cf *CacheFile, games []GameRef, checker WatchAvailabilityChecker, scorer Scorer, scoreThreshold int, now func() time.Time) (removed []string, err error) {
	if cf == nil || checker == nil {
		return nil, fmt.Errorf("cache file and watch checker are required")
	}
	if now == nil {
		now = time.Now
	}
	if scoreThreshold <= 0 {
		scoreThreshold = 60
	}
	if scorer.ScoreThreshold == 0 {
		scorer.ScoreThreshold = scoreThreshold
	}
	gameByID := make(map[string]GameRef, len(games))
	for _, game := range games {
		gameByID[game.GameID] = game
	}
	inspector, canInspect := checker.(WatchPageInspector)

	for gameID, item := range cf.Items {
		if item == nil || (!IsPublishableStatus(item.Status) && item.Status != StatusCandidate) {
			continue
		}
		if len(item.Videos) == 0 {
			continue
		}

		newVideos := make([]Video, 0, len(item.Videos))
		changed := false
		for _, video := range item.Videos {
			originalVideo := video
			videoID := strings.TrimSpace(video.ID)
			if videoID == "" {
				newVideos = append(newVideos, video)
				continue
			}
			inspection := WatchInspection{}
			if canInspect {
				inspection, err = inspector.Inspect(ctx, videoID)
			} else {
				inspection.Exists, err = checker.Exists(ctx, videoID)
			}
			if err != nil {
				log.Printf("WARN validate cached video game=%s video=%s err=%v", gameID, videoID, err)
				newVideos = append(newVideos, video)
				continue
			}
			if !inspection.Exists {
				entryMsg := fmt.Sprintf("removed video for game %s: youtube video %s no longer exists", gameID, videoID)
				log.Println(entryMsg)
				removed = append(removed, entryMsg)
				changed = true
				continue
			}
			if canInspect {
				video = applyWatchDetailsToVideo(video, inspection.Details)
				if game, ok := gameByID[gameID]; ok {
					scoreCachedVideoFromWatchPage(item, video, inspection.Details, game, scorer, scoreThreshold)
				}
			}
			if video != originalVideo {
				changed = true
			}
			newVideos = append(newVideos, video)
		}

		if !changed {
			continue
		}
		item.Videos = newVideos
		item.UpdatedAt = now()
		if len(item.Videos) == 0 {
			item.Status = StatusNone
			item.NeedsReview = false
			item.AutoScoring = nil
			item.Error = "cached YouTube video no longer exists"
		}
	}

	return removed, nil
}

func applyWatchDetailsToVideo(video Video, details VideoDetails) Video {
	if details.Title != "" {
		video.Title = details.Title
	}
	if details.Duration > 0 {
		video.DurationSeconds = int(details.Duration.Seconds())
	}
	return video
}

func scoreCachedVideoFromWatchPage(item *CacheItem, video Video, details VideoDetails, game GameRef, scorer Scorer, scoreThreshold int) int {
	if item == nil {
		return 0
	}
	candidate := Candidate{
		VideoID:     video.ID,
		URL:         video.URL,
		Title:       video.Title,
		Channel:     video.Channel,
		DurationSec: video.DurationSeconds,
		Snippet:     details.Description,
	}
	score := scorer.Score(game, candidate)
	if item.AutoScoring == nil {
		item.AutoScoring = &AutoScoringMeta{}
	}
	item.AutoScoring.ScoreBreakdown.Total = score
	item.AutoScoring.Source = "watch-page-validation"
	item.AutoScoring.Confidence = confidenceFromScore(score)
	item.AutoScoring.NeedsReview = item.NeedsReview
	if score > 0 {
		item.AutoScoring.MatchedTeams = score >= 30
	}
	if details.Duration > 0 {
		item.AutoScoring.DurationISO = formatISO8601(details.Duration)
	}
	if score < scoreThreshold {
		item.NeedsReview = true
		item.AutoScoring.NeedsReview = true
		item.Error = fmt.Sprintf("low watch-page score (%d); reviewer should search alternatives", score)
	}
	return score
}
