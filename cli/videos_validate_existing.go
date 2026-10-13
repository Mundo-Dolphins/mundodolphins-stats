package cli

import (
	"fmt"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
	"github.com/spf13/cobra"
)

var videosValidateExistingCmd = &cobra.Command{
	Use:   "validate-existing",
	Short: "Check cached selected YouTube videos still exist",
	RunE:  runVideosValidateExisting,
}

var (
	videosValidateExistingCacheFile     string
	videosValidateExistingHeadless      bool
	videosValidateExistingDryRun        bool
	videosValidateExistingYouTubeAPIKey string
)

func init() {
	videosValidateExistingCmd.Flags().StringVar(&videosValidateExistingCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosValidateExistingCmd.Flags().BoolVar(&videosValidateExistingHeadless, "headless", true, "Run Rod in headless mode when using the watch-page fallback")
	videosValidateExistingCmd.Flags().BoolVar(&videosValidateExistingDryRun, "dry-run", false, "Preview removals without persisting")
	videosValidateExistingCmd.Flags().StringVar(&videosValidateExistingYouTubeAPIKey, "youtube-api-key", "", "YouTube Data API v3 key (overrides YOUTUBE_API_KEY)")
	videosCmd.AddCommand(videosValidateExistingCmd)
}

func runVideosValidateExisting(cmd *cobra.Command, args []string) error {
	cache := videosCacheFactory(videosValidateExistingCacheFile)
	cf, err := cache.Load()
	if err != nil {
		return err
	}

	checked := countValidatableCachedVideos(cf)
	removed, source, err := validateExistingCachedVideos(cmd, cf)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	for _, msg := range removed {
		fmt.Fprintln(out, msg)
	}
	if len(removed) > 0 && !videosValidateExistingDryRun {
		if err := cache.Save(cf); err != nil {
			return err
		}
	}
	if len(removed) > 0 && videosValidateExistingDryRun {
		fmt.Fprintln(out, "Dry run: cache not updated.")
	}
	fmt.Fprintf(out, "Checked %d cached videos with %s; removed %d unavailable videos.\n", checked, source, len(removed))
	return nil
}

func validateExistingCachedVideos(cmd *cobra.Command, cf *videos.CacheFile) ([]string, string, error) {
	ctx := cmd.Context()
	if apiKey := resolveYouTubeAPIKey(videosValidateExistingYouTubeAPIKey); apiKey != "" {
		removed, err := videos.ValidateExistingVideos(ctx, cf, youtubeAPIFactory(apiKey))
		if err == nil {
			return removed, "youtube-api", nil
		}
		if !youtubeapi.IsQuotaOrRateLimit(err) {
			return nil, "youtube-api", err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "YouTube API quota/rate limit reached; falling back to watch-page checker: %v\n", err)
	}

	checker := videosWatchCheckerFactory(videosValidateExistingHeadless)
	if checker == nil {
		return nil, "watch-page", fmt.Errorf("watch checker is nil")
	}
	defer checker.Close()

	removed, err := videos.ValidateExistingVideosWithChecker(ctx, cf, checker, time.Now)
	return removed, "watch-page", err
}

func countValidatableCachedVideos(cf *videos.CacheFile) int {
	if cf == nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, item := range cf.Items {
		if item == nil || (!videos.IsPublishableStatus(item.Status) && item.Status != videos.StatusCandidate) {
			continue
		}
		for _, video := range item.Videos {
			if video.ID == "" {
				continue
			}
			seen[video.ID] = struct{}{}
		}
	}
	return len(seen)
}
