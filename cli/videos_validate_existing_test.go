package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

func TestRunVideosValidateExistingRemovesUnavailableWithWatchChecker(t *testing.T) {
	prevCacheFactory := videosCacheFactory
	prevWatchCheckerFactory := videosWatchCheckerFactory
	prevCacheFile := videosValidateExistingCacheFile
	prevHeadless := videosValidateExistingHeadless
	prevDryRun := videosValidateExistingDryRun
	prevYouTubeAPIKey := videosValidateExistingYouTubeAPIKey
	defer func() {
		videosCacheFactory = prevCacheFactory
		videosWatchCheckerFactory = prevWatchCheckerFactory
		videosValidateExistingCacheFile = prevCacheFile
		videosValidateExistingHeadless = prevHeadless
		videosValidateExistingDryRun = prevDryRun
		videosValidateExistingYouTubeAPIKey = prevYouTubeAPIKey
	}()

	cache := &stubVideosCache{cf: &videos.CacheFile{Items: map[string]*videos.CacheItem{
		"game1": {
			Status: videos.StatusSelected,
			Videos: []videos.Video{
				{ID: "keep1", URL: "https://www.youtube.com/watch?v=keep1"},
				{ID: "gone1", URL: "https://www.youtube.com/watch?v=gone1"},
			},
		},
	}}}
	checker := &stubCmdWatchChecker{exists: map[string]bool{"keep1": true, "gone1": false}}
	videosCacheFactory = func(path string) videos.Cache { return cache }
	videosWatchCheckerFactory = func(headless bool) videos.WatchAvailabilityChecker { return checker }
	videosValidateExistingCacheFile = "ignored.json"
	videosValidateExistingHeadless = true
	videosValidateExistingDryRun = false
	videosValidateExistingYouTubeAPIKey = ""

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runVideosValidateExisting(cmd, nil); err != nil {
		t.Fatalf("runVideosValidateExisting error: %v", err)
	}
	if !cache.saved {
		t.Fatal("expected cache to be saved after removal")
	}
	item := cache.cf.Items["game1"]
	if item == nil || len(item.Videos) != 1 || item.Videos[0].ID != "keep1" {
		t.Fatalf("expected only available video to remain, got %+v", item)
	}
	text := out.String()
	for _, want := range []string{"removed video for game game1", "Checked 2 cached videos with watch-page; removed 1 unavailable videos."} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q: %s", want, text)
		}
	}
}
