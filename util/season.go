package util

import (
	"fmt"
	"strconv"
	"time"
)

// NFLSeasonFromDate returns the NFL season for a given date.
// NFL season runs from second Tuesday of March to second Tuesday of March next year.
//
// Examples:
//   - 2025-03-11 (second Tuesday of March 2025) → Season 2025
//   - 2025-03-10 → Season 2024
//   - 2026-02-01 → Season 2025 (before second Tuesday of March 2026)
//   - 2026-03-10 (second Tuesday of March 2026) → Season 2026
func NFLSeasonFromDate(dateISO string) (int, error) {
	if len(dateISO) < len("2006-01-02") {
		return 0, fmt.Errorf("invalid date %q", dateISO)
	}

	// Parse ISO format: "2025-12-21T00:00:00Z" or "2025-12-21"
	t, err := time.Parse("2006-01-02", dateISO[:10])
	if err != nil {
		return 0, fmt.Errorf("parse date %q: %w", dateISO, err)
	}

	year := t.Year()
	month := t.Month()

	// If before March, belongs to previous season
	if month < 3 {
		return year - 1, nil
	}

	// If March, check if before or after second Tuesday
	if month == 3 {
		secondTuesday := getSecondTuesdayOfMarch(year)
		if t.Before(secondTuesday) {
			return year - 1, nil
		}
	}

	// March or later, and on/after second Tuesday → current year season
	return year, nil
}

// NFLSeasonFromGameID extracts season from gameID.
// gameID format: "202512210mia" = YYYYMMDD + opponent + abbr
func NFLSeasonFromGameID(gameID string) (int, error) {
	if len(gameID) < 8 {
		if len(gameID) < 4 {
			return 0, fmt.Errorf("invalid gameID %q", gameID)
		}
		// Fallback: use year from gameID
		year, err := strconv.Atoi(gameID[:4])
		if err != nil {
			return 0, fmt.Errorf("invalid gameID %q", gameID)
		}
		return year, nil
	}

	// Parse YYYYMMDD from gameID
	dateStr := gameID[:8]
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return 0, fmt.Errorf("parse gameID date %q: %w", dateStr, err)
	}

	return NFLSeasonFromDate(t.Format("2006-01-02"))
}

// getSecondTuesdayOfMarch calculates the second Tuesday of March for a given year
func getSecondTuesdayOfMarch(year int) time.Time {
	// Start at March 1st
	date := time.Date(year, time.March, 1, 0, 0, 0, 0, time.UTC)

	// Find first Tuesday
	for date.Weekday() != time.Tuesday {
		date = date.AddDate(0, 0, 1)
	}

	// Add 7 days to get second Tuesday
	date = date.AddDate(0, 0, 7)
	return date
}
