package videos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildScorerReport(t *testing.T) {
	root := t.TempDir()
	gamesDir := filepath.Join(root, "games")
	if err := os.MkdirAll(gamesDir, 0o755); err != nil {
		t.Fatalf("mkdir games: %v", err)
	}
	writeFile(t, filepath.Join(gamesDir, "196612180mia.json"), `{"game_id":"196612180mia","data":{"week":"15","scorebox":{"teams":[{"name":{"content":"Houston Oilers"}},{"name":{"content":"Miami Dolphins"}}],"meta":{"date":"Sunday Dec 18, 1966"}}}}`)
	writeFile(t, filepath.Join(gamesDir, "196609180buf.json"), `{"game_id":"196609180buf","data":{"week":"3","scorebox":{"teams":[{"name":{"content":"Miami Dolphins"}},{"name":{"content":"Buffalo Bills"}}],"meta":{"date":"Sunday Sep 18, 1966"}}}}`)

	cachePath := filepath.Join(root, "videos.json")
	cache := NewFileCache(cachePath)
	cf := newCacheFile(time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	cf.Items["196612180mia"] = &CacheItem{
		Status: StatusApproved,
		Videos: []Video{{
			ID:              "approved",
			URL:             "https://www.youtube.com/watch?v=approved",
			Title:           "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game",
			DurationSeconds: 4000,
		}},
	}
	cf.Items["196609180buf"] = &CacheItem{Status: StatusApproved}
	if err := cache.Save(cf); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	dbPath := filepath.Join(root, "discovery.sqlite")
	store, err := NewSQLiteCandidateStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore: %v", err)
	}
	err = store.SaveSearchResults(context.Background(), GameRef{GameID: "196612180mia"}, "Houston Oilers vs Miami Dolphins 1966", "rod", []Candidate{
		{
			VideoID:     "approved",
			URL:         "https://www.youtube.com/watch?v=approved",
			Title:       "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game",
			DurationSec: 4000,
		},
	})
	if err != nil {
		t.Fatalf("seed approved search: %v", err)
	}
	err = store.SaveSearchResults(context.Background(), GameRef{GameID: "196609180buf"}, "Miami Dolphins vs Buffalo Bills 1966-09-18", "web-enriched", []Candidate{
		{
			VideoID:     "noise",
			URL:         "https://www.youtube.com/watch?v=noise",
			Title:       "Josh Allen owns the Dolphins | Buffalo Bills 26-11 win over Phins",
			DurationSec: 4395,
		},
	})
	if err != nil {
		t.Fatalf("seed noisy search: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	report, err := BuildScorerReport(context.Background(), ScorerReportConfig{
		GamesDir:       gamesDir,
		CacheFile:      cachePath,
		DiscoveryDB:    dbPath,
		MinDurationSec: 300,
		ScoreThreshold: 60,
		ExampleLimit:   10,
	})
	if err != nil {
		t.Fatalf("BuildScorerReport: %v", err)
	}
	if report.Summary.GamesLoaded != 2 || report.Summary.ApprovedVideos != 1 || report.Summary.ApprovedWithoutVideos != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if report.Summary.DiscoveryPairs != 2 || report.Summary.DiscoveryCandidatesKept != 1 || report.Summary.DiscoveryGamesZeroKept != 1 {
		t.Fatalf("unexpected discovery summary: %+v", report.Summary)
	}
	if reasonCount(report, "missing_temporal") != 1 || reasonCount(report, "auto_accept") != 1 {
		t.Fatalf("unexpected reason counts: %+v", report.Reasons)
	}
	if len(report.ZeroCandidateGames) != 1 || report.ZeroCandidateGames[0].GameID != "196609180buf" {
		t.Fatalf("unexpected zero candidate games: %+v", report.ZeroCandidateGames)
	}
	if len(report.ApprovedIssues) != 0 {
		t.Fatalf("unexpected approved issues: %+v", report.ApprovedIssues)
	}
}

func reasonCount(report *ScorerReport, reason string) int {
	for _, item := range report.Reasons {
		if item.Reason == reason {
			return item.Count
		}
	}
	return 0
}
