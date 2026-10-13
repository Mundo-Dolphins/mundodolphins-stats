package videos

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type stubSearcher struct {
	results []Candidate
}

func (s stubSearcher) Search(ctx context.Context, query string, maxResults int) ([]Candidate, error) {
	if maxResults > 0 && len(s.results) > maxResults {
		return s.results[:maxResults], nil
	}
	return s.results, nil
}

type errSearcher struct{}

func (s errSearcher) Search(ctx context.Context, query string, maxResults int) ([]Candidate, error) {
	return nil, io.ErrUnexpectedEOF
}

type countingSearcher struct {
	count   int
	results []Candidate
}

func (s *countingSearcher) Search(ctx context.Context, query string, maxResults int) ([]Candidate, error) {
	s.count++
	if maxResults > 0 && len(s.results) > maxResults {
		return s.results[:maxResults], nil
	}
	return s.results, nil
}

type stubCache struct {
	cf *CacheFile
}

func (s *stubCache) Load() (*CacheFile, error) {
	if s.cf == nil {
		s.cf = newCacheFile(time.Now())
	}
	return s.cf, nil
}

func (s *stubCache) Save(cf *CacheFile) error {
	s.cf = cf
	return nil
}

func (s *stubCache) Upsert(gameID string, item *CacheItem) error { return nil }
func (s *stubCache) Get(gameID string) (*CacheItem, bool, error) { return nil, false, nil }

type stubLoader struct {
	games []GameRef
}

func (s stubLoader) LoadGames(yearFrom, yearTo int) ([]GameRef, error) {
	return s.games, nil
}

func keepCandidateDetails(context.Context, string) (VideoDetails, error) {
	return VideoDetails{}, nil
}

func TestServiceDiscoverNonInteractive(t *testing.T) {
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      &stubCache{},
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		Now:        func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	}
	cfg := DiscoverConfig{NonInteractive: true, MaxResults: 5, AutoMax: 1, ScoreThreshold: 60}
	if err := service.Discover(context.Background(), cfg); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	cf, _ := service.Cache.Load()
	item := cf.Items["196612180mia"]
	if item == nil || item.Status != StatusCandidate {
		t.Fatalf("expected candidate item")
	}
	if len(item.Videos) != 1 {
		t.Fatalf("expected 1 video")
	}
}

func TestServiceDiscoverRespectsMinDurationFlag(t *testing.T) {
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 400}}},
		Cache:      &stubCache{},
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(0, 0),
		Details:    keepCandidateDetails,
	}
	cfg := DiscoverConfig{NonInteractive: true, MaxResults: 5, AutoMax: 1, MinDurationSec: 600}
	if err := service.Discover(context.Background(), cfg); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	cf, _ := service.Cache.Load()
	item := cf.Items["196612180mia"]
	if item == nil || item.Status != "none" {
		t.Fatalf("expected none due to min duration filter")
	}
}

func TestRedactURLSecrets(t *testing.T) {
	msg := `Get "https://www.googleapis.com/youtube/v3/search?fields=x&key=secret123&part=snippet": dial tcp`
	got := redactURLSecrets(msg)
	if strings.Contains(got, "secret123") {
		t.Fatalf("secret was not redacted: %s", got)
	}
	if !strings.Contains(got, "key=REDACTED") {
		t.Fatalf("redacted marker missing: %s", got)
	}
}

func TestServiceDiscoverSkipsSelected(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {Status: "selected", Query: "old"},
	}}}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: true}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Query != "old" {
		t.Fatalf("expected cached item unchanged")
	}
}

func TestServiceDiscoverPromptModifyExistingNo(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {Status: "selected", Query: "old", Videos: []Video{{URL: "https://example.com"}}},
	}}}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("n\n"), Out: io.Discard},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Query != "old" {
		t.Fatalf("expected cached item unchanged")
	}
}

func TestServiceDiscoverPromptModifyExistingYes(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {Status: "selected", Query: "old", Videos: []Video{{URL: "https://example.com"}}},
	}}}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("y\n1\n"), Out: io.Discard},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		Now:        func() time.Time { return time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC) },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || len(item.Videos) != 1 || item.Videos[0].URL != "u" {
		t.Fatalf("expected cached item updated")
	}
}

func TestServiceDiscoverValidatesCachedSelectionBeforePromptAndRediscover(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {
			Status: StatusSelected,
			Query:  "old",
			Videos: []Video{{ID: "gone1", URL: "https://www.youtube.com/watch?v=gone1"}},
		},
	}}}
	var out strings.Builder
	searcher := &countingSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}}
	service := Service{
		Searcher:     searcher,
		Cache:        cache,
		GameLoader:   stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Selector:     &Selector{In: strings.NewReader("1\n"), Out: &out},
		Scorer:       NewScorer(3000, 60),
		Details:      keepCandidateDetails,
		WatchChecker: &stubWatchChecker{exists: map[string]bool{"gone1": false}},
		Now:          func() time.Time { return time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC) },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false, ValidateExisting: true}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if searcher.count != 1 {
		t.Fatalf("expected rediscovery search, got %d searches", searcher.count)
	}
	if strings.Contains(out.String(), "Modify existing selection?") {
		t.Fatalf("did not expect modify prompt for unavailable cached video, output:\n%s", out.String())
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != StatusSelected || len(item.Videos) != 1 || item.Videos[0].ID != "abc" {
		t.Fatalf("expected rediscovered selected video, got %+v", item)
	}
}

func TestServiceDiscoverWarnsLowCachedSelectionScoreBeforePrompt(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"202409120mia": {
			Status: StatusSelected,
			Query:  "old",
			Videos: []Video{{ID: "preview1", URL: "https://www.youtube.com/watch?v=preview1", Title: "Old title"}},
		},
	}}}
	var out strings.Builder
	searcher := &countingSearcher{results: []Candidate{{VideoID: "better", URL: "better", Title: "Buffalo Bills vs Miami Dolphins 2024 Full Game", DurationSec: 4000}}}
	service := Service{
		Searcher:   searcher,
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "202409120mia", Year: 2024, AwayTeam: "Buffalo Bills", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("n\n"), Out: &out},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		WatchChecker: &stubWatchChecker{inspections: map[string]WatchInspection{
			"preview1": {
				Exists: true,
				Details: VideoDetails{
					Title:       "Week preview and betting odds",
					Description: "Miami Dolphins vs Buffalo Bills predictions",
					Duration:    7 * time.Minute,
				},
			},
		}},
		Now: func() time.Time { return time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC) },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false, ValidateExisting: true, ScoreThreshold: 60}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if searcher.count != 0 {
		t.Fatalf("expected no search when reviewer keeps cached selection, got %d", searcher.count)
	}
	output := out.String()
	if !strings.Contains(output, "Cached YouTube video score is low") || !strings.Contains(output, "Search for alternatives?") {
		t.Fatalf("expected low-score warning before alternatives prompt, output:\n%s", output)
	}
	if strings.Contains(output, "Modify existing selection?") {
		t.Fatalf("did not expect normal modify prompt for low-score selection, output:\n%s", output)
	}
	item := cache.cf.Items["202409120mia"]
	if !item.NeedsReview || item.AutoScoring == nil || item.AutoScoring.ScoreBreakdown.Total >= 60 {
		t.Fatalf("expected low-score review metadata, got %+v", item)
	}
	if item.Videos[0].Title != "Week preview and betting odds" {
		t.Fatalf("expected metadata from watch page, got %+v", item.Videos[0])
	}
}

func TestServiceDiscoverSearchesAlternativesWhenLowCachedSelectionAccepted(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"202409120mia": {
			Status: StatusSelected,
			Query:  "old",
			Videos: []Video{{ID: "preview1", URL: "https://www.youtube.com/watch?v=preview1", Title: "Old title"}},
		},
	}}}
	var out strings.Builder
	searcher := &countingSearcher{results: []Candidate{{VideoID: "better", URL: "better", Title: "Buffalo Bills vs Miami Dolphins 2024 Full Game", DurationSec: 4000}}}
	service := Service{
		Searcher:   searcher,
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "202409120mia", Year: 2024, AwayTeam: "Buffalo Bills", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("y\n1\n"), Out: &out},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		WatchChecker: &stubWatchChecker{inspections: map[string]WatchInspection{
			"preview1": {
				Exists: true,
				Details: VideoDetails{
					Title:       "Week preview and betting odds",
					Description: "Miami Dolphins vs Buffalo Bills predictions",
					Duration:    7 * time.Minute,
				},
			},
		}},
		Now: func() time.Time { return time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC) },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false, ValidateExisting: true, ScoreThreshold: 60}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if searcher.count != 1 {
		t.Fatalf("expected alternatives search, got %d", searcher.count)
	}
	output := out.String()
	if !strings.Contains(output, "Search for alternatives?") || !strings.Contains(output, "Select videos") {
		t.Fatalf("expected alternatives prompt and candidate selection, output:\n%s", output)
	}
	item := cache.cf.Items["202409120mia"]
	if item == nil || item.Status != StatusSelected || len(item.Videos) != 1 || item.Videos[0].ID != "better" {
		t.Fatalf("expected selected alternative, got %+v", item)
	}
}

func TestServiceDiscoverRetryNone(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {Status: "none", Query: "old"},
	}}}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
	}
	cfg := DiscoverConfig{NonInteractive: true, RetryNone: true, ScoreThreshold: 60}
	if err := service.Discover(context.Background(), cfg); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != StatusCandidate {
		t.Fatalf("expected candidate item after retry")
	}
}

func TestServiceDiscoverInteractiveNone(t *testing.T) {
	cache := &stubCache{}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("n\n"), Out: io.Discard},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		Now:        func() time.Time { return time.Now() },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != "none" {
		t.Fatalf("expected none item")
	}
}

func TestServiceDiscoverInteractiveSkip(t *testing.T) {
	cache := &stubCache{}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("skip\n"), Out: io.Discard},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
	}
	if err := service.Discover(context.Background(), DiscoverConfig{}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(cache.cf.Items) != 0 {
		t.Fatalf("expected no cache updates")
	}
}

func TestServiceDiscoverInteractiveMissingSelector(t *testing.T) {
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{}},
		Cache:      &stubCache{},
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia"}}},
		Scorer:     NewScorer(0, 0),
		Details:    keepCandidateDetails,
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false}); err == nil {
		t.Fatalf("expected error for missing selector")
	}
}

func TestServiceDiscoverSearchErrorStoresCache(t *testing.T) {
	cache := &stubCache{}
	service := Service{
		Searcher:   errSearcher{},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(0, 0),
		Details:    keepCandidateDetails,
		Now:        func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: true}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != "error" {
		t.Fatalf("expected error status cached")
	}
	if item.Error == "" {
		t.Fatalf("expected error message")
	}
}

func TestServiceDiscoverInteractiveSelected(t *testing.T) {
	cache := &stubCache{}
	service := Service{
		Searcher: stubSearcher{results: []Candidate{
			{VideoID: "a", URL: "u1", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000},
			{VideoID: "b", URL: "u2", Title: "Houston Oilers vs Miami Dolphins 1966 Broadcast", DurationSec: 3800},
		}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Selector:   &Selector{In: strings.NewReader("1,2\n"), Out: io.Discard},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		Now:        func() time.Time { return time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC) },
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: false}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != "selected" {
		t.Fatalf("expected selected item")
	}
	if len(item.Videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(item.Videos))
	}
}

func TestServiceDiscoverMissingDeps(t *testing.T) {
	service := Service{}
	if err := service.Discover(context.Background(), DiscoverConfig{}); err == nil {
		t.Fatalf("expected error for missing loader")
	}
	service.GameLoader = stubLoader{}
	if err := service.Discover(context.Background(), DiscoverConfig{}); err == nil {
		t.Fatalf("expected error for missing cache")
	}
	service.Cache = &stubCache{}
	if err := service.Discover(context.Background(), DiscoverConfig{}); err == nil {
		t.Fatalf("expected error for missing searcher")
	}
}

func TestSelectByScore(t *testing.T) {
	game := GameRef{AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins", Year: 1966}
	scorer := NewScorer(3000, 60)
	candidates := []Candidate{
		{Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000},
		{Title: "Highlights Oilers vs Dolphins", DurationSec: 600},
	}
	selected := selectByScore(game, candidates, scorer, 1)
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected")
	}
}

func TestSortGameEntries(t *testing.T) {
	entries := []gameEntry{
		{game: GameRef{GameID: "b"}, bestScore: 80, bestDuration: 1000},
		{game: GameRef{GameID: "a"}, bestScore: 90, bestDuration: 2000},
		{game: GameRef{GameID: "c"}, bestScore: 80, bestDuration: 3000},
	}
	sortGameEntries(entries)
	if entries[0].game.GameID != "a" || entries[1].game.GameID != "c" || entries[2].game.GameID != "b" {
		t.Fatalf("unexpected order: %s %s %s", entries[0].game.GameID, entries[1].game.GameID, entries[2].game.GameID)
	}
}
func TestServiceDiscoverRetryNoneByDefault(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {Status: "none", Query: "old"},
	}}}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
		Now:        func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	}
	// By default (RetryNone is true in CLI), games marked as "none" should be retried
	cfg := DiscoverConfig{NonInteractive: true, RetryNone: true, ScoreThreshold: 60}
	if err := service.Discover(context.Background(), cfg); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != StatusCandidate {
		t.Fatalf("expected candidate item after retry")
	}
}

func TestServiceDiscoverSkipsNoneWhenFlagDisabled(t *testing.T) {
	cache := &stubCache{cf: &CacheFile{Items: map[string]*CacheItem{
		"196612180mia": {Status: "none", Query: "old"},
	}}}
	service := Service{
		Searcher:   stubSearcher{results: []Candidate{{VideoID: "abc", URL: "u", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}},
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(3000, 60),
		Details:    keepCandidateDetails,
	}
	// When RetryNone is explicitly set to false, skip games marked as "none"
	cfg := DiscoverConfig{NonInteractive: true, RetryNone: false, ScoreThreshold: 60}
	if err := service.Discover(context.Background(), cfg); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != "none" {
		t.Fatalf("expected none item unchanged when RetryNone is false")
	}
}

func TestServiceDiscoverAPISuccess(t *testing.T) {
	cache := &stubCache{}
	searcher := &countingSearcher{results: []Candidate{{VideoID: "rod", URL: "u-rod", Title: "rod"}}}
	service := Service{
		Searcher:   searcher,
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(0, 0),
		Selector:   &Selector{Out: io.Discard, In: strings.NewReader("")},
		Details:    keepCandidateDetails,
		APIDiscoverer: func(ctx context.Context, game GameRef, maxResults int, minDurationSec int) ([]Candidate, error) {
			return []Candidate{{VideoID: "api", URL: "u-api", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}, nil
		},
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: true}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if searcher.count != 0 {
		t.Fatalf("expected rod searcher not called when API succeeds")
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || item.Status != StatusCandidate || len(item.Videos) != 1 || item.Videos[0].ID != "api" {
		t.Fatalf("unexpected cache item: %+v", item)
	}
}

func TestServiceDiscoverAPIFallbackToRod(t *testing.T) {
	cache := &stubCache{}
	searcher := &countingSearcher{results: []Candidate{{VideoID: "rod", URL: "u-rod", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000}}}
	service := Service{
		Searcher:   searcher,
		Cache:      cache,
		GameLoader: stubLoader{games: []GameRef{{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}}},
		Scorer:     NewScorer(0, 0),
		Selector:   &Selector{Out: io.Discard, In: strings.NewReader("")},
		Details:    keepCandidateDetails,
		APIDiscoverer: func(ctx context.Context, game GameRef, maxResults int, minDurationSec int) ([]Candidate, error) {
			return nil, fmt.Errorf("quotaExceeded")
		},
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: true}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if searcher.count == 0 {
		t.Fatalf("expected rod searcher called on API fallback")
	}
	item := cache.cf.Items["196612180mia"]
	if item == nil || len(item.Videos) != 1 || item.Videos[0].ID != "rod" {
		t.Fatalf("unexpected fallback result: %+v", item)
	}
}

func TestServiceDiscoverSkipsAPIForRestOfRunAfterQuotaExceeded(t *testing.T) {
	cache := &stubCache{}
	searcher := &countingSearcher{results: []Candidate{{VideoID: "rod", URL: "u-rod", Title: "Miami Dolphins Full Game", DurationSec: 4000}}}
	apiCalls := 0
	service := Service{
		Searcher: searcher,
		Cache:    cache,
		GameLoader: stubLoader{games: []GameRef{
			{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"},
			{GameID: "196612250mia", Year: 1966, AwayTeam: "New York Jets", HomeTeam: "Miami Dolphins"},
		}},
		Scorer:   NewScorer(0, 0),
		Selector: &Selector{Out: io.Discard, In: strings.NewReader("")},
		Details:  keepCandidateDetails,
		APIDiscoverer: func(ctx context.Context, game GameRef, maxResults int, minDurationSec int) ([]Candidate, error) {
			apiCalls++
			return nil, &youtubeapi.GoogleAPIError{
				StatusCode: 429,
				Reasons:    []string{"rateLimitExceeded"},
				Message:    "quota exceeded",
			}
		},
	}
	if err := service.Discover(context.Background(), DiscoverConfig{NonInteractive: true, Force: true}); err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if apiCalls != 1 {
		t.Fatalf("expected API called once after quota exhaustion, got %d", apiCalls)
	}
	if searcher.count != 2 {
		t.Fatalf("expected rod searcher for both games, got %d", searcher.count)
	}
}

func TestRankCandidatesDeterministicOrder(t *testing.T) {
	game := GameRef{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}
	scorer := NewScorer(0, 0)
	candidates := []Candidate{
		{Title: "B", DurationSec: 1000},
		{Title: "A", DurationSec: 1000},
		{Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 2000},
	}
	ranked := rankCandidates(game, candidates, scorer)
	if len(ranked) != 3 {
		t.Fatalf("unexpected length: %d", len(ranked))
	}
	if ranked[0].Title != "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game" {
		t.Fatalf("expected best scored first, got %s", ranked[0].Title)
	}
	if ranked[1].Title != "A" || ranked[2].Title != "B" {
		t.Fatalf("expected title tie-break, got %s and %s", ranked[1].Title, ranked[2].Title)
	}
}

func TestFilterByLegacyRelevanceDropsNoisyCandidates(t *testing.T) {
	game := GameRef{GameID: "196609090mia", Year: 1966, AwayTeam: "New York Jets", HomeTeam: "Miami Dolphins"}
	scorer := NewScorer(300, 60)
	candidates := []Candidate{
		{
			Title:       "New York Jets vs Miami Dolphins 1966-09-09 Full Game",
			Snippet:     "Broadcast copy of the September 9, 1966 AFL game.",
			DurationSec: 7200,
		},
		{
			Title:       "1966 Chiefs at Jets",
			Snippet:     "This is the 11/27/1966 Chiefs at Jets game. It is a narrated highlight film.",
			DurationSec: 1609,
		},
		{
			Title:       "1996 Week 3 New York Jets at Miami Dolphins",
			Snippet:     "Full game.",
			DurationSec: 9288,
		},
		{
			Title: "1966 New York Jets Season Film",
			Snippet: "Week 02 1966-09-09 New York Jets @ Miami Dolphins\n" +
				"Week 03 1966-09-18 Houston Oilers @ New York Jets\n" +
				"Week 04 1966-09-25 New York Jets @ Buffalo Bills",
			DurationSec: 1780,
		},
	}

	filtered := filterByLegacyRelevance(game, candidates, scorer)
	if len(filtered) != 1 {
		t.Fatalf("expected only exact candidate, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].Title != "New York Jets vs Miami Dolphins 1966-09-09 Full Game" {
		t.Fatalf("unexpected candidate kept: %s", filtered[0].Title)
	}
}

func TestFilterByLegacyRelevanceRequiresTemporalEvidence(t *testing.T) {
	game := GameRef{
		GameID:   "196609180buf",
		Year:     1966,
		AwayTeam: "Miami Dolphins",
		HomeTeam: "Buffalo Bills",
		DateRaw:  "Sunday Sep 18, 1966",
		Week:     "3",
	}
	scorer := NewScorer(300, 60)
	candidates := []Candidate{
		{
			Title:       "Bills LIGHT UP the Miami Dolphins 56-26 | Who's NEXT!?!?",
			Snippet:     "Buffalo Bills LIGHT UP the Miami Dolphins 56-26. SUBSCRIBE to get the latest.",
			Channel:     "Buffalo Fanatics",
			DurationSec: 6759,
		},
		{
			Title:       "Josh Allen owns the Dolphins | Buffalo Bills 26-11 win over Phins",
			Snippet:     "A win is a win and Josh Allen and the Bills have had many of them against the Dolphins.",
			Channel:     "Buffalo Fanatics",
			DurationSec: 4395,
		},
		{
			Title:       "Miami Dolphins at Buffalo Bills 1966 Week 3 Full Game",
			Channel:     "Classic NFL",
			DurationSec: 5400,
		},
	}

	filtered := filterByLegacyRelevance(game, candidates, scorer)
	if len(filtered) != 1 {
		t.Fatalf("expected only candidate with year/week evidence, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].Title != "Miami Dolphins at Buffalo Bills 1966 Week 3 Full Game" {
		t.Fatalf("unexpected candidate kept: %s", filtered[0].Title)
	}
}

func TestBuildQueryIncludesSeasonYearForPlayoffs(t *testing.T) {
	game := GameRef{
		GameID:   "198301080mia",
		Year:     1982,
		AwayTeam: "New England Patriots",
		HomeTeam: "Miami Dolphins",
		Week:     "Playoffs",
	}
	got := buildQuery(game)
	want := "New England Patriots vs Miami Dolphins 1982 playoffs 1983-01-08"
	if got != want {
		t.Fatalf("buildQuery() = %q, want %q", got, want)
	}
}

func TestMergeCandidateListsNormalizesAndMergesFields(t *testing.T) {
	embeddable := true
	got := mergeCandidateLists(
		[]Candidate{
			{
				URL:     "https://www.youtube.com/watch?v=abc123",
				Title:   "Miami Dolphins vs Jets full game",
				Channel: "Classic Sports",
				Score:   90,
			},
			{
				Title:   "Duplicate by title",
				Channel: "Archive",
			},
		},
		[]Candidate{
			{
				VideoID:     "abc123",
				DurationSec: 3600,
				Embeddable:  &embeddable,
				Score:       130,
			},
			{
				Title:        "Duplicate by title",
				Channel:      "Archive",
				DurationText: "12:34",
				Snippet:      "same title and channel",
			},
			{},
		},
	)

	if len(got) != 3 {
		t.Fatalf("len(mergeCandidateLists)=%d, want 3: %+v", len(got), got)
	}
	if got[0].VideoID != "abc123" || got[0].DurationSec != 3600 || got[0].Embeddable == nil || !*got[0].Embeddable || got[0].Score != 130 {
		t.Fatalf("first merged candidate not completed: %+v", got[0])
	}
	if got[1].DurationText != "12:34" || got[1].Snippet == "" {
		t.Fatalf("title/channel duplicate not merged: %+v", got[1])
	}
	if got[2].VideoID != "" || got[2].Title != "" {
		t.Fatalf("empty key candidate should be preserved as-is: %+v", got[2])
	}
}

func TestCandidateMergeKeyFallsBackToTitleAndURL(t *testing.T) {
	if got := candidateMergeKey(Candidate{}); got != "" {
		t.Fatalf("candidateMergeKey(empty)=%q, want empty", got)
	}
	if got := candidateMergeKey(Candidate{URL: "https://youtu.be/abc123"}); got != "id:abc123" {
		t.Fatalf("candidateMergeKey(url)=%q, want id:abc123", got)
	}
	if got := candidateMergeKey(Candidate{Title: " Miami  Dolphins ", Channel: "Classic Sports"}); got != "title:miami  dolphins|channel:classic sports" {
		t.Fatalf("candidateMergeKey(title/channel)=%q", got)
	}
}
