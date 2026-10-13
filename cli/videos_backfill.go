package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
	"github.com/spf13/cobra"
)

var videosBackfillCmd = &cobra.Command{
	Use:   "backfill-metadata",
	Short: "Backfill cached video metadata such as channel_id and role",
	RunE:  runVideosBackfill,
}

var (
	videosBackfillCacheFile string
	videosBackfillVideosDir string
	videosBackfillDryRun    bool
	videosBackfillMaxVideos int
)

func init() {
	videosBackfillCmd.Flags().StringVar(&videosBackfillCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosBackfillCmd.Flags().StringVar(&videosBackfillVideosDir, "videos-dir", "cache/videos", "Root directory for videos cache tree")
	videosBackfillCmd.Flags().BoolVar(&videosBackfillDryRun, "dry-run", true, "Preview changes without persisting")
	videosBackfillCmd.Flags().IntVar(&videosBackfillMaxVideos, "max-videos", 0, "Maximum unique videos to backfill (0=all)")
	videosCmd.AddCommand(videosBackfillCmd)
}

func runVideosBackfill(cmd *cobra.Command, args []string) error {
	apiKey := resolveYouTubeAPIKey("")
	var client *youtubeapi.Client
	if apiKey != "" {
		client = youtubeapi.NewClient(apiKey, &http.Client{Timeout: 30 * time.Second})
	}
	stats, err := videos.BackfillVideoMetadata(context.Background(), client, videos.MetadataBackfillConfig{
		CacheFile: videosBackfillCacheFile,
		VideosDir: videosBackfillVideosDir,
		DryRun:    videosBackfillDryRun,
		MaxVideos: videosBackfillMaxVideos,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Metadata backfill: scanned=%d missing=%d cache_hits=%d api_fetched=%d updated_videos=%d updated_items=%d channels_synced=%d dry_run=%v\n",
		stats.Scanned, stats.Missing, stats.CacheHits, stats.APIFetched, stats.UpdatedVideos, stats.UpdatedItems, stats.ChannelsSynced, videosBackfillDryRun)
	return nil
}
