package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/Mundo-Dolphins/mundodolphins-stats/assets"
	"github.com/Mundo-Dolphins/mundodolphins-stats/pagejson"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

// ErrNoVideoSearcher is returned when a command needs to search YouTube pages
// and no searcher was registered.
var ErrNoVideoSearcher = errors.New("no web video searcher configured: pass --youtube-api-key (or set YOUTUBE_API_KEY), or register one with cli.SetVideoSearcher")

type noVideoSearcher struct{}

func (noVideoSearcher) Search(context.Context, string, int) ([]videos.Candidate, error) {
	return nil, ErrNoVideoSearcher
}

// Root returns the root command. This package is meant for main packages: a
// program can add its own subcommands to the result before calling Execute.
func Root() *cobra.Command { return rootCmd }

// SetVideoSearcher registers how the videos commands search for candidates
// without the YouTube Data API. The library ships no implementation: scraping
// YouTube pages is the caller's responsibility.
func SetVideoSearcher(f func(headless bool) videos.Searcher) {
	if f != nil {
		videosSearcherFactory = f
	}
}

// SetWatchChecker registers how the videos commands check that a video is still
// available (optional).
func SetWatchChecker(f func(headless bool) videos.WatchAvailabilityChecker) {
	if f != nil {
		videosWatchCheckerFactory = f
	}
}

// SetVideoDetailsFetcher registers how the title, description and duration of
// a video are read when the search result lacks them (optional).
func SetVideoDetailsFetcher(f func(ctx context.Context, videoID string) (videos.VideoDetails, error)) {
	videosDetailsFetcher = f
}

var (
	assetsDownloader assets.Downloader
	gamebookParser   pagejson.GamebookParser
)

// SetAssetsDownloader registers how remote files (team logos) are downloaded
// by `generate hugo-data --ensure-assets`. Without one, that flag is an error
// when a download is needed.
func SetAssetsDownloader(d assets.Downloader) { assetsDownloader = d }

// SetGamebookParser registers how game book PDFs are read by the commands that
// build game pages (optional).
func SetGamebookParser(p pagejson.GamebookParser) { gamebookParser = p }
