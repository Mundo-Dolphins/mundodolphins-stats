package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

func TestRunVideosScorerReportJSON(t *testing.T) {
	restore := saveVideosScorerReportFlagState()
	defer restore()

	root := t.TempDir()
	gamesDir, cachePath, dbPath := writeScorerReportFixtures(t, root)
	videosScorerReportProcessedDir = gamesDir
	videosScorerReportCacheFile = cachePath
	videosScorerReportDiscoveryDB = dbPath
	videosScorerReportFormat = "json"
	videosScorerReportLimit = 5
	videosScorerReportMinDuration = 5 * time.Minute
	videosScorerReportScoreThreshold = 60

	var out bytes.Buffer
	videosScorerReportCmd.SetOut(&out)
	videosScorerReportCmd.SetContext(context.Background())
	if err := runVideosScorerReport(videosScorerReportCmd, nil); err != nil {
		t.Fatalf("runVideosScorerReport failed: %v", err)
	}
	text := out.String()
	for _, want := range []string{`"games_loaded": 2`, `"approved_videos": 1`, `"approved_without_videos": 1`, `"reason": "auto_accept"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("json report missing %q: %s", want, text)
		}
	}
}

func TestRunVideosScorerReportErrors(t *testing.T) {
	restore := saveVideosScorerReportFlagState()
	defer restore()

	videosScorerReportFromYear = 2024
	videosScorerReportToYear = 2023
	if err := runVideosScorerReport(videosScorerReportCmd, nil); err == nil || !strings.Contains(err.Error(), "from-year") {
		t.Fatalf("expected invalid range error, got %v", err)
	}

	root := t.TempDir()
	gamesDir, cachePath, dbPath := writeScorerReportFixtures(t, root)
	videosScorerReportFromYear = 0
	videosScorerReportToYear = 0
	videosScorerReportProcessedDir = gamesDir
	videosScorerReportCacheFile = cachePath
	videosScorerReportDiscoveryDB = dbPath
	videosScorerReportFormat = "xml"
	videosScorerReportCmd.SetContext(context.Background())
	if err := runVideosScorerReport(videosScorerReportCmd, nil); err == nil || !strings.Contains(err.Error(), "unsupported scorer report format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestPrintScorerReportTableIncludesSections(t *testing.T) {
	report := &videos.ScorerReport{
		Summary: videos.ScorerReportSummary{
			GamesLoaded:             2,
			CacheItems:              2,
			ReviewedGames:           1,
			ApprovedVideos:          1,
			ApprovedVideosKept:      1,
			ApprovedVideosRejected:  0,
			ApprovedBelowThreshold:  0,
			ApprovedWithoutVideos:   1,
			DiscoverySearches:       2,
			DiscoveryPairs:          3,
			DiscoveryCandidatesKept: 1,
			DiscoveryAutoAcceptable: 1,
			DiscoveryGames:          2,
			DiscoveryGamesWithKept:  1,
			DiscoveryGamesZeroKept:  1,
		},
		Reasons: []videos.ScorerReasonCount{{Reason: "auto_accept", Count: 1}},
		ApprovedIssues: []videos.ScorerCandidateFinding{{
			GameID: "g1", Decision: "reject", VideoKind: "preview", Reason: "too_short", Score: 10, DurationSec: 120, DateMatch: "none", WeekMatch: "none", VideoID: "v1", Title: "Preview",
		}},
		AutoApproveCandidates: []videos.ScorerCandidateFinding{{
			GameID: "g2", Decision: "accept", VideoKind: "full_game", Reason: "auto_accept", Score: 92, DurationSec: 3600, DateMatch: "exact", WeekMatch: "exact", VideoID: "v2", Title: "Full game",
		}},
		DiscardedExamples: []videos.ScorerCandidateFinding{{
			GameID: "g3", Decision: "reject", VideoKind: "other", Reason: "wrong_team", Score: 5, DurationSec: 500, DateMatch: "none", WeekMatch: "none", VideoID: "v3", Title: "Other game",
		}},
		ZeroCandidateGames: []videos.ScorerGameFinding{{GameID: "g4", Raw: 2, Kept: 0, Rejected: 2, ApprovedVideos: 0}},
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	printScorerReportTable(cmd, report)
	text := out.String()
	for _, want := range []string{"Scorer report", "Reasons", "Approved Issues", "Auto-Approve Candidates", "Discarded Examples", "Zero Candidate Games", "g4\t2\t0\t2\t0"} {
		if !strings.Contains(text, want) {
			t.Fatalf("table report missing %q: %s", want, text)
		}
	}
}

func TestVideosDiscoveryDBPathDefaultsAndConfigure(t *testing.T) {
	if got := defaultVideosDiscoveryDBPath(""); got != filepath.Join("cache", "videos", "discovery.sqlite") {
		t.Fatalf("unexpected empty cache default path: %s", got)
	}
	if got := defaultVideosDiscoveryDBPath("videos.json"); got != filepath.Join("cache", "videos", "discovery.sqlite") {
		t.Fatalf("unexpected bare cache default path: %s", got)
	}
	if got := defaultVideosDiscoveryDBPath(filepath.Join("tmp", "videos.json")); got != filepath.Join("tmp", "discovery.sqlite") {
		t.Fatalf("unexpected cache-relative default path: %s", got)
	}

	oldDiscoveryDB := videosDiscoveryDB
	oldCacheFile := videosCacheFile
	defer func() {
		videosDiscoveryDB = oldDiscoveryDB
		videosCacheFile = oldCacheFile
	}()

	if closeStore, err := configureVideosDiscoveryStore(nil, nil); err != nil {
		t.Fatalf("nil service should not error: %v", err)
	} else {
		closeStore()
	}
	service := &videos.Service{}
	videosDiscoveryDB = "off"
	if closeStore, err := configureVideosDiscoveryStore(service, videos.NewFileCache(filepath.Join(t.TempDir(), "videos.json"))); err != nil {
		t.Fatalf("off discovery store should not error: %v", err)
	} else {
		closeStore()
	}
	if service.CandidateStore != nil {
		t.Fatalf("off discovery store should leave service store nil")
	}

	root := t.TempDir()
	videosDiscoveryDB = ""
	videosCacheFile = filepath.Join(root, "videos.json")
	service = &videos.Service{}
	closeStore, err := configureVideosDiscoveryStore(service, videos.NewFileCache(videosCacheFile))
	if err != nil {
		t.Fatalf("configure default discovery store: %v", err)
	}
	defer closeStore()
	if service.CandidateStore == nil {
		t.Fatalf("expected default discovery store configured")
	}
	if _, err := os.Stat(filepath.Join(root, "discovery.sqlite")); err != nil {
		t.Fatalf("expected discovery sqlite next to cache: %v", err)
	}
}

func writeScorerReportFixtures(t *testing.T, root string) (string, string, string) {
	t.Helper()
	gamesDir := filepath.Join(root, "games")
	if err := os.MkdirAll(gamesDir, 0o755); err != nil {
		t.Fatalf("mkdir games: %v", err)
	}
	writeCmdFile(t, filepath.Join(gamesDir, "196612180mia.json"), `{"game_id":"196612180mia","data":{"week":"15","scorebox":{"teams":[{"name":{"content":"Houston Oilers"}},{"name":{"content":"Miami Dolphins"}}],"meta":{"date":"Sunday Dec 18, 1966"}}}}`)
	writeCmdFile(t, filepath.Join(gamesDir, "196609180buf.json"), `{"game_id":"196609180buf","data":{"week":"3","scorebox":{"teams":[{"name":{"content":"Miami Dolphins"}},{"name":{"content":"Buffalo Bills"}}],"meta":{"date":"Sunday Sep 18, 1966"}}}}`)

	cachePath := filepath.Join(root, "videos.json")
	writeCmdFile(t, cachePath, `{
	  "schema":{"name":"mundodolphins.youtube.cache","version":"1.4.0"},
	  "generated_at":"2026-08-22T00:00:00Z",
	  "items":{
	    "196612180mia":{"status":"approved","videos":[{"id":"approved","url":"https://www.youtube.com/watch?v=approved","title":"Houston Oilers vs Miami Dolphins 1966-12-18 Full Game","duration_seconds":4000,"selected_at":"2026-08-22T00:00:00Z"}]},
	    "196609180buf":{"status":"approved","videos":[]}
	  }
	}`)

	dbPath := filepath.Join(root, "discovery.sqlite")
	store, err := videos.NewSQLiteCandidateStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteCandidateStore: %v", err)
	}
	if err := store.SaveSearchResults(context.Background(), videos.GameRef{GameID: "196612180mia"}, "Houston Oilers vs Miami Dolphins 1966", "rod", []videos.Candidate{{
		VideoID: "approved", URL: "https://www.youtube.com/watch?v=approved", Title: "Houston Oilers vs Miami Dolphins 1966-12-18 Full Game", DurationSec: 4000,
	}}); err != nil {
		t.Fatalf("seed approved search: %v", err)
	}
	if err := store.SaveSearchResults(context.Background(), videos.GameRef{GameID: "196609180buf"}, "Miami Dolphins vs Buffalo Bills 1966-09-18", "web", []videos.Candidate{{
		VideoID: "noise", URL: "https://www.youtube.com/watch?v=noise", Title: "Josh Allen owns the Dolphins", DurationSec: 4395,
	}}); err != nil {
		t.Fatalf("seed noisy search: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close discovery store: %v", err)
	}
	return gamesDir, cachePath, dbPath
}

func writeCmdFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func saveVideosScorerReportFlagState() func() {
	oldProcessedDir := videosScorerReportProcessedDir
	oldCacheFile := videosScorerReportCacheFile
	oldDiscoveryDB := videosScorerReportDiscoveryDB
	oldFormat := videosScorerReportFormat
	oldLimit := videosScorerReportLimit
	oldMinDuration := videosScorerReportMinDuration
	oldScoreThreshold := videosScorerReportScoreThreshold
	oldFromYear := videosScorerReportFromYear
	oldToYear := videosScorerReportToYear
	oldOut := videosScorerReportCmd.OutOrStdout()
	oldContext := videosScorerReportCmd.Context()
	return func() {
		videosScorerReportProcessedDir = oldProcessedDir
		videosScorerReportCacheFile = oldCacheFile
		videosScorerReportDiscoveryDB = oldDiscoveryDB
		videosScorerReportFormat = oldFormat
		videosScorerReportLimit = oldLimit
		videosScorerReportMinDuration = oldMinDuration
		videosScorerReportScoreThreshold = oldScoreThreshold
		videosScorerReportFromYear = oldFromYear
		videosScorerReportToYear = oldToYear
		videosScorerReportCmd.SetOut(oldOut)
		videosScorerReportCmd.SetContext(oldContext)
	}
}
