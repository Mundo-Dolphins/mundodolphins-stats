package gamemd

import (
	"testing"
)

func TestGetNFLSeasonYear(t *testing.T) {
	tests := []struct {
		name      string
		dateISO   string
		wantYear  int
		wantValid bool
	}{
		// January and February dates belong to previous season
		{"January 2026", "2026-01-04", 2025, true},
		{"February 2026", "2026-02-09", 2025, true},
		{"January 1970", "1970-01-11", 1969, true},

		// March dates before 2nd Tuesday belong to previous season
		{"March 4 2026 (before 2nd Tue)", "2026-03-04", 2025, true},
		{"March 9 2026 (before 2nd Tue)", "2026-03-09", 2025, true},

		// March dates on or after 2nd Tuesday belong to current season
		{"March 10 2026 (2nd Tue)", "2026-03-10", 2026, true},
		{"March 15 2026 (after 2nd Tue)", "2026-03-15", 2026, true},

		// April and later belong to current season
		{"April 2026", "2026-04-05", 2026, true},
		{"September 2025", "2025-09-07", 2025, true},
		{"December 2025", "2025-12-28", 2025, true},

		// Edge cases
		{"Invalid date", "not-a-date", 0, true}, // Fallback returns 0
		{"Short date", "2026", 2026, true},      // Fallback extracts year
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNFLSeasonYear(tt.dateISO)
			if got != tt.wantYear {
				t.Errorf("getNFLSeasonYear(%q) = %d, want %d", tt.dateISO, got, tt.wantYear)
			}
		})
	}
}

func TestGetNFLSeasonYearFromGameID(t *testing.T) {
	tests := []struct {
		name      string
		gameID    string
		wantYear  int
		wantError bool
	}{
		// Games in January 2026 belong to season 2025
		{"202601040mia", "202601040mia", 2025, false},
		{"202602090mia", "202602090mia", 2025, false},

		// Games after March 10 2026 belong to season 2026
		{"202603100mia", "202603100mia", 2026, false},
		{"202603150mia", "202603150mia", 2026, false},
		{"202604050mia", "202604050mia", 2026, false},

		// Games in March before 2nd Tuesday 2026 belong to season 2025
		{"202603090mia", "202603090mia", 2025, false},

		// Invalid formats
		{"short", "short", 0, true},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNFLSeasonYearFromGameID(tt.gameID)
			if (err != nil) != tt.wantError {
				t.Errorf("GetNFLSeasonYearFromGameID error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.wantYear {
				t.Errorf("GetNFLSeasonYearFromGameID(%q) = %d, want %d", tt.gameID, got, tt.wantYear)
			}
		})
	}
}

func TestGetNFLSeasonStartDate(t *testing.T) {
	tests := []struct {
		year      int
		wantMonth int
		wantDay   int
	}{
		{2026, 3, 10}, // 2nd Tuesday of March 2026
		{2025, 3, 11}, // 2nd Tuesday of March 2025
		{2024, 3, 12}, // 2nd Tuesday of March 2024
		{2023, 3, 14}, // 2nd Tuesday of March 2023
		{1966, 3, 8},  // 2nd Tuesday of March 1966
	}

	for _, tt := range tests {
		t.Run("Year "+string(rune(tt.year)), func(t *testing.T) {
			got := getNFLSeasonStartDate(tt.year)
			if got.Month() != 3 || got.Day() != tt.wantDay {
				t.Errorf("getNFLSeasonStartDate(%d) = %d-%02d, want 3-%02d", tt.year, got.Month(), got.Day(), tt.wantDay)
			}
		})
	}
}

func TestBuildSlug(t *testing.T) {
	tests := []struct {
		name    string
		visitor string
		home    string
		dateISO string
		want    string
	}{
		{"known teams", "Miami Dolphins", "Buffalo Bills", "2025-09-18", "mia-buf-20250918"},
		{"historical raiders", "Oakland Raiders", "Miami Dolphins", "1972-01-02", "oak-mia-19720102"},
		{"fallback team", "Unknown Team", "Miami Dolphins", "2024-10-01", "unk-mia-20241001"},
		{"short fallback", "X", "Y", "2024-10-01", "xxx-xxx-20241001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSlug(tt.visitor, tt.home, tt.dateISO)
			if got != tt.want {
				t.Errorf("BuildSlug(%q, %q, %q) = %q, want %q", tt.visitor, tt.home, tt.dateISO, got, tt.want)
			}
		})
	}
}
