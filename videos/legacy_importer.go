package videos

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ImportSummary struct {
	Added           int
	SkippedExisting int
	UnmatchedDates  int
	Duplicates      int
	Unmatched       []string
}

type ImportOptions struct {
	DryRun          bool
	OnlyMissing     bool
	OverwriteStatus string
}

type GameIndex interface {
	DateToGameID(date string) (string, bool)
}

type gameInfoProvider interface {
	GameInfo(date string) (GameRef, bool)
}

type Importer struct {
	Now func() time.Time
}

func (i *Importer) ImportLegacy(
	ctx context.Context,
	legacyPath string,
	cachePath string,
	idx GameIndex,
	opts ImportOptions,
) (ImportSummary, error) {
	if strings.TrimSpace(legacyPath) == "" {
		return ImportSummary{}, fmt.Errorf("legacy file path is empty")
	}
	if idx == nil {
		return ImportSummary{}, fmt.Errorf("game index is nil")
	}
	now := time.Now
	if i != nil && i.Now != nil {
		now = i.Now
	}

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("read legacy file: %w", err)
	}
	var legacy LegacyVideosFile
	if err := json.Unmarshal(data, &legacy); err != nil {
		return ImportSummary{}, fmt.Errorf("parse legacy file: %w", err)
	}

	cache := NewFileCache(cachePath)
	cf, err := cache.Load()
	if err != nil {
		return ImportSummary{}, err
	}
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
	}

	summary := ImportSummary{}
	for _, day := range legacy {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return summary, ctx.Err()
			default:
			}
		}
		date := strings.TrimSpace(day.Date)
		if date == "" {
			summary.UnmatchedDates++
			if len(summary.Unmatched) < 10 {
				summary.Unmatched = append(summary.Unmatched, date)
			}
			continue
		}
		gameID, ok := idx.DateToGameID(date)
		if !ok || gameID == "" {
			summary.UnmatchedDates++
			if len(summary.Unmatched) < 10 {
				summary.Unmatched = append(summary.Unmatched, date)
			}
			continue
		}

		existing := cf.Items[gameID]
		if existing != nil && opts.OnlyMissing {
			summary.SkippedExisting++
			continue
		}

		mappedVideos := mapLegacyVideos(day.Videos, now())
		status := legacyStatusFromMapped(mappedVideos)
		query := buildLegacyQuery(date, idx)

		if existing == nil {
			item := &CacheItem{
				Status:    status,
				Query:     query,
				UpdatedAt: now(),
				Videos:    mappedVideos,
			}
			if len(item.Videos) == 0 {
				item.Videos = nil
			}
			cf.Items[gameID] = item
			summary.Added++
			continue
		}

		updated := false
		if existing.Query == "" && query != "" {
			existing.Query = query
			updated = true
		}

		added, dup := mergeVideos(existing, mappedVideos)
		if added > 0 {
			updated = true
		}
		summary.Duplicates += dup

		if opts.OverwriteStatus != "" && strings.EqualFold(existing.Status, opts.OverwriteStatus) {
			existing.Status = status
			updated = true
		}

		if updated {
			existing.UpdatedAt = now()
		}
	}

	if !opts.DryRun {
		if err := cache.Save(cf); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

func legacyStatusFromMapped(videos []Video) string {
	if len(videos) == 0 {
		return "none"
	}
	return "selected"
}

func mapLegacyVideos(videos []LegacyVideo, now time.Time) []Video {
	if len(videos) == 0 {
		return nil
	}
	mapped := make([]Video, 0, len(videos))
	for _, v := range videos {
		id := strings.TrimSpace(v.YoutubeID)
		if id == "" {
			continue
		}
		duration := legacyDurationSeconds(v.Duration)
		mapped = append(mapped, Video{
			ID:              id,
			URL:             "https://www.youtube.com/watch?v=" + id,
			Title:           "Legacy import",
			Channel:         "Legacy import",
			DurationSeconds: duration,
			SelectedAt:      now,
		})
	}
	if len(mapped) == 0 {
		return nil
	}
	return mapped
}

func legacyDurationSeconds(raw any) int {
	if raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(int64(v) / int64(time.Second))
	case int64:
		return int(v / int64(time.Second))
	case int:
		return v / int(time.Second)
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0
		}
		return int(i / int64(time.Second))
	case string:
		str := strings.TrimSpace(v)
		if str == "" {
			return 0
		}
		if i, err := strconv.ParseInt(str, 10, 64); err == nil {
			return int(i / int64(time.Second))
		}
		if d, err := time.ParseDuration(str); err == nil {
			return int(d.Seconds())
		}
	}
	return 0
}

func mergeVideos(item *CacheItem, incoming []Video) (added int, duplicates int) {
	if item == nil || len(incoming) == 0 {
		return 0, 0
	}
	existing := map[string]bool{}
	for _, v := range item.Videos {
		if v.ID != "" {
			existing[v.ID] = true
		}
	}
	for _, v := range incoming {
		if v.ID == "" {
			continue
		}
		if existing[v.ID] {
			duplicates++
			continue
		}
		item.Videos = append(item.Videos, v)
		existing[v.ID] = true
		added++
	}
	return added, duplicates
}

func buildLegacyQuery(date string, idx GameIndex) string {
	if provider, ok := idx.(gameInfoProvider); ok {
		if ref, found := provider.GameInfo(date); found {
			if ref.AwayTeam != "" && ref.HomeTeam != "" && ref.Year > 0 {
				return fmt.Sprintf("%s vs %s %d", ref.AwayTeam, ref.HomeTeam, ref.Year)
			}
		}
	}
	return fmt.Sprintf("Legacy import %s", date)
}
