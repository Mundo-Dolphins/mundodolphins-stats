package footballdb

import (
	"fmt"
	"strconv"
)

// LeagueForSeason returns the league ("afl" or "nfl") for a given season year.
// AFL seasons: 1966-1969
// NFL seasons: 1970+
func LeagueForSeason(season int) string {
	if season >= 1966 && season <= 1969 {
		return "afl"
	}
	return "nfl"
}

// SeasonFromGameID extracts the season year from a game ID.
// Game ID format: YYYYMMDDGGG where YYYY is the year (first 4 digits).
// Must be at least 8 characters (YYYYMMDD).
// Example: "2025090703" -> 2025, "1967091501" -> 1967
func SeasonFromGameID(gameID string) (int, error) {
	if len(gameID) < 8 {
		return 0, fmt.Errorf("invalid game ID format: %q (too short, minimum 8 characters)", gameID)
	}
	yearStr := gameID[:4]
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return 0, fmt.Errorf("invalid year in game ID %q: %w", gameID, err)
	}
	return year, nil
}

// LeagueForGameID returns the league ("afl" or "nfl") inferred from a game ID.
// Uses SeasonFromGameID to extract the year, then applies LeagueForSeason.
func LeagueForGameID(gameID string) (string, error) {
	season, err := SeasonFromGameID(gameID)
	if err != nil {
		return "", err
	}
	return LeagueForSeason(season), nil
}

// TeamLeagueSegment returns the league path segment for a given season.
// E.g., LeagueForSeason(1967) = "afl", so this returns "teams/afl"
// or LeagueForSeason(1970) = "nfl", so this returns "teams/nfl"
func TeamLeagueSegment(season int) string {
	league := LeagueForSeason(season)
	return fmt.Sprintf("teams/%s", league)
}

// TeamPathForSeason returns the full team path for a given season.
// Example: "teams/afl/miami-dolphins" for 1967, "teams/nfl/miami-dolphins" for 2025
func TeamPathForSeason(season int) string {
	return fmt.Sprintf("/%s/miami-dolphins", TeamLeagueSegment(season))
}

// InjuryTeamPathForSeason returns the team path for injuries URL.
// Injuries follow the same league pattern as other pages.
func InjuryTeamPathForSeason(season int) string {
	return TeamPathForSeason(season)
}

// InjuryTeamPathForGameID returns the team path for injuries URL based on a game ID.
// Uses LeagueForGameID to determine the league.
func InjuryTeamPathForGameID(gameID string) (string, error) {
	league, err := LeagueForGameID(gameID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/teams/%s/miami-dolphins", league), nil
}
