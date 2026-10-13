package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtube"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
	"github.com/spf13/cobra"
)

var videosDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover YouTube videos for games",
	RunE:  runVideosDiscover,
}

var (
	videosYear             int
	videosFromYear         int
	videosToYear           int
	videosProcessedDir     string
	videosCacheFile        string
	videosMaxResults       int
	videosMinDuration      time.Duration
	videosConcurrency      int
	videosHeadless         bool
	videosForce            bool
	videosRetryNone        bool
	videosValidateExisting bool
	videosNonInteractive   bool
	videosScoreThreshold   int
	videosAutoMax          int
	videosYouTubeAPIKey    string
	videosDiscoveryDB      string
)

var (
	videosSearcherFactory     = func(bool) videos.Searcher { return noVideoSearcher{} }
	videosCacheFactory        = func(path string) videos.Cache { return videos.NewFileCache(path) }
	videosLoaderFactory       = func(dir string) videos.GameLoader { return videos.NewJSONGameLoader(dir) }
	videosWatchCheckerFactory = func(bool) videos.WatchAvailabilityChecker { return nil }
	videosDetailsFetcher      func(ctx context.Context, videoID string) (videos.VideoDetails, error)
	youtubeAPIFactory         = func(apiKey string) *youtubeapi.Client {
		return youtubeapi.NewClient(apiKey, &http.Client{Timeout: 15 * time.Second})
	}
)

func init() {
	videosDiscoverCmd.Flags().IntVar(&videosYear, "year", 0, "Season year to discover")
	videosDiscoverCmd.Flags().IntVar(&videosFromYear, "from-year", 0, "From season year")
	videosDiscoverCmd.Flags().IntVar(&videosToYear, "to-year", 0, "To season year")
	videosDiscoverCmd.Flags().StringVar(&videosProcessedDir, "processed-dir", "processed/games", "Directory with processed games JSON")
	videosDiscoverCmd.Flags().StringVar(&videosCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosDiscoverCmd.Flags().IntVar(&videosMaxResults, "max-results", 15, "Max search results per game")
	videosDiscoverCmd.Flags().DurationVar(&videosMinDuration, "min-duration", 5*time.Minute, "Minimum video duration")
	videosDiscoverCmd.Flags().IntVar(&videosConcurrency, "concurrency", 5, "Concurrency for non-interactive mode")
	videosDiscoverCmd.Flags().BoolVar(&videosHeadless, "headless", true, "Run the registered browser-based searcher in headless mode")
	videosDiscoverCmd.Flags().BoolVar(&videosForce, "force", false, "Force re-discover even if cached")
	videosDiscoverCmd.Flags().BoolVar(&videosRetryNone, "retry-none", true, "Retry games marked as none")
	videosDiscoverCmd.Flags().BoolVar(&videosValidateExisting, "validate-existing", true, "Check cached YouTube videos with the registered availability checker before skipping games")
	videosDiscoverCmd.Flags().BoolVar(&videosNonInteractive, "non-interactive", false, "Select videos automatically")
	videosDiscoverCmd.Flags().IntVar(&videosScoreThreshold, "score-threshold", 60, "Minimum score to auto-select")
	videosDiscoverCmd.Flags().IntVar(&videosAutoMax, "auto-max", 1, "Max videos to auto-select")
	videosDiscoverCmd.Flags().StringVar(&videosYouTubeAPIKey, "youtube-api-key", "", "YouTube Data API v3 key (overrides YOUTUBE_API_KEY)")
	videosDiscoverCmd.Flags().StringVar(&videosDiscoveryDB, "discovery-db", "", "SQLite candidate discovery cache path (default: alongside --cache-file; set off to disable)")
}

func runVideosDiscover(cmd *cobra.Command, args []string) error {
	if videosYear == 0 && (videosFromYear == 0 || videosToYear == 0) {
		return fmt.Errorf("must provide --year or --from-year/--to-year")
	}
	if videosYear == 0 && videosFromYear > videosToYear {
		return fmt.Errorf("from-year must be <= to-year")
	}
	searcher := videosSearcherFactory(videosHeadless)
	cache := videosCacheFactory(videosCacheFile)
	loader := videosLoaderFactory(videosProcessedDir)

	service := videos.Service{
		Details:    videosDetailsFetcher,
		Searcher:   searcher,
		Cache:      cache,
		GameLoader: loader,
		Scorer:     videos.NewScorer(int(videosMinDuration.Seconds()), videosScoreThreshold),
	}
	closeDiscoveryStore, err := configureVideosDiscoveryStore(&service, cache)
	if err != nil {
		return err
	}
	defer closeDiscoveryStore()
	if videosValidateExisting {
		checker := videosWatchCheckerFactory(videosHeadless)
		if checker != nil {
			defer checker.Close()
			service.WatchChecker = checker
		}
	}
	if apiKey := resolveYouTubeAPIKey(videosYouTubeAPIKey); apiKey != "" {
		ytClient := youtubeAPIFactory(apiKey)
		service.APIDiscoverer = func(ctx context.Context, game videos.GameRef, maxResults int, minDurationSec int) ([]videos.Candidate, error) {
			criteria := youtube.CriteriaFromGame(game)
			items, err := youtube.DiscoverViaAPI(ctx, ytClient, criteria, maxResults, time.Duration(minDurationSec)*time.Second)
			if err != nil {
				return nil, err
			}
			out := make([]videos.Candidate, 0, len(items))
			for _, item := range items {
				embeddable := item.Embeddable
				out = append(out, videos.Candidate{
					VideoID:      item.ID,
					URL:          item.URL,
					Title:        item.Title,
					Channel:      item.Channel,
					DurationSec:  int(item.Duration.Seconds()),
					DurationText: videos.FormatDuration(item.Duration),
					Snippet:      item.Description,
					Score:        item.Score,
					Embeddable:   &embeddable,
				})
			}
			return out, nil
		}
	}
	if !videosNonInteractive {
		service.Selector = &videos.Selector{In: os.Stdin, Out: os.Stdout}
	}

	cfg := videos.DiscoverConfig{
		Year:             videosYear,
		FromYear:         videosFromYear,
		ToYear:           videosToYear,
		ProcessedDir:     videosProcessedDir,
		CacheFile:        videosCacheFile,
		MaxResults:       videosMaxResults,
		MinDurationSec:   int(videosMinDuration.Seconds()),
		Concurrency:      videosConcurrency,
		Headless:         videosHeadless,
		Force:            videosForce,
		RetryNone:        videosRetryNone,
		ValidateExisting: videosValidateExisting,
		NonInteractive:   videosNonInteractive,
		ScoreThreshold:   videosScoreThreshold,
		AutoMax:          videosAutoMax,
	}

	return service.Discover(context.Background(), cfg)
}

func resolveYouTubeAPIKey(flagValue string) string {
	if key := strings.TrimSpace(flagValue); key != "" {
		return key
	}
	return strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY"))
}
