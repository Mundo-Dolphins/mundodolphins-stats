package videos

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// AutoSyncConfig configures the auto sync operation.
type AutoSyncConfig struct {
	Season            int
	GameID            string
	Limit             int
	Overwrite         bool
	DryRun            bool
	ValidateExisting  bool
	MinDurationSec    int
	MaxResults        int
	SleepBetweenCalls time.Duration
}

// AutoSyncStats tracks the results of a sync operation.
type AutoSyncStats struct {
	GamesProcessed int
	VideosAdded    int
	VideosUpdated  int
	VideosRemoved  int
	GamesSkipped   int
	GamesNoResult  int
	APICalls       int
}

// AutoSyncer coordinates the automatic video discovery flow.
type AutoSyncer struct {
	Client *youtubeapi.Client
	Cache  *FileCache
	Now    func() time.Time
}

// Sync runs the full auto-discovery workflow.
func (s *AutoSyncer) Sync(ctx context.Context, cfg AutoSyncConfig, games []GameRef) (*AutoSyncStats, error) {
	if s.Client == nil {
		return nil, fmt.Errorf("youtube api client is required")
	}
	if s.Cache == nil {
		return nil, fmt.Errorf("cache is required")
	}
	if s.Now == nil {
		s.Now = time.Now
	}

	cf, err := s.Cache.Load()
	if err != nil {
		return nil, fmt.Errorf("load cache: %w", err)
	}

	stats := &AutoSyncStats{}

	// Phase 1: Validate existing entries if requested
	if cfg.ValidateExisting {
		removed, err := ValidateExistingVideos(ctx, cf, s.Client)
		if err != nil {
			return nil, fmt.Errorf("validate existing videos: %w", err)
		}
		stats.VideosRemoved = len(removed)
		stats.APICalls++ // CheckExistence is one logical call (batched internally)
		if !cfg.DryRun && len(removed) > 0 {
			if err := s.Cache.Save(cf); err != nil {
				return nil, fmt.Errorf("save cache after validation: %w", err)
			}
		}
	}

	// Phase 2: Discover videos for target games
	processed := 0
	for _, game := range games {
		if cfg.Limit > 0 && processed >= cfg.Limit {
			break
		}

		item := cf.Items[game.GameID]

		// Skip if already has video and not overwriting
		if item != nil && IsDiscoveryCompleteStatus(item.Status) && !cfg.Overwrite {
			log.Printf("SKIP game %s: already has video", game.GameID)
			stats.GamesSkipped++
			continue
		}

		meta := ExtractGameMetadata(game)
		if meta.Home.FullName == "" || meta.Away.FullName == "" {
			log.Printf("SKIP game %s: missing team names", game.GameID)
			stats.GamesSkipped++
			continue
		}

		// Try primary query, then fallbacks
		queries := append([]string{BuildPrimaryQuery(meta)}, BuildFallbackQueries(meta)...)
		var best *ScoredCandidate
		var usedQuery string

		for _, query := range queries {
			if cfg.SleepBetweenCalls > 0 {
				time.Sleep(cfg.SleepBetweenCalls)
			}

			ids, err := s.Client.SearchVideoIDs(ctx, query, cfg.MaxResults)
			stats.APICalls++
			if err != nil {
				if youtubeapi.IsQuotaOrRateLimit(err) {
					log.Printf("ABORT: YouTube API quota/rate limit reached: %v", err)
					if !cfg.DryRun {
						if saveErr := s.Cache.Save(cf); saveErr != nil {
							return stats, fmt.Errorf("save cache on abort: %w", saveErr)
						}
					}
					return stats, fmt.Errorf("youtube api quota exceeded: %w", err)
				}
				log.Printf("WARN: search failed for game %s query %q: %v", game.GameID, query, err)
				continue
			}
			if len(ids) == 0 {
				continue
			}

			if cfg.SleepBetweenCalls > 0 {
				time.Sleep(cfg.SleepBetweenCalls)
			}

			candidates, err := s.Client.HydrateVideos(ctx, ids)
			stats.APICalls++
			if err != nil {
				if youtubeapi.IsQuotaOrRateLimit(err) {
					log.Printf("ABORT: YouTube API quota/rate limit on hydrate: %v", err)
					if !cfg.DryRun {
						if saveErr := s.Cache.Save(cf); saveErr != nil {
							return stats, fmt.Errorf("save cache on abort: %w", saveErr)
						}
					}
					return stats, fmt.Errorf("youtube api quota exceeded: %w", err)
				}
				log.Printf("WARN: hydrate failed for game %s: %v", game.GameID, err)
				continue
			}

			// Score each candidate
			scored := make([]ScoredCandidate, 0, len(candidates))
			for _, c := range candidates {
				sc := ScoreCandidate(meta, c, cfg.MinDurationSec)
				if sc.Discarded {
					log.Printf("  DISCARD game %s candidate %s: %s", game.GameID, c.ID, sc.DiscardReason)
				} else if sc.Classification.Decision != DecisionAccept {
					log.Printf("  REVIEW game %s candidate %s: decision=%s score=%d reasons=%v",
						game.GameID, c.ID, sc.Classification.Decision, sc.Classification.Score, sc.Classification.Reasons)
				} else if sc.Score.MultiGamePenalty < 0 {
					log.Printf("  PENALIZED game %s candidate %s: multi-game penalty=%d, total=%d",
						game.GameID, c.ID, sc.Score.MultiGamePenalty, sc.Score.Total)
				}
				if sc.Classification.Decision == DecisionAccept {
					scored = append(scored, sc)
				}
			}

			selected := SelectBestCandidate(scored)
			if selected != nil {
				if best == nil || selected.Score.Total > best.Score.Total {
					best = selected
					usedQuery = query
				}
				break // Found a good result with primary/early query
			}
		}

		processed++
		stats.GamesProcessed++

		if best == nil {
			log.Printf("NO_RESULT game %s (%s vs %s): no valid video found",
				game.GameID, game.AwayTeam, game.HomeTeam)
			stats.GamesNoResult++
			if !cfg.DryRun {
				cf.Items[game.GameID] = &CacheItem{
					Status:    StatusNone,
					Query:     queries[0],
					UpdatedAt: s.Now(),
				}
			}
			continue
		}

		// Build the enriched cache entry
		c := best.Candidate
		embeddable := c.Embeddable
		newItem := &CacheItem{
			Status:      StatusCandidate,
			Query:       usedQuery,
			UpdatedAt:   s.Now(),
			NeedsReview: true,
			Videos: []Video{{
				ID:              c.ID,
				URL:             c.URL,
				Title:           c.Title,
				Channel:         c.Channel,
				DurationSeconds: int(c.Duration.Seconds()),
				Embeddable:      &embeddable,
				SelectedAt:      s.Now(),
			}},
			AutoScoring: &AutoScoringMeta{
				ScoreBreakdown: best.Score,
				MatchedTeams:   best.MatchedTeams,
				MatchedDate:    string(best.MatchedDate),
				ViewCount:      c.ViewCount,
				Embeddable:     c.Embeddable,
				DurationISO:    formatISO8601(c.Duration),
				Source:         "youtube-data-api-v3",
				Confidence:     confidenceFromScore(best.Score.Total),
				NeedsReview:    true,
				Classification: &best.Classification,
			},
		}

		isUpdate := item != nil && IsDiscoveryCompleteStatus(item.Status)
		if isUpdate {
			log.Printf("UPDATED game %s: video %s (score=%d, query=%q)",
				game.GameID, c.ID, best.Score.Total, usedQuery)
			stats.VideosUpdated++
		} else {
			log.Printf("ADDED game %s: video %s %q (score=%d, dur=%ds, views=%d, embed=%v)",
				game.GameID, c.ID, c.Title, best.Score.Total, int(c.Duration.Seconds()), c.ViewCount, c.Embeddable)
			stats.VideosAdded++
		}

		if !cfg.DryRun {
			cf.Items[game.GameID] = newItem
		}
	}

	// Final save
	if !cfg.DryRun {
		if err := s.Cache.Save(cf); err != nil {
			return stats, fmt.Errorf("save final cache: %w", err)
		}
	}

	return stats, nil
}

// DiscoverAllGames finds all game files across season directories.
func DiscoverAllGames(processedDir string, seasonFilter int, gameIDFilter string) ([]GameRef, error) {
	loader := NewSeasonAwareGameLoader(processedDir)
	games, err := loader.LoadAllGames()
	if err != nil {
		return nil, err
	}

	// Apply filters
	filtered := make([]GameRef, 0, len(games))
	for _, g := range games {
		if gameIDFilter != "" && g.GameID != gameIDFilter {
			continue
		}
		if seasonFilter > 0 && g.Year != seasonFilter {
			continue
		}
		filtered = append(filtered, g)
	}

	// Sort deterministically by game ID
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].GameID < filtered[j].GameID
	})

	return filtered, nil
}

// formatISO8601 converts a duration to PT format (e.g. PT1H23M45S).
func formatISO8601(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("PT%dH%dM%dS", h, m, sec)
	}
	if m > 0 {
		return fmt.Sprintf("PT%dM%dS", m, sec)
	}
	return fmt.Sprintf("PT%dS", sec)
}
