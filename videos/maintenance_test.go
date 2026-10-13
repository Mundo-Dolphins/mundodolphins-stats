package videos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type stubWatchChecker struct {
	exists      map[string]bool
	inspections map[string]WatchInspection
	errs        map[string]error
	calls       []string
}

func (s *stubWatchChecker) Exists(_ context.Context, videoID string) (bool, error) {
	s.calls = append(s.calls, videoID)
	if s.errs != nil && s.errs[videoID] != nil {
		return false, s.errs[videoID]
	}
	if s.exists == nil {
		return false, nil
	}
	return s.exists[videoID], nil
}

func (s *stubWatchChecker) Inspect(ctx context.Context, videoID string) (WatchInspection, error) {
	s.calls = append(s.calls, videoID)
	if s.errs != nil && s.errs[videoID] != nil {
		return WatchInspection{}, s.errs[videoID]
	}
	if s.inspections != nil {
		if inspection, ok := s.inspections[videoID]; ok {
			return inspection, nil
		}
	}
	if s.exists == nil {
		return WatchInspection{Exists: false}, nil
	}
	return WatchInspection{Exists: s.exists[videoID]}, nil
}

func (s *stubWatchChecker) Close() error { return nil }

func TestValidateExistingVideos_RemovesMissing(t *testing.T) {
	// Mock server: only "exists1" exists, "gone1" is gone
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"items": []map[string]string{{"id": "exists1"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"game1": {
				Status: "selected",
				Videos: []Video{{ID: "exists1", URL: "https://youtube.com/watch?v=exists1"}},
			},
			"game2": {
				Status: "selected",
				Videos: []Video{{ID: "gone1", URL: "https://youtube.com/watch?v=gone1"}},
			},
			"game3": {
				Status: "none",
				Videos: nil,
			},
		},
	}

	removed, err := ValidateExistingVideos(context.Background(), cf, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected 1 removal, got %d: %v", len(removed), removed)
	}

	// game2 should now be status "none"
	if cf.Items["game2"].Status != "none" {
		t.Errorf("game2 status = %q, want 'none'", cf.Items["game2"].Status)
	}
	if len(cf.Items["game2"].Videos) != 0 {
		t.Errorf("game2 should have 0 videos, got %d", len(cf.Items["game2"].Videos))
	}

	// game1 should still be selected
	if cf.Items["game1"].Status != "selected" {
		t.Errorf("game1 status = %q, want 'selected'", cf.Items["game1"].Status)
	}
}

func TestValidateExistingVideos_AllExist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"items": []map[string]string{{"id": "v1"}, {"id": "v2"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"g1": {Status: "selected", Videos: []Video{{ID: "v1"}}},
			"g2": {Status: "selected", Videos: []Video{{ID: "v2"}}},
		},
	}

	removed, err := ValidateExistingVideos(context.Background(), cf, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removals, got %d", len(removed))
	}
}

func TestValidateExistingVideos_EmptyCache(t *testing.T) {
	client := youtubeapi.NewClient("test-key", http.DefaultClient)
	cf := &CacheFile{Items: map[string]*CacheItem{}}

	removed, err := ValidateExistingVideos(context.Background(), cf, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removals, got %d", len(removed))
	}
}

func TestValidateExistingVideos_NilInputs(t *testing.T) {
	client := youtubeapi.NewClient("key", http.DefaultClient)

	_, err := ValidateExistingVideos(context.Background(), nil, client)
	if err == nil {
		t.Error("expected error for nil cache file")
	}

	_, err = ValidateExistingVideos(context.Background(), &CacheFile{}, nil)
	if err == nil {
		t.Error("expected error for nil client")
	}
}

func TestValidateExistingVideos_SkipsNonSelected(t *testing.T) {
	// Server should NOT be called at all since only non-selected items exist
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"g1": {Status: "none"},
			"g2": {Status: "pending"},
		},
	}

	removed, err := ValidateExistingVideos(context.Background(), cf, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removals, got %d", len(removed))
	}
	if called {
		t.Error("server should not have been called for non-selected items")
	}
}

func TestValidateExistingVideosWithCheckerRemovesUnavailable(t *testing.T) {
	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"game1": {
				Status: StatusSelected,
				Videos: []Video{{ID: "exists1"}, {ID: "gone1"}},
			},
			"game2": {
				Status: StatusNone,
				Videos: []Video{{ID: "skip1"}},
			},
		},
	}
	checker := &stubWatchChecker{exists: map[string]bool{"exists1": true, "gone1": false}}

	removed, err := ValidateExistingVideosWithChecker(context.Background(), cf, checker, func() time.Time {
		return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("ValidateExistingVideosWithChecker error: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("removed len = %d, want 1", len(removed))
	}
	item := cf.Items["game1"]
	if item.Status != StatusSelected {
		t.Fatalf("status = %q, want selected", item.Status)
	}
	if len(item.Videos) != 1 || item.Videos[0].ID != "exists1" {
		t.Fatalf("videos = %+v, want only exists1", item.Videos)
	}
	if len(checker.calls) != 2 {
		t.Fatalf("checker calls = %v, want 2 selected videos only", checker.calls)
	}
}

func TestValidateExistingVideosWithCheckerMarksNoneWhenAllGone(t *testing.T) {
	cf := &CacheFile{
		Items: map[string]*CacheItem{
			"game1": {Status: StatusCandidate, NeedsReview: true, AutoScoring: &AutoScoringMeta{}, Videos: []Video{{ID: "gone1"}}},
		},
	}
	checker := &stubWatchChecker{exists: map[string]bool{"gone1": false}}

	removed, err := ValidateExistingVideosWithChecker(context.Background(), cf, checker, nil)
	if err != nil {
		t.Fatalf("ValidateExistingVideosWithChecker error: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("removed len = %d, want 1", len(removed))
	}
	item := cf.Items["game1"]
	if item.Status != StatusNone {
		t.Fatalf("status = %q, want none", item.Status)
	}
	if item.NeedsReview || item.AutoScoring != nil {
		t.Fatalf("review/scoring metadata should be cleared: %+v", item)
	}
}

func TestValidateExistingVideosWithCheckerKeepsOnValidationError(t *testing.T) {
	cf := &CacheFile{Items: map[string]*CacheItem{
		"game1": {Status: StatusSelected, Videos: []Video{{ID: "v1"}}},
	}}
	checker := &stubWatchChecker{errs: map[string]error{"v1": fmt.Errorf("network")}}

	removed, err := ValidateExistingVideosWithChecker(context.Background(), cf, checker, nil)
	if err != nil {
		t.Fatalf("ValidateExistingVideosWithChecker error: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed len = %d, want 0", len(removed))
	}
	if cf.Items["game1"].Status != StatusSelected || len(cf.Items["game1"].Videos) != 1 {
		t.Fatalf("item should be kept on validation error: %+v", cf.Items["game1"])
	}
}

func TestValidateAndScoreExistingVideosWithCheckerMarksLowScoreForReview(t *testing.T) {
	cf := &CacheFile{Items: map[string]*CacheItem{
		"202409120mia": {
			Status: StatusSelected,
			Videos: []Video{{ID: "preview1", Title: "Old title", URL: "https://www.youtube.com/watch?v=preview1"}},
		},
	}}
	checker := &stubWatchChecker{inspections: map[string]WatchInspection{
		"preview1": {
			Exists: true,
			Details: VideoDetails{
				Title:       "Week preview and betting odds",
				Description: "Miami Dolphins vs Buffalo Bills predictions",
				Duration:    7 * time.Minute,
			},
		},
	}}
	games := []GameRef{{GameID: "202409120mia", Year: 2024, AwayTeam: "Buffalo Bills", HomeTeam: "Miami Dolphins"}}

	removed, err := ValidateAndScoreExistingVideosWithChecker(context.Background(), cf, games, checker, NewScorer(300, 60), 60, nil)
	if err != nil {
		t.Fatalf("ValidateAndScoreExistingVideosWithChecker error: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed len=%d, want 0", len(removed))
	}
	item := cf.Items["202409120mia"]
	if !item.NeedsReview {
		t.Fatal("expected item to need review")
	}
	if item.AutoScoring == nil || item.AutoScoring.ScoreBreakdown.Total >= 60 {
		t.Fatalf("expected low auto score, got %+v", item.AutoScoring)
	}
	if !strings.Contains(item.Error, "search alternatives") {
		t.Fatalf("expected search alternatives error, got %q", item.Error)
	}
	if item.Videos[0].Title != "Week preview and betting odds" || item.Videos[0].DurationSeconds != 420 {
		t.Fatalf("expected video metadata updated, got %+v", item.Videos[0])
	}
}

func TestFormatISO8601(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{2*time.Hour + 15*time.Minute + 30*time.Second, "PT2H15M30S"},
		{45*time.Minute + 10*time.Second, "PT45M10S"},
		{30 * time.Second, "PT30S"},
		{1 * time.Hour, "PT1H0M0S"},
	}
	for _, tt := range tests {
		got := formatISO8601(tt.d)
		if got != tt.want {
			t.Errorf("formatISO8601(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
