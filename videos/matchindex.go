package videos

import (
	"fmt"
	"sort"
	"strings"
)

// MatchIndex builds an in-memory index of all games for efficient video matching.
type MatchIndex struct {
	byID       map[string]GameRef   // gameID → GameRef
	byDate     map[string][]GameRef // "1966-09-02" → games on that date
	byYear     map[int][]GameRef    // 1966 → games in that year
	byTeamYear map[string][]GameRef // "dolphins-1966" → games
	allGameIDs []string             // sorted for determinism
}

// NewMatchIndex builds an index from a list of games.
func NewMatchIndex(games []GameRef) *MatchIndex {
	idx := &MatchIndex{
		byID:       make(map[string]GameRef, len(games)),
		byDate:     make(map[string][]GameRef),
		byYear:     make(map[int][]GameRef),
		byTeamYear: make(map[string][]GameRef),
		allGameIDs: make([]string, 0, len(games)),
	}

	for _, g := range games {
		idx.byID[g.GameID] = g
		idx.allGameIDs = append(idx.allGameIDs, g.GameID)

		dateKey := gameDateISO(g)
		if dateKey != "" && dateKey != "unknown" {
			idx.byDate[dateKey] = append(idx.byDate[dateKey], g)
		}

		if g.Year > 0 {
			idx.byYear[g.Year] = append(idx.byYear[g.Year], g)

			homeKey := teamYearKey(g.HomeTeam, g.Year)
			awayKey := teamYearKey(g.AwayTeam, g.Year)
			if homeKey != "" {
				idx.byTeamYear[homeKey] = append(idx.byTeamYear[homeKey], g)
			}
			if awayKey != "" {
				idx.byTeamYear[awayKey] = append(idx.byTeamYear[awayKey], g)
			}
		}
	}

	sort.Strings(idx.allGameIDs)
	return idx
}

// AllGameIDs returns all game IDs in sorted order.
func (idx *MatchIndex) AllGameIDs() []string {
	return idx.allGameIDs
}

// GameByID looks up a game by its ID.
func (idx *MatchIndex) GameByID(gameID string) (GameRef, bool) {
	g, ok := idx.byID[gameID]
	return g, ok
}

// GamesByDate returns all games on a specific date ("2006-01-02" format).
func (idx *MatchIndex) GamesByDate(dateISO string) []GameRef {
	return idx.byDate[dateISO]
}

// GamesByYear returns all games in a season year.
func (idx *MatchIndex) GamesByYear(year int) []GameRef {
	return idx.byYear[year]
}

// TotalGames returns the total number of indexed games.
func (idx *MatchIndex) TotalGames() int {
	return len(idx.byID)
}

// GamesNeedingVideo returns game IDs that don't have a video assigned in the cache.
func (idx *MatchIndex) GamesNeedingVideo(cf *CacheFile) []string {
	need := make([]string, 0)
	for _, id := range idx.allGameIDs {
		item := cf.Items[id]
		if item == nil {
			need = append(need, id)
			continue
		}
		if IsPublishedCacheItem(item) || (item.Status == StatusCandidate && len(item.Videos) > 0) || item.Status == StatusNone || item.Status == StatusRejected {
			continue
		}
		if IsPublishableStatus(item.Status) && len(item.Videos) == 0 {
			need = append(need, id)
			continue
		}
		if !IsDiscoveryCompleteStatus(item.Status) {
			need = append(need, id)
		}
	}
	return need
}

// MatchVideoToGame tries to find which game a video corresponds to.
// Returns the best matching game and a confidence score.
// This is the "reverse lookup" — given a video from a channel, find which game it covers.
func (idx *MatchIndex) MatchVideoToGame(title, description string, minScore int) (GameRef, int, bool) {
	combined := title + " " + description

	// 1. Try to extract a date from the title/description
	dates := CountDistinctDates(combined)

	var bestGame GameRef
	bestScore := 0
	found := false

	// If we found exactly one date, try to match by date
	if len(dates) == 1 {
		dateISO := dates[0]
		games := idx.GamesByDate(dateISO)
		for _, g := range games {
			score := scoreVideoMatchForGame(combined, g)
			if score > bestScore {
				bestScore = score
				bestGame = g
				found = true
			}
		}
	}

	// 2. If no single-date match, try title-based matching across all games
	if !found || bestScore < minScore {
		// Extract year from title
		yearRe := datePattern // reuse from multigame.go
		yearMatches := yearRe.FindAllStringSubmatch(combined, 1)
		var yearHint int
		if len(yearMatches) > 0 {
			_, _ = fmt.Sscanf(yearMatches[0][1], "%d", &yearHint)
		}

		// Narrow search by year if we have one
		searchGames := idx.allGameIDs
		if yearHint > 0 {
			yearGames := idx.GamesByYear(yearHint)
			if len(yearGames) > 0 {
				searchGames = make([]string, 0, len(yearGames))
				for _, g := range yearGames {
					searchGames = append(searchGames, g.GameID)
				}
			}
		}

		for _, gameID := range searchGames {
			g := idx.byID[gameID]
			score := scoreVideoMatchForGame(combined, g)
			if score > bestScore {
				bestScore = score
				bestGame = g
				found = true
			}
		}
	}

	if !found || bestScore < minScore {
		return GameRef{}, 0, false
	}

	return bestGame, bestScore, true
}

// scoreVideoMatchForGame gives a quick score for how well a video matches a game.
// This is a lightweight version of the full ScoreCandidate — just for matching.
func scoreVideoMatchForGame(text string, game GameRef) int {
	home := LookupTeam(game.HomeTeam)
	away := LookupTeam(game.AwayTeam)

	if home.FullName == "" || away.FullName == "" {
		return 0
	}

	score := 0

	// Both teams mentioned
	hasHome := ContainsTeamReference(text, home)
	hasAway := ContainsTeamReference(text, away)

	if hasHome && hasAway {
		score += 50
	} else if hasHome || hasAway {
		score += 10
	} else {
		return 0 // No team match at all
	}

	// Date match
	dateISO := gameDateISO(game)
	if dateISO != "" && dateISO != "unknown" && strings.Contains(text, dateISO) {
		score += 30
	} else if game.Year > 0 && strings.Contains(text, fmt.Sprintf("%d", game.Year)) {
		score += 10
	}

	return score
}

func teamYearKey(teamName string, year int) string {
	norm := strings.ToLower(strings.TrimSpace(teamName))
	if norm == "" {
		return ""
	}
	return fmt.Sprintf("%s-%d", norm, year)
}
