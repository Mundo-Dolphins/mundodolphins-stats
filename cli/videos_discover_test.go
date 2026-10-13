package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type stubVideosSearcher struct {
	results []videos.Candidate
}

func (s stubVideosSearcher) Search(ctx context.Context, query string, maxResults int) ([]videos.Candidate, error) {
	if maxResults > 0 && len(s.results) > maxResults {
		return s.results[:maxResults], nil
	}
	return s.results, nil
}

type stubVideosCache struct {
	cf    *videos.CacheFile
	saved bool
}

func (s *stubVideosCache) Load() (*videos.CacheFile, error) {
	if s.cf == nil {
		s.cf = &videos.CacheFile{Items: map[string]*videos.CacheItem{}}
	}
	return s.cf, nil
}

func (s *stubVideosCache) Save(cf *videos.CacheFile) error {
	s.cf = cf
	s.saved = true
	return nil
}

func (s *stubVideosCache) Upsert(gameID string, item *videos.CacheItem) error { return nil }
func (s *stubVideosCache) Get(gameID string) (*videos.CacheItem, bool, error) { return nil, false, nil }

type stubVideosLoader struct {
	games []videos.GameRef
}

func (s stubVideosLoader) LoadGames(yearFrom, yearTo int) ([]videos.GameRef, error) {
	return s.games, nil
}

type stubCmdWatchChecker struct {
	exists map[string]bool
	closed bool
}

func (s *stubCmdWatchChecker) Exists(_ context.Context, videoID string) (bool, error) {
	return s.exists[videoID], nil
}

func (s *stubCmdWatchChecker) Close() error {
	s.closed = true
	return nil
}

type doerFunc func(req *http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRunVideosDiscoverRequiresYear(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
	}()

	videosYear = 0
	videosFromYear = 0
	videosToYear = 0
	if err := runVideosDiscover(videosDiscoverCmd, nil); err == nil {
		t.Fatalf("expected error for missing year flags")
	}
}

func TestRunVideosDiscoverInvalidRange(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
	}()

	videosYear = 0
	videosFromYear = 2025
	videosToYear = 1966
	if err := runVideosDiscover(videosDiscoverCmd, nil); err == nil {
		t.Fatalf("expected error for invalid year range")
	}
}

func TestRunVideosDiscoverNonInteractive(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	prevNonInteractive := videosNonInteractive
	prevSearcherFactory := videosSearcherFactory
	prevCacheFactory := videosCacheFactory
	prevLoaderFactory := videosLoaderFactory
	prevYouTubeAPIKey := videosYouTubeAPIKey
	prevValidateExisting := videosValidateExisting
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
		videosNonInteractive = prevNonInteractive
		videosSearcherFactory = prevSearcherFactory
		videosCacheFactory = prevCacheFactory
		videosLoaderFactory = prevLoaderFactory
		videosYouTubeAPIKey = prevYouTubeAPIKey
		videosValidateExisting = prevValidateExisting
	}()

	cache := &stubVideosCache{}
	videosSearcherFactory = func(headless bool) videos.Searcher {
		return stubVideosSearcher{results: []videos.Candidate{}}
	}
	videosCacheFactory = func(path string) videos.Cache { return cache }
	videosLoaderFactory = func(dir string) videos.GameLoader {
		return stubVideosLoader{games: []videos.GameRef{
			{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"},
		}}
	}

	videosYear = 1966
	videosFromYear = 0
	videosToYear = 0
	videosNonInteractive = true
	videosYouTubeAPIKey = ""
	videosValidateExisting = false

	if err := runVideosDiscover(videosDiscoverCmd, nil); err != nil {
		t.Fatalf("runVideosDiscover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != "none" {
		t.Fatalf("expected none status cached")
	}
	if !cache.saved {
		t.Fatalf("expected cache saved")
	}
}

func TestRunVideosDiscoverDoesNotBulkValidateCachedVideos(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	prevNonInteractive := videosNonInteractive
	prevValidateExisting := videosValidateExisting
	prevSearcherFactory := videosSearcherFactory
	prevCacheFactory := videosCacheFactory
	prevLoaderFactory := videosLoaderFactory
	prevWatchCheckerFactory := videosWatchCheckerFactory
	prevYouTubeAPIKey := videosYouTubeAPIKey
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
		videosNonInteractive = prevNonInteractive
		videosValidateExisting = prevValidateExisting
		videosSearcherFactory = prevSearcherFactory
		videosCacheFactory = prevCacheFactory
		videosLoaderFactory = prevLoaderFactory
		videosWatchCheckerFactory = prevWatchCheckerFactory
		videosYouTubeAPIKey = prevYouTubeAPIKey
	}()

	cache := &stubVideosCache{cf: &videos.CacheFile{Items: map[string]*videos.CacheItem{
		"196612180mia": {
			Status: videos.StatusSelected,
			Videos: []videos.Video{{ID: "gone1", URL: "https://www.youtube.com/watch?v=gone1"}},
		},
	}}}
	checker := &stubCmdWatchChecker{exists: map[string]bool{"gone1": false}}
	videosSearcherFactory = func(headless bool) videos.Searcher {
		return stubVideosSearcher{results: []videos.Candidate{}}
	}
	videosCacheFactory = func(path string) videos.Cache { return cache }
	videosLoaderFactory = func(dir string) videos.GameLoader {
		return stubVideosLoader{games: []videos.GameRef{
			{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"},
		}}
	}
	videosWatchCheckerFactory = func(headless bool) videos.WatchAvailabilityChecker { return checker }

	videosYear = 1966
	videosFromYear = 0
	videosToYear = 0
	videosNonInteractive = true
	videosValidateExisting = true
	videosYouTubeAPIKey = ""

	if err := runVideosDiscover(videosDiscoverCmd, nil); err != nil {
		t.Fatalf("runVideosDiscover error: %v", err)
	}
	if !checker.closed {
		t.Fatal("expected watch checker to be closed")
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != videos.StatusSelected || len(item.Videos) != 1 {
		t.Fatalf("expected cached video to be left unchanged in non-interactive skip, got %+v", item)
	}
	if cache.saved {
		t.Fatal("expected cache not to be saved by bulk validation")
	}
}

func TestResolveYouTubeAPIKey(t *testing.T) {
	prevEnv := os.Getenv("YOUTUBE_API_KEY")
	t.Cleanup(func() {
		_ = os.Setenv("YOUTUBE_API_KEY", prevEnv)
	})

	if err := os.Setenv("YOUTUBE_API_KEY", "env-key"); err != nil {
		t.Fatalf("set env: %v", err)
	}

	if got := resolveYouTubeAPIKey("flag-key"); got != "flag-key" {
		t.Fatalf("expected flag key, got %q", got)
	}
	if got := resolveYouTubeAPIKey(""); got != "env-key" {
		t.Fatalf("expected env key, got %q", got)
	}
}

func TestRunVideosDiscoverAPIFallbackToSearcher(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	prevNonInteractive := videosNonInteractive
	prevSearcherFactory := videosSearcherFactory
	prevCacheFactory := videosCacheFactory
	prevLoaderFactory := videosLoaderFactory
	prevYouTubeAPIKey := videosYouTubeAPIKey
	prevValidateExisting := videosValidateExisting
	prevYouTubeFactory := youtubeAPIFactory
	prevEnv := os.Getenv("YOUTUBE_API_KEY")
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
		videosNonInteractive = prevNonInteractive
		videosSearcherFactory = prevSearcherFactory
		videosCacheFactory = prevCacheFactory
		videosLoaderFactory = prevLoaderFactory
		videosYouTubeAPIKey = prevYouTubeAPIKey
		videosValidateExisting = prevValidateExisting
		youtubeAPIFactory = prevYouTubeFactory
		_ = os.Setenv("YOUTUBE_API_KEY", prevEnv)
	}()

	cache := &stubVideosCache{}
	videosSearcherFactory = func(headless bool) videos.Searcher {
		return stubVideosSearcher{results: []videos.Candidate{
			{VideoID: "web123", URL: "https://www.youtube.com/watch?v=web123", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", Snippet: "Searcher fallback", DurationSec: 4000, DurationText: "1:06:40"},
		}}
	}
	videosCacheFactory = func(path string) videos.Cache { return cache }
	videosLoaderFactory = func(dir string) videos.GameLoader {
		return stubVideosLoader{games: []videos.GameRef{
			{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"},
		}}
	}
	youtubeAPIFactory = func(apiKey string) *youtubeapi.Client {
		client := youtubeapi.NewClient(apiKey, doerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)),
				Header:     make(http.Header),
			}, nil
		}))
		client.BaseURL = "https://example.test"
		return client
	}

	if err := os.Setenv("YOUTUBE_API_KEY", "env-key"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	videosYear = 1966
	videosFromYear = 0
	videosToYear = 0
	videosNonInteractive = true
	videosYouTubeAPIKey = ""
	videosValidateExisting = false
	videosMinDuration = 5 * time.Minute

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	if err := runVideosDiscover(videosDiscoverCmd, nil); err != nil {
		_ = w.Close()
		os.Stdout = oldStdout
		t.Fatalf("runVideosDiscover error: %v", err)
	}
	_ = w.Close()
	os.Stdout = oldStdout
	outBytes, _ := io.ReadAll(r)
	outText := string(outBytes)
	if !strings.Contains(outText, "URL: https://www.youtube.com/watch?v=web123") {
		t.Fatalf("expected URL in output, got %q", outText)
	}
	if !strings.Contains(outText, "Desc: Searcher fallback") {
		t.Fatalf("expected Desc in output, got %q", outText)
	}
	if !strings.Contains(outText, "Score:") {
		t.Fatalf("expected Score in output, got %q", outText)
	}
	if !strings.Contains(outText, "Channel: unknown") {
		t.Fatalf("expected Channel in output, got %q", outText)
	}

	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != videos.StatusCandidate || len(item.Videos) != 1 || item.Videos[0].ID != "web123" {
		t.Fatalf("expected searcher fallback selection, got %+v", item)
	}
}

func TestRunVideosWebAllowsAllYearsByDefault(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	prevSearcherFactory := videosSearcherFactory
	prevCacheFactory := videosCacheFactory
	prevLoaderFactory := videosLoaderFactory
	prevServe := videosWebServe
	prevAPIKey := videosYouTubeAPIKey
	prevAutoApproveScore := videosWebAutoApproveScore
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
		videosSearcherFactory = prevSearcherFactory
		videosCacheFactory = prevCacheFactory
		videosLoaderFactory = prevLoaderFactory
		videosWebServe = prevServe
		videosYouTubeAPIKey = prevAPIKey
		videosWebAutoApproveScore = prevAutoApproveScore
	}()

	var gotAddr string
	videosWebServe = func(server *http.Server, web *videos.WebServer) error {
		gotAddr = server.Addr
		return nil
	}
	videosSearcherFactory = func(headless bool) videos.Searcher { return stubVideosSearcher{} }
	videosCacheFactory = func(path string) videos.Cache { return &stubVideosCache{} }
	videosLoaderFactory = func(dir string) videos.GameLoader { return stubVideosLoader{} }
	videosYear = 0
	videosFromYear = 0
	videosToYear = 0
	videosYouTubeAPIKey = ""
	videosWebAutoApproveScore = 130
	videosWebAddr = "127.0.0.1:0"

	if err := runVideosWeb(videosWebCmd, nil); err != nil {
		t.Fatalf("runVideosWeb error: %v", err)
	}
	if gotAddr != "127.0.0.1:0" {
		t.Fatalf("unexpected server addr: %q", gotAddr)
	}
}

func TestRunVideosWebInvalidRange(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
	}()
	videosYear = 0
	videosFromYear = 2026
	videosToYear = 1966
	if err := runVideosWeb(videosWebCmd, nil); err == nil {
		t.Fatalf("expected invalid range error")
	}
}

func TestRunVideosWebConfiguresAPIDiscoverer(t *testing.T) {
	prevYear := videosYear
	prevFrom := videosFromYear
	prevTo := videosToYear
	prevMinDuration := videosMinDuration
	prevMaxResults := videosMaxResults
	prevScoreThreshold := videosScoreThreshold
	prevSearcherFactory := videosSearcherFactory
	prevCacheFactory := videosCacheFactory
	prevLoaderFactory := videosLoaderFactory
	prevServe := videosWebServe
	prevAPIKey := videosYouTubeAPIKey
	prevYouTubeFactory := youtubeAPIFactory
	prevAutoApproveScore := videosWebAutoApproveScore
	defer func() {
		videosYear = prevYear
		videosFromYear = prevFrom
		videosToYear = prevTo
		videosMinDuration = prevMinDuration
		videosMaxResults = prevMaxResults
		videosScoreThreshold = prevScoreThreshold
		videosSearcherFactory = prevSearcherFactory
		videosCacheFactory = prevCacheFactory
		videosLoaderFactory = prevLoaderFactory
		videosWebServe = prevServe
		videosYouTubeAPIKey = prevAPIKey
		youtubeAPIFactory = prevYouTubeFactory
		videosWebAutoApproveScore = prevAutoApproveScore
	}()

	youtubeAPIFactory = func(apiKey string) *youtubeapi.Client {
		client := youtubeapi.NewClient(apiKey, doerFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"items":[{"id":{"videoId":"abc123"}}]}`
			if req.URL.Path == "/videos" {
				body = `{"items":[{"id":"abc123","snippet":{"title":"Oakland Raiders vs Miami Dolphins 1966-09-02","description":"Broadcast game film","channelTitle":"Vintage Pro Football"},"contentDetails":{"duration":"PT44M28S"},"status":{"embeddable":true}}]}`
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}))
		client.BaseURL = "https://example.test"
		return client
	}

	var captured *videos.WebServer
	videosWebServe = func(server *http.Server, web *videos.WebServer) error {
		captured = web
		return nil
	}
	videosSearcherFactory = func(headless bool) videos.Searcher { return stubVideosSearcher{} }
	videosCacheFactory = func(path string) videos.Cache { return &stubVideosCache{} }
	videosLoaderFactory = func(dir string) videos.GameLoader { return stubVideosLoader{} }
	videosYear = 1966
	videosFromYear = 0
	videosToYear = 0
	videosMinDuration = 5 * time.Minute
	videosMaxResults = 7
	videosScoreThreshold = 60
	videosYouTubeAPIKey = "test-key"
	videosWebAutoApproveScore = 145
	videosWebAddr = "127.0.0.1:0"

	if err := runVideosWeb(videosWebCmd, nil); err != nil {
		t.Fatalf("runVideosWeb error: %v", err)
	}
	if captured == nil || captured.Service.APIDiscoverer == nil {
		t.Fatalf("expected captured web server with API discoverer")
	}
	if captured.Config.Year != 1966 || captured.Config.MaxResults != 7 || captured.Config.MinDurationSec != 300 || captured.Config.AutoApproveScore != 145 {
		t.Fatalf("unexpected config: %+v", captured.Config)
	}
	items, err := captured.Service.APIDiscoverer(context.Background(), videos.GameRef{
		GameID:   "196609020mia",
		Year:     1966,
		AwayTeam: "Oakland Raiders",
		HomeTeam: "Miami Dolphins",
	}, 5, 300)
	if err != nil {
		t.Fatalf("APIDiscoverer error: %v", err)
	}
	if len(items) != 1 || items[0].VideoID != "abc123" || items[0].DurationSec != 2668 || items[0].Snippet != "Broadcast game film" {
		t.Fatalf("unexpected candidates: %+v", items)
	}
}
