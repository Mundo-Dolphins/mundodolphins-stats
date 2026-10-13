package videos

import (
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestClassifyVideoForGame(t *testing.T) {
	game := GameMetadata{
		GameID:   "202409120mia",
		DateISO:  "2024-09-12",
		GameDate: time.Date(2024, 9, 12, 0, 0, 0, 0, time.UTC),
		Year:     2024,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Buffalo Bills"),
	}

	tests := []struct {
		name       string
		candidate  youtubeapi.VideoCandidate
		policy     ClassificationPolicy
		want       ClassificationDecision
		wantReason string
	}{
		{
			name: "clear full game with exact date is accepted",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Full Game 2024-09-12",
				Description: "Complete NFL game broadcast from September 12, 2024",
				Channel:     "NFL Throwback",
				Duration:    2*time.Hour + 20*time.Minute,
				Embeddable:  true,
			},
			policy: DefaultClassificationPolicy(),
			want:   DecisionAccept,
		},
		{
			name: "preview is rejected",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Week Preview and Predictions",
				Description: "Betting odds and matchup preview for Thursday night",
				Duration:    30 * time.Minute,
				Embeddable:  true,
			},
			policy:     DefaultClassificationPolicy(),
			want:       DecisionReject,
			wantReason: "preview",
		},
		{
			name: "madden simulation is rejected",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Full Game 2024-09-12 Madden Simulation",
				Description: "CPU vs CPU simulated game",
				Duration:    2 * time.Hour,
				Embeddable:  true,
			},
			policy:     DefaultClassificationPolicy(),
			want:       DecisionReject,
			wantReason: "madden",
		},
		{
			name: "generic video without both teams is rejected",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Miami Dolphins 2024 Season Highlights",
				Description: "Best plays from the season",
				Duration:    45 * time.Minute,
				Embeddable:  true,
			},
			policy:     DefaultClassificationPolicy(),
			want:       DecisionReject,
			wantReason: "team",
		},
		{
			name: "wrong year is rejected",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Full Game 2023-09-12",
				Description: "Complete NFL game broadcast",
				Duration:    2 * time.Hour,
				Embeddable:  true,
			},
			policy:     DefaultClassificationPolicy(),
			want:       DecisionReject,
			wantReason: "wrong year",
		},
		{
			name: "year only full game goes to review",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Full Game 2024",
				Description: "Complete NFL broadcast",
				Duration:    2 * time.Hour,
				Embeddable:  true,
			},
			policy: func() ClassificationPolicy {
				policy := DefaultClassificationPolicy()
				policy.RequireDateForAccept = true
				return policy
			}(),
			want:       DecisionReview,
			wantReason: "review",
		},
		{
			name: "highlights default to review",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Highlights 2024-09-12",
				Description: "NFL game highlights",
				Duration:    35 * time.Minute,
				Embeddable:  true,
			},
			policy:     DefaultClassificationPolicy(),
			want:       DecisionReview,
			wantReason: "review",
		},
		{
			name: "highlights can be accepted when policy allows them",
			candidate: youtubeapi.VideoCandidate{
				Title:       "Buffalo Bills vs Miami Dolphins Highlights 2024-09-12",
				Description: "NFL game highlights",
				Duration:    35 * time.Minute,
				Embeddable:  true,
			},
			policy: func() ClassificationPolicy {
				policy := DefaultClassificationPolicy()
				policy.AllowHighlights = true
				policy.AcceptThreshold = 75
				return policy
			}(),
			want: DecisionAccept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyVideoForGame(game, tt.candidate, tt.policy)
			if got.Decision != tt.want {
				t.Fatalf("decision=%s want=%s result=%+v", got.Decision, tt.want, got)
			}
			if got.Score == 0 && tt.want != DecisionReject {
				t.Fatalf("expected explanatory score, got 0")
			}
			if len(got.Evidence) == 0 {
				t.Fatalf("expected evidence")
			}
			if tt.wantReason != "" && !containsReason(got.Reasons, tt.wantReason) {
				t.Fatalf("expected reason containing %q, got %v", tt.wantReason, got.Reasons)
			}
		})
	}
}

func TestClassifyVideoForGame_EvidenceIncludesSignals(t *testing.T) {
	game := GameMetadata{
		GameID:   "196612180mia",
		DateISO:  "1966-12-18",
		GameDate: time.Date(1966, 12, 18, 0, 0, 0, 0, time.UTC),
		Year:     1966,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Houston Oilers"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:       "Houston Oilers vs Miami Dolphins Full Game 1966-12-18",
		Description: "AFL complete game broadcast",
		Duration:    90 * time.Minute,
		Embeddable:  true,
	}
	got := ClassifyVideoForGame(game, candidate, DefaultClassificationPolicy())
	if got.Decision != DecisionAccept {
		t.Fatalf("decision=%s want=%s", got.Decision, DecisionAccept)
	}
	if !got.Signals.TeamsMatched || got.Signals.DateMatch != string(DateMatchFull) {
		t.Fatalf("expected teams/date signals, got %+v", got.Signals)
	}
	if len(got.Signals.RealGameTerms) == 0 {
		t.Fatalf("expected real game terms")
	}
}

func TestClassifyVideoForGame_AcceptsLegacyBroadcastTitle(t *testing.T) {
	game := GameMetadata{
		GameID:   "198009280mia",
		DateISO:  "1980-09-28",
		GameDate: time.Date(1980, 9, 28, 0, 0, 0, 0, time.UTC),
		Year:     1980,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("New Orleans Saints"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:      "NFL 1980 09 28 80 New Orleans Saints at Miami Dolphins pt 1",
		Duration:   3586 * time.Second,
		Embeddable: true,
	}

	got := ClassifyVideoForGame(game, candidate, DefaultClassificationPolicy())
	if got.Decision != DecisionAccept {
		t.Fatalf("decision=%s want=%s result=%+v", got.Decision, DecisionAccept, got)
	}
	if got.Signals.DateMatch != string(DateMatchFull) {
		t.Fatalf("expected full date match, got %+v", got.Signals)
	}
	if !containsString(got.Signals.RealGameTerms, "nfl") || !containsString(got.Signals.RealGameTerms, "part") {
		t.Fatalf("expected nfl and part real-game terms, got %+v", got.Signals.RealGameTerms)
	}
}

func TestClassifyVideoForGame_AcceptsOfficialHighlightWithWeek(t *testing.T) {
	game := GameMetadata{
		GameID:   "202411240mia",
		DateISO:  "2024-11-24",
		GameDate: time.Date(2024, 11, 24, 0, 0, 0, 0, time.UTC),
		Year:     2024,
		Week:     "12",
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("New England Patriots"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:      "New England Patriots vs. Miami Dolphins Game Highlights | NFL 2024 Season Week 12",
		Channel:    "NFL",
		Duration:   702 * time.Second,
		Embeddable: true,
	}
	got := ScoreCandidate(game, candidate, 300)
	if got.Discarded {
		t.Fatalf("candidate discarded: %s", got.DiscardReason)
	}
	if got.Classification.Decision != DecisionAccept {
		t.Fatalf("decision=%s want=%s result=%+v", got.Classification.Decision, DecisionAccept, got.Classification)
	}
	if got.MatchedWeek != DateMatchFull {
		t.Fatalf("week=%s want=%s", got.MatchedWeek, DateMatchFull)
	}
	if got.Classification.Signals.ContentRole != string(ContentRoleOfficialHighlight) {
		t.Fatalf("role=%s want=%s", got.Classification.Signals.ContentRole, ContentRoleOfficialHighlight)
	}
}

func TestClassifyVideoForGame_AcceptsExactDateLegacyTitleWithoutContentTerm(t *testing.T) {
	game := GameMetadata{
		GameID:   "197510120mia",
		DateISO:  "1975-10-12",
		GameDate: time.Date(1975, 10, 12, 0, 0, 0, 0, time.UTC),
		Year:     1975,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Philadelphia Eagles"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:      "1975-10-12 PHIL at MIA",
		Duration:   1000 * time.Second,
		Embeddable: true,
	}
	got := ScoreCandidate(game, candidate, 300)
	if got.Discarded {
		t.Fatalf("candidate discarded: %s", got.DiscardReason)
	}
	if got.Classification.Decision != DecisionAccept {
		t.Fatalf("decision=%s want=%s result=%+v", got.Classification.Decision, DecisionAccept, got.Classification)
	}
	if !ShouldAutoApprove(got, 130) {
		t.Fatalf("expected legacy exact-date title to be auto-approved: %+v", got)
	}
}

func TestClassifyVideoForGame_AutoApprovesExactDateHighlight(t *testing.T) {
	game := GameMetadata{
		GameID:   "197710300mia",
		DateISO:  "1977-10-30",
		GameDate: time.Date(1977, 10, 30, 0, 0, 0, 0, time.UTC),
		Year:     1977,
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("San Diego Chargers"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:      "1977-10-30 San Diego Chargers @ Miami Dolphins Highlights",
		Duration:   20 * time.Minute,
		Embeddable: true,
	}
	got := ScoreCandidate(game, candidate, 300)
	if got.Discarded {
		t.Fatalf("candidate discarded: %s", got.DiscardReason)
	}
	if got.Classification.Decision != DecisionAccept {
		t.Fatalf("decision=%s want=%s result=%+v", got.Classification.Decision, DecisionAccept, got.Classification)
	}
	if !ShouldAutoApprove(got, 130) {
		t.Fatalf("expected exact-date highlight to be auto-approved: %+v", got)
	}
}

func TestClassifyVideoForGame_PostgameShowNeedsReview(t *testing.T) {
	game := GameMetadata{
		GameID:   "202412290cle",
		DateISO:  "2024-12-29",
		GameDate: time.Date(2024, 12, 29, 0, 0, 0, 0, time.UTC),
		Year:     2024,
		Home:     LookupTeam("Cleveland Browns"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:      "Big O Radio Show Miami Dolphins Postgame Show vs Browns, Sunday, 12/29/24",
		Duration:   2 * time.Hour,
		Embeddable: true,
	}
	got := ScoreCandidate(game, candidate, 300)
	if ShouldAutoApprove(got, 130) {
		t.Fatalf("postgame show should not be auto-approved: %+v", got)
	}
	if len(got.Classification.Signals.ReviewTerms) == 0 {
		t.Fatalf("expected postgame review term, got %+v", got.Classification)
	}
}

func TestClassifyVideoForGame_RejectsTabletopReplaySimulation(t *testing.T) {
	game := GameMetadata{
		GameID:   "197712050mia",
		DateISO:  "1977-12-05",
		GameDate: time.Date(1977, 12, 5, 0, 0, 0, 0, time.UTC),
		Year:     1977,
		Week:     "12",
		Home:     LookupTeam("Miami Dolphins"),
		Away:     LookupTeam("Baltimore Colts"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:      "Pro Strategy Football 2021 - 1977 NFL Colts vs Dolphins Week 12",
		Duration:   90 * time.Minute,
		Embeddable: true,
	}
	got := ScoreCandidate(game, candidate, 300)
	if got.Classification.Decision != DecisionReject {
		t.Fatalf("expected simulation replay to be rejected, got %+v", got)
	}
	if ShouldAutoApprove(got, 130) {
		t.Fatalf("simulation replay should not be auto-approved: %+v", got)
	}
}

func TestDetectGameWeekMatchGameNumber(t *testing.T) {
	game := GameMetadata{Week: "5"}
	got := DetectGameWeekMatch("1972 Game 5 - CHARGERS at DOLPHINS - Full Game - Complete Radio Broadcast", game)
	if got != DateMatchFull {
		t.Fatalf("DetectGameWeekMatch()=%s want=%s", got, DateMatchFull)
	}
}

func TestDetectGameWeekMatchCompactWeekAgainstPreseason(t *testing.T) {
	game := GameMetadata{Week: "Preseason 2"}
	got := DetectGameWeekMatch("2002-03 W09 Miami Dolphins 10 @ Green Bay Packers 24", game)
	if got != DateMatchWrong {
		t.Fatalf("DetectGameWeekMatch()=%s want=%s", got, DateMatchWrong)
	}
}

func TestClassifyVideoForGame_RejectsSeasonTopPlaysFalsePositive(t *testing.T) {
	game := GameMetadata{
		GameID:   "202512070nyj",
		DateISO:  "2025-12-07",
		GameDate: time.Date(2025, 12, 7, 0, 0, 0, 0, time.UTC),
		Year:     2025,
		Week:     "14",
		Home:     LookupTeam("New York Jets"),
		Away:     LookupTeam("Miami Dolphins"),
	}
	candidate := youtubeapi.VideoCandidate{
		Title:       "Every Teams' Top 10 Plays of 2025 Season!",
		Description: "Miami Dolphins and New York Jets appear in the season countdown",
		Channel:     "NFL",
		Duration:    2*time.Hour + 13*time.Minute,
		Embeddable:  true,
	}
	got := ScoreCandidate(game, candidate, 300)
	if !got.Discarded {
		t.Fatalf("expected discard for season top plays, got score=%+v classification=%+v", got.Score, got.Classification)
	}
}

func TestDetectGameWeekMatchPreseasonSpanish(t *testing.T) {
	game := GameMetadata{Week: "P2"}
	got := DetectGameWeekMatch("Miami Dolphins vs. Houston Texans | Pretemporada NFL 2023 | Resumen Highlights | Semana 2", game)
	if got != DateMatchFull {
		t.Fatalf("DetectGameWeekMatch()=%s want=%s", got, DateMatchFull)
	}
}

func containsReason(reasons []string, token string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, token) {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
