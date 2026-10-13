package videos

import "testing"

func TestSanitizeSnippet(t *testing.T) {
	got := SanitizeSnippet("  hello\nworld   ")
	if got != "hello world" {
		t.Fatalf("unexpected sanitized value: %q", got)
	}
}

func TestSanitizeSnippetEmpty(t *testing.T) {
	if got := SanitizeSnippet("\n \t"); got != "(no description)" {
		t.Fatalf("unexpected empty fallback: %q", got)
	}
}
