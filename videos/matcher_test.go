package videos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// matcherTestServer serves /search and /videos responses for matcher tests.
func matcherTestServer(t *testing.T, searchIDs []string, videos []youtubeapi.VideoCandidate) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			items := make([]map[string]any, 0, len(searchIDs))
			for _, id := range searchIDs {
				items = append(items, map[string]any{
					"id": map[string]string{"videoId": id},
				})
			}
			resp := map[string]any{"items": items}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		case "/videos":
			items := make([]map[string]any, 0, len(videos))
			for _, v := range videos {
				dur := v.Duration
				h := int(dur.Hours())
				m := int(dur.Minutes()) % 60
				s := int(dur.Seconds()) % 60
				// Simpler approach:
				var dStr string
				if h > 0 {
					dStr = "PT" + itoa(h) + "H" + itoa(m) + "M" + itoa(s) + "S"
				} else if m > 0 {
					dStr = "PT" + itoa(m) + "M" + itoa(s) + "S"
				} else {
					dStr = "PT" + itoa(s) + "S"
				}
				items = append(items, map[string]any{
					"id": v.ID,
					"snippet": map[string]any{
						"title":        v.Title,
						"description":  v.Description,
						"channelTitle": v.Channel,
					},
					"contentDetails": map[string]any{
						"duration": dStr,
					},
					"status": map[string]any{
						"embeddable": v.Embeddable,
					},
					"statistics": map[string]any{
						"viewCount": itoa(int(v.ViewCount)),
					},
				})
			}
			resp := map[string]any{"items": items}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func TestMatcher_MatchFromChannels(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	videos := []youtubeapi.VideoCandidate{
		{
			ID:          "vid_match",
			Title:       "Oakland Raiders vs Miami Dolphins 1966 AFL Full Game",
			Description: "Full game from September 2, 1966",
			Channel:     "NFL Throwback",
			Duration:    90 * time.Minute,
			Embeddable:  true,
			ViewCount:   50000,
		},
	}

	srv := matcherTestServer(t, nil, videos)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)

	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
	}
	index := NewMatchIndex(games)

	matcher := &Matcher{
		Client:         client,
		DiskCache:      diskCache,
		Quota:          quota,
		Index:          index,
		MinDurationSec: 300,
		MaxResults:     25,
	}

	channelItems := []youtubeapi.PlaylistItem{
		{VideoID: "vid_match", Title: "Oakland Raiders vs Miami Dolphins 1966 AFL Full Game", ChannelID: "ch1"},
	}

	cf := &CacheFile{
		Items: make(map[string]*CacheItem),
	}
	state := &PipelineState{
		ChannelMatches: make(map[string]string),
	}

	matched, err := matcher.MatchFromChannels(context.Background(), channelItems, state, cf)
	if err != nil {
		t.Fatalf("MatchFromChannels: %v", err)
	}
	if matched != 1 {
		t.Errorf("matched = %d, want 1", matched)
	}

	item, ok := cf.Items["196609020mia"]
	if !ok {
		t.Fatal("game should be in cache")
	}
	if item.Status != StatusCandidate {
		t.Errorf("Status = %q, want candidate", item.Status)
	}
	if !state.HasVideo("196609020mia") {
		t.Error("state should mark game as having video")
	}
}

func TestMatcher_MatchFromChannels_NoDoubleAssign(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// One video, two possible games
	videos := []youtubeapi.VideoCandidate{
		{
			ID:          "vid_one",
			Title:       "Oakland Raiders vs Miami Dolphins 1966",
			Description: "Full AFL game",
			Channel:     "NFL Throwback",
			Duration:    60 * time.Minute,
			Embeddable:  true,
			ViewCount:   10000,
		},
	}

	srv := matcherTestServer(t, nil, videos)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)

	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
		{GameID: "196611130mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "November 13, 1966"},
	}
	index := NewMatchIndex(games)

	matcher := &Matcher{
		Client:         client,
		DiskCache:      diskCache,
		Quota:          quota,
		Index:          index,
		MinDurationSec: 300,
		MaxResults:     25,
	}

	channelItems := []youtubeapi.PlaylistItem{
		{VideoID: "vid_one", Title: "Oakland Raiders vs Miami Dolphins 1966"},
	}

	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{ChannelMatches: make(map[string]string)}

	matched, err := matcher.MatchFromChannels(context.Background(), channelItems, state, cf)
	if err != nil {
		t.Fatalf("MatchFromChannels: %v", err)
	}

	// Only ONE game should get this video
	if matched != 1 {
		t.Errorf("matched = %d, want 1", matched)
	}

	// Count how many games got the video
	assigned := 0
	for _, item := range cf.Items {
		if item != nil && item.Status == StatusCandidate {
			assigned++
		}
	}
	if assigned != 1 {
		t.Errorf("assigned games = %d, want 1 (uniqueness violated)", assigned)
	}
}

func TestMatcher_MatchFromChannels_EmptyItems(t *testing.T) {
	matcher := &Matcher{}
	matched, err := matcher.MatchFromChannels(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if matched != 0 {
		t.Errorf("matched = %d, want 0", matched)
	}
}

func TestMatcher_SearchFallbackAutoApprovesHighConfidenceCandidate(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	videos := []youtubeapi.VideoCandidate{
		{
			ID:          "search_vid",
			Title:       "Oakland Raiders vs Miami Dolphins September 2 1966 Full Game",
			Description: "Complete AFL game broadcast from September 2, 1966",
			Channel:     "Vintage Pro Football",
			ChannelID:   "UCvintage",
			Duration:    90 * time.Minute,
			Embeddable:  true,
			ViewCount:   50000,
		},
	}
	srv := matcherTestServer(t, []string{"search_vid"}, videos)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)
	index := NewMatchIndex([]GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966", Week: "1"},
	})
	matcher := &Matcher{
		Client:               client,
		DiskCache:            diskCache,
		Quota:                quota,
		Index:                index,
		MinDurationSec:       300,
		MaxResults:           25,
		AutoApprove:          true,
		AutoApproveThreshold: 130,
	}
	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{
		SearchQueueRemaining: []string{"196609020mia"},
		ChannelMatches:       make(map[string]string),
	}

	added, noResult, err := matcher.SearchFallback(context.Background(), state, cf)
	if err != nil {
		t.Fatalf("SearchFallback: %v", err)
	}
	if added != 1 || noResult != 0 {
		t.Fatalf("added=%d noResult=%d, want 1/0", added, noResult)
	}
	item := cf.Items["196609020mia"]
	if item == nil || item.Status != StatusApproved || item.NeedsReview {
		t.Fatalf("item=%+v, want approved without review", item)
	}
	if got := item.Videos[0].Role; got != string(ContentRoleFullGame) {
		t.Fatalf("role=%s want=%s", got, ContentRoleFullGame)
	}
}

func TestMatcher_SearchFallbackGroupsCompleteMultipartCandidate(t *testing.T) {
	videos := []youtubeapi.VideoCandidate{
		{
			ID:         "part2",
			Title:      "NFL 1983 09-19-83 Miami Dolphins at Los Angeles Raiders pt 2 of 2",
			Channel:    "Larry's Classic Sports",
			Duration:   63 * time.Minute,
			Embeddable: true,
			ViewCount:  1000,
		},
		{
			ID:         "part1",
			Title:      "NFL 1983 09-19-83 Miami Dolphins at Los Angeles Raiders pt 1 of 2",
			Channel:    "Larry's Classic Sports",
			Duration:   62 * time.Minute,
			Embeddable: true,
			ViewCount:  1000,
		},
	}
	srv := matcherTestServer(t, nil, videos)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	if err := diskCache.SaveSearchCache(&SearchCacheEntry{
		GameID:   "198309190rai",
		Query:    "Miami Dolphins - Los Angeles Raiders 1983-09-19",
		VideoIDs: []string{"part2", "part1"},
	}); err != nil {
		t.Fatal(err)
	}
	quota := NewQuotaManager(10000)
	index := NewMatchIndex([]GameRef{
		{GameID: "198309190rai", Year: 1983, HomeTeam: "Los Angeles Raiders", AwayTeam: "Miami Dolphins", DateRaw: "September 19, 1983", Week: "2"},
	})
	matcher := &Matcher{
		Client:               client,
		DiskCache:            diskCache,
		Quota:                quota,
		Index:                index,
		MinDurationSec:       300,
		MaxResults:           25,
		AutoApprove:          true,
		AutoApproveThreshold: 130,
	}
	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{
		SearchQueueRemaining: []string{"198309190rai"},
		ChannelMatches:       make(map[string]string),
	}

	added, noResult, err := matcher.SearchFallback(context.Background(), state, cf)
	if err != nil {
		t.Fatalf("SearchFallback: %v", err)
	}
	if added != 1 || noResult != 0 {
		t.Fatalf("added=%d noResult=%d, want 1/0", added, noResult)
	}
	item := cf.Items["198309190rai"]
	if item == nil || len(item.Videos) != 2 {
		t.Fatalf("videos=%+v, want two parts", item)
	}
	if item.Videos[0].ID != "part1" || item.Videos[0].Part != 1 || item.Videos[1].ID != "part2" || item.Videos[1].Part != 2 {
		t.Fatalf("unexpected part order: %+v", item.Videos)
	}
	if item.Status != StatusApproved {
		t.Fatalf("status=%s want=%s", item.Status, StatusApproved)
	}
}

func TestMatcher_SearchFallback_CachedResults(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	videos := []youtubeapi.VideoCandidate{
		{
			ID:          "search_vid",
			Title:       "Oakland Raiders vs Miami Dolphins September 2 1966 Full Game",
			Description: "Complete 1966 AFL game",
			Channel:     "NFL Throwback",
			Duration:    90 * time.Minute,
			Embeddable:  true,
			ViewCount:   50000,
		},
	}

	srv := matcherTestServer(t, nil, videos)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)

	// Pre-populate search cache
	err := diskCache.SaveSearchCache(&SearchCacheEntry{
		GameID:    "196609020mia",
		Query:     "Oakland Raiders vs Miami Dolphins 1966-09-02",
		FetchedAt: now,
		VideoIDs:  []string{"search_vid"},
	})
	if err != nil {
		t.Fatal(err)
	}

	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
	}
	index := NewMatchIndex(games)

	matcher := &Matcher{
		Client:         client,
		DiskCache:      diskCache,
		Quota:          quota,
		Index:          index,
		MinDurationSec: 300,
		MaxResults:     25,
	}

	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{
		SearchQueueRemaining: []string{"196609020mia"},
		ChannelMatches:       make(map[string]string),
	}

	added, noResult, err := matcher.SearchFallback(context.Background(), state, cf)
	if err != nil {
		t.Fatalf("SearchFallback: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if noResult != 0 {
		t.Errorf("noResult = %d, want 0", noResult)
	}

	// Should NOT have used expensive search quota (cache hit)
	// Only videos.list for hydration (1 unit)
	if quota.Used() > 1 {
		t.Errorf("quota used = %d, want <= 1 (cached search should be cheap)", quota.Used())
	}
}

func TestMatcher_SearchFallback_QuotaStop(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Server that would serve search results
	srv := matcherTestServer(t, []string{"vid1"}, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }

	// Only 50 units — not enough for a search (costs 100)
	quota := NewQuotaManager(50)

	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
	}
	index := NewMatchIndex(games)

	matcher := &Matcher{
		Client:         client,
		DiskCache:      diskCache,
		Quota:          quota,
		Index:          index,
		MinDurationSec: 300,
		MaxResults:     25,
	}

	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{
		SearchQueueRemaining: []string{"196609020mia"},
		ChannelMatches:       make(map[string]string),
	}

	added, _, err := matcher.SearchFallback(context.Background(), state, cf)
	if err != nil {
		t.Fatalf("SearchFallback: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0 (should stop due to quota)", added)
	}

	// Game should be re-queued since we couldn't afford search
	if len(state.SearchQueueRemaining) != 1 {
		t.Errorf("search queue = %d, want 1 (game re-queued)", len(state.SearchQueueRemaining))
	}
}

func TestCollectUsedVideoIDs(t *testing.T) {
	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"game1": {
				Status: "selected",
				Videos: []Video{{ID: "vid1"}, {ID: "vid2"}},
			},
			"game2": {
				Status: "none",
				Videos: []Video{{ID: "vid3"}},
			},
			"game3": nil,
		},
	}

	used := collectUsedVideoIDs(cf)
	if !used["vid1"] || !used["vid2"] {
		t.Errorf("vid1 and vid2 should be used: %v", used)
	}
	if used["vid3"] {
		t.Error("vid3 should not be used (status=none)")
	}
}

func TestCollectUsedVideoIDs_NilCache(t *testing.T) {
	used := collectUsedVideoIDs(nil)
	if len(used) != 0 {
		t.Errorf("expected empty map, got %d entries", len(used))
	}
}

func TestMatcher_SearchFallback_LiveSearch(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	videos := []youtubeapi.VideoCandidate{
		{
			ID:          "live_vid",
			Title:       "Oakland Raiders vs Miami Dolphins 1966-09-02 Full Game",
			Description: "Complete 1966 AFL game Oakland Raiders at Miami Dolphins",
			Channel:     "NFL Throwback",
			Duration:    90 * time.Minute,
			Embeddable:  true,
			ViewCount:   50000,
		},
	}

	srv := matcherTestServer(t, []string{"live_vid"}, videos)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)

	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
	}
	index := NewMatchIndex(games)

	matcher := &Matcher{
		Client:         client,
		DiskCache:      diskCache,
		Quota:          quota,
		Index:          index,
		MinDurationSec: 300,
		MaxResults:     25,
	}

	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{
		SearchQueueRemaining: []string{"196609020mia"},
		ChannelMatches:       make(map[string]string),
	}

	added, noResult, err := matcher.SearchFallback(context.Background(), state, cf)
	if err != nil {
		t.Fatalf("SearchFallback: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	if noResult != 0 {
		t.Errorf("noResult = %d, want 0", noResult)
	}

	// Should have used search quota (100 units) + hydration (1 unit)
	if quota.Used() < 100 {
		t.Errorf("quota used = %d, want >= 100 (live search used)", quota.Used())
	}

	// Game should be in cache with selected status
	item, ok := cf.Items["196609020mia"]
	if !ok {
		t.Fatal("game should be in cache")
	}
	if item.Status != StatusCandidate {
		t.Errorf("Status = %q, want candidate", item.Status)
	}

	// Search results should be disk-cached
	if !diskCache.HasSearchCache("196609020mia") {
		t.Error("search results should be disk-cached")
	}
}

func TestMatcher_SearchFallback_NoResults(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Server returns empty search results
	srv := matcherTestServer(t, nil, nil)
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	tmpDir := t.TempDir()
	diskCache := NewDiskCacheManager(tmpDir)
	diskCache.nowFunc = func() time.Time { return now }
	quota := NewQuotaManager(10000)

	games := []GameRef{
		{GameID: "196609020mia", Year: 1966, HomeTeam: "Miami Dolphins", AwayTeam: "Oakland Raiders", DateRaw: "September 2, 1966"},
	}
	index := NewMatchIndex(games)

	matcher := &Matcher{
		Client:         client,
		DiskCache:      diskCache,
		Quota:          quota,
		Index:          index,
		MinDurationSec: 300,
		MaxResults:     25,
	}

	cf := &CacheFile{Items: make(map[string]*CacheItem)}
	state := &PipelineState{
		SearchQueueRemaining: []string{"196609020mia"},
		ChannelMatches:       make(map[string]string),
	}

	added, noResult, err := matcher.SearchFallback(context.Background(), state, cf)
	if err != nil {
		t.Fatalf("SearchFallback: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
	if noResult != 1 {
		t.Errorf("noResult = %d, want 1", noResult)
	}
}
