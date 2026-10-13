package footballdb

import (
	"testing"
)

func TestLeagueForSeason(t *testing.T) {
	tests := []struct {
		season int
		want   string
	}{
		{1966, "afl"},
		{1967, "afl"},
		{1968, "afl"},
		{1969, "afl"},
		{1970, "nfl"},
		{1971, "nfl"},
		{1988, "nfl"},
		{2023, "nfl"},
		{2025, "nfl"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.season)), func(t *testing.T) {
			got := LeagueForSeason(tt.season)
			if got != tt.want {
				t.Errorf("LeagueForSeason(%d) = %q, want %q", tt.season, got, tt.want)
			}
		})
	}
}

func TestSeasonFromGameID(t *testing.T) {
	tests := []struct {
		gameID  string
		want    int
		wantErr bool
	}{
		{"2025090703", 2025, false},
		{"1967091501", 1967, false},
		{"1970110101", 1970, false},
		{"202501", 0, true},    // too short
		{"abcd90703", 0, true}, // non-numeric year
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.gameID, func(t *testing.T) {
			got, err := SeasonFromGameID(tt.gameID)
			if (err != nil) != tt.wantErr {
				t.Errorf("SeasonFromGameID(%q) error = %v, wantErr %v", tt.gameID, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SeasonFromGameID(%q) = %d, want %d", tt.gameID, got, tt.want)
			}
		})
	}
}

func TestLeagueForGameID(t *testing.T) {
	tests := []struct {
		gameID  string
		want    string
		wantErr bool
	}{
		{"2025090703", "nfl", false},
		{"1967091501", "afl", false},
		{"1970110101", "nfl", false},
		{"1969123101", "afl", false},
		{"abcd90703", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.gameID, func(t *testing.T) {
			got, err := LeagueForGameID(tt.gameID)
			if (err != nil) != tt.wantErr {
				t.Errorf("LeagueForGameID(%q) error = %v, wantErr %v", tt.gameID, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LeagueForGameID(%q) = %q, want %q", tt.gameID, got, tt.want)
			}
		})
	}
}

func TestTeamLeagueSegment(t *testing.T) {
	tests := []struct {
		season int
		want   string
	}{
		{1967, "teams/afl"},
		{1970, "teams/nfl"},
		{2025, "teams/nfl"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.season)), func(t *testing.T) {
			got := TeamLeagueSegment(tt.season)
			if got != tt.want {
				t.Errorf("TeamLeagueSegment(%d) = %q, want %q", tt.season, got, tt.want)
			}
		})
	}
}

func TestTeamPathForSeason(t *testing.T) {
	tests := []struct {
		season int
		want   string
	}{
		{1967, "/teams/afl/miami-dolphins"},
		{1970, "/teams/nfl/miami-dolphins"},
		{2025, "/teams/nfl/miami-dolphins"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.season)), func(t *testing.T) {
			got := TeamPathForSeason(tt.season)
			if got != tt.want {
				t.Errorf("TeamPathForSeason(%d) = %q, want %q", tt.season, got, tt.want)
			}
		})
	}
}

func TestInjuryTeamPathForSeason(t *testing.T) {
	tests := []struct {
		season int
		want   string
	}{
		{1967, "/teams/afl/miami-dolphins"},
		{1970, "/teams/nfl/miami-dolphins"},
		{2025, "/teams/nfl/miami-dolphins"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.season)), func(t *testing.T) {
			got := InjuryTeamPathForSeason(tt.season)
			if got != tt.want {
				t.Errorf("InjuryTeamPathForSeason(%d) = %q, want %q", tt.season, got, tt.want)
			}
		})
	}
}

func TestInjuryTeamPathForGameID(t *testing.T) {
	tests := []struct {
		gameID  string
		want    string
		wantErr bool
	}{
		{"1967091501", "/teams/afl/miami-dolphins", false},
		{"2025090703", "/teams/nfl/miami-dolphins", false},
		{"1970110101", "/teams/nfl/miami-dolphins", false},
		{"abcd90703", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.gameID, func(t *testing.T) {
			got, err := InjuryTeamPathForGameID(tt.gameID)
			if (err != nil) != tt.wantErr {
				t.Errorf("InjuryTeamPathForGameID(%q) error = %v, wantErr %v", tt.gameID, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("InjuryTeamPathForGameID(%q) = %q, want %q", tt.gameID, got, tt.want)
			}
		})
	}
}
