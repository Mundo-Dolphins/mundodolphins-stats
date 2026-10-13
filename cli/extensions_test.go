package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/nflcom"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

type stubDownloader struct{}

func (stubDownloader) EnsureDownloaded(context.Context, string, string) error { return nil }

type stubGamebookParser struct{}

func (stubGamebookParser) ParseGamebook(string) (*nflcom.GamebookData, error) { return nil, nil }

func TestRootIsTheCommandRegisteredByTheLibrary(t *testing.T) {
	if Root() != rootCmd {
		t.Fatal("Root must return the command the library registers its subcommands on")
	}
}

func TestExtensionSettersStoreTheRegisteredImplementations(t *testing.T) {
	oldD, oldG := assetsDownloader, gamebookParser
	defer func() { assetsDownloader, gamebookParser = oldD, oldG }()

	SetAssetsDownloader(stubDownloader{})
	SetGamebookParser(stubGamebookParser{})
	if assetsDownloader == nil || gamebookParser == nil {
		t.Fatal("setters must store the implementations")
	}
}

func TestVideoSearcherDefaultsToAnExplicitError(t *testing.T) {
	_, err := noVideoSearcher{}.Search(context.Background(), "q", 5)
	if !errors.Is(err, ErrNoVideoSearcher) {
		t.Fatalf("expected ErrNoVideoSearcher, got %v", err)
	}
	if videosWatchCheckerFactory(true) != nil {
		t.Fatal("no availability checker is registered by default")
	}
}

func TestSetVideoSearcherIgnoresNil(t *testing.T) {
	old := videosSearcherFactory
	defer func() { videosSearcherFactory = old }()

	SetVideoSearcher(nil)
	if _, ok := videosSearcherFactory(false).(noVideoSearcher); !ok {
		t.Fatal("a nil factory must not replace the default")
	}
	custom := func(bool) videos.Searcher { return stubSearcher{} }
	SetVideoSearcher(custom)
	if _, ok := videosSearcherFactory(false).(stubSearcher); !ok {
		t.Fatal("a registered factory must be used")
	}
}

type stubSearcher struct{}

func (stubSearcher) Search(context.Context, string, int) ([]videos.Candidate, error) {
	return nil, nil
}
