package cli

import (
	"context"
	"fmt"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

var videosClassifyCacheCmd = &cobra.Command{
	Use:   "classify-cache",
	Short: "Classify cached discovery searches into a videos.json file",
	RunE:  runVideosClassifyCache,
}

var (
	classifyCacheProcessedDir     string
	classifyCacheDiscoveryDB      string
	classifyCacheOutput           string
	classifyCacheSeason           int
	classifyCacheGameID           string
	classifyCacheMinDurationSec   int
	classifyCacheMaxResults       int
	classifyCacheScoreThreshold   int
	classifyCacheAutoApproveScore int
)

func init() {
	videosClassifyCacheCmd.Flags().StringVar(&classifyCacheProcessedDir, "processed-dir", "processed/games", "Base directory for processed game JSON files")
	videosClassifyCacheCmd.Flags().StringVar(&classifyCacheDiscoveryDB, "discovery-db", "cache/videos/discovery.sqlite", "SQLite discovery cache path")
	videosClassifyCacheCmd.Flags().StringVar(&classifyCacheOutput, "output", "cache/videos/videos.json", "Output videos.json path")
	videosClassifyCacheCmd.Flags().IntVar(&classifyCacheSeason, "season", 0, "Process only this NFL season (0=all)")
	videosClassifyCacheCmd.Flags().StringVar(&classifyCacheGameID, "game-id", "", "Process only this game ID")
	videosClassifyCacheCmd.Flags().IntVar(&classifyCacheMinDurationSec, "min-duration-seconds", 300, "Minimum video duration in seconds")
	videosClassifyCacheCmd.Flags().IntVar(&classifyCacheMaxResults, "max-results", 0, "Max ranked candidates per game before strict classification (0=all)")
	videosClassifyCacheCmd.Flags().IntVar(&classifyCacheScoreThreshold, "score-threshold", 60, "Minimum score for candidate relevance")
	videosClassifyCacheCmd.Flags().IntVar(&classifyCacheAutoApproveScore, "auto-approve-score", 130, "Minimum strict classifier score for automatic approval")
	videosCmd.AddCommand(videosClassifyCacheCmd)
}

func runVideosClassifyCache(cmd *cobra.Command, args []string) error {
	stats, err := videos.RunCachedClassifier(context.Background(), videos.CachedClassifierConfig{
		ProcessedDir:     classifyCacheProcessedDir,
		DiscoveryDB:      classifyCacheDiscoveryDB,
		OutputFile:       classifyCacheOutput,
		Season:           classifyCacheSeason,
		GameID:           classifyCacheGameID,
		MinDurationSec:   classifyCacheMinDurationSec,
		MaxResults:       classifyCacheMaxResults,
		ScoreThreshold:   classifyCacheScoreThreshold,
		AutoApproveScore: classifyCacheAutoApproveScore,
	})
	if stats != nil {
		printCachedClassifierSummary(stats)
	}
	return err
}

func printCachedClassifierSummary(stats *videos.CachedClassifierStats) {
	if stats == nil {
		return
	}
	fmt.Println("Cached classifier summary:")
	fmt.Printf("  Games total:        %d\n", stats.GamesTotal)
	fmt.Printf("  Games processed:    %d\n", stats.GamesProcessed)
	fmt.Printf("  Auto-approved:      %d\n", stats.AutoApproved)
	fmt.Printf("  Needs review:       %d\n", stats.NeedsReview)
	fmt.Printf("  None:               %d\n", stats.None)
	fmt.Printf("  No cached search:   %d\n", stats.NoCachedSearch)
	fmt.Printf("  Candidates seen:    %d\n", stats.CandidatesSeen)
	fmt.Printf("  Candidates kept:    %d\n", stats.CandidatesKept)
	fmt.Printf("  Auto-approve score: %d\n", stats.AutoApproveScore)
	fmt.Printf("  Output:             %s\n", stats.OutputFile)
}
