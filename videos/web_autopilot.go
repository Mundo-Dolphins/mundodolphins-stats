package videos

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type webAutopilotStepResponse struct {
	Action       string      `json:"action"`
	StopReason   string      `json:"stop_reason,omitempty"`
	Message      string      `json:"message,omitempty"`
	Game         *webGame    `json:"game,omitempty"`
	Query        string      `json:"query,omitempty"`
	Found        int         `json:"found,omitempty"`
	Eligible     int         `json:"eligible,omitempty"`
	Processed    int         `json:"processed,omitempty"`
	AutoApproved int         `json:"auto_approved,omitempty"`
	MarkedNone   int         `json:"marked_none,omitempty"`
	Removed      int         `json:"removed,omitempty"`
	Candidates   []Candidate `json:"candidates,omitempty"`
}

func (s *WebServer) handleAutopilotStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	resp, err := s.runAutopilotStep(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *WebServer) runAutopilotStep(ctx context.Context) (webAutopilotStepResponse, error) {
	games, cf, err := s.loadState()
	if err != nil {
		return webAutopilotStepResponse{}, err
	}
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
	}
	sortAutopilotGamesByDate(games)
	usedVideoIDs := collectUsedVideoIDs(cf)
	for _, game := range games {
		if isAutopilotFutureGame(game, s.now()) {
			out := s.webGameFromCache(game, cf)
			return webAutopilotStepResponse{
				Action:     "done",
				StopReason: "future_game",
				Message:    fmt.Sprintf("Autopilot stopped at future game %s: %s @ %s.", gameDateISO(game), game.AwayTeam, game.HomeTeam),
				Game:       &out,
			}, nil
		}
		item := cf.Items[game.GameID]
		removed := 0
		if s.shouldValidateAutopilotExisting(game.GameID, item) {
			var err error
			removed, err = s.validateAutopilotExistingVideos(ctx, cf, game, item)
			if err != nil {
				log.Printf("WARN web autopilot validation failed for game=%s: %v", game.GameID, err)
			}
			item = cf.Items[game.GameID]
			usedVideoIDs = collectUsedVideoIDs(cf)
		}
		if webAutopilotShouldStopForReview(item) {
			out := s.webGameFromCache(game, cf)
			return webAutopilotStepResponse{
				Action:     "needs_review",
				StopReason: "existing_candidate",
				Message:    "Autopilot stopped on an existing review candidate.",
				Game:       &out,
				Removed:    removed,
			}, nil
		}
		if !webAutopilotNeedsSearch(item) {
			continue
		}
		resp, err := s.runAutopilotSearch(ctx, game, usedVideoIDs)
		if removed > 0 {
			resp.Removed += removed
			if resp.Message != "" {
				resp.Message = fmt.Sprintf("Removed %d unavailable cached video(s). %s", removed, resp.Message)
			}
		}
		return resp, err
	}
	return webAutopilotStepResponse{
		Action:     "done",
		StopReason: "done",
		Message:    "No pending games.",
	}, nil
}

func sortAutopilotGamesByDate(games []GameRef) {
	sort.SliceStable(games, func(i, j int) bool {
		left := gameDateISO(games[i])
		right := gameDateISO(games[j])
		leftOK := len(left) == len("2006-01-02")
		rightOK := len(right) == len("2006-01-02")
		if leftOK && rightOK && left != right {
			return left < right
		}
		if leftOK != rightOK {
			return leftOK
		}
		return games[i].GameID < games[j].GameID
	})
}

func isAutopilotFutureGame(game GameRef, now time.Time) bool {
	gameDate, ok := parseAutopilotGameDate(game)
	if !ok || now.IsZero() {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	gameDay := time.Date(gameDate.Year(), gameDate.Month(), gameDate.Day(), 0, 0, 0, 0, today.Location())
	return gameDay.After(today)
}

func parseAutopilotGameDate(game GameRef) (time.Time, bool) {
	dateISO := gameDateISO(game)
	if len(dateISO) != len("2006-01-02") {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", dateISO)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func (s *WebServer) runAutopilotSearch(ctx context.Context, game GameRef, usedVideoIDs map[string]bool) (webAutopilotStepResponse, error) {
	query := buildQuery(game)
	minDurationSec := s.Config.MinDurationSec
	if minDurationSec <= 0 {
		minDurationSec = 300
	}
	maxResults := s.Config.MaxResults
	if maxResults <= 0 {
		maxResults = 15
	}
	scorer := s.scorer()
	candidates, err := s.Service.findCandidates(ctx, game, query, maxResults, minDurationSec)
	if err != nil {
		return webAutopilotStepResponse{}, fmt.Errorf("search %s: %w", game.GameID, err)
	}
	detailsFetcher := s.Service.Details
	filtered := enrichCandidates(ctx, candidates, 20, detailsFetcher)
	s.Service.saveDiscoveredCandidates(ctx, game, query, "web-autopilot-enriched", filtered)
	filtered = filterByMinDuration(filtered, minDurationSec)
	filtered = filterByLegacyRelevance(game, filtered, scorer)
	filtered = s.filterExistingCandidates(game.GameID, filtered)
	filtered = rankCandidates(game, filtered, scorer)
	if len(filtered) == 0 {
		status, err := s.markNoEligibleSearchIfEmptySelection(game, query)
		if err != nil {
			return webAutopilotStepResponse{}, err
		}
		cf, _ := s.Service.Cache.Load()
		out := s.webGameFromCache(game, cf)
		return webAutopilotStepResponse{
			Action:     "none",
			Message:    "No eligible candidates.",
			Game:       &out,
			Query:      query,
			Found:      len(candidates),
			Eligible:   0,
			Processed:  1,
			MarkedNone: boolToCount(status == StatusNone),
		}, nil
	}

	scored := scoreWebCandidatesForAutopilot(game, filtered, minDurationSec, usedVideoIDs)
	bestSet := selectAutopilotBestSet(scored)
	if len(bestSet) > 0 && ShouldAutoApproveSet(bestSet, s.autoApproveThreshold()) {
		out, err := s.saveAutopilotCandidateSet(game, query, bestSet, true)
		if err != nil {
			return webAutopilotStepResponse{}, err
		}
		return webAutopilotStepResponse{
			Action:       "approved",
			Message:      "Autopilot approved a high-confidence candidate.",
			Game:         &out,
			Query:        query,
			Found:        len(candidates),
			Eligible:     len(filtered),
			Processed:    1,
			AutoApproved: 1,
		}, nil
	}

	reviewSet := bestSet
	if len(reviewSet) == 0 && len(scored) > 0 {
		reviewSet = []ScoredCandidate{scored[0]}
	}
	if len(reviewSet) > 0 {
		if _, err := s.saveAutopilotCandidateSet(game, query, reviewSet, false); err != nil {
			return webAutopilotStepResponse{}, err
		}
	}
	cf, _ := s.Service.Cache.Load()
	out := s.webGameFromCache(game, cf)
	return webAutopilotStepResponse{
		Action:     "needs_review",
		StopReason: "candidate_review",
		Message:    "Autopilot stopped on a candidate that needs review.",
		Game:       &out,
		Query:      query,
		Found:      len(candidates),
		Eligible:   len(filtered),
		Processed:  1,
		Candidates: filtered,
	}, nil
}

func (s *WebServer) saveAutopilotCandidateSet(game GameRef, query string, scored []ScoredCandidate, autoApprove bool) (webGame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cf, err := s.Service.Cache.Load()
	if err != nil {
		return webGame{}, err
	}
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
	}
	assignCandidatesToCache(cf, game.GameID, scored, query, autoApprove, s.autoApproveThreshold())
	item := cf.Items[game.GameID]
	if item != nil {
		item.UpdatedAt = s.now()
		for i := range item.Videos {
			item.Videos[i].SelectedAt = item.UpdatedAt
		}
	}
	if err := s.Service.Cache.Save(cf); err != nil {
		return webGame{}, err
	}
	return s.webGameFromCache(game, cf), nil
}

func (s *WebServer) autoApproveThreshold() int {
	if s.Config.AutoApproveScore > 0 {
		return s.Config.AutoApproveScore
	}
	return 130
}

func webAutopilotShouldStopForReview(item *CacheItem) bool {
	return item != nil && len(item.Videos) > 0 && IsReviewableCacheItem(item)
}

func webAutopilotNeedsSearch(item *CacheItem) bool {
	if item == nil {
		return true
	}
	if IsPublishedCacheItem(item) || item.Status == StatusNone || item.Status == StatusRejected {
		return false
	}
	if len(item.Videos) == 0 {
		return true
	}
	return !IsDiscoveryCompleteStatus(item.Status)
}

func (s *WebServer) shouldValidateAutopilotExisting(gameID string, item *CacheItem) bool {
	if s == nil || s.Service == nil || s.Service.WatchChecker == nil {
		return false
	}
	if item == nil || len(item.Videos) == 0 {
		return false
	}
	if !IsPublishableStatus(item.Status) && item.Status != StatusCandidate {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.autopilotValidatedExisting[gameID]
}

func (s *WebServer) markAutopilotExistingValidated(gameID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.autopilotValidatedExisting == nil {
		s.autopilotValidatedExisting = map[string]bool{}
	}
	s.autopilotValidatedExisting[gameID] = true
}

func (s *WebServer) validateAutopilotExistingVideos(ctx context.Context, cf *CacheFile, game GameRef, item *CacheItem) (int, error) {
	defer s.markAutopilotExistingValidated(game.GameID)
	if cf == nil || item == nil || s == nil || s.Service == nil || s.Service.WatchChecker == nil {
		return 0, nil
	}

	checker := s.Service.WatchChecker
	inspector, canInspect := checker.(WatchPageInspector)
	removed := 0
	changed := false
	newVideos := make([]Video, 0, len(item.Videos))
	for _, video := range item.Videos {
		originalVideo := video
		videoID := video.ID
		if videoID == "" {
			videoID = extractYouTubeVideoID(video.URL)
		}
		if videoID == "" {
			newVideos = append(newVideos, video)
			continue
		}

		inspection := WatchInspection{}
		var err error
		if canInspect {
			inspection, err = inspector.Inspect(ctx, videoID)
		} else {
			inspection.Exists, err = checker.Exists(ctx, videoID)
		}
		if err != nil {
			newVideos = append(newVideos, video)
			continue
		}
		if !inspection.Exists {
			removed++
			changed = true
			continue
		}
		if canInspect {
			video = applyWatchDetailsToVideo(video, inspection.Details)
		}
		if video != originalVideo {
			changed = true
		}
		newVideos = append(newVideos, video)
	}

	if !changed {
		return removed, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	item.Videos = newVideos
	item.UpdatedAt = s.now()
	if len(item.Videos) == 0 {
		item.Status = StatusUnknown
		item.NeedsReview = false
		item.AutoScoring = nil
		item.Error = "cached YouTube video no longer exists"
	} else {
		item.Error = ""
	}
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
	}
	cf.Items[game.GameID] = item
	if err := s.Service.Cache.Save(cf); err != nil {
		return removed, err
	}
	return removed, nil
}

func scoreWebCandidatesForAutopilot(game GameRef, candidates []Candidate, minDurationSec int, usedVideoIDs map[string]bool) []ScoredCandidate {
	meta := ExtractGameMetadata(game)
	scored := make([]ScoredCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ytCandidate := webCandidateToAPICandidate(candidate)
		if ytCandidate.ID != "" && usedVideoIDs[ytCandidate.ID] {
			continue
		}
		scored = append(scored, ScoreCandidate(meta, ytCandidate, minDurationSec))
	}
	return scored
}

func selectAutopilotBestSet(scored []ScoredCandidate) []ScoredCandidate {
	accepted := make([]ScoredCandidate, 0, len(scored))
	for _, item := range scored {
		if !item.Discarded && item.MatchedTeams && item.Classification.Decision == DecisionAccept {
			accepted = append(accepted, item)
		}
	}
	best := SelectBestCandidate(accepted)
	if best == nil {
		return nil
	}
	if group := selectMultipartGroup(*best, accepted); len(group) > 0 {
		return group
	}
	return []ScoredCandidate{*best}
}

func webCandidateToAPICandidate(candidate Candidate) youtubeapi.VideoCandidate {
	durationSec := candidateDurationSec(candidate)
	embeddable := false
	if candidate.Embeddable != nil {
		embeddable = *candidate.Embeddable
	}
	return youtubeapi.VideoCandidate{
		ID:          candidate.VideoID,
		URL:         candidate.URL,
		Title:       candidate.Title,
		Description: candidate.Snippet,
		Channel:     candidate.Channel,
		Duration:    time.Duration(durationSec) * time.Second,
		Embeddable:  embeddable,
		Score:       candidate.Score,
	}
}

func boolToCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
