package youtube

import (
	"fmt"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

type Criteria struct {
	HomeTeam    string
	VisitorTeam string
	DateISO     string
}

func BuildQuery(c Criteria) string {
	parts := []string{strings.TrimSpace(c.VisitorTeam), "vs", strings.TrimSpace(c.HomeTeam)}
	if strings.TrimSpace(c.DateISO) != "" {
		parts = append(parts, strings.TrimSpace(c.DateISO))
	}
	return strings.Join(parts, " ")
}

func CriteriaFromGame(game videos.GameRef) Criteria {
	return Criteria{
		HomeTeam:    strings.TrimSpace(game.HomeTeam),
		VisitorTeam: strings.TrimSpace(game.AwayTeam),
		DateISO:     dateISOFromGameID(game.GameID),
	}
}

func dateISOFromGameID(gameID string) string {
	if len(gameID) < 8 {
		return ""
	}
	raw := gameID[:8]
	for _, r := range raw {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return fmt.Sprintf("%s-%s-%s", raw[:4], raw[4:6], raw[6:8])
}

func requiredTokens(team string) []string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(team)))
	if len(parts) == 0 {
		return nil
	}
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) < 3 {
			continue
		}
		tokens = append(tokens, part)
	}
	return tokens
}
