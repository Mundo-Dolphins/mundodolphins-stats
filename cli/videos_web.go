package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/Mundo-Dolphins/mundodolphins-stats/youtube"
	"github.com/spf13/cobra"
)

var videosWebCmd = &cobra.Command{
	Use:   "web",
	Short: "Run an interactive web UI for YouTube video discovery",
	RunE:  runVideosWeb,
}

var videosWebAddr string
var videosWebAutoApproveScore int

var videosWebServe = func(server *http.Server, web *videos.WebServer) error {
	return server.ListenAndServe()
}

func init() {
	videosWebCmd.Flags().StringVar(&videosWebAddr, "addr", "127.0.0.1:8787", "Address for the local web server")
	videosWebCmd.Flags().IntVar(&videosYear, "year", 0, "Season year to discover")
	videosWebCmd.Flags().IntVar(&videosFromYear, "from-year", 0, "From season year")
	videosWebCmd.Flags().IntVar(&videosToYear, "to-year", 0, "To season year")
	videosWebCmd.Flags().StringVar(&videosProcessedDir, "processed-dir", "processed/games", "Directory with processed game JSON files")
	videosWebCmd.Flags().StringVar(&videosCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosWebCmd.Flags().IntVar(&videosMaxResults, "max-results", 15, "Max search results per game")
	videosWebCmd.Flags().DurationVar(&videosMinDuration, "min-duration", 5*time.Minute, "Minimum video duration")
	videosWebCmd.Flags().BoolVar(&videosHeadless, "headless", true, "Run Rod in headless mode")
	videosWebCmd.Flags().IntVar(&videosScoreThreshold, "score-threshold", 60, "Minimum score for candidate relevance")
	videosWebCmd.Flags().IntVar(&videosWebAutoApproveScore, "auto-approve-score", 130, "Minimum strict classifier score for web autopilot approval")
	videosWebCmd.Flags().StringVar(&videosYouTubeAPIKey, "youtube-api-key", "", "YouTube Data API v3 key (overrides YOUTUBE_API_KEY)")
	videosWebCmd.Flags().StringVar(&videosDiscoveryDB, "discovery-db", "", "SQLite candidate discovery cache path (default: alongside --cache-file; set off to disable)")
	videosCmd.AddCommand(videosWebCmd)
}

func runVideosWeb(cmd *cobra.Command, args []string) error {
	if videosYear == 0 && videosFromYear > 0 && videosToYear > 0 && videosFromYear > videosToYear {
		return fmt.Errorf("from-year must be <= to-year")
	}

	service := videos.Service{
		Details:    videosDetailsFetcher,
		Searcher:   videosSearcherFactory(videosHeadless),
		Cache:      videosCacheFactory(videosCacheFile),
		GameLoader: videosLoaderFactory(videosProcessedDir),
		Scorer:     videos.NewScorer(int(videosMinDuration.Seconds()), videosScoreThreshold),
		Now:        time.Now,
	}
	if checker := videosWatchCheckerFactory(videosHeadless); checker != nil {
		service.WatchChecker = checker
		defer checker.Close()
	}
	closeDiscoveryStore, err := configureVideosDiscoveryStore(&service, service.Cache)
	if err != nil {
		return err
	}
	defer closeDiscoveryStore()
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

	web := &videos.WebServer{
		Service: &service,
		Config: videos.WebConfig{
			Year:             videosYear,
			FromYear:         videosFromYear,
			ToYear:           videosToYear,
			ProcessedDir:     videosProcessedDir,
			CacheFile:        videosCacheFile,
			MaxResults:       videosMaxResults,
			MinDurationSec:   int(videosMinDuration.Seconds()),
			ScoreThreshold:   videosScoreThreshold,
			AutoApproveScore: videosWebAutoApproveScore,
		},
	}

	cmd.Printf("Videos discovery web UI: http://%s\n", videosWebAddr)
	cmd.Printf("Cache: %s\n", videosCacheFile)
	server := &http.Server{
		Addr:              videosWebAddr,
		Handler:           web.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return videosWebServe(server, web)
}
