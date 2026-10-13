package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

var videosReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "List video candidates that need editorial review",
	RunE:  runVideosReview,
}

var (
	videosReviewCacheFile string
	videosReviewFormat    string
	videosReviewLimit     int
)

func init() {
	videosReviewCmd.Flags().StringVar(&videosReviewCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosReviewCmd.Flags().StringVar(&videosReviewFormat, "format", "json", "Output format: json or table")
	videosReviewCmd.Flags().IntVar(&videosReviewLimit, "limit", 0, "Limit number of review items")
	videosCmd.AddCommand(videosReviewCmd)
}

func runVideosReview(cmd *cobra.Command, args []string) error {
	cf, err := videos.NewFileCache(videosReviewCacheFile).Load()
	if err != nil {
		return err
	}
	queue := videos.BuildReviewQueue(cf)
	if videosReviewLimit > 0 && videosReviewLimit < len(queue.Items) {
		queue.Items = queue.Items[:videosReviewLimit]
	}

	switch strings.ToLower(strings.TrimSpace(videosReviewFormat)) {
	case "", "json":
		data, err := json.MarshalIndent(queue, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal review queue: %w", err)
		}
		cmd.Println(string(data))
		return nil
	case "table":
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "GAME_ID\tRISK\tSCORE\tCONFIDENCE\tDATE\tVIDEO_ID\tTITLE")
		for _, item := range queue.Items {
			fmt.Fprintf(out, "%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
				item.GameID,
				item.RiskScore,
				item.Score,
				item.Confidence,
				item.MatchedDate,
				item.VideoID,
				item.Title,
			)
		}
		return nil
	default:
		return fmt.Errorf("unsupported review format %q", videosReviewFormat)
	}
}
