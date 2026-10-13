package videos

import (
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestFilterByMinDuration(t *testing.T) {
	candidates := []Candidate{
		{Title: "Short", DurationSec: 120},
		{Title: "Zero", DurationSec: 0},
		{Title: "Exact", DurationSec: 300},
		{Title: "Long", DurationSec: 7200},
	}
	filtered := filterByMinDuration(candidates, 300)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(filtered))
	}
	if filtered[0].Title != "Exact" || filtered[1].Title != "Long" {
		t.Fatalf("unexpected filtered order: %s, %s", filtered[0].Title, filtered[1].Title)
	}
}

func TestFilterByMinDurationParsesDurationText(t *testing.T) {
	candidates := []Candidate{
		{Title: "Short text", DurationText: "5:02"},
		{Title: "Long text", DurationText: "2:16:52"},
	}

	filtered := filterByMinDuration(candidates, 600)
	if len(filtered) != 1 {
		t.Fatalf("expected one candidate, got %+v", filtered)
	}
	if filtered[0].Title != "Long text" || filtered[0].DurationSec != 8212 {
		t.Fatalf("unexpected candidate after parsing duration text: %+v", filtered[0])
	}
}

func newTestMeta() GameMetadata {
	return GameMetadata{
		GameID:   "196909140cin",
		DateISO:  "1969-09-14",
		GameDate: time.Date(1969, 9, 14, 0, 0, 0, 0, time.UTC),
		Year:     1969,
		Home:     LookupTeam("Cincinnati Bengals"),
		Away:     LookupTeam("Miami Dolphins"),
	}
}

func newValidCandidate() youtubeapi.VideoCandidate {
	return youtubeapi.VideoCandidate{
		ID:          "abc123",
		Title:       "Miami Dolphins vs Cincinnati Bengals 1969 Full Game",
		Description: "Dolphins at Bengals 1969 season opener",
		Duration:    90 * time.Minute,
		Embeddable:  true,
		ViewCount:   10000,
	}
}

func TestIsValidCandidate_AcceptsValidVideo(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	result := IsValidCandidate(c, meta, 600)
	if !result.Accepted {
		t.Errorf("expected accepted, got rejected: %s", result.Reason)
	}
}

func TestIsValidCandidate_BannedTitleWords(t *testing.T) {
	meta := newTestMeta()
	bannedWords := []string{
		"preview", "recap", "reaction", "analysis", "breakdown",
		"interview", "press conference", "podcast", "gmfb",
		"top 10", "top plays", "free agency", "mock draft",
		"fantasy", "betting", "prediction", "madden", "simulation",
	}
	for _, word := range bannedWords {
		c := newValidCandidate()
		c.Title = "Dolphins vs Bengals " + word + " 1969"
		result := IsValidCandidate(c, meta, 600)
		if result.Accepted {
			t.Errorf("expected rejection for banned word %q in title", word)
		}
		if result.Reason == "" {
			t.Errorf("expected reason for banned word %q", word)
		}
	}
}

func TestIsValidCandidate_BannedDescriptionWords(t *testing.T) {
	meta := newTestMeta()
	bannedWords := []string{
		"press conference", "podcast", "gmfb", "good morning football", "fantasy football",
	}
	for _, word := range bannedWords {
		c := newValidCandidate()
		c.Description = "Dolphins at Bengals 1969 " + word
		result := IsValidCandidate(c, meta, 600)
		if result.Accepted {
			t.Errorf("expected rejection for banned word %q in description", word)
		}
	}
}

func TestIsValidCandidate_DurationTooShort(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Duration = 5 * time.Minute // 300s < 600s minimum
	result := IsValidCandidate(c, meta, 600)
	if result.Accepted {
		t.Error("expected rejection for short duration")
	}
}

func TestIsValidCandidate_DurationFloor600(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Duration = 5 * time.Minute // 300s
	// Even if caller passes minDuration=100, hard floor is 600s
	result := IsValidCandidate(c, meta, 100)
	if result.Accepted {
		t.Error("expected rejection: 300s < 600s hard floor")
	}
}

func TestIsValidCandidate_DurationZeroAccepted(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Duration = 0 // unknown duration = don't reject
	result := IsValidCandidate(c, meta, 600)
	if !result.Accepted {
		t.Errorf("expected zero-duration to be accepted (unknown), got: %s", result.Reason)
	}
}

func TestIsValidCandidate_MissingTeams(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Title = "NFL Game 1969 Full Game"
	c.Description = "A football game from 1969"
	result := IsValidCandidate(c, meta, 600)
	if result.Accepted {
		t.Error("expected rejection when both teams not mentioned")
	}
}

func TestIsValidCandidate_WrongYear(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Title = "Miami Dolphins vs Cincinnati Bengals 2020 Full Game"
	c.Description = "Dolphins at Bengals 2020"
	result := IsValidCandidate(c, meta, 600)
	if result.Accepted {
		t.Error("expected rejection for wrong year")
	}
}

func TestIsValidCandidate_MultiMatchupPatterns(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Title = "Dolphins vs Bengals vs Bills vs Jets 1969"
	c.Description = ""
	result := IsValidCandidate(c, meta, 600)
	if result.Accepted {
		t.Error("expected rejection for multiple vs patterns")
	}
}

func TestIsValidCandidate_MultiWeekReferences(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Title = "Dolphins vs Bengals Week 1 Week 2 Week 3 1969"
	c.Description = ""
	result := IsValidCandidate(c, meta, 600)
	if result.Accepted {
		t.Error("expected rejection for multiple week references")
	}
}

func TestIsValidCandidate_TooManyTeams(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Title = "Miami Dolphins vs Cincinnati Bengals vs Buffalo Bills 1969"
	c.Description = "Featuring the New York Jets"
	result := IsValidCandidate(c, meta, 600)
	if result.Accepted {
		t.Error("expected rejection for >2 distinct teams")
	}
}

func TestIsValidCandidate_ExactlyTwoTeamsAccepted(t *testing.T) {
	meta := newTestMeta()
	c := newValidCandidate()
	c.Title = "Miami Dolphins vs Cincinnati Bengals 1969"
	c.Description = "Complete game Dolphins Bengals"
	result := IsValidCandidate(c, meta, 600)
	if !result.Accepted {
		t.Errorf("expected accepted for exactly 2 teams, got: %s", result.Reason)
	}
}

func TestCountDistinctNFLTeams(t *testing.T) {
	meta := newTestMeta()
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "two teams",
			text: "miami dolphins vs cincinnati bengals",
			want: 2,
		},
		{
			name: "three teams",
			text: "miami dolphins vs cincinnati bengals and buffalo bills",
			want: 3,
		},
		{
			name: "no teams",
			text: "some random text about football",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDistinctNFLTeams(tt.text, meta)
			if got != tt.want {
				t.Errorf("countDistinctNFLTeams() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsBlockedChannel(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		want      bool
	}{
		{"NFL official", "UCDVYQ4Zhbm3S2dlz7P1GBDg", true},
		{"ESPN", "UCYTBCo_CVBXblEOalwGPJqg", true},
		{"Vintage Pro Football", "UCpKizgBSJOBYnfn5z-VjNZg", false},
		{"NFL Throwback", "UCJ0YXSM_M4mHiiVQJEX6JQ", false},
		{"random channel", "UCrandom123", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBlockedChannel(tt.channelID)
			if got != tt.want {
				t.Errorf("IsBlockedChannel(%q) = %v, want %v", tt.channelID, got, tt.want)
			}
		})
	}
}
