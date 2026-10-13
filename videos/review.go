package videos

import (
	"sort"
	"strings"
	"time"
)

type ReviewQueue struct {
	Items []ReviewItem `json:"items"`
}

type ReviewItem struct {
	GameID          string    `json:"game_id"`
	Status          string    `json:"status"`
	NeedsReview     bool      `json:"needs_review"`
	RiskScore       int       `json:"risk_score"`
	RiskReasons     []string  `json:"risk_reasons"`
	Confidence      string    `json:"confidence"`
	Score           int       `json:"score"`
	MatchedTeams    bool      `json:"matched_teams"`
	MatchedDate     string    `json:"matched_date_type"`
	Embeddable      bool      `json:"embeddable"`
	ViewCount       uint64    `json:"view_count"`
	DurationSeconds int       `json:"duration_seconds"`
	Query           string    `json:"query"`
	UpdatedAt       time.Time `json:"updated_at"`
	VideoID         string    `json:"video_id,omitempty"`
	Title           string    `json:"title,omitempty"`
	Channel         string    `json:"channel,omitempty"`
	URL             string    `json:"url,omitempty"`
	Recommendation  string    `json:"recommendation,omitempty"`
}

func BuildReviewQueue(cf *CacheFile) ReviewQueue {
	if cf == nil || cf.Items == nil {
		return ReviewQueue{Items: []ReviewItem{}}
	}
	items := make([]ReviewItem, 0)
	for gameID, item := range cf.Items {
		if !IsReviewableCacheItem(item) {
			continue
		}
		review := buildReviewItem(gameID, item)
		items = append(items, review)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RiskScore != items[j].RiskScore {
			return items[i].RiskScore > items[j].RiskScore
		}
		if items[i].Score != items[j].Score {
			return items[i].Score < items[j].Score
		}
		return items[i].GameID < items[j].GameID
	})
	return ReviewQueue{Items: items}
}

func buildReviewItem(gameID string, item *CacheItem) ReviewItem {
	review := ReviewItem{
		GameID:      gameID,
		Status:      item.Status,
		NeedsReview: item.NeedsReview,
		Query:       item.Query,
		UpdatedAt:   item.UpdatedAt,
		Confidence:  "unknown",
	}
	if len(item.Videos) > 0 {
		v := item.Videos[0]
		review.VideoID = v.ID
		review.Title = v.Title
		review.Channel = v.Channel
		review.URL = v.URL
		review.DurationSeconds = v.DurationSeconds
	}
	if item.AutoScoring != nil {
		review.Score = item.AutoScoring.ScoreBreakdown.Total
		review.MatchedTeams = item.AutoScoring.MatchedTeams
		review.MatchedDate = item.AutoScoring.MatchedDate
		review.Embeddable = item.AutoScoring.Embeddable
		review.ViewCount = item.AutoScoring.ViewCount
		review.Confidence = item.AutoScoring.Confidence
		if review.Confidence == "" {
			review.Confidence = confidenceFromScore(review.Score)
		}
	}
	if strings.Contains(strings.ToLower(item.Error), "search alternatives") {
		review.Recommendation = "Search alternative videos before approving this cache entry."
	}
	review.RiskScore, review.RiskReasons = ComputeReviewRisk(review)
	return review
}

func ComputeReviewRisk(item ReviewItem) (int, []string) {
	score := 0
	reasons := []string{}
	if item.Status == StatusCandidate || item.NeedsReview {
		score += 30
		reasons = append(reasons, "automatic candidate")
	}
	if item.Score > 0 && item.Score < 95 {
		score += 25
		reasons = append(reasons, "low score")
	}
	if item.Recommendation != "" {
		score += 25
		reasons = append(reasons, "search alternatives recommended")
	}
	if item.Score == 0 {
		score += 20
		reasons = append(reasons, "missing score")
	}
	if item.MatchedDate != "" && item.MatchedDate != string(DateMatchFull) {
		score += 20
		reasons = append(reasons, "date not exact")
	}
	if !item.MatchedTeams {
		score += 20
		reasons = append(reasons, "teams not confirmed")
	}
	if !item.Embeddable {
		score += 10
		reasons = append(reasons, "not embeddable")
	}
	if item.DurationSeconds > 0 && item.DurationSeconds < 600 {
		score += 20
		reasons = append(reasons, "short duration")
	}
	if risk := riskyLanguageReason(item.Title + " " + item.Query); risk != "" {
		score += 40
		reasons = append(reasons, risk)
	}
	if len(item.RiskReasons) > 0 {
		reasons = append(reasons, item.RiskReasons...)
	}
	return score, dedupeStrings(reasons)
}

func riskyLanguageReason(text string) string {
	norm := NormalizeText(text)
	for _, token := range []string{
		"week preview",
		"preview",
		"prediction",
		"predictions",
		"betting",
		"odds",
		"simulation",
		"madden",
		"podcast",
		"press conference",
	} {
		if strings.Contains(norm, NormalizeText(token)) {
			return "risky language: " + token
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
