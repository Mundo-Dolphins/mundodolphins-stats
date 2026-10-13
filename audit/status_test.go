package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildStatusManifestInventoryAndWarnings(t *testing.T) {
	root := t.TempDir()
	processedDir := filepath.Join(root, "processed")
	gamesDir := filepath.Join(processedDir, "games")
	articlesDir := filepath.Join(root, "articles")
	videosCache := filepath.Join(root, "cache", "videos", "videos.json")
	outputDir := filepath.Join(root, "output", "data", "historical")

	writeTestFile(t, filepath.Join(processedDir, "2025.json"), `{
		"schema":{"name":"mundodolphins.pfr.team-season","version":"1.5.0"},
		"season":2025,
		"data":{}
	}`)
	writeTestFile(t, filepath.Join(processedDir, "2026.json"), `{
		"schema":{"name":"mundodolphins.pfr.team-season","version":"1.5.0"},
		"season":2026,
		"data":{}
	}`)
	writeTestGame(t, filepath.Join(gamesDir, "2025", "202509070mia.json"), "202509070mia")
	writeTestGame(t, filepath.Join(gamesDir, "2026", "202602010mia.json"), "202602010mia")
	writeTestGame(t, filepath.Join(gamesDir, "2026", "202603100mia.json"), "202603100mia")
	writeTestFile(t, filepath.Join(articlesDir, "2025.md"), "# 2025\n")
	writeTestFile(t, videosCache, `{
		"schema":{"name":"mundodolphins.youtube.cache","version":"1.2.0"},
		"generated_at":"2026-01-01T00:00:00Z",
		"items":{
			"202509070mia":{
				"status":"selected",
				"query":"game",
				"updated_at":"2026-01-01T00:00:00Z",
				"videos":[{"id":"v1","url":"https://example.invalid/v1","title":"Game","duration_seconds":500,"selected_at":"2026-01-01T00:00:00Z"}]
			},
			"202602010mia":{"status":"none","query":"game","updated_at":"2026-01-01T00:00:00Z","videos":[]},
			"202603100mia":{"status":"selected","query":"game","updated_at":"2026-01-01T00:00:00Z","videos":[]},
			"199901010mia":{"status":"pending","query":"extra","updated_at":"2026-01-01T00:00:00Z","videos":[]}
		}
	}`)
	writeTestFile(t, filepath.Join(outputDir, "seasons", "2025.json"), sectionedPageJSON())
	writeTestFile(t, filepath.Join(outputDir, "season_stats", "2025.json"), sectionedPageJSON())
	writeTestFile(t, filepath.Join(outputDir, "games", "2025", "202509070mia.json"), sectionedPageJSON())

	manifest, err := BuildStatusManifest(StatusOptions{
		ProcessedDir:       processedDir,
		GamesDir:           gamesDir,
		ArticlesDir:        articlesDir,
		VideosCacheFile:    videosCache,
		OutputDir:          outputDir,
		IncludeGeneratedAt: true,
		Now: func() time.Time {
			return time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("BuildStatusManifest error: %v", err)
	}

	if manifest.GeneratedAt != "2026-07-31T10:00:00Z" {
		t.Fatalf("GeneratedAt = %q", manifest.GeneratedAt)
	}
	if manifest.Seasons.Total != 2 {
		t.Fatalf("seasons total = %d, want 2", manifest.Seasons.Total)
	}
	assertInts(t, manifest.Seasons.YearsPresent, []int{2025, 2026})
	assertInts(t, manifest.Articles.YearsMissing, []int{2026})
	if manifest.Games.Total != 3 {
		t.Fatalf("games total = %d, want 3", manifest.Games.Total)
	}
	assertYearCounts(t, manifest.Games.BySeason, []YearCount{{Year: 2025, Count: 2}, {Year: 2026, Count: 1}})
	assertYearCounts(t, manifest.Games.ByDirectorySeason, []YearCount{{Year: 2025, Count: 1}, {Year: 2026, Count: 2}})
	if len(manifest.Games.Misplaced) != 1 {
		t.Fatalf("misplaced games = %d, want 1", len(manifest.Games.Misplaced))
	}
	if got := manifest.Games.Misplaced[0]; got.GameID != "202602010mia" || got.LocatedSeason != 2026 || got.ExpectedSeason != 2025 {
		t.Fatalf("unexpected misplaced game: %+v", got)
	}

	if manifest.Videos.Total != 4 || manifest.Videos.TotalItems != 4 || manifest.Videos.TotalVideos != 1 {
		t.Fatalf("unexpected videos totals: %+v", manifest.Videos)
	}
	assertStatusCounts(t, manifest.Videos.Statuses, []StatusCount{
		{Status: "none", Count: 1},
		{Status: "pending", Count: 1},
		{Status: "selected", Count: 2},
	})
	if manifest.Videos.Coverage.GamesWithSelectedVideo != 1 || manifest.Videos.Coverage.GamesMissingVideo != 2 {
		t.Fatalf("unexpected video coverage: %+v", manifest.Videos.Coverage)
	}
	assertStrings(t, manifest.Videos.Coverage.MissingGameIDs, []string{"202602010mia", "202603100mia"})
	assertStrings(t, manifest.Videos.ExtraCacheGameIDs, []string{"199901010mia"})
	assertStrings(t, manifest.Videos.SelectedWithoutVideoIDs, []string{"202603100mia"})

	if manifest.Output.TotalJSON != 3 || manifest.Output.GamesTotal != 1 || manifest.Output.PagesTotal != 0 {
		t.Fatalf("unexpected output status: %+v", manifest.Output)
	}
	assertInts(t, manifest.Output.SeasonYears, []int{2025})
	assertInts(t, manifest.Output.SeasonStatsYears, []int{2025})

	for _, code := range []string{
		"missing_articles",
		"misplaced_games",
		"videos_cache_extra_games",
		"videos_missing_for_games",
		"videos_selected_without_media",
	} {
		if !hasWarningCode(manifest.Warnings, code) {
			t.Fatalf("expected warning code %q in %+v", code, manifest.Warnings)
		}
	}
}

func TestBuildStatusManifestMissingDirectories(t *testing.T) {
	root := t.TempDir()
	manifest, err := BuildStatusManifest(StatusOptions{
		ProcessedDir:    filepath.Join(root, "processed"),
		GamesDir:        filepath.Join(root, "processed", "games"),
		ArticlesDir:     filepath.Join(root, "articles"),
		VideosCacheFile: filepath.Join(root, "cache", "videos", "videos.json"),
		OutputDir:       filepath.Join(root, "output", "data", "historical"),
	})
	if err != nil {
		t.Fatalf("BuildStatusManifest error: %v", err)
	}
	if manifest.Seasons.Total != 0 || manifest.Games.Total != 0 || manifest.Articles.Total != 0 {
		t.Fatalf("expected empty manifest counts: %+v", manifest)
	}
	if manifest.Videos.Coverage.GamesMissingVideo != 0 {
		t.Fatalf("expected no missing videos without games: %+v", manifest.Videos.Coverage)
	}
	if countWarningCode(manifest.Warnings, "input_missing") != 5 {
		t.Fatalf("expected five input_missing warnings, got %+v", manifest.Warnings)
	}
	for _, input := range manifest.Inputs {
		if input.Exists {
			t.Fatalf("expected input to be missing: %+v", input)
		}
	}
}

func writeTestGame(t *testing.T, path, gameID string) {
	t.Helper()
	writeTestFile(t, path, `{
		"schema":{"name":"mundodolphins.pfr.boxscore","version":"1.2.0"},
		"game_id":"`+gameID+`",
		"data":{}
	}`)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func sectionedPageJSON() string {
	return `{"schema":{"name":"mundodolphins.hugo.sectioned-page","version":"1.0.0"},"sections":{}}`
}

func assertInts(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func assertYearCounts(t *testing.T, got, want []YearCount) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func assertStatusCounts(t *testing.T, got, want []StatusCount) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func hasWarningCode(warnings []ManifestWarning, code string) bool {
	return countWarningCode(warnings, code) > 0
}

func countWarningCode(warnings []ManifestWarning, code string) int {
	count := 0
	for _, warning := range warnings {
		if warning.Code == code {
			count++
		}
	}
	return count
}
