package youtube

import (
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

var penaltyWords = []string{
	"highlights", "highlight", "shorts", "short", "clip", "clips", "recap", "reaction", "preview",
}

func MatchesCriteria(v youtubeapi.VideoCandidate, criteria Criteria, minDur time.Duration) bool {
	if !v.Embeddable {
		return false
	}
	if minDur > 0 && v.Duration < minDur {
		return false
	}
	if !containsAnyToken(v, requiredTokens(criteria.HomeTeam)) {
		return false
	}
	if !containsAnyToken(v, requiredTokens(criteria.VisitorTeam)) {
		return false
	}
	policy := videos.DefaultClassificationPolicy()
	if minDur > 0 {
		policy.MinDurationSec = int(minDur.Seconds())
	}
	classification := videos.ClassifyVideoForGame(gameMetadataFromCriteria(criteria), v, policy)
	return classification.Decision != videos.DecisionReject
}

func ScoreVideo(v youtubeapi.VideoCandidate, criteria Criteria) int {
	combined := strings.ToLower(v.Title + " " + v.Description)
	score := 0

	homeTokens := requiredTokens(criteria.HomeTeam)
	visitorTokens := requiredTokens(criteria.VisitorTeam)

	if containsAny(combined, homeTokens) {
		score += 5
	}
	if containsAny(combined, visitorTokens) {
		score += 5
	}
	if containsAny(combined, homeTokens) && containsAny(combined, visitorTokens) {
		score += 10
	}

	if criteria.DateISO != "" && strings.Contains(combined, strings.ToLower(criteria.DateISO)) {
		score += 12
	} else {
		year := criteria.DateISO
		if len(year) >= 4 {
			year = year[:4]
		}
		if len(year) == 4 && strings.Contains(combined, year) {
			score += 6
		}
	}

	durationMinutes := int(v.Duration / time.Minute)
	score += minInt(durationMinutes/15, 5)

	for _, word := range penaltyWords {
		if strings.Contains(combined, word) {
			score -= 4
		}
	}

	return score
}

func SortCandidates(candidates []youtubeapi.VideoCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].Duration == candidates[j].Duration {
				return strings.ToLower(candidates[i].Title) < strings.ToLower(candidates[j].Title)
			}
			return candidates[i].Duration > candidates[j].Duration
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func containsAnyToken(v youtubeapi.VideoCandidate, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	combined := strings.ToLower(v.Title + " " + v.Description)
	return containsAny(combined, tokens)
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if token != "" && strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func gameMetadataFromCriteria(criteria Criteria) videos.GameMetadata {
	meta := videos.GameMetadata{
		DateISO: criteria.DateISO,
		Home:    videos.LookupTeam(criteria.HomeTeam),
		Away:    videos.LookupTeam(criteria.VisitorTeam),
	}
	if len(criteria.DateISO) >= 4 {
		meta.Year = atoiYear(criteria.DateISO[:4])
	}
	if len(criteria.DateISO) == 10 {
		meta.GameDate, _ = time.Parse("2006-01-02", criteria.DateISO)
	}
	return meta
}

func atoiYear(value string) int {
	year := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		year = year*10 + int(r-'0')
	}
	return year
}
