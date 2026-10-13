package videos

import "testing"

func TestLookupTeam_CaseInsensitiveAndSynthesize(t *testing.T) {
	got := LookupTeam("miami dolphins")
	if got.Abbr != "MIA" {
		t.Fatalf("expected MIA, got %q", got.Abbr)
	}

	synth := LookupTeam("Custom Team")
	if synth.FullName != "Custom Team" || synth.City != "Custom" || synth.Mascot != "Team" {
		t.Fatalf("unexpected synthesized aliases: %+v", synth)
	}

	single := LookupTeam("Dolphins")
	if single.City != "Dolphins" || single.Mascot != "Dolphins" {
		t.Fatalf("unexpected single-word aliases: %+v", single)
	}

	empty := LookupTeam("   ")
	if empty.FullName != "" || empty.City != "" || empty.Mascot != "" {
		t.Fatalf("expected zero value for empty name, got %+v", empty)
	}
}
