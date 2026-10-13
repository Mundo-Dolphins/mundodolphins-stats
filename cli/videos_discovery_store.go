package cli

import (
	"path/filepath"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

func configureVideosDiscoveryStore(service *videos.Service, cache videos.Cache) (func(), error) {
	if service == nil {
		return func() {}, nil
	}
	if _, ok := cache.(*videos.FileCache); !ok {
		return func() {}, nil
	}
	path := strings.TrimSpace(videosDiscoveryDB)
	if strings.EqualFold(path, "off") || strings.EqualFold(path, "none") || strings.EqualFold(path, "false") {
		return func() {}, nil
	}
	if path == "" {
		path = defaultVideosDiscoveryDBPath(videosCacheFile)
	}
	store, err := videos.NewSQLiteCandidateStore(path)
	if err != nil {
		return nil, err
	}
	service.CandidateStore = store
	return func() { _ = store.Close() }, nil
}

func defaultVideosDiscoveryDBPath(cacheFile string) string {
	dir := filepath.Dir(strings.TrimSpace(cacheFile))
	if dir == "." || dir == "" {
		dir = filepath.Join("cache", "videos")
	}
	return filepath.Join(dir, "discovery.sqlite")
}
