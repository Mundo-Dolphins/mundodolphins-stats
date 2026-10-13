package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

func TestRunVideosAutoRequiresAPIKey(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "")

	err := runVideosAuto(videosAutoCmd, nil)
	if err == nil {
		t.Fatal("expected error when YOUTUBE_API_KEY is missing")
	}
	if !strings.Contains(err.Error(), "YOUTUBE_API_KEY") {
		t.Fatalf("expected YOUTUBE_API_KEY error, got %v", err)
	}
}

func TestRunVideosAutoDryRunNoGamesBuildsPipelineOffline(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "test-key")

	prevMaxQuota := autoMaxQuota
	prevSeason := autoSeason
	prevGameID := autoGameID
	prevDryRun := autoDryRun
	prevReset := autoReset
	prevValidateExisting := autoValidateExisting
	prevNoChannelScan := autoNoChannelScan
	prevNoSearch := autoNoSearch
	prevAutoApprove := autoApprove
	prevAutoApproveScore := autoApproveScore
	prevMinDuration := autoMinDuration
	prevMaxResults := autoMaxResults
	prevProcessedDir := autoProcessedDir
	prevVideosDir := autoVideosDir
	defer func() {
		autoMaxQuota = prevMaxQuota
		autoSeason = prevSeason
		autoGameID = prevGameID
		autoDryRun = prevDryRun
		autoReset = prevReset
		autoValidateExisting = prevValidateExisting
		autoNoChannelScan = prevNoChannelScan
		autoNoSearch = prevNoSearch
		autoApprove = prevAutoApprove
		autoApproveScore = prevAutoApproveScore
		autoMinDuration = prevMinDuration
		autoMaxResults = prevMaxResults
		autoProcessedDir = prevProcessedDir
		autoVideosDir = prevVideosDir
	}()

	tmpDir := t.TempDir()
	autoMaxQuota = 10
	autoSeason = 1966
	autoGameID = ""
	autoDryRun = true
	autoReset = true
	autoValidateExisting = false
	autoNoChannelScan = true
	autoNoSearch = true
	autoApprove = false
	autoApproveScore = 130
	autoMinDuration = 300
	autoMaxResults = 5
	autoProcessedDir = filepath.Join(tmpDir, "processed", "games")
	autoVideosDir = filepath.Join(tmpDir, "cache", "videos")
	if err := os.MkdirAll(autoProcessedDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	output := captureStdout(t, func() {
		if err := runVideosAuto(videosAutoCmd, nil); err != nil {
			t.Fatalf("runVideosAuto failed: %v", err)
		}
	})
	if !strings.Contains(output, "Pipeline summary:") {
		t.Fatalf("expected summary output, got:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(autoVideosDir, "state.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run with no games should not persist state")
	}
}

func TestPrintPipelineSummary(t *testing.T) {
	stats := &videos.PipelineStats{
		GamesTotal:        25,
		GamesNeedingVideo: 20,
		ChannelMatched:    18,
		SearchAdded:       2,
		SearchNoResult:    3,
		VideosRemoved:     5,
		QuotaUsed:         9000,
		QuotaMax:          9000,
		StoppedByQuota:    true,
	}

	output := captureStdout(t, func() {
		printPipelineSummary(stats)
	})

	for _, want := range []string{
		"Pipeline summary:",
		"Total games:           25",
		"Games needing video:   20",
		"Matched from channels: 18",
		"Added via search:      2",
		"Search no result:      3",
		"Auto-approved items:   0",
		"Review candidates:     0",
		"Videos removed:        5",
		"Quota used:            9000/9000",
		"Stopped by quota",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected summary output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestPrintPipelineSummaryNil(t *testing.T) {
	output := captureStdout(t, func() {
		printPipelineSummary(nil)
	})
	if output != "" {
		t.Fatalf("expected no output for nil stats, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = original
	})

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(out)
}
