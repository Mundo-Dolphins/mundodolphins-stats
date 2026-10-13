package videos

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSQLiteCandidateStoreSaveAndLoadCandidates(t *testing.T) {
	store, err := NewSQLiteCandidateStore(filepath.Join(t.TempDir(), "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	defer store.Close()
	store.Now = func() time.Time { return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC) }

	embeddable := false
	err = store.SaveSearchResults(context.Background(), GameRef{GameID: "198212120mia"}, "Miami Dolphins vs New England Patriots 1982-12-12", "rod", []Candidate{
		{
			VideoID:      "wildcard",
			URL:          "https://www.youtube.com/watch?v=wildcard",
			Title:        "Miami Dolphins vs New England Patriots January 8 1983 AFC Wild Card Full Game",
			Channel:      "Classic NFL",
			DurationText: "2:10:00",
			DurationSec:  7800,
			Snippet:      "Full broadcast from the 1982 season AFC wild card playoff.",
			Embeddable:   &embeddable,
		},
	})
	if err != nil {
		t.Fatalf("SaveSearchResults error: %v", err)
	}

	candidates, err := store.LoadCandidates(context.Background(), 10)
	if err != nil {
		t.Fatalf("LoadCandidates error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates)=%d, want 1", len(candidates))
	}
	got := candidates[0]
	if got.VideoID != "wildcard" || got.Title == "" || got.DurationSec != 7800 {
		t.Fatalf("unexpected candidate loaded: %+v", got)
	}
	if got.Embeddable == nil || *got.Embeddable {
		t.Fatalf("expected embeddable=false, got %+v", got.Embeddable)
	}
}

func TestSQLiteCandidateStoreLoadCandidatesForGame(t *testing.T) {
	store, err := NewSQLiteCandidateStore(filepath.Join(t.TempDir(), "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	defer store.Close()

	game := GameRef{GameID: "198301080mia", Year: 1982, AwayTeam: "New England Patriots", HomeTeam: "Miami Dolphins", DateRaw: "January 8, 1983", Week: "Wild Card"}
	other := GameRef{GameID: "198212120mia", Year: 1982, AwayTeam: "Buffalo Bills", HomeTeam: "Miami Dolphins", DateRaw: "December 12, 1982", Week: "5"}
	embeddable := true
	if err := store.SaveSearchResults(context.Background(), game, "patriots dolphins", "test", []Candidate{
		{VideoID: "v1", URL: "https://www.youtube.com/watch?v=v1", Title: "Patriots Dolphins", DurationSec: 3600, Embeddable: &embeddable},
	}); err != nil {
		t.Fatalf("save game results: %v", err)
	}
	if err := store.SaveSearchResults(context.Background(), other, "bills dolphins", "test", []Candidate{
		{VideoID: "v2", URL: "https://www.youtube.com/watch?v=v2", Title: "Bills Dolphins", DurationSec: 3600},
	}); err != nil {
		t.Fatalf("save other results: %v", err)
	}

	candidates, err := store.LoadCandidatesForGame(context.Background(), game, 0)
	if err != nil {
		t.Fatalf("LoadCandidatesForGame error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].VideoID != "v1" || candidates[0].Embeddable == nil || !*candidates[0].Embeddable {
		t.Fatalf("unexpected game candidates: %+v", candidates)
	}
}

func TestSQLiteCandidateStoreConcurrentSaves(t *testing.T) {
	store, err := NewSQLiteCandidateStore(filepath.Join(t.TempDir(), "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	defer store.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 25)
	for i := 0; i < 25; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := store.SaveSearchResults(context.Background(), GameRef{GameID: "196612180mia"}, "Houston Oilers vs Miami Dolphins 1966", "web-enriched", []Candidate{
				{
					VideoID:     fmt.Sprintf("video-%02d", i),
					URL:         fmt.Sprintf("https://www.youtube.com/watch?v=video-%02d", i),
					Title:       "Houston Oilers vs Miami Dolphins 1966 Full Game",
					DurationSec: 4000,
				},
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("SaveSearchResults concurrent error: %v", err)
		}
	}

	candidates, err := store.LoadCandidates(context.Background(), 50)
	if err != nil {
		t.Fatalf("LoadCandidates error: %v", err)
	}
	if len(candidates) != 25 {
		t.Fatalf("len(candidates)=%d, want 25", len(candidates))
	}
}

func TestServiceFindCandidatesMergesSQLiteStoreWithExternalSearch(t *testing.T) {
	store, err := NewSQLiteCandidateStore(filepath.Join(t.TempDir(), "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	defer store.Close()

	err = store.SaveSearchResults(context.Background(), GameRef{GameID: "198212120mia"}, "Miami Dolphins vs New England Patriots 1982 week 14", "rod", []Candidate{
		{
			VideoID:      "wildcard",
			URL:          "https://www.youtube.com/watch?v=wildcard",
			Title:        "Miami Dolphins vs New England Patriots January 8 1983 AFC Wild Card Full Game",
			Channel:      "Classic NFL",
			DurationText: "2:10:00",
			DurationSec:  7800,
			Snippet:      "Full broadcast from the 1982 season AFC wild card playoff.",
		},
	})
	if err != nil {
		t.Fatalf("seed sqlite store: %v", err)
	}

	searcher := &countingSearcher{results: []Candidate{
		{
			VideoID:     "wildcard",
			URL:         "https://www.youtube.com/watch?v=wildcard",
			Title:       "Duplicate should be deduped",
			DurationSec: 0,
		},
		{
			VideoID:     "remote",
			URL:         "https://www.youtube.com/watch?v=remote",
			Title:       "New England Patriots vs Miami Dolphins 1982 AFC Wild Card Full Game",
			Channel:     "Classic NFL",
			DurationSec: 7600,
		},
	}}
	service := Service{
		Searcher:       searcher,
		Scorer:         NewScorer(300, 60),
		CandidateStore: store,
	}
	game := GameRef{GameID: "198301080mia", Year: 1982, AwayTeam: "New England Patriots", HomeTeam: "Miami Dolphins", DateRaw: "January 8, 1983", Week: "Wild Card"}
	candidates, err := service.findCandidates(context.Background(), game, "New England Patriots vs Miami Dolphins 1982 wild card", 10, 300)
	if err != nil {
		t.Fatalf("findCandidates error: %v", err)
	}
	if searcher.count != 1 {
		t.Fatalf("external search called %d times, want 1", searcher.count)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected merged and deduped candidates, got %+v", candidates)
	}
	ids := map[string]bool{}
	for _, candidate := range candidates {
		ids[candidate.VideoID] = true
		if candidate.VideoID == "wildcard" && candidate.Title == "Duplicate should be deduped" {
			t.Fatalf("expected cached enriched wildcard fields to be preserved, got %+v", candidate)
		}
	}
	if !ids["wildcard"] || !ids["remote"] {
		t.Fatalf("expected local and remote candidates, got %+v", candidates)
	}
}

func TestServiceFindCandidatesFallsBackToSQLiteStoreWhenExternalSearchFails(t *testing.T) {
	store, err := NewSQLiteCandidateStore(filepath.Join(t.TempDir(), "discovery.sqlite"))
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore error: %v", err)
	}
	defer store.Close()

	err = store.SaveSearchResults(context.Background(), GameRef{GameID: "196612180mia"}, "Houston Oilers vs Miami Dolphins 1966", "rod", []Candidate{
		{
			VideoID:     "cached",
			URL:         "https://www.youtube.com/watch?v=cached",
			Title:       "Houston Oilers vs Miami Dolphins 1966 Full Game",
			DurationSec: 4000,
		},
	})
	if err != nil {
		t.Fatalf("seed sqlite store: %v", err)
	}

	service := Service{
		Searcher:       errSearcher{},
		Scorer:         NewScorer(300, 60),
		CandidateStore: store,
	}
	game := GameRef{GameID: "196612180mia", Year: 1966, AwayTeam: "Houston Oilers", HomeTeam: "Miami Dolphins"}
	candidates, err := service.findCandidates(context.Background(), game, "Houston Oilers vs Miami Dolphins 1966", 10, 300)
	if err != nil {
		t.Fatalf("findCandidates should return cached candidates when external search fails: %v", err)
	}
	if len(candidates) != 1 || candidates[0].VideoID != "cached" {
		t.Fatalf("expected cached fallback candidate, got %+v", candidates)
	}
}
