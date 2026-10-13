package util

import (
	"testing"
	"time"
)

func TestNFLSeasonFromDate(t *testing.T) {
	tests := []struct {
		name    string
		dateISO string
		want    int
	}{
		// January/February dates belong to previous season
		{
			name:    "Jan 2026 belongs to season 2025",
			dateISO: "2026-01-15T00:00:00Z",
			want:    2025,
		},
		{
			name:    "Feb 2026 belongs to season 2025",
			dateISO: "2026-02-01T00:00:00Z",
			want:    2025,
		},
		{
			name:    "Feb 10 2026 (before 2nd Tuesday) belongs to season 2025",
			dateISO: "2026-02-10T00:00:00Z",
			want:    2025,
		},

		// March dates - depends on second Tuesday
		// Second Tuesday of March 2026 is March 10
		{
			name:    "Mar 09 2026 (before 2nd Tuesday) belongs to season 2025",
			dateISO: "2026-03-09T00:00:00Z",
			want:    2025,
		},
		{
			name:    "Mar 10 2026 (2nd Tuesday) belongs to season 2026",
			dateISO: "2026-03-10T00:00:00Z",
			want:    2026,
		},
		{
			name:    "Mar 11 2026 (after 2nd Tuesday) belongs to season 2026",
			dateISO: "2026-03-11T00:00:00Z",
			want:    2026,
		},

		// Second Tuesday of March 2025 is March 11
		{
			name:    "Mar 10 2025 (before 2nd Tuesday) belongs to season 2024",
			dateISO: "2025-03-10T00:00:00Z",
			want:    2024,
		},
		{
			name:    "Mar 11 2025 (2nd Tuesday) belongs to season 2025",
			dateISO: "2025-03-11T00:00:00Z",
			want:    2025,
		},

		// Regular season dates
		{
			name:    "Dec 2025 belongs to season 2025",
			dateISO: "2025-12-21T00:00:00Z",
			want:    2025,
		},
		{
			name:    "Sep 2024 belongs to season 2024",
			dateISO: "2024-09-08T00:00:00Z",
			want:    2024,
		},

		// Historical dates
		{
			name:    "Sep 1966 belongs to season 1966",
			dateISO: "1966-09-02T00:00:00Z",
			want:    1966,
		},
		{
			name:    "Jan 1967 belongs to season 1966",
			dateISO: "1967-01-15T00:00:00Z",
			want:    1966,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NFLSeasonFromDate(tt.dateISO)
			if err != nil {
				t.Fatalf("NFLSeasonFromDate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("NFLSeasonFromDate() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNFLSeasonFromDateRejectsShortInput(t *testing.T) {
	if _, err := NFLSeasonFromDate("2026-1"); err == nil {
		t.Fatal("expected error for short date input")
	}
}

func TestNFLSeasonFromGameID(t *testing.T) {
	tests := []struct {
		name    string
		gameID  string
		want    int
		wantErr bool
	}{
		// Format: YYYYMMDD + opponent + abbr
		{
			name:    "2025 middle of season game",
			gameID:  "202512210mia",
			want:    2025,
			wantErr: false,
		},
		{
			name:    "Feb 2026 game belongs to season 2025",
			gameID:  "202602010mia",
			want:    2025,
			wantErr: false,
		},
		{
			name:    "Jan 2026 game belongs to season 2025",
			gameID:  "202601050mia",
			want:    2025,
			wantErr: false,
		},
		{
			name:    "Sep 1966 game belongs to season 1966",
			gameID:  "196609020mia",
			want:    1966,
			wantErr: false,
		},
		{
			name:    "Jan 1967 game belongs to season 1966",
			gameID:  "196701150mia",
			want:    1966,
			wantErr: false,
		},
		{
			name:    "Invalid gameID",
			gameID:  "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "Too short gameID",
			gameID:  "bad",
			want:    0,
			wantErr: true,
		},
		{
			name:    "Short valid gameID (fallback to year)",
			gameID:  "2025",
			want:    2025,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NFLSeasonFromGameID(tt.gameID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NFLSeasonFromGameID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NFLSeasonFromGameID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetSecondTuesdayOfMarch(t *testing.T) {
	tests := []struct {
		year int
		want time.Time
	}{
		{2026, time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)},
		{2025, time.Date(2025, time.March, 11, 0, 0, 0, 0, time.UTC)},
		{2024, time.Date(2024, time.March, 12, 0, 0, 0, 0, time.UTC)},
		{1966, time.Date(1966, time.March, 8, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		actual := getSecondTuesdayOfMarch(tt.year)
		if actual != tt.want {
			t.Errorf("getSecondTuesdayOfMarch(%d) = %v, want %v", tt.year, actual, tt.want)
		}
		// Verify it's actually a Tuesday
		if actual.Weekday() != time.Tuesday {
			t.Errorf("getSecondTuesdayOfMarch(%d) returned %v, not a Tuesday", tt.year, actual)
		}
	}
}
