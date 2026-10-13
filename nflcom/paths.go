package nflcom

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

const baseURL = "https://www.nfl.com"

func GetScheduleCachePath(cacheRoot string, year int) string {
	return filepath.Join(cacheRoot, "nflcom", "schedules", fmt.Sprintf("%d.html", year))
}

func GetGameCachePath(cacheRoot string, season int, slug string) string {
	return filepath.Join(cacheRoot, "nflcom", "games", fmt.Sprintf("%d", season), SlugFromGameURL(slug)+".html")
}

func GetGamebookCachePath(cacheRoot string, season int, gameSlug string, gamebookURL string) string {
	filename := path.Base(strings.Split(gamebookURL, "?")[0])
	if filename == "." || filename == "/" || filename == "" || !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename = SlugFromGameURL(gameSlug) + "-gamebook.pdf"
	}
	return filepath.Join(cacheRoot, "nflcom", "gamebooks", fmt.Sprintf("%d", season), filename)
}

func SlugFromGameURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, baseURL)
	raw = strings.TrimPrefix(raw, "/games/")
	raw = strings.TrimPrefix(raw, "games/")
	raw = strings.Split(raw, "?")[0]
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return "unknown"
	}
	return raw
}

func AbsolutizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "/") {
		return baseURL + raw
	}
	return baseURL + "/" + raw
}
