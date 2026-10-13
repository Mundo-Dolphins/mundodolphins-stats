package videos

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// PipelineConfig holds all configuration for a pipeline run.
type PipelineConfig struct {
	MaxQuota         int
	ProcessedDir     string // base dir for processed game JSONs
	VideosDir        string // root dir for videos cache tree (e.g. "cache/videos")
	DryRun           bool
	Reset            bool
	ValidateExisting bool
	NoChannelScan    bool
	NoSearch         bool
	AutoApprove      bool
	AutoApproveScore int
	MinDurationSec   int
	MaxResults       int
	Season           int    // 0 = all seasons
	GameID           string // "" = all games
}

// PipelineStats summarizes the outcome of a pipeline run.
type PipelineStats struct {
	GamesTotal        int
	GamesNeedingVideo int
	ChannelMatched    int
	SearchAdded       int
	SearchNoResult    int
	AutoApproved      int
	CandidatesAdded   int
	VideosRemoved     int
	QuotaUsed         int
	QuotaMax          int
	StoppedByQuota    bool
}

// Pipeline orchestrates the full incremental video discovery workflow.
type Pipeline struct {
	Client *youtubeapi.Client
	Config PipelineConfig
	Now    func() time.Time
}

// Run executes the full pipeline: init → validate → index → channel scan → match → search → save.
func (p *Pipeline) Run(ctx context.Context) (*PipelineStats, error) {
	if p.Client == nil {
		return nil, fmt.Errorf("youtube api client is required")
	}
	if p.Now == nil {
		p.Now = time.Now
	}

	stats := &PipelineStats{
		QuotaMax: p.Config.MaxQuota,
	}

	// --- Phase 0: Initialize ---
	paths := NewPaths(p.Config.VideosDir)
	log.Printf("using cache directory: %s", paths.BaseDir)

	// Migrate legacy videos/ directory if present
	if err := paths.MigrateFromLegacy("videos"); err != nil {
		log.Printf("WARN: legacy migration: %v", err)
	}

	// Ensure all required directories exist
	if err := paths.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("create directories: %w", err)
	}

	log.Printf("loading state from %s", paths.StateFile)
	stateMgr := NewStateManager(paths.StateFile)
	stateMgr.nowFunc = p.Now

	var state *PipelineState
	var err error
	if p.Config.Reset {
		state = stateMgr.Reset()
		log.Println("STATE: reset all progress")
	} else {
		state, err = stateMgr.Load()
		if err != nil {
			return nil, fmt.Errorf("load state: %w", err)
		}
		log.Printf("STATE: loaded (processed=%d, with_video=%d, without_video=%d, channels_scanned=%d)",
			len(state.ProcessedGames), len(state.GamesWithVideo), len(state.GamesWithoutVideo), len(state.ChannelsScanned))
	}

	// Init quota manager with daily reset
	quota := NewQuotaManager(p.Config.MaxQuota)
	quota.nowFunc = p.Now
	quota.SetLastResetDate(state.LastQuotaReset)
	if quota.ResetIfNewDay() {
		log.Println("QUOTA: new day detected, resetting counter")
		state.QuotaUsedToday = 0
	} else {
		quota.SetUsed(state.QuotaUsedToday)
		log.Printf("QUOTA: restored %s", quota)
	}

	// Load or create video cache (final output)
	log.Printf("loading videos from %s", paths.VideosFile)
	fileCache := NewFileCache(paths.VideosFile)
	cf, err := fileCache.Load()
	if err != nil {
		return nil, fmt.Errorf("load video cache: %w", err)
	}
	state.ReconcileWithCache(cf)
	previouslySelected := selectedVideoIDSet(cf)

	registry := NewChannelRegistry(paths.ChannelsFile)
	registry.now = p.Now
	channelRegistry, err := registry.Load()
	if err != nil {
		return nil, fmt.Errorf("load channels registry: %w", err)
	}
	sanitized, err := registry.Sanitize(ctx, channelRegistry, cf, p.Client)
	if err != nil {
		return nil, fmt.Errorf("sanitize channels registry: %w", err)
	}
	log.Printf("LOADED channels: %d", len(channelRegistry.Channels))
	if sanitized.Validated > 0 || sanitized.Removed > 0 || sanitized.Recovered > 0 || sanitized.Merged > 0 {
		log.Printf("CHANNEL REGISTRY: validated=%d removed=%d recovered=%d merged=%d",
			sanitized.Validated, sanitized.Removed, sanitized.Recovered, sanitized.Merged)
	}

	diskCache := NewDiskCacheManager(paths.CacheDir)
	diskCache.nowFunc = p.Now

	// --- Phase 1: Validate existing videos ---
	if p.Config.ValidateExisting && !p.Config.DryRun {
		log.Println("VALIDATED: checking existing video availability...")
		removed, err := ValidateExistingVideos(ctx, cf, p.Client)
		if err != nil {
			log.Printf("WARN: video validation failed: %v", err)
		} else {
			stats.VideosRemoved = len(removed)
			quota.ForceConsume(QuotaCostVideosList) // CheckExistence = 1 unit per batch
			if len(removed) > 0 {
				log.Printf("VALIDATED: removed %d unavailable videos", len(removed))
				// Update state: remove games whose videos were invalidated
				for _, gameID := range gamesWithoutValidVideo(cf) {
					state.GamesWithVideo = removeFromSlice(state.GamesWithVideo, gameID)
					if !state.IsGameProcessed(gameID) {
						state.MarkProcessed(gameID)
					}
				}
			}
		}
	}

	// --- Phase 2: Discover and index all games ---
	log.Println("INDEX: loading game files...")
	games, err := DiscoverAllGames(p.Config.ProcessedDir, p.Config.Season, p.Config.GameID)
	if err != nil {
		return nil, fmt.Errorf("discover games: %w", err)
	}
	if len(games) == 0 {
		log.Println("No games found matching filters.")
		return stats, nil
	}

	index := NewMatchIndex(games)
	stats.GamesTotal = index.TotalGames()
	gamesNeedingVideo := index.GamesNeedingVideo(cf)
	stats.GamesNeedingVideo = len(gamesNeedingVideo)

	log.Printf("INDEX: %d total games, %d needing video", stats.GamesTotal, stats.GamesNeedingVideo)

	if stats.GamesNeedingVideo == 0 {
		log.Println("All games have videos assigned. Nothing to do.")
		return stats, nil
	}

	// --- Phase 3: Channel scan (cheap — 1 unit per page of 50 videos) ---
	if !p.Config.NoChannelScan && !state.ChannelScanCompleted {
		for _, ch := range channelRegistry.Channels {
			if IsBlockedChannel(ch.ID) {
				log.Printf("SKIPPED channel %s (%s): blocked", ch.Name, ch.ID)
				continue
			}
			if ch.VideosCount < DefaultMinChannelVideos || ch.Score < DefaultMinChannelScore {
				log.Printf("SKIPPED channel %s (%s): low score/count (score=%.2f, videos=%d)",
					ch.Name, ch.ID, ch.Score, ch.VideosCount)
			}
		}
		selectedChannels := registry.SelectForScan(channelRegistry, DefaultMinChannelVideos, DefaultMinChannelScore)
		if len(selectedChannels) == 0 {
			log.Println("CHANNEL SCAN: no validated channels available")
		}
		for _, ch := range selectedChannels {
			log.Printf("SCANNING channel %s (%s)", ch.Name, ch.ID)
		}
		log.Println("CHANNEL SCAN: starting...")
		scanner := &ChannelScanner{
			Client:   p.Client,
			Cache:    diskCache,
			Quota:    quota,
			Channels: selectedChannels,
		}
		results, err := scanner.ScanAll(ctx, state)
		if err != nil {
			log.Printf("WARN: channel scan error: %v", err)
		}
		totalVids := 0
		for _, r := range results {
			totalVids += r.VideoCount
		}
		log.Printf("CHANNEL SCAN: %d channels scanned, %d total videos collected", len(results), totalVids)

		// --- Phase 4: Match channel videos → games ---
		if totalVids > 0 {
			log.Println("MATCH: matching channel videos to games...")
			channelItems, err := scanner.CollectAllChannelVideos()
			if err != nil {
				log.Printf("WARN: collecting channel videos: %v", err)
			} else {
				matcher := &Matcher{
					Client:               p.Client,
					DiskCache:            diskCache,
					Quota:                quota,
					Index:                index,
					MinDurationSec:       p.Config.MinDurationSec,
					MaxResults:           p.Config.MaxResults,
					AutoApprove:          p.Config.AutoApprove,
					AutoApproveThreshold: p.Config.AutoApproveScore,
				}
				matched, err := matcher.MatchFromChannels(ctx, channelItems, state, cf)
				if err != nil {
					log.Printf("WARN: channel matching error: %v", err)
				}
				stats.ChannelMatched = matched
				log.Printf("MATCH: %d games matched from channel videos", matched)
			}
		}
	} else if p.Config.NoChannelScan {
		log.Println("CHANNEL SCAN: skipped (--no-channel-scan)")
	} else {
		log.Println("CHANNEL SCAN: already completed, skipping")
	}

	// Save intermediate progress
	if !p.Config.DryRun {
		state.QuotaUsedToday = quota.Used()
		state.LastQuotaReset = quota.LastResetDate()
		if err := stateMgr.Save(state); err != nil {
			log.Printf("WARN: failed to save intermediate state: %v", err)
		}
		if err := fileCache.Save(cf); err != nil {
			log.Printf("WARN: failed to save intermediate cache: %v", err)
		}
	}

	// --- Phase 5: Search fallback (expensive — 100 units per search) ---
	if !p.Config.NoSearch {
		// Build search queue from remaining unmatched games
		state.BuildSearchQueue(index.GamesNeedingVideo(cf))
		queueSize := len(state.SearchQueueRemaining)

		if queueSize > 0 && !quota.IsExhausted() {
			log.Printf("SEARCH: %d games in search queue, quota=%s", queueSize, quota)

			matcher := &Matcher{
				Client:               p.Client,
				DiskCache:            diskCache,
				Quota:                quota,
				Index:                index,
				MinDurationSec:       p.Config.MinDurationSec,
				MaxResults:           p.Config.MaxResults,
				AutoApprove:          p.Config.AutoApprove,
				AutoApproveThreshold: p.Config.AutoApproveScore,
			}
			added, noResult, err := matcher.SearchFallback(ctx, state, cf)
			if err != nil {
				log.Printf("WARN: search fallback error: %v", err)
			}
			stats.SearchAdded = added
			stats.SearchNoResult = noResult
			log.Printf("SEARCH: added=%d, no_result=%d", added, noResult)
		} else if quota.IsExhausted() {
			stats.StoppedByQuota = true
			log.Printf("QUOTA: exhausted (%s), search skipped entirely", quota)
		} else {
			log.Println("SEARCH: no games need searching")
		}
	} else {
		log.Println("SEARCH: skipped (--no-search)")
	}

	// --- Phase 6: Save final state ---
	allowedNewChannels := map[string]bool{}
	if !p.Config.DryRun {
		discovery := NewChannelDiscoveryManager(paths, diskCache, p.Client, quota)
		discovery.now = p.Now
		candidates, err := discovery.DiscoverFromSearchMatches(ctx, cf, channelRegistry, previouslySelected)
		if err != nil {
			log.Printf("WARN: channel discovery failed: %v", err)
		} else {
			allowedNewChannels = candidates
		}
	}

	addedChannels, updatedChannels := registry.UpdateFromAccepted(channelRegistry, cf, previouslySelected, allowedNewChannels)
	if addedChannels > 0 || updatedChannels > 0 {
		log.Printf("CHANNEL LEARNING: discovered=%d updated=%d", addedChannels, updatedChannels)
	}

	stats.QuotaUsed = quota.Used()
	stats.AutoApproved = countAutoApproved(cf)
	stats.CandidatesAdded = countCandidatesNeedingReview(cf)
	if quota.IsExhausted() {
		stats.StoppedByQuota = true
	}

	if !p.Config.DryRun {
		state.QuotaUsedToday = quota.Used()
		state.LastQuotaReset = quota.LastResetDate()
		if err := stateMgr.Save(state); err != nil {
			return stats, fmt.Errorf("save final state: %w", err)
		}
		if err := fileCache.Save(cf); err != nil {
			return stats, fmt.Errorf("save final cache: %w", err)
		}
		if err := registry.Save(channelRegistry); err != nil {
			return stats, fmt.Errorf("save channels registry: %w", err)
		}
		log.Printf("STOP: saved state and cache (quota=%s)", quota)
	} else {
		log.Printf("STOP: dry-run complete (quota=%s)", quota)
	}

	return stats, nil
}

// gamesWithoutValidVideo returns game IDs from the cache where a publishable status has no videos.
func gamesWithoutValidVideo(cf *CacheFile) []string {
	var result []string
	for gameID, item := range cf.Items {
		if item != nil && IsPublishableStatus(item.Status) && len(item.Videos) == 0 {
			result = append(result, gameID)
		}
	}
	return result
}

func countAutoApproved(cf *CacheFile) int {
	if cf == nil {
		return 0
	}
	total := 0
	for _, item := range cf.Items {
		if item == nil || item.Status != StatusApproved || item.AutoScoring == nil {
			continue
		}
		if !item.NeedsReview && !item.AutoScoring.NeedsReview {
			total++
		}
	}
	return total
}

func countCandidatesNeedingReview(cf *CacheFile) int {
	if cf == nil {
		return 0
	}
	total := 0
	for _, item := range cf.Items {
		if IsReviewableCacheItem(item) {
			total++
		}
	}
	return total
}
