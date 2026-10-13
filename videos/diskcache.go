package videos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// DiskCacheManager handles all persistent caches for the pipeline.
// Layout (relative to Paths.CacheDir):
//
//	cache/videos/cache/channels/{channel_id}.json
//	cache/videos/cache/search/{game_id}.json
//	cache/videos/cache/videos/{video_id}.json
type DiskCacheManager struct {
	BaseDir string
	nowFunc func() time.Time
}

// NewDiskCacheManager creates a cache manager rooted at the given directory.
func NewDiskCacheManager(baseDir string) *DiskCacheManager {
	return &DiskCacheManager{BaseDir: baseDir, nowFunc: time.Now}
}

// --- Channel cache ---

// ChannelCacheEntry holds the cached playlist items for one channel.
type ChannelCacheEntry struct {
	ChannelID  string                    `json:"channel_id"`
	Title      string                    `json:"title"`
	FetchedAt  time.Time                 `json:"fetched_at"`
	TotalItems int                       `json:"total_items"`
	Videos     []youtubeapi.PlaylistItem `json:"videos"`
}

// LoadChannelCache reads cached channel data if available.
func (m *DiskCacheManager) LoadChannelCache(channelID string) (*ChannelCacheEntry, error) {
	path := m.channelCachePath(channelID)
	return loadJSON[ChannelCacheEntry](path)
}

// SaveChannelCache writes channel data to cache.
func (m *DiskCacheManager) SaveChannelCache(entry *ChannelCacheEntry) error {
	if entry == nil {
		return fmt.Errorf("channel cache entry is nil")
	}
	path := m.channelCachePath(entry.ChannelID)
	return saveJSON(path, entry)
}

// HasChannelCache checks if a channel cache file exists.
func (m *DiskCacheManager) HasChannelCache(channelID string) bool {
	path := m.channelCachePath(channelID)
	_, err := os.Stat(path)
	return err == nil
}

func (m *DiskCacheManager) channelCachePath(channelID string) string {
	return filepath.Join(m.BaseDir, "channels", channelID+".json")
}

// --- Search cache ---

// SearchCacheEntry holds cached search results for one game.
type SearchCacheEntry struct {
	GameID    string    `json:"game_id"`
	Query     string    `json:"query"`
	FetchedAt time.Time `json:"fetched_at"`
	VideoIDs  []string  `json:"video_ids"`
}

// LoadSearchCache reads cached search results for a game.
func (m *DiskCacheManager) LoadSearchCache(gameID string) (*SearchCacheEntry, error) {
	path := m.searchCachePath(gameID)
	return loadJSON[SearchCacheEntry](path)
}

// SaveSearchCache writes search results to cache.
func (m *DiskCacheManager) SaveSearchCache(entry *SearchCacheEntry) error {
	if entry == nil {
		return fmt.Errorf("search cache entry is nil")
	}
	path := m.searchCachePath(entry.GameID)
	return saveJSON(path, entry)
}

// HasSearchCache checks if search results are cached for a game.
func (m *DiskCacheManager) HasSearchCache(gameID string) bool {
	path := m.searchCachePath(gameID)
	_, err := os.Stat(path)
	return err == nil
}

func (m *DiskCacheManager) searchCachePath(gameID string) string {
	// Base keeps a crafted ID such as "../../x" inside the cache directory.
	return filepath.Join(m.BaseDir, "search", filepath.Base(gameID)+".json")
}

// --- Video details cache ---

// VideoCacheEntry holds cached details for one YouTube video.
type VideoCacheEntry struct {
	VideoID   string                    `json:"video_id"`
	FetchedAt time.Time                 `json:"fetched_at"`
	Video     youtubeapi.VideoCandidate `json:"video"`
}

// LoadVideoCache reads cached video details.
func (m *DiskCacheManager) LoadVideoCache(videoID string) (*VideoCacheEntry, error) {
	path := m.videoCachePath(videoID)
	return loadJSON[VideoCacheEntry](path)
}

// SaveVideoCache writes video details to cache.
func (m *DiskCacheManager) SaveVideoCache(entry *VideoCacheEntry) error {
	if entry == nil {
		return fmt.Errorf("video cache entry is nil")
	}
	path := m.videoCachePath(entry.VideoID)
	return saveJSON(path, entry)
}

// HasVideoCache checks if video details are cached.
func (m *DiskCacheManager) HasVideoCache(videoID string) bool {
	path := m.videoCachePath(videoID)
	_, err := os.Stat(path)
	return err == nil
}

// LoadMultiVideoCache loads cached video details for multiple IDs at once.
// Returns the ones found in cache and the list of missing IDs.
func (m *DiskCacheManager) LoadMultiVideoCache(videoIDs []string) (found []youtubeapi.VideoCandidate, missing []string) {
	for _, id := range videoIDs {
		entry, err := m.LoadVideoCache(id)
		if err != nil || entry == nil {
			missing = append(missing, id)
			continue
		}
		found = append(found, entry.Video)
	}
	return found, missing
}

// SaveMultiVideoCache saves multiple video details to cache.
func (m *DiskCacheManager) SaveMultiVideoCache(candidates []youtubeapi.VideoCandidate) error {
	now := m.now()
	for _, c := range candidates {
		entry := &VideoCacheEntry{
			VideoID:   c.ID,
			FetchedAt: now,
			Video:     c,
		}
		if err := m.SaveVideoCache(entry); err != nil {
			return fmt.Errorf("save video cache %s: %w", c.ID, err)
		}
	}
	return nil
}

func (m *DiskCacheManager) videoCachePath(videoID string) string {
	return filepath.Join(m.BaseDir, "videos", videoID+".json")
}

func (m *DiskCacheManager) now() time.Time {
	if m.nowFunc != nil {
		return m.nowFunc()
	}
	return time.Now()
}

// --- Generic JSON helpers ---

func loadJSON[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &v, nil
}

func saveJSON[T any](path string, v *T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}
