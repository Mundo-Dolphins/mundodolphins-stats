package nflcom

import (
	"path/filepath"
	"testing"
)

func TestSlugFromGameURL(t *testing.T) {
	cases := map[string]string{
		"https://www.nfl.com/games/dolphins-at-bills-2025-reg-1?tab=stats": "dolphins-at-bills-2025-reg-1",
		"/games/dolphins-at-bills-2025-reg-1/":                             "dolphins-at-bills-2025-reg-1",
		"games/dolphins-at-bills-2025-reg-1":                               "dolphins-at-bills-2025-reg-1",
		"dolphins-at-bills-2025-reg-1":                                     "dolphins-at-bills-2025-reg-1",
		"":                                                                 "unknown",
		"  /games/  ":                                                      "unknown",
	}
	for in, want := range cases {
		if got := SlugFromGameURL(in); got != want {
			t.Errorf("SlugFromGameURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAbsolutizeURL(t *testing.T) {
	cases := map[string]string{
		"":                           "",
		"https://example.com/a.pdf":  "https://example.com/a.pdf",
		"//static.example.com/a.pdf": "https://static.example.com/a.pdf",
		"/games/x":                   "https://www.nfl.com/games/x",
		"games/x":                    "https://www.nfl.com/games/x",
	}
	for in, want := range cases {
		if got := AbsolutizeURL(in); got != want {
			t.Errorf("AbsolutizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCachePaths(t *testing.T) {
	root := "cache"
	if got, want := GetScheduleCachePath(root, 2025), filepath.Join(root, "nflcom", "schedules", "2025.html"); got != want {
		t.Errorf("schedule path = %q, want %q", got, want)
	}
	if got, want := GetGameCachePath(root, 2025, "/games/some-game"), filepath.Join(root, "nflcom", "games", "2025", "some-game.html"); got != want {
		t.Errorf("game path = %q, want %q", got, want)
	}
	if got, want := GetGamebookCachePath(root, 2025, "some-game", "https://example.com/files/book.pdf?x=1"), filepath.Join(root, "nflcom", "gamebooks", "2025", "book.pdf"); got != want {
		t.Errorf("gamebook path = %q, want %q", got, want)
	}
	// Without a usable file name in the URL the game slug names the file.
	if got, want := GetGamebookCachePath(root, 2025, "some-game", "https://example.com/download"), filepath.Join(root, "nflcom", "gamebooks", "2025", "some-game-gamebook.pdf"); got != want {
		t.Errorf("fallback gamebook path = %q, want %q", got, want)
	}
}
