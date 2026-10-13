package videos

import (
	"context"
	"fmt"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

type MetadataBackfillConfig struct {
	CacheFile string
	VideosDir string
	DryRun    bool
	MaxVideos int
}

type MetadataBackfillStats struct {
	Scanned        int
	Missing        int
	CacheHits      int
	APIFetched     int
	UpdatedVideos  int
	UpdatedItems   int
	ChannelsSynced int
}

func BackfillVideoMetadata(ctx context.Context, client *youtubeapi.Client, cfg MetadataBackfillConfig) (*MetadataBackfillStats, error) {
	if cfg.CacheFile == "" {
		return nil, fmt.Errorf("cache file is empty")
	}
	if cfg.VideosDir == "" {
		cfg.VideosDir = "cache/videos"
	}
	fileCache := NewFileCache(cfg.CacheFile)
	cf, err := fileCache.Load()
	if err != nil {
		return nil, err
	}
	paths := NewPaths(cfg.VideosDir)
	diskCache := NewDiskCacheManager(paths.CacheDir)

	stats := &MetadataBackfillStats{}
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, item := range cf.Items {
		if item == nil {
			continue
		}
		for _, video := range item.Videos {
			stats.Scanned++
			if video.ID == "" || seen[video.ID] {
				continue
			}
			if needsMetadataBackfill(video) {
				stats.Missing++
				seen[video.ID] = true
				ids = append(ids, video.ID)
				if cfg.MaxVideos > 0 && len(ids) >= cfg.MaxVideos {
					break
				}
			}
		}
		if cfg.MaxVideos > 0 && len(ids) >= cfg.MaxVideos {
			break
		}
	}
	if len(ids) == 0 {
		return stats, nil
	}

	cached, missing := diskCache.LoadMultiVideoCache(ids)
	stats.CacheHits = len(cached)
	if cfg.DryRun {
		return stats, nil
	}
	fresh := []youtubeapi.VideoCandidate(nil)
	if len(missing) > 0 {
		if client == nil {
			return stats, fmt.Errorf("youtube api client is required for %d missing video details", len(missing))
		}
		fresh, err = client.HydrateVideos(ctx, missing)
		if err != nil {
			return stats, err
		}
		stats.APIFetched = len(fresh)
		if !cfg.DryRun {
			if err := diskCache.SaveMultiVideoCache(fresh); err != nil {
				return stats, err
			}
		}
	}

	details := map[string]youtubeapi.VideoCandidate{}
	for _, candidate := range append(cached, fresh...) {
		if candidate.ID != "" {
			details[candidate.ID] = candidate
		}
	}
	for _, item := range cf.Items {
		if item == nil {
			continue
		}
		itemUpdated := false
		for i := range item.Videos {
			candidate, ok := details[item.Videos[i].ID]
			if !ok {
				continue
			}
			if applyCandidateMetadata(&item.Videos[i], candidate) {
				stats.UpdatedVideos++
				itemUpdated = true
			}
			if i == 0 && item.AutoScoring != nil {
				item.AutoScoring.ViewCount = candidate.ViewCount
				item.AutoScoring.Embeddable = candidate.Embeddable
				item.AutoScoring.DurationISO = formatISO8601(candidate.Duration)
				item.AutoScoring.ContentRole = item.Videos[i].Role
			}
		}
		if itemUpdated {
			item.UpdatedAt = time.Now()
			stats.UpdatedItems++
		}
	}
	if err := fileCache.Save(cf); err != nil {
		return stats, err
	}
	synced, err := syncChannelRegistryFromCache(paths.ChannelsFile, cf)
	if err != nil {
		return stats, err
	}
	stats.ChannelsSynced = synced
	return stats, nil
}

func needsMetadataBackfill(video Video) bool {
	return video.ChannelID == "" || video.Role == "" || video.DurationSeconds == 0 || video.Embeddable == nil
}

func applyCandidateMetadata(video *Video, candidate youtubeapi.VideoCandidate) bool {
	if video == nil {
		return false
	}
	changed := false
	if video.URL == "" && candidate.URL != "" {
		video.URL = candidate.URL
		changed = true
	}
	if video.Title == "" && candidate.Title != "" {
		video.Title = candidate.Title
		changed = true
	}
	if video.Channel == "" && candidate.Channel != "" {
		video.Channel = candidate.Channel
		changed = true
	}
	if video.ChannelID == "" && candidate.ChannelID != "" {
		video.ChannelID = candidate.ChannelID
		changed = true
	}
	if video.DurationSeconds == 0 && candidate.Duration > 0 {
		video.DurationSeconds = int(candidate.Duration.Seconds())
		changed = true
	}
	if video.Embeddable == nil {
		embeddable := candidate.Embeddable
		video.Embeddable = &embeddable
		changed = true
	}
	role := ClassifyContentRole(valueOr(video.Title, candidate.Title), candidate.Description, valueOr(video.Channel, candidate.Channel), video.DurationSeconds)
	if video.Role == "" && role != "" {
		video.Role = string(role)
		changed = true
	}
	part := DetectPartInfo(valueOr(video.Title, candidate.Title), candidate.Description)
	if video.Part == 0 && part.Part > 0 {
		video.Part = part.Part
		changed = true
	}
	if video.PartTotal == 0 && part.Total > 0 {
		video.PartTotal = part.Total
		changed = true
	}
	return changed
}

func valueOr(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func syncChannelRegistryFromCache(path string, cf *CacheFile) (int, error) {
	registry := NewChannelRegistry(path)
	reg, err := registry.Load()
	if err != nil {
		return 0, err
	}
	type aggregate struct {
		name  string
		count int
	}
	counts := map[string]aggregate{}
	for _, item := range cf.Items {
		if item == nil || !IsPublishableStatus(item.Status) {
			continue
		}
		for _, video := range item.Videos {
			if video.ChannelID == "" || IsBlockedChannel(video.ChannelID) {
				continue
			}
			current := counts[video.ChannelID]
			current.count++
			if video.Channel != "" {
				current.name = video.Channel
			}
			counts[video.ChannelID] = current
		}
	}
	if len(counts) == 0 {
		return 0, nil
	}
	index := map[string]int{}
	for i, channel := range reg.Channels {
		index[channel.ID] = i
	}
	now := time.Now().Format("2006-01-02")
	changed := 0
	for id, aggregate := range counts {
		if idx, ok := index[id]; ok {
			if reg.Channels[idx].VideosCount != aggregate.count {
				reg.Channels[idx].VideosCount = aggregate.count
				changed++
			}
			if reg.Channels[idx].AnalyzedVideos < aggregate.count {
				reg.Channels[idx].AnalyzedVideos = aggregate.count
			}
			if reg.Channels[idx].Name == "" && aggregate.name != "" {
				reg.Channels[idx].Name = aggregate.name
			}
			if reg.Channels[idx].LastSeen == "" {
				reg.Channels[idx].LastSeen = now
			}
			if reg.Channels[idx].Score == 0 {
				reg.Channels[idx].Score = 1
			}
			continue
		}
		name := aggregate.name
		if name == "" {
			name = id
		}
		reg.Channels = append(reg.Channels, ChannelRecord{
			ID:             id,
			Name:           name,
			VideosCount:    aggregate.count,
			AnalyzedVideos: aggregate.count,
			DiscoveredAt:   now,
			LastSeen:       now,
			Score:          1,
			Source:         "cache-backfill",
		})
		changed++
	}
	if changed == 0 {
		return 0, nil
	}
	return changed, registry.Save(reg)
}
