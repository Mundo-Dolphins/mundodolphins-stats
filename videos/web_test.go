package videos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWebServerGamesAndSearch(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196811170mia", "Cincinnati Bengals", "Miami Dolphins", "Sunday Nov 17, 1968", "11")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	server := newTestWebServer(dir, cache, stubSearcher{results: []Candidate{
		{VideoID: "v1", URL: "https://www.youtube.com/watch?v=v1", Title: "Cincinnati Bengals 1968 Season Week 11 vs Dolphins", DurationSec: 400},
		{VideoID: "bad", URL: "https://www.youtube.com/watch?v=bad", Title: "Cincinnati Bengals 1999 vs Dolphins", DurationSec: 400},
	}})

	req := newLocalRequest(http.MethodGet, "/api/games", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/games status=%d body=%s", rr.Code, rr.Body.String())
	}

	var gamesResp struct {
		Games []webGame `json:"games"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &gamesResp); err != nil {
		t.Fatalf("decode games: %v", err)
	}
	if len(gamesResp.Games) != 1 || gamesResp.Games[0].Game.Week != "11" {
		t.Fatalf("unexpected games response: %+v", gamesResp)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196811170mia/search", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST search status=%d body=%s", rr.Code, rr.Body.String())
	}
	var searchResp struct {
		Candidates []Candidate `json:"candidates"`
		Eligible   int         `json:"eligible"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if searchResp.Eligible != 1 || len(searchResp.Candidates) != 1 || searchResp.Candidates[0].VideoID != "v1" {
		t.Fatalf("unexpected search response: %+v", searchResp)
	}
}

func TestWebServerSearchFiltersAlreadySelectedVideos(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196811170mia", "Cincinnati Bengals", "Miami Dolphins", "Sunday Nov 17, 1968", "11")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	cf := newCacheFile(time.Now())
	cf.Items["196811170mia"] = &CacheItem{
		Status: StatusSelected,
		Videos: []Video{{URL: "https://youtu.be/v1"}},
	}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	server := newTestWebServer(dir, cache, stubSearcher{results: []Candidate{
		{VideoID: "v1", URL: "https://www.youtube.com/watch?v=v1", Title: "Cincinnati Bengals vs Miami Dolphins 1968-11-17 Full Game", DurationSec: 400},
		{VideoID: "v2", URL: "https://www.youtube.com/watch?v=v2", Title: "Cincinnati Bengals vs Miami Dolphins 1968-11-17 Full Game Radio", DurationSec: 500},
	}})

	req := newLocalRequest(http.MethodPost, "/api/games/196811170mia/search", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST search status=%d body=%s", rr.Code, rr.Body.String())
	}
	var searchResp struct {
		Candidates []Candidate `json:"candidates"`
		Eligible   int         `json:"eligible"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if searchResp.Eligible != 1 || len(searchResp.Candidates) != 1 || searchResp.Candidates[0].VideoID != "v2" {
		t.Fatalf("expected duplicate candidate filtered, got %+v", searchResp)
	}
}

func TestWebServerIndexHasValidStatusButtonQuoting(t *testing.T) {
	server := newTestWebServer(t.TempDir(), NewFileCache(filepath.Join(t.TempDir(), "videos.json")), stubSearcher{})
	req := newLocalRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, `statusAction(\\'`) {
		t.Fatalf("index contains broken escaped quote sequence")
	}
	if !strings.Contains(body, `statusAction(&quot;approve&quot;)`) {
		t.Fatalf("index missing approve action quoting")
	}
	for _, want := range []string{`reviewedThisSession`, `localStorage`, `reviewProgressKey`, `videosDiscovery.reviewProgress:`, `resetReviewProgress`, `Reset progress`, `Searching YouTube`, `Search in progress`, `searchInFlight`, `isSearchBusy`, `button:disabled`, `.game.disabled`, `if (isSearchBusy()) return;`, `hasNoCurrentVideos`, `if (autoSearchAttempted.has(selected.game.GameID)) return;`, `status === 'none'`, `link only`, `videos.every(v => embeddableValue(v) === false)`, `checking embed`, `iframe_api`, `onYouTubeIframeAPIReady`, `Add video`, `video-link`, `visibleCandidateCount`, `Only candidate selected and approved`, `/approve`, `keepCurrent()`, `maybeAutoSearch()`, `autoSearchAttempted.delete`, `autoSearchReason`, `currentVideoKeys`, `durationText`, `duration_seconds`, `actionDisabled`, `setCurrentVideoEmbeddable`, `removeCurrentVideo`, `Remove`, `nextAutopilotGame`, `prepareAutopilotStepDisplay`, `Searching next`, `gameLabel`, `autopilotPausedForReview`, `Resume Autopilot`, `shouldResumeAutopilotAfterReview`, `Resuming autopilot`, `removed`, `yt-embed`, `youtube.com/embed/`, `allowfullscreen`, `data.message || autopilotSummary('Done')`} {
		if !strings.Contains(body, want) {
			t.Fatalf("index missing %q", want)
		}
	}
	if strings.Contains(body, `if (!hasNoCurrentVideos && autoSearchAttempted.has(selected.game.GameID)) return;`) {
		t.Fatalf("index still guards auto-search attempts only when current videos exist")
	}
}

func TestWebServerSearchNoEligibleMarksNoneForEmptySelection(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "200008120pit", "Miami Dolphins", "Pittsburgh Steelers", "Saturday Aug 12, 2000", "Pre1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	server := newTestWebServer(dir, cache, stubSearcher{})

	req := newLocalRequest(http.MethodPost, "/api/games/200008120pit/search", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST search status=%d body=%s", rr.Code, rr.Body.String())
	}
	var searchResp struct {
		Found    int    `json:"found"`
		Eligible int    `json:"eligible"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if searchResp.Found != 0 || searchResp.Eligible != 0 || searchResp.Status != StatusNone {
		t.Fatalf("unexpected empty search response: %+v", searchResp)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["200008120pit"]
	if item == nil || item.Status != StatusNone || len(item.Videos) != 0 || item.NeedsReview || item.Error != "" || item.Query == "" {
		t.Fatalf("expected empty search to mark none, got %+v", item)
	}
}

func TestWebServerSearchNoEligiblePreservesExistingVideos(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "200008120pit", "Miami Dolphins", "Pittsburgh Steelers", "Saturday Aug 12, 2000", "Pre1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	cf := newCacheFile(time.Now())
	cf.Items["200008120pit"] = &CacheItem{
		Status: StatusSelected,
		Query:  "old",
		Videos: []Video{{ID: "keep", URL: "https://www.youtube.com/watch?v=keep"}},
	}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	server := newTestWebServer(dir, cache, stubSearcher{})

	req := newLocalRequest(http.MethodPost, "/api/games/200008120pit/search", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST search status=%d body=%s", rr.Code, rr.Body.String())
	}
	var searchResp struct {
		Eligible int    `json:"eligible"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if searchResp.Eligible != 0 || searchResp.Status != StatusSelected {
		t.Fatalf("unexpected search response: %+v", searchResp)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["200008120pit"]
	if item == nil || item.Status != StatusSelected || len(item.Videos) != 1 || item.Videos[0].ID != "keep" || item.Query != "old" {
		t.Fatalf("expected existing video preserved, got %+v", item)
	}
}

func TestWebServerAutopilotApprovesHighConfidenceCandidate(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	embeddable := true
	server := newTestWebServer(dir, cache, stubSearcher{results: []Candidate{{
		VideoID:     "auto1",
		URL:         "https://www.youtube.com/watch?v=auto1",
		Title:       "Oakland Raiders at Miami Dolphins 1966-09-02 Week 1 Full Game Broadcast",
		Channel:     "Vintage Pro Football",
		DurationSec: 7200,
		Snippet:     "Full game broadcast.",
		Embeddable:  &embeddable,
	}}})

	req := newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp webAutopilotStepResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	if resp.Action != "approved" || resp.AutoApproved != 1 || resp.Processed != 1 {
		t.Fatalf("unexpected autopilot response: %+v", resp)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil || item.Status != StatusApproved || item.NeedsReview || len(item.Videos) != 1 || item.Videos[0].ID != "auto1" {
		t.Fatalf("expected approved cache item, got %+v", item)
	}
	if item.AutoScoring == nil || item.AutoScoring.ContentRole != string(ContentRoleFullGame) || item.AutoScoring.ScoreBreakdown.Total < 130 {
		t.Fatalf("expected strict auto-scoring metadata, got %+v", item.AutoScoring)
	}
}

func TestWebServerAutopilotStopsOnReviewCandidate(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	embeddable := true
	server := newTestWebServer(dir, cache, stubSearcher{results: []Candidate{{
		VideoID:     "review1",
		URL:         "https://www.youtube.com/watch?v=review1",
		Title:       "Oakland Raiders vs Miami Dolphins 1966 Week 1 Top 10 Highlights",
		Channel:     "Vintage Pro Football",
		DurationSec: 700,
		Snippet:     "Highlights.",
		Embeddable:  &embeddable,
	}}})

	req := newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp webAutopilotStepResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	if resp.Action != "needs_review" || resp.StopReason != "candidate_review" || len(resp.Candidates) != 1 {
		t.Fatalf("unexpected review response: %+v", resp)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil || item.Status != StatusCandidate || !item.NeedsReview || len(item.Videos) != 1 || item.Videos[0].ID != "review1" {
		t.Fatalf("expected review cache item, got %+v", item)
	}
}

func TestWebServerAutopilotMarksNoneForNoCandidates(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "200008120pit", "Miami Dolphins", "Pittsburgh Steelers", "Saturday Aug 12, 2000", "Pre1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	server := newTestWebServer(dir, cache, stubSearcher{})

	req := newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp webAutopilotStepResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	if resp.Action != "none" || resp.MarkedNone != 1 || resp.Processed != 1 {
		t.Fatalf("unexpected none response: %+v", resp)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["200008120pit"]
	if item == nil || item.Status != StatusNone || len(item.Videos) != 0 {
		t.Fatalf("expected none cache item, got %+v", item)
	}
}

func TestWebServerAutopilotStopsAtFutureGame(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "202609030mia", "Buffalo Bills", "Miami Dolphins", "Thursday Sep 3, 2026", "1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	searcher := &countingSearcher{}
	server := newTestWebServer(dir, cache, searcher)
	server.Service.Now = func() time.Time {
		return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	}

	req := newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp webAutopilotStepResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	if resp.Action != "done" || resp.StopReason != "future_game" || resp.Game == nil {
		t.Fatalf("expected autopilot to stop at future game, got %+v", resp)
	}
	if searcher.count != 0 {
		t.Fatalf("expected no search for future game, got %d calls", searcher.count)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if item := cf.Items["202609030mia"]; item != nil {
		t.Fatalf("expected future game cache to remain untouched, got %+v", item)
	}
}

func TestWebServerAutopilotValidatesExistingAndSearchesWhenGone(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "200208150mia", "New Orleans Saints", "Miami Dolphins", "Thursday Aug 15, 2002", "Preseason 2")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	cf := newCacheFile(time.Now())
	cf.Items["200208150mia"] = &CacheItem{
		Status: StatusSelected,
		Videos: []Video{{
			ID:  "gone1",
			URL: "https://www.youtube.com/watch?v=gone1",
		}},
	}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	embeddable := true
	checker := &stubWatchChecker{exists: map[string]bool{"gone1": false}}
	server := newTestWebServer(dir, cache, stubSearcher{results: []Candidate{{
		VideoID:     "new1",
		URL:         "https://www.youtube.com/watch?v=new1",
		Title:       "New Orleans Saints at Miami Dolphins 2002-08-15 Preseason Week 2 Full Game Broadcast",
		Channel:     "Vintage Pro Football",
		DurationSec: 7200,
		Snippet:     "Full game broadcast.",
		Embeddable:  &embeddable,
	}}})
	server.Service.WatchChecker = checker

	req := newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp webAutopilotStepResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	if resp.Action != "approved" || resp.Removed != 1 || resp.AutoApproved != 1 || resp.Processed != 1 {
		t.Fatalf("expected removal followed by auto-approval, got %+v", resp)
	}
	if len(checker.calls) != 1 || checker.calls[0] != "gone1" {
		t.Fatalf("expected one validation call for gone1, got %+v", checker.calls)
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["200208150mia"]
	if item == nil || item.Status != StatusApproved || len(item.Videos) != 1 || item.Videos[0].ID != "new1" || item.Error != "" {
		t.Fatalf("expected gone video replaced by approved candidate, got %+v", item)
	}
}

func TestSortAutopilotGamesByDate(t *testing.T) {
	games := []GameRef{
		{GameID: "202609130mia"},
		{GameID: "202608300mia"},
		{GameID: "unknown", Year: 2026},
	}

	sortAutopilotGamesByDate(games)

	if got, want := games[0].GameID, "202608300mia"; got != want {
		t.Fatalf("first game=%s want %s", got, want)
	}
	if got, want := games[1].GameID, "202609130mia"; got != want {
		t.Fatalf("second game=%s want %s", got, want)
	}
}

func TestWebServerAutopilotStopsOnExistingReviewAndDone(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	writeWebGameJSON(t, dir, "196609090mia", "New York Jets", "Miami Dolphins", "Friday Sep 9, 1966", "2")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	cf := newCacheFile(time.Now())
	cf.Items["196609020mia"] = &CacheItem{
		Status:      StatusCandidate,
		NeedsReview: true,
		Videos:      []Video{{ID: "review", URL: "https://www.youtube.com/watch?v=review"}},
	}
	cf.Items["196609090mia"] = &CacheItem{
		Status: StatusApproved,
		Videos: []Video{{ID: "approved", URL: "https://www.youtube.com/watch?v=approved"}},
	}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	server := newTestWebServer(dir, cache, stubSearcher{})

	req := newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp webAutopilotStepResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot: %v", err)
	}
	if resp.Action != "needs_review" || resp.StopReason != "existing_candidate" {
		t.Fatalf("unexpected existing review response: %+v", resp)
	}

	cf.Items["196609020mia"].Status = StatusApproved
	cf.Items["196609020mia"].NeedsReview = false
	if err := cache.Save(cf); err != nil {
		t.Fatalf("save approved cache: %v", err)
	}
	req = newLocalRequest(http.MethodPost, "/api/autopilot/step", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST autopilot done status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode autopilot done: %v", err)
	}
	if resp.Action != "done" || resp.StopReason != "done" {
		t.Fatalf("unexpected done response: %+v", resp)
	}

	req = newLocalRequest(http.MethodGet, "/api/autopilot/step", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rr.Code)
	}
}

func TestWebAutopilotNeedsSearchVariants(t *testing.T) {
	tests := []struct {
		name string
		item *CacheItem
		want bool
	}{
		{name: "missing", want: true},
		{name: "approved with video", item: &CacheItem{Status: StatusApproved, Videos: []Video{{ID: "v"}}}, want: false},
		{name: "approved empty", item: &CacheItem{Status: StatusApproved}, want: true},
		{name: "none", item: &CacheItem{Status: StatusNone}, want: false},
		{name: "rejected", item: &CacheItem{Status: StatusRejected}, want: false},
		{name: "pending empty", item: &CacheItem{Status: StatusPending}, want: true},
		{name: "candidate with video complete", item: &CacheItem{Status: StatusCandidate, Videos: []Video{{ID: "v"}}}, want: false},
		{name: "unknown with video", item: &CacheItem{Status: StatusUnknown, Videos: []Video{{ID: "v"}}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := webAutopilotNeedsSearch(tt.item); got != tt.want {
				t.Fatalf("webAutopilotNeedsSearch()=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestWebAutopilotThresholdAndBoolCount(t *testing.T) {
	defaultServer := &WebServer{}
	if got := defaultServer.autoApproveThreshold(); got != 130 {
		t.Fatalf("default autoApproveThreshold()=%d, want 130", got)
	}

	configuredServer := &WebServer{Config: WebConfig{AutoApproveScore: 95}}
	if got := configuredServer.autoApproveThreshold(); got != 95 {
		t.Fatalf("configured autoApproveThreshold()=%d, want 95", got)
	}

	if boolToCount(true) != 1 || boolToCount(false) != 0 {
		t.Fatal("boolToCount returned unexpected values")
	}
}

func TestWebServerSelectAndApprove(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	server := newTestWebServer(dir, cache, stubSearcher{})

	body := `{"candidate":{"VideoID":"v1","URL":"https://www.youtube.com/watch?v=v1","Title":"1966-09-02 RAID at MIA","Channel":"Vintage Pro Football","DurationSec":2668,"Embeddable":false}}`
	req := newLocalRequest(http.MethodPost, "/api/games/196609020mia/select", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST select status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil || item.Status != StatusSelected || len(item.Videos) != 1 || item.Videos[0].ID != "v1" {
		t.Fatalf("unexpected selected item: %+v", item)
	}
	if item.Videos[0].Embeddable == nil || *item.Videos[0].Embeddable {
		t.Fatalf("expected selected video to preserve non-embeddable metadata: %+v", item.Videos[0])
	}

	body = `{"candidate":{"VideoID":"v2","URL":"https://www.youtube.com/watch?v=v2","Title":"Second angle","Channel":"Vintage Pro Football","DurationSec":1800,"Embeddable":true}}`
	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/select", strings.NewReader(body))
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST second select status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err = cache.Load()
	if err != nil {
		t.Fatalf("reload cache after second select: %v", err)
	}
	item = cf.Items["196609020mia"]
	if item == nil || len(item.Videos) != 2 || item.Videos[0].ID != "v1" || item.Videos[1].ID != "v2" {
		t.Fatalf("expected two selected videos, got %+v", item)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/embed", strings.NewReader(`{"video_id":"v1","embeddable":true}`))
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST embed status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err = cache.Load()
	if err != nil {
		t.Fatalf("reload cache after embed update: %v", err)
	}
	item = cf.Items["196609020mia"]
	if item.Videos[0].Embeddable == nil || !*item.Videos[0].Embeddable {
		t.Fatalf("expected v1 embeddable metadata updated: %+v", item.Videos[0])
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/remove", strings.NewReader(`{"video_id":"v1"}`))
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST remove status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err = cache.Load()
	if err != nil {
		t.Fatalf("reload cache after remove: %v", err)
	}
	item = cf.Items["196609020mia"]
	if item == nil || len(item.Videos) != 1 || item.Videos[0].ID != "v2" {
		t.Fatalf("expected only v2 after removing v1, got %+v", item)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/approve", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST approve status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err = cache.Load()
	if err != nil {
		t.Fatalf("reload cache: %v", err)
	}
	item = cf.Items["196609020mia"]
	if item == nil || item.Status != StatusApproved || item.NeedsReview {
		t.Fatalf("unexpected approved item: %+v", item)
	}
}

func TestWebServerEmbedAndRemoveByURL(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	cf := newCacheFile(time.Now())
	cf.Items["196609020mia"] = &CacheItem{
		Status: StatusSelected,
		Videos: []Video{
			{ID: "v1", URL: "https://www.youtube.com/watch?v=v1"},
			{ID: "v2", URL: "https://youtu.be/v2"},
		},
	}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	server := newTestWebServer(dir, cache, stubSearcher{})

	req := newLocalRequest(http.MethodPost, "/api/games/196609020mia/embed", strings.NewReader(`{"url":"https://youtu.be/v2","embeddable":false}`))
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST embed by URL status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache after embed: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil || len(item.Videos) != 2 || item.Videos[1].Embeddable == nil || *item.Videos[1].Embeddable {
		t.Fatalf("expected v2 marked non-embeddable by URL, got %+v", item)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/remove", strings.NewReader(`{"url":"https://youtu.be/v2"}`))
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST remove by URL status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err = cache.Load()
	if err != nil {
		t.Fatalf("load cache after remove: %v", err)
	}
	item = cf.Items["196609020mia"]
	if item == nil || len(item.Videos) != 1 || item.Videos[0].ID != "v1" || item.Status != StatusSelected {
		t.Fatalf("expected only v1 after URL removal, got %+v", item)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/remove", strings.NewReader(`{"url":"https://www.youtube.com/watch?v=v1"}`))
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST remove last video status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err = cache.Load()
	if err != nil {
		t.Fatalf("load cache after removing last video: %v", err)
	}
	item = cf.Items["196609020mia"]
	if item == nil || len(item.Videos) != 0 || item.Status != StatusUnknown || item.NeedsReview || item.AutoScoring != nil {
		t.Fatalf("expected empty selection reset to unknown, got %+v", item)
	}
}

func TestWebServerStatusActionsAndErrors(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	cache := NewFileCache(filepath.Join(dir, "videos.json"))
	server := newTestWebServer(dir, cache, stubSearcher{})

	cf := newCacheFile(time.Now())
	cf.Items["196609020mia"] = &CacheItem{
		Status:      StatusCandidate,
		NeedsReview: true,
		Error:       "old",
		Videos:      []Video{{ID: "v1", URL: "https://www.youtube.com/watch?v=v1"}},
		AutoScoring: &AutoScoringMeta{NeedsReview: true, ScoreBreakdown: ScoreBreakdown{Total: 40}},
	}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	req := newLocalRequest(http.MethodPost, "/api/games/196609020mia/none", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST none status=%d body=%s", rr.Code, rr.Body.String())
	}
	cf, err := cache.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil || item.Status != StatusNone || len(item.Videos) != 0 || item.AutoScoring != nil || item.Error != "" || item.NeedsReview {
		t.Fatalf("unexpected none item: %+v", item)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/missing/none", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected missing game 404, got %d", rr.Code)
	}

	req = newLocalRequest(http.MethodGet, "/api/games/196609020mia/search", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rr.Code)
	}

	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/select", strings.NewReader(`{"candidate":{}}`))
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid select 400, got %d", rr.Code)
	}

	req = newLocalRequest(http.MethodGet, "/missing", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected index 404, got %d", rr.Code)
	}
}

func TestWebServerLoadStateDependencyErrors(t *testing.T) {
	server := &WebServer{}
	_, _, err := server.loadState()
	if err == nil || !strings.Contains(err.Error(), "service is nil") {
		t.Fatalf("expected service error, got %v", err)
	}
	server.Service = &Service{}
	_, _, err = server.loadState()
	if err == nil || !strings.Contains(err.Error(), "game loader is nil") {
		t.Fatalf("expected game loader error, got %v", err)
	}
	server.Service.GameLoader = stubLoader{}
	_, _, err = server.loadState()
	if err == nil || !strings.Contains(err.Error(), "cache is nil") {
		t.Fatalf("expected cache error, got %v", err)
	}
}

func TestWebServerHandlesCacheAndSearchErrors(t *testing.T) {
	dir := t.TempDir()
	writeWebGameJSON(t, dir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")

	server := newTestWebServer(dir, errWebCache{}, stubSearcher{})
	req := newLocalRequest(http.MethodGet, "/api/games", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected cache error 500, got %d", rr.Code)
	}

	server = newTestWebServer(dir, NewFileCache(filepath.Join(dir, "videos.json")), errSearcher{})
	req = newLocalRequest(http.MethodPost, "/api/games/196609020mia/search", nil)
	rr = httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected search error 502, got %d", rr.Code)
	}
}

func TestWebGameFromCacheVariants(t *testing.T) {
	server := newTestWebServer(t.TempDir(), NewFileCache(filepath.Join(t.TempDir(), "videos.json")), stubSearcher{})
	game := GameRef{GameID: "g1", AwayTeam: "A", HomeTeam: "B", Year: 1966}
	if got := server.webGameFromCache(game, nil); got.Status != StatusUnknown {
		t.Fatalf("expected unknown with nil cache, got %+v", got)
	}
	cf := newCacheFile(time.Now())
	if got := server.webGameFromCache(game, cf); got.Status != StatusUnknown {
		t.Fatalf("expected unknown with missing item, got %+v", got)
	}
}

func TestWebVideoHelpers(t *testing.T) {
	replacement := Video{ID: "v2", URL: "https://www.youtube.com/watch?v=new", Title: "replacement"}
	videos := appendOrReplaceVideo([]Video{
		{ID: "v1", URL: "https://www.youtube.com/watch?v=old1"},
		{ID: "v2", URL: "https://www.youtube.com/watch?v=old2"},
	}, replacement)
	if len(videos) != 2 || videos[1].URL != replacement.URL || videos[1].Title != replacement.Title {
		t.Fatalf("expected replacement by ID, got %+v", videos)
	}

	videos = appendOrReplaceVideo(videos, Video{URL: "https://youtu.be/v1", Title: "new item"})
	if len(videos) != 3 || videos[2].Title != "new item" {
		t.Fatalf("expected URL-only video appended, got %+v", videos)
	}

	if id := extractYouTubeVideoID("/watch?v=abc123"); id != "abc123" {
		t.Fatalf("expected relative watch URL ID, got %q", id)
	}
	if id := extractYouTubeVideoID("https://www.youtube.com/embed/embed123"); id != "embed123" {
		t.Fatalf("expected embed URL ID, got %q", id)
	}
	if id := extractYouTubeVideoID("https://www.youtube.com/shorts/short123"); id != "short123" {
		t.Fatalf("expected shorts URL ID, got %q", id)
	}
	if id := extractYouTubeVideoID("://bad-url"); id != "" {
		t.Fatalf("expected bad URL to return empty ID, got %q", id)
	}

	keys := videoKeys("", "https://www.youtube.com/live/live123")
	if len(keys) != 1 || keys[0] != "id:live123" {
		t.Fatalf("expected live URL key, got %+v", keys)
	}
	keys = videoKeys("", "https://example.com/video")
	if len(keys) != 1 || keys[0] != "url:https://example.com/video" {
		t.Fatalf("expected raw URL fallback key, got %+v", keys)
	}

	embeddable := false
	videos = []Video{{ID: "v1"}, {URL: "https://youtu.be/v2"}}
	if !setVideoEmbeddable(videos, "", "https://youtu.be/v2", embeddable) {
		t.Fatalf("expected set embeddable by URL")
	}
	if videos[1].Embeddable == nil || *videos[1].Embeddable {
		t.Fatalf("expected second video marked non-embeddable, got %+v", videos[1])
	}
	if setVideoEmbeddable(videos, "missing", "", true) {
		t.Fatalf("expected missing embeddable update to fail")
	}

	remaining, removed := removeVideo(videos, "v1", "")
	if !removed || len(remaining) != 1 || remaining[0].URL != "https://youtu.be/v2" {
		t.Fatalf("expected remove by ID, removed=%v remaining=%+v", removed, remaining)
	}
	remaining, removed = removeVideo(remaining, "", "missing")
	if removed || len(remaining) != 1 {
		t.Fatalf("expected missing remove to preserve slice, removed=%v remaining=%+v", removed, remaining)
	}
}

type errWebCache struct{}

func (errWebCache) Load() (*CacheFile, error) { return nil, fmt.Errorf("cache boom") }
func (errWebCache) Save(*CacheFile) error     { return fmt.Errorf("cache boom") }
func (errWebCache) Upsert(string, *CacheItem) error {
	return fmt.Errorf("cache boom")
}
func (errWebCache) Get(string) (*CacheItem, bool, error) {
	return nil, false, fmt.Errorf("cache boom")
}

func newTestWebServer(dir string, cache Cache, searcher Searcher) *WebServer {
	return &WebServer{
		Service: &Service{
			Searcher:   searcher,
			Cache:      cache,
			GameLoader: NewJSONGameLoader(dir),
			Scorer:     NewScorer(300, 60),
			Details:    keepCandidateDetails,
			Now:        func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		},
		Config: WebConfig{
			ProcessedDir:   dir,
			CacheFile:      filepath.Join(dir, "videos.json"),
			MaxResults:     10,
			MinDurationSec: 300,
			ScoreThreshold: 60,
		},
	}
}

func writeWebGameJSON(t *testing.T, dir string, gameID string, away string, home string, date string, week string) {
	t.Helper()
	data := `{"game_id":"` + gameID + `","data":{"week":"` + week + `","scorebox":{"teams":[{"name":{"content":"` + away + `"}},{"name":{"content":"` + home + `"}}],"meta":{"date":"` + date + `"}}}}`
	if err := os.WriteFile(filepath.Join(dir, gameID+".json"), []byte(data), 0644); err != nil {
		t.Fatalf("write game json: %v", err)
	}
}
