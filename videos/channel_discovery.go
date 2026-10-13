package videos

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

const minQuotaForDiscovery = 200

var yearInTextRe = regexp.MustCompile(`\b(19[5-9]\d|20[0-4]\d)\b`)

// ChannelBlacklistFile stores channels that should not be discovered again.
type ChannelBlacklistFile struct {
	Channels []BlacklistedChannel `json:"channels"`
}

// BlacklistedChannel captures channel-level rejection reasons.
type BlacklistedChannel struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Reason      string  `json:"reason"`
	ValidRatio  float64 `json:"valid_ratio"`
	ValidCount  int     `json:"valid_count"`
	TotalSample int     `json:"total_sample"`
	AddedAt     string  `json:"added_at"`
}

// ChannelDiscoveryCacheEntry avoids re-validating the same channel repeatedly.
type ChannelDiscoveryCacheEntry struct {
	ChannelID   string    `json:"channel_id"`
	ChannelName string    `json:"channel_name"`
	ValidatedAt time.Time `json:"validated_at"`
	Accepted    bool      `json:"accepted"`
	ValidCount  int       `json:"valid_count"`
	TotalSample int       `json:"total_sample"`
	Ratio       float64   `json:"ratio"`
	Reason      string    `json:"reason"`
}

// ChannelDiscoveryManager validates and promotes channels discovered from search matches.
type ChannelDiscoveryManager struct {
	Paths  Paths
	Cache  *DiskCacheManager
	Client *youtubeapi.Client
	Quota  *QuotaManager
	now    func() time.Time
}

func NewChannelDiscoveryManager(paths Paths, cache *DiskCacheManager, client *youtubeapi.Client, quota *QuotaManager) *ChannelDiscoveryManager {
	return &ChannelDiscoveryManager{
		Paths:  paths,
		Cache:  cache,
		Client: client,
		Quota:  quota,
		now:    time.Now,
	}
}

func (m *ChannelDiscoveryManager) DiscoverFromSearchMatches(ctx context.Context, cf *CacheFile, reg *ChannelRegistryFile, previouslySelected map[string]bool) (map[string]bool, error) {
	acceptedNew := make(map[string]bool)
	if cf == nil || reg == nil {
		return acceptedNew, nil
	}
	if m.Quota == nil || m.Client == nil || m.Cache == nil {
		return acceptedNew, fmt.Errorf("discovery manager not fully initialized")
	}

	if m.Quota.Remaining() < minQuotaForDiscovery {
		log.Printf("DISCOVERY: skipped (quota remaining=%d < %d)", m.Quota.Remaining(), minQuotaForDiscovery)
		return acceptedNew, nil
	}

	known := make(map[string]bool, len(reg.Channels))
	for _, ch := range reg.Channels {
		known[ch.ID] = true
	}

	blacklist, err := m.loadBlacklist()
	if err != nil {
		return acceptedNew, err
	}
	blacklisted := make(map[string]bool, len(blacklist.Channels))
	for _, b := range blacklist.Channels {
		blacklisted[b.ID] = true
	}

	candidates := m.collectNewSearchChannels(cf, previouslySelected)
	if len(candidates) == 0 {
		return acceptedNew, nil
	}

	sort.Strings(candidates)
	for _, channelID := range candidates {
		if channelID == "" || known[channelID] || blacklisted[channelID] || IsBlockedChannel(channelID) {
			continue
		}

		if entry, _ := m.loadDiscoveryCache(channelID); entry != nil {
			if entry.Accepted {
				acceptedNew[channelID] = true
				log.Printf("DISCOVERY CACHE HIT: accepted channel %s (ratio=%.2f)", channelID, entry.Ratio)
			} else {
				log.Printf("DISCOVERY CACHE HIT: rejected channel %s (%s)", channelID, entry.Reason)
			}
			continue
		}

		log.Printf("VALIDATING CHANNEL %s", channelID)
		result, err := m.validateChannel(ctx, channelID)
		if err != nil {
			log.Printf("WARN: channel validation failed for %s: %v", channelID, err)
			continue
		}
		if err := m.saveDiscoveryCache(result); err != nil {
			log.Printf("WARN: save channel discovery cache for %s: %v", channelID, err)
		}

		if result.Accepted {
			acceptedNew[channelID] = true
			log.Printf("CHANNEL ACCEPTED: %s (valid=%d ratio=%.2f)", channelID, result.ValidCount, result.Ratio)
			continue
		}

		log.Printf("CHANNEL REJECTED: %s (valid=%d total=%d ratio=%.2f)", channelID, result.ValidCount, result.TotalSample, result.Ratio)
		m.addToBlacklist(blacklist, result)
	}

	if err := m.saveBlacklist(blacklist); err != nil {
		return acceptedNew, err
	}

	return acceptedNew, nil
}

func (m *ChannelDiscoveryManager) collectNewSearchChannels(cf *CacheFile, previouslySelected map[string]bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range cf.Items {
		if item == nil || !IsPublishableStatus(item.Status) || item.AutoScoring == nil {
			continue
		}
		if item.AutoScoring.Source != "search" && item.AutoScoring.Source != "search-cached" {
			continue
		}
		if len(item.Videos) == 0 {
			continue
		}
		v := item.Videos[0]
		if v.ID == "" || v.ChannelID == "" || previouslySelected[v.ID] || seen[v.ChannelID] {
			continue
		}
		seen[v.ChannelID] = true
		out = append(out, v.ChannelID)
	}
	return out
}

func (m *ChannelDiscoveryManager) validateChannel(ctx context.Context, channelID string) (*ChannelDiscoveryCacheEntry, error) {
	if !m.Quota.TryConsume(QuotaCostChannelsList) {
		return nil, fmt.Errorf("discovery quota exhausted for channels.list")
	}

	info, err := m.Client.GetChannelInfo(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("get channel info: %w", err)
	}

	playlistIDs := make([]string, 0, 100)
	pageToken := ""
	pagesToSample := 2
	for i := 0; i < pagesToSample; i++ {
		if !m.Quota.TryConsume(QuotaCostPlaylistItems) {
			break
		}
		page, err := m.Client.ListPlaylistPage(ctx, info.UploadsPlaylistID, pageToken, 50)
		if err != nil {
			return nil, fmt.Errorf("list playlist page: %w", err)
		}
		for _, it := range page.Items {
			if it.VideoID != "" {
				playlistIDs = append(playlistIDs, it.VideoID)
			}
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	if len(playlistIDs) == 0 {
		return &ChannelDiscoveryCacheEntry{
			ChannelID:   channelID,
			ChannelName: info.Title,
			ValidatedAt: m.now(),
			Accepted:    false,
			Reason:      "no sample videos",
		}, nil
	}

	batches := (len(playlistIDs) + 49) / 50
	if !m.Quota.TryConsume(batches * QuotaCostVideosList) {
		return nil, fmt.Errorf("discovery quota exhausted for videos.list")
	}

	videos, err := m.Client.HydrateVideos(ctx, playlistIDs)
	if err != nil {
		return nil, fmt.Errorf("hydrate videos: %w", err)
	}

	valid := 0
	for _, v := range videos {
		if isValidDiscoverySample(v) {
			valid++
		}
	}
	total := len(videos)
	ratio := 0.0
	if total > 0 {
		ratio = float64(valid) / float64(total)
	}
	accepted := valid >= 2 && ratio > 0.3
	reason := "valid ratio too low"
	if accepted {
		reason = "accepted"
	}

	return &ChannelDiscoveryCacheEntry{
		ChannelID:   channelID,
		ChannelName: info.Title,
		ValidatedAt: m.now(),
		Accepted:    accepted,
		ValidCount:  valid,
		TotalSample: total,
		Ratio:       ratio,
		Reason:      reason,
	}, nil
}

func isValidDiscoverySample(v youtubeapi.VideoCandidate) bool {
	combined := strings.ToLower(v.Title + " " + v.Description)
	for _, word := range bannedTitleWords {
		if strings.Contains(combined, word) {
			return false
		}
	}
	if !strings.Contains(combined, "full game") && !strings.Contains(combined, "full match") {
		return false
	}
	if int(v.Duration.Seconds()) < 600 {
		return false
	}
	if len(yearInTextRe.FindAllString(combined, -1)) == 0 {
		return false
	}
	teams := countDistinctNFLTeams(combined, GameMetadata{})
	if teams < 2 || teams > 2 {
		return false
	}
	return true
}

func (m *ChannelDiscoveryManager) loadBlacklist() (*ChannelBlacklistFile, error) {
	bf, err := loadJSON[ChannelBlacklistFile](m.Paths.BlacklistFile)
	if err != nil {
		return nil, fmt.Errorf("load channels blacklist: %w", err)
	}
	if bf == nil {
		return &ChannelBlacklistFile{Channels: []BlacklistedChannel{}}, nil
	}
	if bf.Channels == nil {
		bf.Channels = []BlacklistedChannel{}
	}
	return bf, nil
}

func (m *ChannelDiscoveryManager) saveBlacklist(bf *ChannelBlacklistFile) error {
	if bf == nil {
		return nil
	}
	sort.Slice(bf.Channels, func(i, j int) bool { return bf.Channels[i].ID < bf.Channels[j].ID })
	if err := saveJSON(m.Paths.BlacklistFile, bf); err != nil {
		return fmt.Errorf("save channels blacklist: %w", err)
	}
	return nil
}

func (m *ChannelDiscoveryManager) addToBlacklist(bf *ChannelBlacklistFile, result *ChannelDiscoveryCacheEntry) {
	if bf == nil || result == nil {
		return
	}
	for _, ch := range bf.Channels {
		if ch.ID == result.ChannelID {
			return
		}
	}
	bf.Channels = append(bf.Channels, BlacklistedChannel{
		ID:          result.ChannelID,
		Name:        result.ChannelName,
		Reason:      result.Reason,
		ValidRatio:  result.Ratio,
		ValidCount:  result.ValidCount,
		TotalSample: result.TotalSample,
		AddedAt:     m.now().Format("2006-01-02"),
	})
}

func (m *ChannelDiscoveryManager) discoveryCachePath(channelID string) string {
	return filepath.Join(m.Paths.ChannelDiscoveryCache, channelID+".json")
}

func (m *ChannelDiscoveryManager) loadDiscoveryCache(channelID string) (*ChannelDiscoveryCacheEntry, error) {
	return loadJSON[ChannelDiscoveryCacheEntry](m.discoveryCachePath(channelID))
}

func (m *ChannelDiscoveryManager) saveDiscoveryCache(entry *ChannelDiscoveryCacheEntry) error {
	if entry == nil {
		return nil
	}
	return saveJSON(m.discoveryCachePath(entry.ChannelID), entry)
}
