package videos

import (
	"context"
	"fmt"
	"time"
)

type CachedClassifierConfig struct {
	ProcessedDir     string
	DiscoveryDB      string
	OutputFile       string
	Season           int
	GameID           string
	MinDurationSec   int
	MaxResults       int
	ScoreThreshold   int
	AutoApproveScore int
}

type CachedClassifierStats struct {
	GamesTotal       int
	GamesProcessed   int
	AutoApproved     int
	NeedsReview      int
	None             int
	NoCachedSearch   int
	CandidatesSeen   int
	CandidatesKept   int
	OutputFile       string
	AutoApproveScore int
}

func RunCachedClassifier(ctx context.Context, cfg CachedClassifierConfig) (*CachedClassifierStats, error) {
	if cfg.ProcessedDir == "" {
		cfg.ProcessedDir = "processed/games"
	}
	if cfg.DiscoveryDB == "" {
		cfg.DiscoveryDB = "cache/videos/discovery.sqlite"
	}
	if cfg.OutputFile == "" {
		cfg.OutputFile = "cache/videos/videos.json"
	}
	if cfg.MinDurationSec <= 0 {
		cfg.MinDurationSec = 300
	}
	if cfg.ScoreThreshold <= 0 {
		cfg.ScoreThreshold = 60
	}
	if cfg.AutoApproveScore <= 0 {
		cfg.AutoApproveScore = 130
	}

	games, err := DiscoverAllGames(cfg.ProcessedDir, cfg.Season, cfg.GameID)
	if err != nil {
		return nil, fmt.Errorf("discover games: %w", err)
	}
	store, err := NewSQLiteCandidateStore(cfg.DiscoveryDB)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	now := time.Now().UTC()
	cf := newCacheFile(now)
	stats := &CachedClassifierStats{
		GamesTotal:       len(games),
		OutputFile:       cfg.OutputFile,
		AutoApproveScore: cfg.AutoApproveScore,
	}
	scorer := NewScorer(cfg.MinDurationSec, cfg.ScoreThreshold)
	usedVideoIDs := map[string]bool{}

	for _, game := range games {
		candidates, err := store.LoadCandidatesForGame(ctx, game, 0)
		if err != nil {
			return stats, err
		}
		if len(candidates) == 0 {
			stats.NoCachedSearch++
			assignNoCachedCandidate(cf, game, now)
			continue
		}
		stats.CandidatesSeen += len(candidates)
		filtered := filterByMinDuration(candidates, cfg.MinDurationSec)
		filtered = filterByLegacyRelevance(game, filtered, scorer)
		filtered = rankCandidates(game, filtered, scorer)
		if cfg.MaxResults > 0 && len(filtered) > cfg.MaxResults {
			filtered = filtered[:cfg.MaxResults]
		}
		stats.CandidatesKept += len(filtered)
		stats.GamesProcessed++
		if len(filtered) == 0 {
			stats.None++
			assignNoCachedCandidate(cf, game, now)
			continue
		}

		scored := scoreWebCandidatesForAutopilot(game, filtered, cfg.MinDurationSec, usedVideoIDs)
		bestSet := selectAutopilotBestSet(scored)
		if len(bestSet) > 0 && ShouldAutoApproveSet(bestSet, cfg.AutoApproveScore) {
			assignCandidatesToCache(cf, game.GameID, bestSet, buildQuery(game), true, cfg.AutoApproveScore)
			markAssignedAt(cf.Items[game.GameID], now)
			markUsedVideoIDs(usedVideoIDs, bestSet)
			stats.AutoApproved++
			continue
		}

		reviewSet := bestSet
		if len(reviewSet) == 0 && len(scored) > 0 {
			reviewSet = []ScoredCandidate{scored[0]}
		}
		if len(reviewSet) == 0 {
			stats.None++
			assignNoCachedCandidate(cf, game, now)
			continue
		}
		assignCandidatesToCache(cf, game.GameID, reviewSet, buildQuery(game), false, cfg.AutoApproveScore)
		markAssignedAt(cf.Items[game.GameID], now)
		markUsedVideoIDs(usedVideoIDs, reviewSet)
		stats.NeedsReview++
	}

	if err := NewFileCache(cfg.OutputFile).Save(cf); err != nil {
		return stats, fmt.Errorf("save output cache: %w", err)
	}
	return stats, nil
}

func assignNoCachedCandidate(cf *CacheFile, game GameRef, now time.Time) {
	cf.Items[game.GameID] = &CacheItem{
		Status:      StatusNone,
		Query:       buildQuery(game),
		UpdatedAt:   now,
		Videos:      nil,
		NeedsReview: false,
	}
}

func markAssignedAt(item *CacheItem, now time.Time) {
	if item == nil {
		return
	}
	item.UpdatedAt = now
	for i := range item.Videos {
		item.Videos[i].SelectedAt = now
	}
}

func markUsedVideoIDs(used map[string]bool, scored []ScoredCandidate) {
	for _, item := range scored {
		if item.Candidate.ID != "" {
			used[item.Candidate.ID] = true
		}
	}
}
