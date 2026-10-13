package videos

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type WebConfig struct {
	Year             int
	FromYear         int
	ToYear           int
	ProcessedDir     string
	CacheFile        string
	MaxResults       int
	MinDurationSec   int
	ScoreThreshold   int
	AutoApproveScore int
}

type WebServer struct {
	Service *Service
	Config  WebConfig

	mu                         sync.Mutex
	autopilotValidatedExisting map[string]bool
}

type webGame struct {
	Game        GameRef    `json:"game"`
	Status      string     `json:"status"`
	NeedsReview bool       `json:"needs_review"`
	Score       int        `json:"score"`
	Confidence  string     `json:"confidence"`
	Error       string     `json:"error,omitempty"`
	Videos      []Video    `json:"videos,omitempty"`
	Query       string     `json:"query"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type webCandidateRequest struct {
	Candidate Candidate `json:"candidate"`
}

type webVideoEmbeddableRequest struct {
	VideoID    string `json:"video_id"`
	URL        string `json:"url"`
	Embeddable bool   `json:"embeddable"`
}

func (s *WebServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/autopilot/step", s.handleAutopilotStep)
	mux.HandleFunc("/api/games", s.handleGames)
	mux.HandleFunc("/api/games/", s.handleGameAction)
	return localhostOnly(mux)
}

// localhostOnly protects the local review UI against requests coming from
// other web pages. The UI changes files on disk through POST endpoints, so a
// page opened in the same browser must not be able to drive it:
//
//   - the Host header must name a loopback host (this stops DNS rebinding);
//   - when the browser sends an Origin header it must match the Host.
func localhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webIndexTemplate.Execute(w, map[string]any{
		"CacheFile":      s.Config.CacheFile,
		"ProcessedDir":   s.Config.ProcessedDir,
		"ScoreThreshold": s.scorer().ScoreThreshold,
	})
}

func (s *WebServer) handleGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	games, cf, err := s.loadState()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	items := make([]webGame, 0, len(games))
	for _, game := range games {
		items = append(items, s.webGameFromCache(game, cf))
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": items})
}

func (s *WebServer) handleGameAction(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/games/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	gameID, action := parts[0], parts[1]

	switch action {
	case "search":
		s.handleSearch(w, r, gameID)
	case "select":
		s.handleSelect(w, r, gameID)
	case "embed":
		s.handleSetVideoEmbeddable(w, r, gameID)
	case "remove":
		s.handleRemoveVideo(w, r, gameID)
	case "approve":
		s.handleStatus(w, r, gameID, StatusApproved)
	case "reject":
		s.handleStatus(w, r, gameID, StatusRejected)
	case "none":
		s.handleStatus(w, r, gameID, StatusNone)
	default:
		http.NotFound(w, r)
	}
}

func (s *WebServer) handleSearch(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	game, err := s.findGame(gameID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
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
	candidates, err := s.Service.findCandidates(r.Context(), game, query, maxResults, minDurationSec)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	detailsFetcher := s.Service.Details
	filtered := enrichCandidates(r.Context(), candidates, 20, detailsFetcher)
	s.Service.saveDiscoveredCandidates(r.Context(), game, query, "web-enriched", filtered)
	filtered = filterByMinDuration(filtered, minDurationSec)
	filtered = filterByLegacyRelevance(game, filtered, scorer)
	filtered = s.filterExistingCandidates(gameID, filtered)
	filtered = rankCandidates(game, filtered, scorer)
	status := ""
	if len(filtered) == 0 {
		status, err = s.markNoEligibleSearchIfEmptySelection(game, query)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game":       game,
		"query":      query,
		"found":      len(candidates),
		"eligible":   len(filtered),
		"status":     status,
		"candidates": filtered,
	})
}

func (s *WebServer) handleSelect(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req webCandidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode selection: %w", err))
		return
	}
	if strings.TrimSpace(req.Candidate.VideoID) == "" && strings.TrimSpace(req.Candidate.URL) == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("candidate requires video_id or url"))
		return
	}
	game, err := s.findGame(gameID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	now := s.now()
	video := candidateToVideo(req.Candidate, now)
	s.mu.Lock()
	cf, err := s.Service.Cache.Load()
	if err == nil {
		item := cf.Items[gameID]
		if item == nil {
			item = &CacheItem{Query: buildQuery(game)}
			cf.Items[gameID] = item
		}
		item.Status = StatusSelected
		item.Query = buildQuery(game)
		item.UpdatedAt = now
		item.NeedsReview = false
		item.Error = ""
		item.AutoScoring = nil
		item.Videos = appendOrReplaceVideo(item.Videos, video)
		err = s.Service.Cache.Save(cf)
	}
	s.mu.Unlock()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": cf.Items[gameID]})
}

func (s *WebServer) handleSetVideoEmbeddable(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req webVideoEmbeddableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode video embeddable: %w", err))
		return
	}
	if strings.TrimSpace(req.VideoID) == "" && strings.TrimSpace(req.URL) == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("video requires video_id or url"))
		return
	}
	s.mu.Lock()
	cf, err := s.Service.Cache.Load()
	if err == nil {
		item := cf.Items[gameID]
		if item == nil {
			err = fmt.Errorf("game %s has no cached videos", gameID)
		} else if !setVideoEmbeddable(item.Videos, req.VideoID, req.URL, req.Embeddable) {
			err = fmt.Errorf("video not found for game %s", gameID)
		} else {
			item.UpdatedAt = s.now()
			err = s.Service.Cache.Save(cf)
		}
	}
	s.mu.Unlock()
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": cf.Items[gameID]})
}

func (s *WebServer) handleRemoveVideo(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req webVideoEmbeddableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode video removal: %w", err))
		return
	}
	if strings.TrimSpace(req.VideoID) == "" && strings.TrimSpace(req.URL) == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("video requires video_id or url"))
		return
	}
	s.mu.Lock()
	cf, err := s.Service.Cache.Load()
	if err == nil {
		item := cf.Items[gameID]
		if item == nil {
			err = fmt.Errorf("game %s has no cached videos", gameID)
		} else if videos, ok := removeVideo(item.Videos, req.VideoID, req.URL); !ok {
			err = fmt.Errorf("video not found for game %s", gameID)
		} else {
			item.Videos = videos
			item.UpdatedAt = s.now()
			if len(item.Videos) == 0 {
				item.Status = StatusUnknown
				item.NeedsReview = false
				item.AutoScoring = nil
			}
			err = s.Service.Cache.Save(cf)
		}
	}
	s.mu.Unlock()
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": cf.Items[gameID]})
}

func (s *WebServer) handleStatus(w http.ResponseWriter, r *http.Request, gameID string, status string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	game, err := s.findGame(gameID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	s.mu.Lock()
	cf, err := s.Service.Cache.Load()
	if err == nil {
		item := cf.Items[gameID]
		if item == nil {
			item = &CacheItem{Query: buildQuery(game)}
			cf.Items[gameID] = item
		}
		item.Status = status
		item.UpdatedAt = s.now()
		item.NeedsReview = false
		item.Error = ""
		if status == StatusNone || status == StatusRejected {
			item.Videos = nil
			item.AutoScoring = nil
		}
		if item.AutoScoring != nil {
			item.AutoScoring.NeedsReview = false
		}
		err = s.Service.Cache.Save(cf)
	}
	s.mu.Unlock()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (s *WebServer) loadState() ([]GameRef, *CacheFile, error) {
	if s.Service == nil {
		return nil, nil, fmt.Errorf("service is nil")
	}
	if s.Service.GameLoader == nil {
		return nil, nil, fmt.Errorf("game loader is nil")
	}
	if s.Service.Cache == nil {
		return nil, nil, fmt.Errorf("cache is nil")
	}
	fromYear, toYear := s.Config.FromYear, s.Config.ToYear
	if s.Config.Year > 0 {
		fromYear, toYear = s.Config.Year, s.Config.Year
	}
	games, err := s.Service.GameLoader.LoadGames(fromYear, toYear)
	if err != nil {
		return nil, nil, err
	}
	cf, err := s.Service.Cache.Load()
	if err != nil {
		return nil, nil, err
	}
	return games, cf, nil
}

func (s *WebServer) findGame(gameID string) (GameRef, error) {
	games, _, err := s.loadState()
	if err != nil {
		return GameRef{}, err
	}
	for _, game := range games {
		if game.GameID == gameID {
			return game, nil
		}
	}
	return GameRef{}, fmt.Errorf("game %s not found", gameID)
}

func (s *WebServer) webGameFromCache(game GameRef, cf *CacheFile) webGame {
	out := webGame{Game: game, Status: StatusUnknown, Query: buildQuery(game)}
	if cf == nil || cf.Items == nil {
		return out
	}
	item := cf.Items[game.GameID]
	if item == nil {
		return out
	}
	out.Status = item.Status
	out.NeedsReview = item.NeedsReview
	out.Error = item.Error
	out.Videos = item.Videos
	out.Query = item.Query
	if !item.UpdatedAt.IsZero() {
		updatedAt := item.UpdatedAt
		out.UpdatedAt = &updatedAt
	}
	if item.AutoScoring != nil {
		out.Score = item.AutoScoring.ScoreBreakdown.Total
		out.Confidence = item.AutoScoring.Confidence
	}
	return out
}

func (s *WebServer) scorer() Scorer {
	scorer := s.Service.Scorer
	if scorer.MinDurationSec == 0 {
		scorer.MinDurationSec = s.Config.MinDurationSec
	}
	if scorer.ScoreThreshold == 0 {
		scorer.ScoreThreshold = s.Config.ScoreThreshold
	}
	return scorer
}

func (s *WebServer) now() time.Time {
	if s.Service.Now != nil {
		return s.Service.Now()
	}
	return time.Now()
}

func (s *WebServer) filterExistingCandidates(gameID string, candidates []Candidate) []Candidate {
	if s.Service.Cache == nil || len(candidates) == 0 {
		return candidates
	}
	cf, err := s.Service.Cache.Load()
	if err != nil {
		return candidates
	}
	item := cf.Items[gameID]
	if item == nil || len(item.Videos) == 0 {
		return candidates
	}
	existing := map[string]bool{}
	for _, video := range item.Videos {
		for _, key := range videoKeys(video.ID, video.URL) {
			existing[key] = true
		}
	}
	filtered := candidates[:0]
	for _, candidate := range candidates {
		duplicate := false
		for _, key := range videoKeys(candidate.VideoID, candidate.URL) {
			if existing[key] {
				duplicate = true
				break
			}
		}
		if !duplicate {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func (s *WebServer) markNoEligibleSearchIfEmptySelection(game GameRef, query string) (string, error) {
	if s.Service.Cache == nil {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cf, err := s.Service.Cache.Load()
	if err != nil {
		return "", err
	}
	item := cf.Items[game.GameID]
	if item != nil && len(item.Videos) > 0 {
		return item.Status, nil
	}
	if item == nil {
		item = &CacheItem{}
		cf.Items[game.GameID] = item
	}
	item.Status = StatusNone
	item.Query = query
	item.UpdatedAt = s.now()
	item.Videos = nil
	item.NeedsReview = false
	item.Error = ""
	item.AutoScoring = nil
	if err := s.Service.Cache.Save(cf); err != nil {
		return "", err
	}
	return item.Status, nil
}

func appendOrReplaceVideo(videos []Video, video Video) []Video {
	keyID := strings.TrimSpace(video.ID)
	keyURL := strings.TrimSpace(video.URL)
	for i, existing := range videos {
		if keyID != "" && strings.TrimSpace(existing.ID) == keyID {
			videos[i] = video
			return videos
		}
		if keyURL != "" && strings.TrimSpace(existing.URL) == keyURL {
			videos[i] = video
			return videos
		}
	}
	return append(videos, video)
}

func videoKeys(videoID string, rawURL string) []string {
	keys := make([]string, 0, 2)
	if id := strings.TrimSpace(videoID); id != "" {
		keys = append(keys, "id:"+id)
	}
	if id := extractYouTubeVideoID(rawURL); id != "" {
		key := "id:" + id
		if len(keys) == 0 || keys[0] != key {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		if rawURL = strings.TrimSpace(rawURL); rawURL != "" {
			keys = append(keys, "url:"+rawURL)
		}
	}
	return keys
}

func extractYouTubeVideoID(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "/watch") {
		rawURL = "https://www.youtube.com" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if id := strings.TrimSpace(u.Query().Get("v")); id != "" {
		return id
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	switch {
	case host == "youtu.be":
		return strings.TrimSpace(parts[0])
	case len(parts) >= 2 && (parts[0] == "embed" || parts[0] == "shorts" || parts[0] == "live" || parts[0] == "v"):
		return strings.TrimSpace(parts[1])
	default:
		return ""
	}
}

func setVideoEmbeddable(videos []Video, videoID string, videoURL string, embeddable bool) bool {
	videoID = strings.TrimSpace(videoID)
	videoURL = strings.TrimSpace(videoURL)
	for i := range videos {
		if videoID != "" && strings.TrimSpace(videos[i].ID) == videoID {
			videos[i].Embeddable = &embeddable
			return true
		}
		if videoURL != "" && strings.TrimSpace(videos[i].URL) == videoURL {
			videos[i].Embeddable = &embeddable
			return true
		}
	}
	return false
}

func removeVideo(videos []Video, videoID string, videoURL string) ([]Video, bool) {
	videoID = strings.TrimSpace(videoID)
	videoURL = strings.TrimSpace(videoURL)
	out := videos[:0]
	removed := false
	for _, video := range videos {
		match := false
		if videoID != "" && strings.TrimSpace(video.ID) == videoID {
			match = true
		}
		if videoURL != "" && strings.TrimSpace(video.URL) == videoURL {
			match = true
		}
		if match {
			removed = true
			continue
		}
		out = append(out, video)
	}
	return out, removed
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST")
	writeJSONError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
