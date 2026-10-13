package videos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCachedClassifierWritesOutput(t *testing.T) {
	dir := t.TempDir()
	processedDir := filepath.Join(dir, "processed")
	seasonDir := filepath.Join(processedDir, "1966")
	if err := os.MkdirAll(seasonDir, 0o755); err != nil {
		t.Fatalf("create processed dir: %v", err)
	}
	writeWebGameJSON(t, seasonDir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")
	writeWebGameJSON(t, seasonDir, "196609090mia", "New York Jets", "Miami Dolphins", "Friday Sep 9, 1966", "2")

	store, err := NewSQLiteCandidateStore(filepath.Join(dir, "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	embeddable := true
	if err := store.SaveSearchResults(context.Background(), GameRef{
		GameID:   "196609020mia",
		Year:     1966,
		AwayTeam: "Oakland Raiders",
		HomeTeam: "Miami Dolphins",
		DateRaw:  "Friday Sep 2, 1966",
		Week:     "1",
	}, "Oakland Raiders vs Miami Dolphins 1966-09-02", "test", []Candidate{{
		VideoID:     "auto1",
		URL:         "https://www.youtube.com/watch?v=auto1",
		Title:       "Oakland Raiders at Miami Dolphins 1966-09-02 Week 1 Full Game Broadcast",
		Channel:     "Vintage Pro Football",
		DurationSec: 7200,
		Snippet:     "Full game broadcast.",
		Embeddable:  &embeddable,
	}}); err != nil {
		t.Fatalf("seed discovery: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	output := filepath.Join(dir, "videos.json")
	stats, err := RunCachedClassifier(context.Background(), CachedClassifierConfig{
		ProcessedDir:     processedDir,
		DiscoveryDB:      filepath.Join(dir, "discovery.sqlite"),
		OutputFile:       output,
		MinDurationSec:   300,
		ScoreThreshold:   60,
		AutoApproveScore: 130,
	})
	if err != nil {
		t.Fatalf("RunCachedClassifier error: %v", err)
	}
	if stats.GamesTotal != 2 || stats.AutoApproved != 1 || stats.NoCachedSearch != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	cf, err := NewFileCache(output).Load()
	if err != nil {
		t.Fatalf("load output: %v", err)
	}
	if item := cf.Items["196609020mia"]; item == nil || item.Status != StatusApproved || item.NeedsReview || len(item.Videos) != 1 {
		t.Fatalf("expected approved item, got %+v", item)
	}
	if item := cf.Items["196609090mia"]; item == nil || item.Status != StatusNone || len(item.Videos) != 0 {
		t.Fatalf("expected none item, got %+v", item)
	}
}

func TestRunCachedClassifierWritesReviewCandidateAndErrors(t *testing.T) {
	dir := t.TempDir()
	processedDir := filepath.Join(dir, "processed")
	seasonDir := filepath.Join(processedDir, "1966")
	if err := os.MkdirAll(seasonDir, 0o755); err != nil {
		t.Fatalf("create processed dir: %v", err)
	}
	writeWebGameJSON(t, seasonDir, "196609020mia", "Oakland Raiders", "Miami Dolphins", "Friday Sep 2, 1966", "1")

	store, err := NewSQLiteCandidateStore(filepath.Join(dir, "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	embeddable := true
	if err := store.SaveSearchResults(context.Background(), GameRef{
		GameID:   "196609020mia",
		Year:     1966,
		AwayTeam: "Oakland Raiders",
		HomeTeam: "Miami Dolphins",
		DateRaw:  "Friday Sep 2, 1966",
		Week:     "1",
	}, "Oakland Raiders vs Miami Dolphins 1966-09-02", "test", []Candidate{{
		VideoID:     "review1",
		URL:         "https://www.youtube.com/watch?v=review1",
		Title:       "Oakland Raiders vs Miami Dolphins 1966 Week 1 Top 10 Highlights",
		Channel:     "Vintage Pro Football",
		DurationSec: 700,
		Snippet:     "Highlights.",
		Embeddable:  &embeddable,
	}}); err != nil {
		t.Fatalf("seed discovery: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	output := filepath.Join(dir, "videos-review.json")
	stats, err := RunCachedClassifier(context.Background(), CachedClassifierConfig{
		ProcessedDir:     processedDir,
		DiscoveryDB:      filepath.Join(dir, "discovery.sqlite"),
		OutputFile:       output,
		MinDurationSec:   300,
		ScoreThreshold:   60,
		AutoApproveScore: 130,
	})
	if err != nil {
		t.Fatalf("RunCachedClassifier error: %v", err)
	}
	if stats.GamesProcessed != 1 || stats.NeedsReview != 1 || stats.AutoApproved != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	cf, err := NewFileCache(output).Load()
	if err != nil {
		t.Fatalf("load output: %v", err)
	}
	item := cf.Items["196609020mia"]
	if item == nil || item.Status != StatusCandidate || !item.NeedsReview || len(item.Videos) != 1 {
		t.Fatalf("expected review item, got %+v", item)
	}

	if _, err := RunCachedClassifier(context.Background(), CachedClassifierConfig{ProcessedDir: filepath.Join(dir, "missing"), DiscoveryDB: filepath.Join(dir, "discovery.sqlite"), OutputFile: filepath.Join(dir, "out.json")}); err == nil {
		t.Fatalf("expected missing processed dir error")
	}
	if _, err := RunCachedClassifier(context.Background(), CachedClassifierConfig{ProcessedDir: processedDir, DiscoveryDB: filepath.Join(dir, "missing.sqlite"), OutputFile: filepath.Join(dir, "out.json")}); err != nil {
		t.Fatalf("missing sqlite should initialize empty store, got %v", err)
	}
}
