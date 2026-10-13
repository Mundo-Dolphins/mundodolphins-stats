package videos

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

// ChannelScanner iterates configured channels and fetches all their playlist items,
// caching the results on disk. Each page costs 1 quota unit (50 items).
type ChannelScanner struct {
	Client   *youtubeapi.Client
	Cache    *DiskCacheManager
	Quota    *QuotaManager
	Channels []ChannelConfig
}

// ScanResult summarizes a single channel scan.
type ScanResult struct {
	ChannelID   string
	ChannelName string
	VideoCount  int
	PagesUsed   int
	FromCache   bool
}

// ScanAll iterates all configured channels and fetches their uploads.
// Skips channels that are already scanned (from state) or cached.
// Stops early if quota is about to run out.
func (s *ChannelScanner) ScanAll(ctx context.Context, state *PipelineState) ([]ScanResult, error) {
	results := make([]ScanResult, 0, len(s.Channels))

	for _, ch := range s.Channels {
		if state.IsChannelScanned(ch.ID) && s.Cache.HasChannelCache(ch.ID) {
			// Already done — load from cache for summary
			entry, err := s.Cache.LoadChannelCache(ch.ID)
			if err == nil && entry != nil {
				log.Printf("CHANNEL SCAN %s (%s): cached, %d videos", ch.ID, ch.Name, len(entry.Videos))
				results = append(results, ScanResult{
					ChannelID:   ch.ID,
					ChannelName: ch.Name,
					VideoCount:  len(entry.Videos),
					FromCache:   true,
				})
			}
			continue
		}

		result, err := s.scanChannel(ctx, ch)
		if err != nil {
			// Quota exhausted — stop scanning but don't fail
			if s.Quota.IsExhausted() {
				log.Printf("QUOTA: exhausted during channel scan, stopping")
				break
			}
			if strings.Contains(err.Error(), "not found") {
				log.Printf("INVALID CHANNEL ID: %s", ch.ID)
			}
			log.Printf("WARN: channel scan %s (%s) failed: %v", ch.ID, ch.Name, err)
			continue
		}

		state.MarkChannelScanned(ch.ID)
		results = append(results, *result)
	}

	// Mark channel scan as completed if all channels were processed
	if len(results) == len(s.Channels) {
		allDone := true
		for _, ch := range s.Channels {
			if !state.IsChannelScanned(ch.ID) {
				allDone = false
				break
			}
		}
		if allDone {
			state.ChannelScanCompleted = true
		}
	}

	return results, nil
}

func (s *ChannelScanner) scanChannel(ctx context.Context, ch ChannelConfig) (*ScanResult, error) {
	// 1. Resolve channel → uploads playlist (1 unit)
	if !s.Quota.TryConsume(QuotaCostChannelsList) {
		return nil, fmt.Errorf("quota exhausted for channels.list")
	}

	info, err := s.Client.GetChannelInfo(ctx, ch.ID)
	if err != nil {
		s.Quota.ForceConsume(0) // Already consumed above
		return nil, fmt.Errorf("get channel info %s: %w", ch.ID, err)
	}
	if info.UploadsPlaylistID == "" {
		return nil, fmt.Errorf("channel %s has no uploads playlist", ch.ID)
	}

	log.Printf("CHANNEL SCAN %s (%s): playlist=%s", ch.ID, info.Title, info.UploadsPlaylistID)

	// 2. Paginate through all uploads
	var allItems []youtubeapi.PlaylistItem
	pageToken := ""
	pagesUsed := 0

	for {
		if !s.Quota.TryConsume(QuotaCostPlaylistItems) {
			log.Printf("QUOTA: exhausted during playlist iteration for %s (collected %d videos so far)", ch.Name, len(allItems))
			break
		}

		page, err := s.Client.ListPlaylistPage(ctx, info.UploadsPlaylistID, pageToken, 50)
		if err != nil {
			return nil, fmt.Errorf("list playlist page %s: %w", info.UploadsPlaylistID, err)
		}
		pagesUsed++
		allItems = append(allItems, page.Items...)

		log.Printf("  page %d: %d items (total so far: %d)", pagesUsed, len(page.Items), len(allItems))

		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	// 3. Save to disk cache
	entry := &ChannelCacheEntry{
		ChannelID:  ch.ID,
		Title:      info.Title,
		FetchedAt:  s.Cache.now(),
		TotalItems: len(allItems),
		Videos:     allItems,
	}
	if err := s.Cache.SaveChannelCache(entry); err != nil {
		return nil, fmt.Errorf("save channel cache %s: %w", ch.ID, err)
	}

	log.Printf("CHANNEL SCAN %s (%s): complete, %d videos in %d pages", ch.ID, ch.Name, len(allItems), pagesUsed)

	return &ScanResult{
		ChannelID:   ch.ID,
		ChannelName: ch.Name,
		VideoCount:  len(allItems),
		PagesUsed:   pagesUsed,
	}, nil
}

// CollectAllChannelVideos loads all cached channel videos into a flat list.
func (s *ChannelScanner) CollectAllChannelVideos() ([]youtubeapi.PlaylistItem, error) {
	var all []youtubeapi.PlaylistItem
	for _, ch := range s.Channels {
		entry, err := s.Cache.LoadChannelCache(ch.ID)
		if err != nil {
			return nil, fmt.Errorf("load channel cache %s: %w", ch.ID, err)
		}
		if entry == nil {
			continue
		}
		all = append(all, entry.Videos...)
	}
	return all, nil
}
