package videos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// ChannelRecord tracks channel quality learned from accepted videos.
type ChannelRecord struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	VideosCount    int     `json:"videos_count"`
	AnalyzedVideos int     `json:"analyzed_videos,omitempty"`
	DiscoveredAt   string  `json:"discovered_at,omitempty"`
	LastSeen       string  `json:"last_seen"`
	Score          float64 `json:"score"`
	Source         string  `json:"source,omitempty"`
}

// ChannelRegistryFile is persisted at cache/videos/channels.json.
type ChannelRegistryFile struct {
	Channels []ChannelRecord `json:"channels"`
}

// ChannelRegistry handles loading/selecting/updating learned channels.
type ChannelRegistry struct {
	Path string
	now  func() time.Time
}

// ChannelRegistrySanitization summarizes startup registry cleanup.
type ChannelRegistrySanitization struct {
	Validated int
	Removed   int
	Recovered int
	Merged    int
}

// NewChannelRegistry creates a channel registry manager.
func NewChannelRegistry(path string) *ChannelRegistry {
	return &ChannelRegistry{Path: path, now: time.Now}
}

// Load returns registry contents, or an empty registry if file does not exist.
func (r *ChannelRegistry) Load() (*ChannelRegistryFile, error) {
	data, err := os.ReadFile(r.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ChannelRegistryFile{Channels: []ChannelRecord{}}, nil
		}
		return nil, fmt.Errorf("read channels file: %w", err)
	}
	var reg ChannelRegistryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse channels file: %w", err)
	}
	if reg.Channels == nil {
		reg.Channels = []ChannelRecord{}
	}
	return &reg, nil
}

// Save writes registry contents sorted by quality.
func (r *ChannelRegistry) Save(reg *ChannelRegistryFile) error {
	if reg == nil {
		return fmt.Errorf("channel registry is nil")
	}
	r.sortChannels(reg.Channels)
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal channels file: %w", err)
	}
	if err := os.WriteFile(r.Path, data, 0o644); err != nil {
		return fmt.Errorf("write channels file: %w", err)
	}
	return nil
}

// SelectForScan chooses channels eligible for scanning.
func (r *ChannelRegistry) SelectForScan(reg *ChannelRegistryFile, minVideos int, minScore float64) []ChannelConfig {
	if reg == nil || len(reg.Channels) == 0 {
		return nil
	}

	r.sortChannels(reg.Channels)
	selected := make([]ChannelConfig, 0, len(reg.Channels))
	for _, ch := range reg.Channels {
		if ch.ID == "" || IsBlockedChannel(ch.ID) {
			continue
		}
		if ch.VideosCount < minVideos || ch.Score < minScore {
			continue
		}
		selected = append(selected, ChannelConfig{ID: ch.ID, Name: ch.Name})
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

// ValidateChannelID checks whether a channel ID exists in the YouTube API.
func ValidateChannelID(ctx context.Context, client *youtubeapi.Client, channelID string) bool {
	if client == nil || strings.TrimSpace(channelID) == "" {
		return false
	}
	_, err := client.GetChannelInfo(ctx, channelID)
	return err == nil
}

// Sanitize validates all registry channel IDs, removes invalid entries, and tries to recover them from accepted videos.
func (r *ChannelRegistry) Sanitize(ctx context.Context, reg *ChannelRegistryFile, cf *CacheFile, client *youtubeapi.Client) (*ChannelRegistrySanitization, error) {
	stats := &ChannelRegistrySanitization{}
	if reg == nil {
		return stats, nil
	}
	if reg.Channels == nil {
		reg.Channels = []ChannelRecord{}
		return stats, nil
	}

	videoCandidates := indexChannelCandidatesFromVideos(cf)
	validationCache := map[string]bool{}
	kept := make([]ChannelRecord, 0, len(reg.Channels))

	for _, ch := range reg.Channels {
		ch.ID = strings.TrimSpace(ch.ID)
		ch.Name = strings.TrimSpace(ch.Name)
		if ch.ID == "" {
			stats.Removed++
			continue
		}

		log.Printf("VALIDATING channel %s", ch.ID)
		stats.Validated++
		if validateChannelCached(ctx, client, ch.ID, validationCache) {
			kept = append(kept, ch)
			continue
		}

		log.Printf("INVALID CHANNEL ID: %s", ch.ID)
		recovered, ok := recoverChannelRecord(ctx, client, ch, videoCandidates, validationCache)
		if ok {
			log.Printf("RECOVERED channel %s -> %s from videos", ch.ID, recovered.ID)
			kept = append(kept, recovered)
			stats.Recovered++
			continue
		}

		log.Printf("REMOVED invalid channel: %s", ch.ID)
		stats.Removed++
	}

	merged, mergedCount := mergeChannelRecords(kept)
	stats.Merged = mergedCount
	reg.Channels = merged
	r.sortChannels(reg.Channels)
	return stats, nil
}

// UpdateFromAccepted adds or updates channels only from newly accepted videos.
// The map keys are video IDs that were already selected before the current run.
func (r *ChannelRegistry) UpdateFromAccepted(reg *ChannelRegistryFile, cf *CacheFile, previouslySelected map[string]bool, allowedNewChannels map[string]bool) (added, updated int) {
	if reg == nil || cf == nil {
		return 0, 0
	}
	if reg.Channels == nil {
		reg.Channels = []ChannelRecord{}
	}

	index := make(map[string]int, len(reg.Channels))
	for i, ch := range reg.Channels {
		if ch.ID != "" {
			index[ch.ID] = i
		}
	}

	seenThisRun := make(map[string]bool)
	lastSeen := r.now().Format("2006-01-02")

	for _, item := range cf.Items {
		if item == nil || !IsPublishableStatus(item.Status) || len(item.Videos) == 0 {
			continue
		}
		v := item.Videos[0]
		if v.ID == "" || v.ChannelID == "" {
			continue
		}
		if previouslySelected[v.ID] || seenThisRun[v.ID] {
			continue
		}
		seenThisRun[v.ID] = true
		if IsBlockedChannel(v.ChannelID) {
			continue
		}

		if i, ok := index[v.ChannelID]; ok {
			reg.Channels[i].VideosCount++
			if reg.Channels[i].AnalyzedVideos < reg.Channels[i].VideosCount {
				reg.Channels[i].AnalyzedVideos = reg.Channels[i].VideosCount
			}
			if v.Channel != "" {
				reg.Channels[i].Name = v.Channel
			}
			reg.Channels[i].LastSeen = lastSeen
			if reg.Channels[i].AnalyzedVideos > 0 {
				reg.Channels[i].Score = float64(reg.Channels[i].VideosCount) / float64(reg.Channels[i].AnalyzedVideos)
			} else {
				reg.Channels[i].Score = float64(reg.Channels[i].VideosCount)
			}
			log.Printf("UPDATED channel %s from video %s", v.ChannelID, v.ID)
			updated++
			continue
		}

		source := ""
		if item.AutoScoring != nil {
			source = item.AutoScoring.Source
		}
		if allowedNewChannels != nil && (source == "search" || source == "search-cached") {
			if !allowedNewChannels[v.ChannelID] {
				continue
			}
		}

		name := v.Channel
		if name == "" {
			name = v.ChannelID
		}
		reg.Channels = append(reg.Channels, ChannelRecord{
			ID:             v.ChannelID,
			Name:           name,
			VideosCount:    1,
			AnalyzedVideos: 1,
			DiscoveredAt:   lastSeen,
			LastSeen:       lastSeen,
			Score:          1,
			Source:         "search",
		})
		log.Printf("ADDED channel %s from video %s", v.ChannelID, v.ID)
		index[v.ChannelID] = len(reg.Channels) - 1
		added++
	}

	r.sortChannels(reg.Channels)
	return added, updated
}

func (r *ChannelRegistry) sortChannels(channels []ChannelRecord) {
	sort.Slice(channels, func(i, j int) bool {
		if channels[i].Score != channels[j].Score {
			return channels[i].Score > channels[j].Score
		}
		if channels[i].VideosCount != channels[j].VideosCount {
			return channels[i].VideosCount > channels[j].VideosCount
		}
		return channels[i].ID < channels[j].ID
	})
}

func validateChannelCached(ctx context.Context, client *youtubeapi.Client, channelID string, validationCache map[string]bool) bool {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return false
	}
	if ok, exists := validationCache[channelID]; exists {
		return ok
	}
	ok := ValidateChannelID(ctx, client, channelID)
	validationCache[channelID] = ok
	return ok
}

type videoChannelCandidate struct {
	ID    string
	Name  string
	Count int
}

func indexChannelCandidatesFromVideos(cf *CacheFile) map[string][]videoChannelCandidate {
	byName := map[string]map[string]*videoChannelCandidate{}
	if cf == nil {
		return map[string][]videoChannelCandidate{}
	}
	for _, item := range cf.Items {
		if item == nil {
			continue
		}
		for _, v := range item.Videos {
			channelID := strings.TrimSpace(v.ChannelID)
			channelName := strings.TrimSpace(v.Channel)
			if channelID == "" || channelName == "" {
				continue
			}
			key := normalizeChannelName(channelName)
			if key == "" {
				continue
			}
			if byName[key] == nil {
				byName[key] = map[string]*videoChannelCandidate{}
			}
			candidate := byName[key][channelID]
			if candidate == nil {
				candidate = &videoChannelCandidate{ID: channelID, Name: channelName}
				byName[key][channelID] = candidate
			}
			candidate.Count++
		}
	}

	result := make(map[string][]videoChannelCandidate, len(byName))
	for key, bucket := range byName {
		candidates := make([]videoChannelCandidate, 0, len(bucket))
		for _, candidate := range bucket {
			candidates = append(candidates, *candidate)
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].Count != candidates[j].Count {
				return candidates[i].Count > candidates[j].Count
			}
			return candidates[i].ID < candidates[j].ID
		})
		result[key] = candidates
	}
	return result
}

func recoverChannelRecord(ctx context.Context, client *youtubeapi.Client, original ChannelRecord, candidates map[string][]videoChannelCandidate, validationCache map[string]bool) (ChannelRecord, bool) {
	nameKey := normalizeChannelName(original.Name)
	for _, candidate := range candidates[nameKey] {
		if !validateChannelCached(ctx, client, candidate.ID, validationCache) {
			continue
		}
		original.ID = candidate.ID
		original.Name = candidate.Name
		if original.Source == "" {
			original.Source = "video-recovery"
		}
		return original, true
	}
	return ChannelRecord{}, false
}

func mergeChannelRecords(channels []ChannelRecord) ([]ChannelRecord, int) {
	mergedByID := make(map[string]ChannelRecord, len(channels))
	mergedCount := 0
	for _, ch := range channels {
		existing, ok := mergedByID[ch.ID]
		if !ok {
			mergedByID[ch.ID] = ch
			continue
		}
		mergedCount++
		if ch.Name != "" {
			existing.Name = ch.Name
		}
		existing.VideosCount += ch.VideosCount
		if ch.AnalyzedVideos > existing.AnalyzedVideos {
			existing.AnalyzedVideos = ch.AnalyzedVideos
		}
		if existing.AnalyzedVideos < existing.VideosCount {
			existing.AnalyzedVideos = existing.VideosCount
		}
		if ch.DiscoveredAt != "" && (existing.DiscoveredAt == "" || ch.DiscoveredAt < existing.DiscoveredAt) {
			existing.DiscoveredAt = ch.DiscoveredAt
		}
		if ch.LastSeen > existing.LastSeen {
			existing.LastSeen = ch.LastSeen
		}
		if ch.Source != "" {
			existing.Source = ch.Source
		}
		if existing.AnalyzedVideos > 0 {
			existing.Score = float64(existing.VideosCount) / float64(existing.AnalyzedVideos)
		}
		mergedByID[ch.ID] = existing
	}

	result := make([]ChannelRecord, 0, len(mergedByID))
	for _, ch := range mergedByID {
		result = append(result, ch)
	}
	return result, mergedCount
}

func normalizeChannelName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Join(strings.Fields(name), " ")
	return name
}

func selectedVideoIDSet(cf *CacheFile) map[string]bool {
	ids := make(map[string]bool)
	if cf == nil {
		return ids
	}
	for _, item := range cf.Items {
		if item == nil || !IsPublishableStatus(item.Status) {
			continue
		}
		for _, v := range item.Videos {
			if v.ID != "" {
				ids[v.ID] = true
			}
		}
	}
	return ids
}
