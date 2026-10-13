package videos

import (
	"testing"
	"time"
)

func TestBuildReviewQueueOrdersByRisk(t *testing.T) {
	cf := &CacheFile{Items: map[string]*CacheItem{
		"low-risk": {
			Status:      StatusCandidate,
			NeedsReview: true,
			UpdatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Videos:      []Video{{ID: "good", Title: "Bills vs Dolphins Full Game 2024-09-12", DurationSeconds: 7200}},
			AutoScoring: &AutoScoringMeta{
				ScoreBreakdown: ScoreBreakdown{Total: 140},
				MatchedTeams:   true,
				MatchedDate:    string(DateMatchFull),
				Embeddable:     true,
				Confidence:     "high",
			},
		},
		"high-risk": {
			Status:      StatusCandidate,
			NeedsReview: true,
			Query:       "preview",
			Videos:      []Video{{ID: "bad", Title: "Bills vs Dolphins Week Preview and Predictions", DurationSeconds: 500}},
			AutoScoring: &AutoScoringMeta{
				ScoreBreakdown: ScoreBreakdown{Total: 55},
				MatchedTeams:   true,
				MatchedDate:    string(DateMatchYearOnly),
				Embeddable:     false,
				Confidence:     "low",
			},
		},
		"approved": {
			Status: StatusApproved,
			Videos: []Video{{ID: "approved"}},
		},
	}}

	queue := BuildReviewQueue(cf)
	if len(queue.Items) != 2 {
		t.Fatalf("review items=%d want 2", len(queue.Items))
	}
	if queue.Items[0].GameID != "high-risk" {
		t.Fatalf("first review item=%q want high-risk", queue.Items[0].GameID)
	}
	if queue.Items[0].RiskScore <= queue.Items[1].RiskScore {
		t.Fatalf("expected high-risk item first: %+v", queue.Items)
	}
}

func TestComputeReviewRiskIncludesRiskyLanguage(t *testing.T) {
	item := ReviewItem{
		GameID:          "game",
		Status:          StatusCandidate,
		NeedsReview:     true,
		Title:           "Miami Dolphins Week Preview and Predictions",
		Score:           50,
		MatchedTeams:    true,
		MatchedDate:     string(DateMatchYearOnly),
		DurationSeconds: 300,
	}
	score, reasons := ComputeReviewRisk(item)
	if score < 100 {
		t.Fatalf("risk=%d want >=100, reasons=%v", score, reasons)
	}
	found := false
	for _, reason := range reasons {
		if reason == "risky language: week preview" || reason == "risky language: preview" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected risky language reason, got %v", reasons)
	}
}

func TestBuildReviewQueueIncludesSearchAlternativesRecommendation(t *testing.T) {
	cf := &CacheFile{Items: map[string]*CacheItem{
		"low-score": {
			Status:      StatusSelected,
			NeedsReview: true,
			Error:       "low watch-page score (15); reviewer should search alternatives",
			Videos:      []Video{{ID: "v1", Title: "Week preview", DurationSeconds: 420}},
			AutoScoring: &AutoScoringMeta{
				ScoreBreakdown: ScoreBreakdown{Total: 15},
				Confidence:     "low",
				NeedsReview:    true,
			},
		},
	}}
	queue := BuildReviewQueue(cf)
	if len(queue.Items) != 1 {
		t.Fatalf("review items=%d want 1", len(queue.Items))
	}
	if queue.Items[0].Recommendation == "" {
		t.Fatalf("expected recommendation, got %+v", queue.Items[0])
	}
	found := false
	for _, reason := range queue.Items[0].RiskReasons {
		if reason == "search alternatives recommended" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected search alternatives risk reason, got %v", queue.Items[0].RiskReasons)
	}
}
