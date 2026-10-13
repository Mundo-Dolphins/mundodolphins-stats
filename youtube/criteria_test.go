package youtube

import (
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

func TestBuildQuery(t *testing.T) {
	q := BuildQuery(Criteria{VisitorTeam: "Oakland Raiders", HomeTeam: "Miami Dolphins", DateISO: "1966-09-02"})
	if q != "Oakland Raiders vs Miami Dolphins 1966-09-02" {
		t.Fatalf("unexpected query: %q", q)
	}
}

func TestCriteriaFromGame(t *testing.T) {
	c := CriteriaFromGame(videos.GameRef{GameID: "196609020mia", AwayTeam: "Oakland Raiders", HomeTeam: "Miami Dolphins"})
	if c.DateISO != "1966-09-02" {
		t.Fatalf("unexpected date: %q", c.DateISO)
	}
	if c.VisitorTeam != "Oakland Raiders" || c.HomeTeam != "Miami Dolphins" {
		t.Fatalf("unexpected teams: %+v", c)
	}
}

func TestDateISOFromGameIDInvalid(t *testing.T) {
	if got := dateISOFromGameID("bad"); got != "" {
		t.Fatalf("expected empty date, got %q", got)
	}
	if got := dateISOFromGameID("1966zzzzmia"); got != "" {
		t.Fatalf("expected empty date, got %q", got)
	}
}

func TestRequiredTokens(t *testing.T) {
	tokens := requiredTokens("Los Angeles Rams")
	if len(tokens) != 3 {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
}
