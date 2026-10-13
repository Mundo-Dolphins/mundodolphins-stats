package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

var videosScorerReportCmd = &cobra.Command{
	Use:   "scorer-report",
	Short: "Evaluate video scorer behavior against approved videos and discovery cache",
	RunE:  runVideosScorerReport,
}

var (
	videosScorerReportProcessedDir   string
	videosScorerReportCacheFile      string
	videosScorerReportDiscoveryDB    string
	videosScorerReportFormat         string
	videosScorerReportLimit          int
	videosScorerReportMinDuration    time.Duration
	videosScorerReportScoreThreshold int
	videosScorerReportFromYear       int
	videosScorerReportToYear         int
)

func init() {
	videosScorerReportCmd.Flags().StringVar(&videosScorerReportProcessedDir, "processed-dir", "processed/games", "Directory with processed game JSON files")
	videosScorerReportCmd.Flags().StringVar(&videosScorerReportCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosScorerReportCmd.Flags().StringVar(&videosScorerReportDiscoveryDB, "discovery-db", "cache/videos/discovery.sqlite", "SQLite candidate discovery cache path")
	videosScorerReportCmd.Flags().StringVar(&videosScorerReportFormat, "format", "table", "Output format: table or json")
	videosScorerReportCmd.Flags().IntVar(&videosScorerReportLimit, "limit", 20, "Maximum examples per section")
	videosScorerReportCmd.Flags().DurationVar(&videosScorerReportMinDuration, "min-duration", 5*time.Minute, "Minimum video duration")
	videosScorerReportCmd.Flags().IntVar(&videosScorerReportScoreThreshold, "score-threshold", 60, "Score threshold used for approved issue reporting")
	videosScorerReportCmd.Flags().IntVar(&videosScorerReportFromYear, "from-year", 0, "From season year")
	videosScorerReportCmd.Flags().IntVar(&videosScorerReportToYear, "to-year", 0, "To season year")
	videosCmd.AddCommand(videosScorerReportCmd)
}

func runVideosScorerReport(cmd *cobra.Command, args []string) error {
	if videosScorerReportFromYear > 0 && videosScorerReportToYear > 0 && videosScorerReportFromYear > videosScorerReportToYear {
		return fmt.Errorf("from-year must be <= to-year")
	}
	report, err := videos.BuildScorerReport(cmd.Context(), videos.ScorerReportConfig{
		GamesDir:       videosScorerReportProcessedDir,
		CacheFile:      videosScorerReportCacheFile,
		DiscoveryDB:    videosScorerReportDiscoveryDB,
		MinDurationSec: int(videosScorerReportMinDuration.Seconds()),
		ScoreThreshold: videosScorerReportScoreThreshold,
		ExampleLimit:   videosScorerReportLimit,
		FromYear:       videosScorerReportFromYear,
		ToYear:         videosScorerReportToYear,
	})
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(videosScorerReportFormat)) {
	case "", "table":
		printScorerReportTable(cmd, report)
		return nil
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal scorer report: %w", err)
		}
		cmd.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unsupported scorer report format %q", videosScorerReportFormat)
	}
}

func printScorerReportTable(cmd *cobra.Command, report *videos.ScorerReport) {
	out := cmd.OutOrStdout()
	s := report.Summary
	fmt.Fprintln(out, "Scorer report")
	fmt.Fprintf(out, "Games loaded:              %d\n", s.GamesLoaded)
	fmt.Fprintf(out, "Cache items:               %d\n", s.CacheItems)
	fmt.Fprintf(out, "Reviewed games:            %d\n", s.ReviewedGames)
	fmt.Fprintf(out, "Approved videos:           %d (kept=%d rejected=%d below_threshold=%d)\n", s.ApprovedVideos, s.ApprovedVideosKept, s.ApprovedVideosRejected, s.ApprovedBelowThreshold)
	fmt.Fprintf(out, "Approved auto-eligible:    %d\n", s.ApprovedAutoEligible)
	fmt.Fprintf(out, "Approved without videos:   %d\n", s.ApprovedWithoutVideos)
	fmt.Fprintf(out, "Discovery searches:        %d\n", s.DiscoverySearches)
	fmt.Fprintf(out, "Discovery pairs:           %d\n", s.DiscoveryPairs)
	fmt.Fprintf(out, "Discovery candidates kept: %d\n", s.DiscoveryCandidatesKept)
	fmt.Fprintf(out, "Discovery auto-acceptable: %d\n", s.DiscoveryAutoAcceptable)
	fmt.Fprintf(out, "Discovery auto-eligible:   %d\n", s.DiscoveryAutoEligible)
	fmt.Fprintf(out, "Discovery games:           %d (with_kept=%d zero_kept=%d)\n", s.DiscoveryGames, s.DiscoveryGamesWithKept, s.DiscoveryGamesZeroKept)

	printReasonCounts(out, report.Reasons)
	printCandidateFindings(out, "Approved Issues", report.ApprovedIssues)
	printCandidateFindings(out, "Auto-Approve Candidates", report.AutoApproveCandidates)
	printCandidateFindings(out, "Discarded Examples", report.DiscardedExamples)
	printGameFindings(out, "Zero Candidate Games", report.ZeroCandidateGames)
}

func printReasonCounts(out interface{ Write([]byte) (int, error) }, reasons []videos.ScorerReasonCount) {
	if len(reasons) == 0 {
		return
	}
	fmt.Fprintln(out, "\nReasons")
	fmt.Fprintln(out, "REASON\tCOUNT")
	for _, item := range reasons {
		fmt.Fprintf(out, "%s\t%d\n", item.Reason, item.Count)
	}
}

func printCandidateFindings(out interface{ Write([]byte) (int, error) }, title string, findings []videos.ScorerCandidateFinding) {
	if len(findings) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s\n", title)
	fmt.Fprintln(out, "GAME_ID\tDECISION\tKIND\tREASON\tSCORE\tDUR\tDATE\tWEEK\tVIDEO_ID\tTITLE")
	for _, item := range findings {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
			item.GameID,
			item.Decision,
			item.VideoKind,
			item.Reason,
			item.Score,
			item.DurationSec,
			item.DateMatch,
			item.WeekMatch,
			item.VideoID,
			item.Title,
		)
	}
}

func printGameFindings(out interface{ Write([]byte) (int, error) }, title string, findings []videos.ScorerGameFinding) {
	if len(findings) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s\n", title)
	fmt.Fprintln(out, "GAME_ID\tRAW\tKEPT\tREJECTED\tAPPROVED_VIDEOS")
	for _, item := range findings {
		fmt.Fprintf(out, "%s\t%d\t%d\t%d\t%d\n", item.GameID, item.Raw, item.Kept, item.Rejected, item.ApprovedVideos)
	}
}
