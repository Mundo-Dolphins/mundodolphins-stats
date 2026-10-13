package videos

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type DiscoverConfig struct {
	ProcessedDir     string
	CacheFile        string
	Year             int
	FromYear         int
	ToYear           int
	MaxResults       int
	MinDurationSec   int
	Concurrency      int
	Headless         bool
	Force            bool
	RetryNone        bool
	ValidateExisting bool
	NonInteractive   bool
	ScoreThreshold   int
	AutoMax          int
}

type Service struct {
	Searcher       Searcher
	Cache          Cache
	GameLoader     GameLoader
	Selector       *Selector
	Scorer         Scorer
	Now            func() time.Time
	Details        func(ctx context.Context, videoID string) (VideoDetails, error)
	APIDiscoverer  func(ctx context.Context, game GameRef, maxResults int, minDurationSec int) ([]Candidate, error)
	WatchChecker   WatchAvailabilityChecker
	CandidateStore CandidateStore

	apiMu              sync.Mutex
	apiQuotaExhausted  bool
	apiQuotaExhaustion string
}

type gameCandidateStore interface {
	LoadCandidatesForGame(ctx context.Context, game GameRef, limit int) ([]Candidate, error)
}

type gameEntry struct {
	game         GameRef
	candidates   []Candidate
	query        string
	bestScore    int
	bestDuration int
}

func (s *Service) Discover(ctx context.Context, cfg DiscoverConfig) error {
	if s.GameLoader == nil {
		return fmt.Errorf("game loader is nil")
	}
	if s.Cache == nil {
		return fmt.Errorf("cache is nil")
	}
	if s.Searcher == nil {
		return fmt.Errorf("searcher is nil")
	}

	fromYear := cfg.FromYear
	toYear := cfg.ToYear
	if cfg.Year > 0 {
		fromYear = cfg.Year
		toYear = cfg.Year
	}
	games, err := s.GameLoader.LoadGames(fromYear, toYear)
	if err != nil {
		return err
	}
	printInfof(s.Selector, "Loaded %d game(s) from %s for years %d-%d.\n", len(games), cfg.ProcessedDir, fromYear, toYear)
	if len(games) == 0 {
		printInfof(s.Selector, "No games found. Expected JSON files at %s/*.json or %s/<season>/*.json.\n", cfg.ProcessedDir, cfg.ProcessedDir)
		return nil
	}
	cf, err := s.Cache.Load()
	if err != nil {
		return err
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if cfg.AutoMax <= 0 {
		cfg.AutoMax = 1
	}
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 15
	}
	minDurationSec := cfg.MinDurationSec
	if minDurationSec <= 0 {
		minDurationSec = 300
	}
	scorer := s.Scorer
	if scorer.MinDurationSec == 0 {
		scorer.MinDurationSec = cfg.MinDurationSec
	}
	if scorer.ScoreThreshold == 0 {
		scorer.ScoreThreshold = cfg.ScoreThreshold
	}
	if !cfg.NonInteractive && s.Selector == nil {
		return fmt.Errorf("selector is nil for interactive mode")
	}

	entries := make([]gameEntry, 0, len(games))
	for _, game := range games {
		item, ok := cf.Items[game.GameID]
		if ok && item != nil {
			if item.Status == StatusNone && !cfg.Force && !cfg.RetryNone {
				continue
			}
			if item.Status != StatusNone && IsDiscoveryCompleteStatus(item.Status) && !cfg.Force {
				if cfg.NonInteractive {
					continue
				}
				if s.Selector == nil || s.Selector.In == nil || s.Selector.Out == nil {
					continue
				}
				if cfg.ValidateExisting && s.WatchChecker != nil {
					rediscover, changed, reviewAlternatives, err := s.validateCachedSelection(ctx, game, item, scorer, cfg.ScoreThreshold)
					if err != nil {
						return err
					}
					if changed {
						if err := s.Cache.Save(cf); err != nil {
							return err
						}
					}
					if rediscover {
						item.Status = StatusNone
						item.UpdatedAt = s.Now()
						if err := s.Cache.Save(cf); err != nil {
							return err
						}
						goto search
					}
					if reviewAlternatives {
						shouldSearch, err := promptSearchAlternatives(s.Selector, game, item)
						if err != nil {
							return err
						}
						if !shouldSearch {
							continue
						}
						goto search
					}
				}
				shouldModify, err := promptModifyExisting(s.Selector, game, item)
				if err != nil {
					return err
				}
				if !shouldModify {
					continue
				}
			}
		}

	search:
		announceSearch(game, s.Selector)
		query := buildQuery(game)
		candidates, err := s.findCandidates(ctx, game, query, cfg.MaxResults, minDurationSec)
		if err != nil {
			printInfof(s.Selector, "Search failed for %s: %v\n", game.GameID, err)
			cf.Items[game.GameID] = &CacheItem{Status: StatusError, Query: query, UpdatedAt: s.Now(), Error: err.Error()}
			if err := s.Cache.Save(cf); err != nil {
				return err
			}
			continue
		}
		if len(candidates) == 0 {
			printInfof(s.Selector, "No search results found for this game.\n")
		}

		detailsFetcher := s.Details
		filtered := enrichCandidates(ctx, candidates, 20, detailsFetcher)
		s.saveDiscoveredCandidates(ctx, game, query, "enriched", filtered)
		filtered = filterByMinDuration(filtered, minDurationSec)
		filtered = filterByLegacyRelevance(game, filtered, scorer)
		filtered = rankCandidates(game, filtered, scorer)
		printInfof(s.Selector, "Search summary: found=%d eligible=%d\n", len(candidates), len(filtered))
		if len(candidates) > 0 && len(filtered) == 0 {
			printInfof(s.Selector, "Candidates found, but none met minimum criteria (duration/relevance).\n")
		}

		bestScore, bestDuration := scoreCandidates(game, filtered, scorer)
		if !cfg.NonInteractive {
			if len(filtered) == 0 {
				cf.Items[game.GameID] = &CacheItem{Status: StatusNone, Query: query, UpdatedAt: s.Now(), Videos: nil}
				if err := s.Cache.Save(cf); err != nil {
					return err
				}
				continue
			}
			result, err := s.Selector.Prompt(game, filtered)
			if err != nil {
				return err
			}
			switch result.Action {
			case "skip":
				continue
			case "none":
				cf.Items[game.GameID] = &CacheItem{Status: StatusNone, Query: query, UpdatedAt: s.Now(), Videos: nil}
				if err := s.Cache.Save(cf); err != nil {
					return err
				}
			case "selected":
				videos := make([]Video, 0, len(result.Picks))
				for _, idx := range result.Picks {
					if idx <= 0 || idx > len(filtered) {
						continue
					}
					videos = append(videos, candidateToVideo(filtered[idx-1], s.Now()))
				}
				cf.Items[game.GameID] = &CacheItem{Status: StatusSelected, Query: query, UpdatedAt: s.Now(), Videos: videos}
				if err := s.Cache.Save(cf); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown selection action: %s", result.Action)
			}
			continue
		}
		entries = append(entries, gameEntry{
			game:         game,
			candidates:   filtered,
			query:        query,
			bestScore:    bestScore,
			bestDuration: bestDuration,
		})
	}

	if !cfg.NonInteractive {
		return nil
	}

	sortGameEntries(entries)

	for _, entry := range entries {
		game := entry.game
		filtered := entry.candidates
		query := entry.query
		printCandidates(game, filtered, s.Selector)
		selected := selectByScore(game, filtered, scorer, cfg.AutoMax)
		if len(selected) == 0 {
			if len(filtered) > 0 {
				out := io.Writer(os.Stdout)
				if s.Selector != nil && s.Selector.Out != nil {
					out = s.Selector.Out
				}
				fmt.Fprintf(out, "No candidates reached score threshold (%d); saving status=none.\n", scorer.ScoreThreshold)
			}
			cf.Items[game.GameID] = &CacheItem{Status: StatusNone, Query: query, UpdatedAt: s.Now(), Videos: nil}
			if err := s.Cache.Save(cf); err != nil {
				return err
			}
			continue
		}
		videos := make([]Video, 0, len(selected))
		for _, c := range selected {
			videos = append(videos, candidateToVideo(c, s.Now()))
		}
		policy := DefaultClassificationPolicy()
		if scorer.MinDurationSec > 0 {
			policy.MinDurationSec = scorer.MinDurationSec
		}
		if scorer.ScoreThreshold > 0 {
			policy.AcceptThreshold = scorer.ScoreThreshold
		}
		classification := ClassifyLegacyCandidate(game, selected[0], policy)
		cf.Items[game.GameID] = &CacheItem{
			Status:      StatusCandidate,
			Query:       query,
			UpdatedAt:   s.Now(),
			Videos:      videos,
			NeedsReview: true,
			AutoScoring: &AutoScoringMeta{
				ScoreBreakdown: ScoreBreakdown{Total: selected[0].Score},
				MatchedTeams:   true,
				MatchedDate:    "legacy-discover",
				Source:         "discover-non-interactive",
				Confidence:     confidenceFromScore(selected[0].Score),
				NeedsReview:    true,
				Classification: &classification,
			},
		}
		if err := s.Cache.Save(cf); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateCachedSelection(ctx context.Context, game GameRef, item *CacheItem, scorer Scorer, scoreThreshold int) (rediscover bool, changed bool, reviewAlternatives bool, err error) {
	if item == nil || s.WatchChecker == nil {
		return false, false, false, nil
	}
	if scoreThreshold <= 0 {
		scoreThreshold = 60
	}
	out := io.Writer(os.Stdout)
	if s.Selector != nil && s.Selector.Out != nil {
		out = s.Selector.Out
	}

	inspector, canInspect := s.WatchChecker.(WatchPageInspector)
	updatedVideos := make([]Video, 0, len(item.Videos))
	for _, video := range item.Videos {
		videoID := strings.TrimSpace(video.ID)
		if videoID == "" {
			updatedVideos = append(updatedVideos, video)
			continue
		}

		inspection := WatchInspection{}
		if canInspect {
			inspection, err = inspector.Inspect(ctx, videoID)
		} else {
			inspection.Exists, err = s.WatchChecker.Exists(ctx, videoID)
		}
		if err != nil {
			fmt.Fprintf(out, "\nCould not validate cached YouTube video %s for %s: %v\n", videoID, game.GameID, err)
			fmt.Fprintln(out, "Keeping the cached selection until YouTube explicitly reports it unavailable.")
			updatedVideos = append(updatedVideos, video)
			continue
		}
		if !inspection.Exists {
			fmt.Fprintf(out, "\nCached YouTube video is no longer available for %s vs %s (%d):\n", game.AwayTeam, game.HomeTeam, game.Year)
			fmt.Fprintf(out, "- https://www.youtube.com/watch?v=%s\n", videoID)
			fmt.Fprintln(out, "The game will be searched again.")
			item.Videos = nil
			item.NeedsReview = false
			item.AutoScoring = nil
			item.Error = fmt.Sprintf("cached YouTube video %s no longer exists", videoID)
			return true, true, false, nil
		}

		originalVideo := video
		if canInspect {
			video = applyWatchDetailsToVideo(video, inspection.Details)
			score := scoreCachedVideoFromWatchPage(item, video, inspection.Details, game, scorer, scoreThreshold)
			if score < scoreThreshold {
				fmt.Fprintf(out, "\nCached YouTube video score is low for %s vs %s (%d): %d/%d\n", game.AwayTeam, game.HomeTeam, game.Year, score, scoreThreshold)
				fmt.Fprintln(out, "Reviewer should search alternatives before keeping this selection.")
				changed = true
				reviewAlternatives = true
			}
		}
		if video != originalVideo {
			changed = true
		}
		updatedVideos = append(updatedVideos, video)
	}

	if changed {
		item.Videos = updatedVideos
		item.UpdatedAt = s.Now()
	}
	return false, changed, reviewAlternatives, nil
}

func (s *Service) findCandidates(ctx context.Context, game GameRef, query string, maxResults int, minDurationSec int) ([]Candidate, error) {
	localCandidates, err := s.findCandidatesFromStore(ctx, game, maxResults, minDurationSec)
	if err != nil {
		util.Warnf("Discovery SQLite cache failed for game=%s: %v", game.GameID, err)
	}

	var remoteCandidates []Candidate
	var remoteErr error
	if s.APIDiscoverer != nil && !s.isAPIQuotaExhausted() {
		candidates, err := s.APIDiscoverer(ctx, game, maxResults, minDurationSec)
		if err == nil {
			s.saveDiscoveredCandidates(ctx, game, query, "youtube-api", candidates)
			remoteCandidates = candidates
		} else {
			if youtubeapi.IsQuotaOrRateLimit(err) {
				s.markAPIQuotaExhausted(err)
			}
			util.Warnf("YouTube API failed for game=%s query=%q, falling back to rod: %v", game.GameID, query, redactURLSecrets(err.Error()))
			remoteErr = err
		}
	}
	if len(remoteCandidates) == 0 {
		candidates, err := s.Searcher.Search(ctx, query, maxResults)
		if err == nil {
			s.saveDiscoveredCandidates(ctx, game, query, "rod", candidates)
			remoteCandidates = candidates
			remoteErr = nil
		} else {
			remoteErr = err
		}
	}

	merged := mergeCandidateLists(localCandidates, remoteCandidates)
	if len(merged) > 0 {
		scorer := s.Scorer
		if scorer.MinDurationSec == 0 {
			scorer.MinDurationSec = minDurationSec
		}
		if scorer.ScoreThreshold == 0 {
			scorer.ScoreThreshold = 60
		}
		return rankCandidates(game, merged, scorer), nil
	}
	if remoteErr != nil {
		return nil, remoteErr
	}
	return nil, nil
}

func (s *Service) findCandidatesFromStore(ctx context.Context, game GameRef, maxResults int, minDurationSec int) ([]Candidate, error) {
	if s.CandidateStore == nil {
		return nil, nil
	}
	var candidates []Candidate
	var err error
	if store, ok := s.CandidateStore.(gameCandidateStore); ok {
		candidates, err = store.LoadCandidatesForGame(ctx, game, 0)
		if err == nil && len(candidates) == 0 {
			candidates, err = s.CandidateStore.LoadCandidates(ctx, 5000)
		}
	} else {
		candidates, err = s.CandidateStore.LoadCandidates(ctx, 5000)
	}
	if err != nil {
		return nil, err
	}
	scorer := s.Scorer
	if scorer.MinDurationSec == 0 {
		scorer.MinDurationSec = minDurationSec
	}
	if scorer.ScoreThreshold == 0 {
		scorer.ScoreThreshold = 60
	}
	candidates = filterByMinDuration(candidates, minDurationSec)
	candidates = filterByLegacyRelevance(game, candidates, scorer)
	candidates = rankCandidates(game, candidates, scorer)
	if maxResults > 0 && len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}
	return candidates, nil
}

func (s *Service) saveDiscoveredCandidates(ctx context.Context, game GameRef, query string, source string, candidates []Candidate) {
	if s.CandidateStore == nil || len(candidates) == 0 {
		return
	}
	if err := s.CandidateStore.SaveSearchResults(ctx, game, query, source, candidates); err != nil {
		util.Warnf("Discovery SQLite save failed for game=%s source=%s: %v", game.GameID, source, err)
	}
}

func mergeCandidateLists(lists ...[]Candidate) []Candidate {
	merged := make([]Candidate, 0)
	seen := map[string]int{}
	for _, candidates := range lists {
		for _, candidate := range candidates {
			candidate = normalizeCandidateForMerge(candidate)
			key := candidateMergeKey(candidate)
			if key == "" {
				merged = append(merged, candidate)
				continue
			}
			if idx, ok := seen[key]; ok {
				merged[idx] = mergeCandidateFields(merged[idx], candidate)
				continue
			}
			seen[key] = len(merged)
			merged = append(merged, candidate)
		}
	}
	return merged
}

func normalizeCandidateForMerge(candidate Candidate) Candidate {
	if candidate.VideoID == "" {
		candidate.VideoID = normalizeCandidateID(candidate)
	}
	if candidate.URL == "" && candidate.VideoID != "" {
		candidate.URL = "https://www.youtube.com/watch?v=" + candidate.VideoID
	}
	return candidate
}

func candidateMergeKey(candidate Candidate) string {
	for _, key := range videoKeys(candidate.VideoID, candidate.URL) {
		if key != "" {
			return key
		}
	}
	title := NormalizeText(candidate.Title)
	channel := NormalizeText(candidate.Channel)
	if title == "" {
		return ""
	}
	return "title:" + title + "|channel:" + channel
}

func mergeCandidateFields(existing Candidate, incoming Candidate) Candidate {
	if existing.VideoID == "" {
		existing.VideoID = incoming.VideoID
	}
	if existing.URL == "" {
		existing.URL = incoming.URL
	}
	if existing.Title == "" {
		existing.Title = incoming.Title
	}
	if existing.Channel == "" {
		existing.Channel = incoming.Channel
	}
	if existing.DurationText == "" {
		existing.DurationText = incoming.DurationText
	}
	if existing.DurationSec == 0 {
		existing.DurationSec = incoming.DurationSec
	}
	if existing.Snippet == "" {
		existing.Snippet = incoming.Snippet
	}
	if existing.Embeddable == nil {
		existing.Embeddable = incoming.Embeddable
	}
	if incoming.Score > existing.Score {
		existing.Score = incoming.Score
	}
	return existing
}

func (s *Service) isAPIQuotaExhausted() bool {
	s.apiMu.Lock()
	defer s.apiMu.Unlock()
	return s.apiQuotaExhausted
}

func (s *Service) markAPIQuotaExhausted(err error) {
	s.apiMu.Lock()
	defer s.apiMu.Unlock()
	s.apiQuotaExhausted = true
	s.apiQuotaExhaustion = redactURLSecrets(err.Error())
}

func redactURLSecrets(s string) string {
	for _, key := range []string{"key", "api_key", "access_token"} {
		s = redactQueryValue(s, key)
	}
	return s
}

func redactQueryValue(s string, key string) string {
	needle := key + "="
	idx := strings.Index(s, needle)
	for idx >= 0 {
		start := idx + len(needle)
		end := start
		for end < len(s) {
			switch s[end] {
			case '&', ' ', '"', '\'':
				goto replace
			default:
				end++
			}
		}
	replace:
		s = s[:start] + "REDACTED" + s[end:]
		nextFrom := start + len("REDACTED")
		idx = strings.Index(s[nextFrom:], needle)
		if idx >= 0 {
			idx += nextFrom
		}
	}
	return s
}

func promptModifyExisting(selector *Selector, game GameRef, item *CacheItem) (bool, error) {
	if selector == nil || selector.In == nil || selector.Out == nil {
		return false, fmt.Errorf("selector requires input and output")
	}
	fmt.Fprintf(selector.Out, "\nThere is already a cached video selection for %s vs %s (%d):\n", game.AwayTeam, game.HomeTeam, game.Year)
	printCachedVideos(selector.Out, item)
	fmt.Fprint(selector.Out, "Modify existing selection? (y/N): ")
	return readYesNo(selector.In)
}

func promptSearchAlternatives(selector *Selector, game GameRef, item *CacheItem) (bool, error) {
	if selector == nil || selector.In == nil || selector.Out == nil {
		return false, fmt.Errorf("selector requires input and output")
	}
	fmt.Fprintf(selector.Out, "\nCurrent cached video selection for %s vs %s (%d):\n", game.AwayTeam, game.HomeTeam, game.Year)
	printCachedVideos(selector.Out, item)
	fmt.Fprint(selector.Out, "Search for alternatives? (y/N): ")
	return readYesNo(selector.In)
}

func printCachedVideos(out io.Writer, item *CacheItem) {
	if out == nil || item == nil {
		return
	}
	for _, v := range item.Videos {
		url := v.URL
		if url == "" {
			url = "(no url)"
		}
		fmt.Fprintf(out, "- %s\n", url)
	}
}

func readYesNo(in io.Reader) (bool, error) {
	line, err := readLine(in)
	if err != nil && err != io.EOF {
		return false, err
	}
	resp := strings.TrimSpace(strings.ToLower(line))
	if resp == "y" || resp == "yes" {
		return true, nil
	}
	return false, nil
}

func readLine(r io.Reader) (string, error) {
	if r == nil {
		return "", io.EOF
	}
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return strings.TrimRight(b.String(), "\r"), nil
			}
			b.WriteByte(buf[0])
		}
		if err != nil {
			if err == io.EOF {
				return strings.TrimRight(b.String(), "\r"), io.EOF
			}
			return "", err
		}
	}
}

func announceSearch(game GameRef, selector *Selector) {
	msg := fmt.Sprintf("\nSearching videos for %s vs %s (%s)...\n", game.AwayTeam, game.HomeTeam, gameDateISO(game))
	if selector != nil && selector.Out != nil {
		fmt.Fprint(selector.Out, msg)
		return
	}
	fmt.Print(msg)
}

func buildQuery(game GameRef) string {
	parts := []string{game.AwayTeam, "vs", game.HomeTeam}
	dateISO := gameDateISO(game)
	if isPlayoffWeek(game.Week) && game.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", game.Year), "playoffs")
		if dateISO != "unknown" {
			parts = append(parts, dateISO)
		}
		return strings.Join(parts, " ")
	}
	if dateISO != "unknown" {
		parts = append(parts, dateISO)
	} else if game.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", game.Year))
	}
	return strings.Join(parts, " ")
}

func candidateToVideo(c Candidate, now time.Time) Video {
	durationSec := candidateDurationSec(c)
	role := ClassifyContentRole(c.Title, c.Snippet, c.Channel, durationSec)
	part := DetectPartInfo(c.Title, c.Snippet)
	return Video{
		ID:              c.VideoID,
		URL:             c.URL,
		Title:           c.Title,
		Channel:         c.Channel,
		Role:            string(role),
		Part:            part.Part,
		PartTotal:       part.Total,
		DurationSeconds: durationSec,
		Embeddable:      c.Embeddable,
		SelectedAt:      now,
	}
}

func selectByScore(game GameRef, candidates []Candidate, scorer Scorer, max int) []Candidate {
	scoredList := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		c.DurationSec = candidateDurationSec(c)
		eval := scorer.Evaluate(game, c)
		c.Score = eval.Score
		if eval.Decision != DecisionAccept {
			continue
		}
		if scorer.ScoreThreshold > 0 && eval.Score < scorer.ScoreThreshold {
			continue
		}
		scoredList = append(scoredList, c)
	}
	sortCandidatesByScore(scoredList)
	if max <= 0 || max > len(scoredList) {
		max = len(scoredList)
	}
	selected := make([]Candidate, 0, max)
	for i := 0; i < max; i++ {
		selected = append(selected, scoredList[i])
	}
	return selected
}

func scoreCandidates(game GameRef, candidates []Candidate, scorer Scorer) (int, int) {
	bestScore := 0
	bestDuration := 0
	for _, c := range candidates {
		durationSec := candidateDurationSec(c)
		score := scorer.Score(game, c)
		if score > bestScore {
			bestScore = score
			bestDuration = durationSec
			continue
		}
		if score == bestScore && durationSec > bestDuration {
			bestDuration = durationSec
		}
	}
	return bestScore, bestDuration
}

func filterByLegacyRelevance(game GameRef, candidates []Candidate, scorer Scorer) []Candidate {
	if len(candidates) == 0 {
		return candidates
	}
	filtered := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if !isLegacyRelevantCandidate(game, c, scorer) {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

func isLegacyRelevantCandidate(game GameRef, c Candidate, scorer Scorer) bool {
	c.DurationSec = candidateDurationSec(c)
	return scorer.Evaluate(game, c).Decision != DecisionReject
}

func hasLegacyTemporalEvidence(game GameRef, dateMatch DateMatchType, weekMatch DateMatchType) bool {
	if dateMatch == DateMatchFull || dateMatch == DateMatchYearOnly {
		return true
	}
	if weekMatch == DateMatchFull {
		return true
	}
	return game.Year == 0 && strings.TrimSpace(game.DateRaw) == "" && strings.TrimSpace(game.GameID) == ""
}

func rankCandidates(game GameRef, candidates []Candidate, scorer Scorer) []Candidate {
	if len(candidates) == 0 {
		return candidates
	}
	ranked := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		c.DurationSec = candidateDurationSec(c)
		c.Score = scorer.Score(game, c)
		ranked = append(ranked, c)
	}
	sortCandidatesByScore(ranked)
	return ranked
}

func sortCandidatesByScore(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].DurationSec == candidates[j].DurationSec {
				return strings.ToLower(candidates[i].Title) < strings.ToLower(candidates[j].Title)
			}
			return candidates[i].DurationSec > candidates[j].DurationSec
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func printCandidates(game GameRef, candidates []Candidate, selector *Selector) {
	out := io.Writer(os.Stdout)
	if selector != nil && selector.Out != nil {
		out = selector.Out
	}
	PrintCandidates(out, game, candidates)
}

func printInfof(selector *Selector, format string, args ...any) {
	out := io.Writer(os.Stdout)
	if selector != nil && selector.Out != nil {
		out = selector.Out
	}
	fmt.Fprintf(out, format, args...)
}

func sortGameEntries(entries []gameEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].bestScore == entries[j].bestScore {
			if entries[i].bestDuration == entries[j].bestDuration {
				return entries[i].game.GameID < entries[j].game.GameID
			}
			return entries[i].bestDuration > entries[j].bestDuration
		}
		return entries[i].bestScore > entries[j].bestScore
	})
}
