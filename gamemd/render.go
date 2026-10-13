package gamemd

import (
	"strconv"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
)

// getNFLSeasonStartDate returns the date when the NFL season starts for a given year.
// The season starts on the second Tuesday of March.
func getNFLSeasonStartDate(year int) time.Time {
	date := time.Date(year, time.March, 1, 0, 0, 0, 0, time.UTC)
	for date.Weekday() != time.Tuesday {
		date = date.AddDate(0, 0, 1)
	}
	return date.AddDate(0, 0, 7)
}

// GetNFLSeasonYear calculates the NFL season year for a given date.
// If the date is before the second Tuesday of March, it belongs to the previous season.
// This is exported for use in filtering and classification logic.
func GetNFLSeasonYear(dateISO string) int {
	return getNFLSeasonYear(dateISO)
}

// GetNFLSeasonYearFromGameID extracts the NFL season year from a game ID (YYYYMMDD format).
// For example, 202601040mia played on 2026-01-04 belongs to season 2025.
func GetNFLSeasonYearFromGameID(gameID string) (int, error) {
	return util.NFLSeasonFromGameID(gameID)
}

// getNFLSeasonYear returns the NFL season year for a given date.
// If the date is before the second Tuesday of March, it belongs to the previous season.
func getNFLSeasonYear(dateISO string) int {
	if len(dateISO) < 10 {
		if len(dateISO) < 4 {
			return 0
		}
		year, err := strconv.Atoi(dateISO[:4])
		if err != nil {
			return 0
		}
		return year
	}

	season, err := util.NFLSeasonFromDate(dateISO)
	if err != nil {
		return 0
	}
	return season
}
