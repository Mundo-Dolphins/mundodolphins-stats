package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVideosReviewJSON(t *testing.T) {
	restore := saveVideosReviewFlagState()
	defer restore()

	cachePath := writeReviewCache(t)
	videosReviewCacheFile = cachePath
	videosReviewFormat = "json"
	videosReviewLimit = 1

	var out bytes.Buffer
	videosReviewCmd.SetOut(&out)
	if err := runVideosReview(videosReviewCmd, nil); err != nil {
		t.Fatalf("runVideosReview failed: %v", err)
	}
	if !strings.Contains(out.String(), `"game_id": "game1"`) {
		t.Fatalf("expected review item in json output: %s", out.String())
	}
	if strings.Contains(out.String(), `"game_id": "game2"`) {
		t.Fatalf("limit should suppress second item: %s", out.String())
	}
}

func TestRunVideosReviewTable(t *testing.T) {
	restore := saveVideosReviewFlagState()
	defer restore()

	cachePath := writeReviewCache(t)
	videosReviewCacheFile = cachePath
	videosReviewFormat = "table"
	videosReviewLimit = 0

	var out bytes.Buffer
	videosReviewCmd.SetOut(&out)
	if err := runVideosReview(videosReviewCmd, nil); err != nil {
		t.Fatalf("runVideosReview failed: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "GAME_ID\tRISK\tSCORE") || !strings.Contains(text, "game1") {
		t.Fatalf("unexpected table output: %s", text)
	}
}

func TestRunVideosReviewInvalidFormat(t *testing.T) {
	restore := saveVideosReviewFlagState()
	defer restore()

	videosReviewCacheFile = writeReviewCache(t)
	videosReviewFormat = "xml"

	if err := runVideosReview(videosReviewCmd, nil); err == nil {
		t.Fatalf("expected invalid format error")
	}
}

func writeReviewCache(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "videos.json")
	data := `{
	  "schema":{"name":"mundodolphins.youtube.cache","version":"1.3.0"},
	  "items":{
	    "game1":{
	      "status":"candidate",
	      "needs_review":true,
	      "query":"preview",
	      "updated_at":"2026-01-01T00:00:00Z",
	      "videos":[{"id":"v1","url":"u","title":"Dolphins Week Preview","selected_at":"2026-01-01T00:00:00Z"}],
	      "auto_scoring":{"score_breakdown":{"total":50},"matched_teams":true,"matched_date_type":"year_only","source":"test","needs_review":true}
	    },
	    "game2":{"status":"approved","videos":[{"id":"v2","selected_at":"2026-01-01T00:00:00Z"}]}
	  }
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	return path
}

func saveVideosReviewFlagState() func() {
	oldCacheFile := videosReviewCacheFile
	oldFormat := videosReviewFormat
	oldLimit := videosReviewLimit
	oldOut := videosReviewCmd.OutOrStdout()
	return func() {
		videosReviewCacheFile = oldCacheFile
		videosReviewFormat = oldFormat
		videosReviewLimit = oldLimit
		videosReviewCmd.SetOut(oldOut)
	}
}
