package videos

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// Matcher assigns videos to games using scoring, respecting uniqueness constraints.
type Matcher struct {
	Client               *youtubeapi.Client
	DiskCache            *DiskCacheManager
	Quota                *QuotaManager
	Index                *MatchIndex
	MinDurationSec       int
	MaxResults           int
	AutoApprove          bool
	AutoApproveThreshold int
}

// MatchFromChannels takes channel-discovered playlist items and matches them to games.
// Each video can only be assigned to ONE game, and each game gets at most one video.
func (m *Matcher) MatchFromChannels(ctx context.Context, items []youtubeapi.PlaylistItem, state *PipelineState, cf *CacheFile) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	// Collect unique video IDs from channel items that might be relevant
	videoIDs := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if item.VideoID == "" || seen[item.VideoID] {
			continue
		}
		seen[item.VideoID] = true
		videoIDs = append(videoIDs, item.VideoID)
	}

	// Hydrate video details (from cache first, then API)
	candidates, err := m.hydrateVideos(ctx, videoIDs)
	if err != nil {
		return 0, fmt.Errorf("hydrate channel videos: %w", err)
	}

	// Score each candidate against all unmatched games, pick best assignment
	matched := 0
	usedVideoIDs := collectUsedVideoIDs(cf)

	// Build a list of candidates per game for scoring
	type gameMatch struct {
		gameID string
		video  youtubeapi.VideoCandidate
		score  int
	}
	var allMatches []gameMatch

	for _, c := range candidates {
		if usedVideoIDs[c.ID] {
			continue
		}
		// Block videos from banned channels
		if c.ChannelID != "" && IsBlockedChannel(c.ChannelID) {
			log.Printf("BLOCKED channel %s (%s) for video %s", c.ChannelID, c.Channel, c.ID)
			continue
		}
		// Score against all games needing video
		for _, gameID := range m.Index.AllGameIDs() {
			if state.HasVideo(gameID) {
				continue
			}
			game, ok := m.Index.GameByID(gameID)
			if !ok {
				continue
			}
			meta := ExtractGameMetadata(game)
			if meta.Home.FullName == "" || meta.Away.FullName == "" {
				continue
			}
			scored := ScoreCandidate(meta, c, m.MinDurationSec)
			if scored.Discarded || !scored.MatchedTeams || scored.Classification.Decision != DecisionAccept {
				continue
			}
			allMatches = append(allMatches, gameMatch{
				gameID: gameID,
				video:  c,
				score:  scored.Score.Total,
			})
		}
	}

	// Sort by score descending for greedy assignment
	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].score != allMatches[j].score {
			return allMatches[i].score > allMatches[j].score
		}
		return allMatches[i].gameID < allMatches[j].gameID
	})

	// Greedy: assign best match first, skip used videos or games
	assignedGames := map[string]bool{}
	for _, gm := range allMatches {
		if usedVideoIDs[gm.video.ID] || assignedGames[gm.gameID] {
			continue
		}

		// Assign
		game, _ := m.Index.GameByID(gm.gameID)
		scored := ScoreCandidate(ExtractGameMetadata(game), gm.video, m.MinDurationSec)
		assignCandidateToCache(cf, gm.gameID, scored, "channel-scan", m.AutoApprove, m.AutoApproveThreshold)
		state.MarkWithVideo(gm.gameID)
		state.ChannelMatches[gm.gameID] = gm.video.ID
		usedVideoIDs[gm.video.ID] = true
		assignedGames[gm.gameID] = true
		matched++

		log.Printf("MATCH game %s ← video %s %q (score=%d, source=channel)",
			gm.gameID, gm.video.ID, gm.video.Title, gm.score)
	}

	return matched, nil
}

// SearchFallback performs YouTube search for games that didn't match from channels.
// Processes games from the search queue, stopping when quota runs out.
func (m *Matcher) SearchFallback(ctx context.Context, state *PipelineState, cf *CacheFile) (added, noResult int, err error) {
	usedVideoIDs := collectUsedVideoIDs(cf)

	for {
		gameID, ok := state.PopSearchQueue()
		if !ok {
			break // Queue empty
		}

		if state.HasVideo(gameID) {
			continue
		}

		game, ok := m.Index.GameByID(gameID)
		if !ok {
			state.MarkWithoutVideo(gameID)
			continue
		}

		meta := ExtractGameMetadata(game)
		if meta.Home.FullName == "" || meta.Away.FullName == "" {
			log.Printf("SKIP game %s: missing team names", gameID)
			state.MarkWithoutVideo(gameID)
			noResult++
			continue
		}

		// Try to use cached search results first
		found, err := m.trySearchFromCache(ctx, gameID, meta, cf, state, usedVideoIDs)
		if err != nil {
			return added, noResult, fmt.Errorf("search cache for %s: %w", gameID, err)
		}
		if found {
			added++
			continue
		}

		// Perform live search: costs 100 units
		if !m.Quota.CanAfford(QuotaCostSearch) {
			// Put it back at the front for next run
			state.SearchQueueRemaining = append([]string{gameID}, state.SearchQueueRemaining...)
			log.Printf("QUOTA: cannot afford search (cost=%d, remaining=%d), stopping", QuotaCostSearch, m.Quota.Remaining())
			break
		}

		result, err := m.searchForGame(ctx, gameID, meta, cf, state, usedVideoIDs)
		if err != nil {
			if m.Quota.IsExhausted() {
				state.SearchQueueRemaining = append([]string{gameID}, state.SearchQueueRemaining...)
				log.Printf("QUOTA: exhausted during search, stopping")
				break
			}
			log.Printf("WARN: search failed for game %s: %v", gameID, err)
			continue
		}
		if result {
			added++
		} else {
			noResult++
		}
	}

	return added, noResult, nil
}

func (m *Matcher) trySearchFromCache(ctx context.Context, gameID string, meta GameMetadata, cf *CacheFile, state *PipelineState, usedVideoIDs map[string]bool) (bool, error) {
	if !m.DiskCache.HasSearchCache(gameID) {
		return false, nil
	}

	entry, err := m.DiskCache.LoadSearchCache(gameID)
	if err != nil || entry == nil || len(entry.VideoIDs) == 0 {
		return false, nil
	}

	// Hydrate and score cached search results
	candidates, err := m.hydrateVideos(ctx, entry.VideoIDs)
	if err != nil {
		return false, err
	}

	bestSet := m.scoreBestSet(meta, candidates, usedVideoIDs)
	if len(bestSet) == 0 {
		state.MarkWithoutVideo(gameID)
		return false, nil
	}

	assignCandidatesToCache(cf, gameID, bestSet, "search-cached", m.AutoApprove, m.AutoApproveThreshold)
	state.MarkWithVideo(gameID)
	for _, best := range bestSet {
		usedVideoIDs[best.Candidate.ID] = true
	}
	log.Printf("MATCH game %s ← %d video(s) (score=%d, source=search-cached)", gameID, len(bestSet), bestSet[0].Score.Total)
	return true, nil
}

func (m *Matcher) searchForGame(ctx context.Context, gameID string, meta GameMetadata, cf *CacheFile, state *PipelineState, usedVideoIDs map[string]bool) (bool, error) {
	queries := append([]string{BuildPrimaryQuery(meta)}, BuildFallbackQueries(meta)...)

	for _, query := range queries {
		if !m.Quota.TryConsume(QuotaCostSearch) {
			return false, fmt.Errorf("quota exhausted for search")
		}

		log.Printf("SEARCH game %s: %q (%s)", gameID, query, m.Quota)

		ids, err := m.Client.SearchVideoIDs(ctx, query, m.MaxResults)
		if err != nil {
			if youtubeapi.IsQuotaOrRateLimit(err) {
				return false, fmt.Errorf("youtube quota/rate limit: %w", err)
			}
			log.Printf("WARN: search query failed for %s: %v", gameID, err)
			continue
		}

		if len(ids) == 0 {
			continue
		}

		// Cache search results
		if err := m.DiskCache.SaveSearchCache(&SearchCacheEntry{
			GameID:    gameID,
			Query:     query,
			FetchedAt: m.DiskCache.now(),
			VideoIDs:  ids,
		}); err != nil {
			log.Printf("WARN: failed to cache search results for %s: %v", gameID, err)
		}

		// Hydrate and score
		candidates, err := m.hydrateVideos(ctx, ids)
		if err != nil {
			log.Printf("WARN: hydrate failed for %s: %v", gameID, err)
			continue
		}

		bestSet := m.scoreBestSet(meta, candidates, usedVideoIDs)
		if len(bestSet) > 0 {
			assignCandidatesToCache(cf, gameID, bestSet, "search", m.AutoApprove, m.AutoApproveThreshold)
			state.MarkWithVideo(gameID)
			for _, best := range bestSet {
				usedVideoIDs[best.Candidate.ID] = true
			}
			log.Printf("ADDED game %s: %d video(s) (score=%d, source=search)", gameID, len(bestSet), bestSet[0].Score.Total)
			return true, nil
		}
	}

	state.MarkWithoutVideo(gameID)
	log.Printf("NO_RESULT game %s (%s vs %s): no valid video found via search", gameID, meta.Away.FullName, meta.Home.FullName)
	return false, nil
}

// hydrateVideos fetches full video details, using disk cache first, then API.
func (m *Matcher) hydrateVideos(ctx context.Context, videoIDs []string) ([]youtubeapi.VideoCandidate, error) {
	cached, missing := m.DiskCache.LoadMultiVideoCache(videoIDs)

	if len(missing) == 0 {
		return cached, nil
	}

	// API call: videos.list on missing IDs (1 unit per 50)
	batches := (len(missing) + 49) / 50
	cost := batches * QuotaCostVideosList
	if !m.Quota.TryConsume(cost) {
		// Return what we have from cache
		return cached, nil
	}

	fresh, err := m.Client.HydrateVideos(ctx, missing)
	if err != nil {
		return cached, fmt.Errorf("hydrate videos: %w", err)
	}

	// Cache freshly fetched videos
	if err := m.DiskCache.SaveMultiVideoCache(fresh); err != nil {
		log.Printf("WARN: failed to cache video details: %v", err)
	}

	return append(cached, fresh...), nil
}

func (m *Matcher) scoreBestSet(meta GameMetadata, candidates []youtubeapi.VideoCandidate, usedVideoIDs map[string]bool) []ScoredCandidate {
	scored := m.scoreAccepted(meta, candidates, usedVideoIDs)
	best := SelectBestCandidate(scored)
	if best == nil {
		return nil
	}
	group := selectMultipartGroup(*best, scored)
	if len(group) > 0 {
		return group
	}
	return []ScoredCandidate{*best}
}

func (m *Matcher) scoreAccepted(meta GameMetadata, candidates []youtubeapi.VideoCandidate, usedVideoIDs map[string]bool) []ScoredCandidate {
	scored := make([]ScoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		if usedVideoIDs[c.ID] {
			continue
		}
		sc := ScoreCandidate(meta, c, m.MinDurationSec)
		if c.ChannelID != "" && IsBlockedChannel(c.ChannelID) && !allowBlockedChannelCandidate(meta, c, sc) {
			continue
		}
		if !sc.Discarded && sc.MatchedTeams && sc.Classification.Decision == DecisionAccept {
			scored = append(scored, sc)
		}
	}
	return scored
}

func selectMultipartGroup(best ScoredCandidate, scored []ScoredCandidate) []ScoredCandidate {
	bestPart := DetectPartInfo(best.Candidate.Title, best.Candidate.Description)
	if bestPart.Part == 0 {
		return nil
	}
	key := MultipartGroupKey(best.Candidate.Title, best.Candidate.Description, best.Candidate.Channel)
	if key == "" {
		return nil
	}
	group := make([]ScoredCandidate, 0)
	for _, candidate := range scored {
		if MultipartGroupKey(candidate.Candidate.Title, candidate.Candidate.Description, candidate.Candidate.Channel) == key {
			group = append(group, candidate)
		}
	}
	sort.SliceStable(group, func(i, j int) bool {
		left := DetectPartInfo(group[i].Candidate.Title, group[i].Candidate.Description)
		right := DetectPartInfo(group[j].Candidate.Title, group[j].Candidate.Description)
		if left.Part != right.Part {
			return left.Part < right.Part
		}
		return group[i].Candidate.ID < group[j].Candidate.ID
	})
	if multipartGroupComplete(group) {
		return group
	}
	return nil
}

func collectUsedVideoIDs(cf *CacheFile) map[string]bool {
	used := make(map[string]bool)
	if cf == nil {
		return used
	}
	for _, item := range cf.Items {
		if item == nil || (!IsPublishableStatus(item.Status) && item.Status != StatusCandidate) {
			continue
		}
		for _, v := range item.Videos {
			if v.ID != "" {
				used[v.ID] = true
			}
		}
	}
	return used
}

func assignCandidateToCache(cf *CacheFile, gameID string, scored ScoredCandidate, source string, autoApprove bool, autoApproveThreshold int) {
	assignCandidatesToCache(cf, gameID, []ScoredCandidate{scored}, source, autoApprove, autoApproveThreshold)
}

func assignCandidatesToCache(cf *CacheFile, gameID string, scored []ScoredCandidate, source string, autoApprove bool, autoApproveThreshold int) {
	if len(scored) == 0 {
		return
	}
	approved := autoApprove && ShouldAutoApproveSet(scored, autoApproveThreshold)
	status := StatusCandidate
	needsReview := true
	if approved {
		status = StatusApproved
		needsReview = false
	}
	videos := make([]Video, 0, len(scored))
	for _, item := range scored {
		videos = append(videos, videoFromScoredCandidate(item))
	}
	first := scored[0]
	role := ContentRole(first.Classification.Signals.ContentRole)
	if role == "" {
		role = ClassifyContentRole(first.Candidate.Title, first.Candidate.Description, first.Candidate.Channel, int(first.Candidate.Duration.Seconds()))
	}
	cf.Items[gameID] = &CacheItem{
		Status:      status,
		Query:       source,
		UpdatedAt:   time.Now(),
		NeedsReview: needsReview,
		Videos:      videos,
		AutoScoring: &AutoScoringMeta{
			ScoreBreakdown: first.Score,
			MatchedTeams:   first.MatchedTeams,
			MatchedDate:    string(first.MatchedDate),
			MatchedWeek:    string(first.MatchedWeek),
			ContentRole:    string(role),
			ViewCount:      first.Candidate.ViewCount,
			Embeddable:     first.Candidate.Embeddable,
			DurationISO:    formatISO8601(first.Candidate.Duration),
			Source:         source,
			Confidence:     confidenceFromScore(first.Score.Total),
			NeedsReview:    needsReview,
			Classification: &first.Classification,
		},
	}
}

func videoFromScoredCandidate(scored ScoredCandidate) Video {
	c := scored.Candidate
	embeddable := c.Embeddable
	role := ClassifyContentRole(c.Title, c.Description, c.Channel, int(c.Duration.Seconds()))
	part := DetectPartInfo(c.Title, c.Description)
	return Video{
		ID:              c.ID,
		URL:             c.URL,
		Title:           c.Title,
		Channel:         c.Channel,
		ChannelID:       c.ChannelID,
		Role:            string(role),
		Part:            part.Part,
		PartTotal:       part.Total,
		DurationSeconds: int(c.Duration.Seconds()),
		Embeddable:      &embeddable,
		SelectedAt:      time.Now(),
	}
}

func allowBlockedChannelCandidate(meta GameMetadata, c youtubeapi.VideoCandidate, scored ScoredCandidate) bool {
	role := ClassifyContentRole(c.Title, c.Description, c.Channel, int(c.Duration.Seconds()))
	if role != ContentRoleOfficialHighlight {
		return false
	}
	if !ContainsBothTeams(c.Title, "", meta.Home, meta.Away) {
		return false
	}
	return scored.MatchedDate == DateMatchFull || scored.MatchedWeek == DateMatchFull
}

func ShouldAutoApprove(scored ScoredCandidate, threshold int) bool {
	if threshold <= 0 {
		threshold = 130
	}
	if scored.Discarded || !scored.MatchedTeams || scored.Score.Total < threshold {
		return false
	}
	if scored.Classification.Decision != DecisionAccept {
		return false
	}
	if riskyAutonomousChannel(scored.Candidate.Channel) {
		return false
	}
	if len(scored.Classification.Signals.MixedSignals) > 0 || len(scored.Classification.Signals.ReviewTerms) > 0 {
		return false
	}
	if scored.MatchedDate != DateMatchFull && scored.MatchedWeek != DateMatchFull {
		return false
	}
	role := ContentRole(scored.Classification.Signals.ContentRole)
	if !scored.Classification.Signals.TitleTeams && role != ContentRoleRadio && role != ContentRoleGameFilm {
		return false
	}
	switch role {
	case ContentRoleFullGame, ContentRoleRadio, ContentRoleGameFilm, ContentRolePart:
		return true
	case ContentRoleHighlight:
		return scored.Classification.Signals.TitleTeams &&
			scored.Classification.Signals.DurationSeconds >= 300
	case ContentRoleOfficialHighlight:
		return isOfficialNFLChannel(scored.Candidate.Channel) && scored.Classification.Signals.TitleTeams
	case ContentRoleUnknown:
		return scored.MatchedDate == DateMatchFull &&
			scored.Classification.Signals.TitleTeams &&
			scored.Classification.Signals.DurationSeconds >= 300
	default:
		return false
	}
}

func riskyAutonomousChannel(channel string) bool {
	norm := NormalizeText(channel)
	for _, term := range []string{"stream", "streams", "live"} {
		if containsNormalizedTerm(norm, term) {
			return true
		}
	}
	return false
}

func ShouldAutoApproveSet(scored []ScoredCandidate, threshold int) bool {
	if len(scored) == 0 {
		return false
	}
	for _, item := range scored {
		if !ShouldAutoApprove(item, threshold) {
			return false
		}
	}
	if DetectPartInfo(scored[0].Candidate.Title, scored[0].Candidate.Description).Part > 0 {
		return multipartGroupComplete(scored)
	}
	return len(scored) == 1
}

func multipartGroupComplete(scored []ScoredCandidate) bool {
	if len(scored) < 2 {
		return false
	}
	total := 0
	seen := map[int]bool{}
	for _, item := range scored {
		part := DetectPartInfo(item.Candidate.Title, item.Candidate.Description)
		if part.Part <= 0 {
			return false
		}
		seen[part.Part] = true
		if part.Total > total {
			total = part.Total
		}
	}
	if total == 0 {
		total = len(seen)
	}
	if len(seen) < total {
		return false
	}
	for i := 1; i <= total; i++ {
		if !seen[i] {
			return false
		}
	}
	return true
}
