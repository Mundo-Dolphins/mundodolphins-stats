package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
	"github.com/spf13/cobra"
)

var videosAutoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Incrementally discover and assign YouTube videos for all games",
	Long: `Production-grade incremental crawler for YouTube video discovery.

Phase A: Scans known YouTube channels' uploads (cheap: 1 quota unit per 50 videos).
Phase B: Falls back to search.list for unmatched games (expensive: 100 units each).

Features:
  - Resumable across multiple days (state persisted in cache/videos/state.json)
  - Daily quota tracking with automatic reset
  - Three-tier disk cache (channels, search results, video details)
  - Each video assigned to at most ONE game (unique constraint)
  - Deterministic, incremental, explainable

Typical usage:
  # First run: scans channels + searches with remaining quota
  dolphins-stats videos auto --max-quota 9000

  # Next day: resumes where it left off
  dolphins-stats videos auto

  # Reset all progress and start fresh
  dolphins-stats videos auto --reset`,
	RunE: runVideosAuto,
}

var (
	autoMaxQuota         int
	autoSeason           int
	autoGameID           string
	autoDryRun           bool
	autoReset            bool
	autoValidateExisting bool
	autoNoChannelScan    bool
	autoNoSearch         bool
	autoApprove          bool
	autoApproveScore     int
	autoMinDuration      int
	autoMaxResults       int
	autoProcessedDir     string
	autoVideosDir        string
)

func init() {
	videosAutoCmd.Flags().IntVar(&autoMaxQuota, "max-quota", 9000, "Maximum YouTube API quota units to consume (daily limit is 10000)")
	videosAutoCmd.Flags().IntVar(&autoSeason, "season", 0, "Process only this NFL season (0=all)")
	videosAutoCmd.Flags().StringVar(&autoGameID, "game-id", "", "Process only this game ID")
	videosAutoCmd.Flags().BoolVar(&autoDryRun, "dry-run", false, "Preview changes without persisting")
	videosAutoCmd.Flags().BoolVar(&autoReset, "reset", false, "Reset all progress and start fresh")
	videosAutoCmd.Flags().BoolVar(&autoValidateExisting, "validate-existing", true, "Check if cached videos still exist on YouTube")
	videosAutoCmd.Flags().BoolVar(&autoNoChannelScan, "no-channel-scan", false, "Skip channel scanning phase")
	videosAutoCmd.Flags().BoolVar(&autoNoSearch, "no-search", false, "Skip search fallback phase")
	videosAutoCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Persist high-confidence matches as approved without manual review")
	videosAutoCmd.Flags().IntVar(&autoApproveScore, "auto-approve-score", 130, "Minimum scorer total for --auto-approve")
	videosAutoCmd.Flags().IntVar(&autoMinDuration, "min-duration-seconds", 300, "Minimum video duration in seconds")
	videosAutoCmd.Flags().IntVar(&autoMaxResults, "max-results", 10, "Max search candidates per query")
	videosAutoCmd.Flags().StringVar(&autoProcessedDir, "processed-dir", "processed/games", "Base directory for processed game JSON files")
	videosAutoCmd.Flags().StringVar(&autoVideosDir, "videos-dir", "cache/videos", "Root directory for videos output, state, and caches")
	videosCmd.AddCommand(videosAutoCmd)
}

func runVideosAuto(cmd *cobra.Command, args []string) error {
	apiKey := resolveYouTubeAPIKey("")
	if apiKey == "" {
		return fmt.Errorf("YOUTUBE_API_KEY environment variable is required (set in .env or export)")
	}

	ytClient := youtubeapi.NewClient(apiKey, &http.Client{Timeout: 30 * time.Second})

	pipeline := &videos.Pipeline{
		Client: ytClient,
		Config: videos.PipelineConfig{
			MaxQuota:         autoMaxQuota,
			ProcessedDir:     autoProcessedDir,
			VideosDir:        autoVideosDir,
			DryRun:           autoDryRun,
			Reset:            autoReset,
			ValidateExisting: autoValidateExisting,
			NoChannelScan:    autoNoChannelScan,
			NoSearch:         autoNoSearch,
			AutoApprove:      autoApprove,
			AutoApproveScore: autoApproveScore,
			MinDurationSec:   autoMinDuration,
			MaxResults:       autoMaxResults,
			Season:           autoSeason,
			GameID:           autoGameID,
		},
	}

	if autoDryRun {
		log.Println("DRY RUN: no changes will be persisted")
	}

	stats, err := pipeline.Run(context.Background())
	if stats != nil {
		printPipelineSummary(stats)
	}
	if err != nil {
		return err
	}

	return nil
}

func printPipelineSummary(stats *videos.PipelineStats) {
	if stats == nil {
		return
	}

	fmt.Println(strings.Repeat("─", 55))
	fmt.Println("Pipeline summary:")
	fmt.Printf("  Total games:           %d\n", stats.GamesTotal)
	fmt.Printf("  Games needing video:   %d\n", stats.GamesNeedingVideo)
	fmt.Printf("  Matched from channels: %d\n", stats.ChannelMatched)
	fmt.Printf("  Added via search:      %d\n", stats.SearchAdded)
	fmt.Printf("  Search no result:      %d\n", stats.SearchNoResult)
	fmt.Printf("  Auto-approved items:   %d\n", stats.AutoApproved)
	fmt.Printf("  Review candidates:     %d\n", stats.CandidatesAdded)
	fmt.Printf("  Videos removed:        %d\n", stats.VideosRemoved)
	fmt.Printf("  Quota used:            %d/%d\n", stats.QuotaUsed, stats.QuotaMax)
	if stats.StoppedByQuota {
		fmt.Println("  ⚠ Stopped by quota — run again tomorrow to continue")
	}
	fmt.Println(strings.Repeat("─", 55))
}
