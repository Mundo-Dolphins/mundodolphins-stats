package videos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheSchemaName    = "mundodolphins.youtube.cache"
	cacheSchemaVersion = "1.4.0"
)

type FileCache struct {
	Path string
}

func NewFileCache(path string) *FileCache {
	return &FileCache{Path: path}
}

func (c *FileCache) Load() (*CacheFile, error) {
	if c == nil || c.Path == "" {
		return newCacheFile(time.Now()), nil
	}
	data, err := os.ReadFile(c.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newCacheFile(time.Now()), nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}
	var cf CacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
	}
	if cf.Schema.Name == "" {
		cf.Schema = CacheSchema{Name: cacheSchemaName, Version: cacheSchemaVersion}
	}
	normalizeCacheFile(&cf)
	return &cf, nil
}

func (c *FileCache) Save(cf *CacheFile) error {
	if c == nil || c.Path == "" {
		return fmt.Errorf("cache path is empty")
	}
	if cf == nil {
		return fmt.Errorf("cache file is nil")
	}
	if cf.Schema.Name == "" {
		cf.Schema = CacheSchema{Name: cacheSchemaName, Version: cacheSchemaVersion}
	}
	cf.GeneratedAt = time.Now()
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
	}
	normalizeCacheFile(cf)
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	tmpPath := c.Path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp cache: %w", err)
	}
	if err := os.Rename(tmpPath, c.Path); err != nil {
		return fmt.Errorf("rename cache: %w", err)
	}
	return nil
}

func (c *FileCache) Get(gameID string) (*CacheItem, bool, error) {
	cf, err := c.Load()
	if err != nil {
		return nil, false, err
	}
	item, ok := cf.Items[gameID]
	return item, ok, nil
}

func (c *FileCache) Upsert(gameID string, item *CacheItem) error {
	if gameID == "" {
		return fmt.Errorf("gameID is empty")
	}
	cf, err := c.Load()
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("cache item is nil")
	}
	item.UpdatedAt = time.Now()
	cf.Items[gameID] = item
	return c.Save(cf)
}

func newCacheFile(now time.Time) *CacheFile {
	return &CacheFile{
		Schema:      CacheSchema{Name: cacheSchemaName, Version: cacheSchemaVersion},
		GeneratedAt: now,
		Items:       map[string]*CacheItem{},
	}
}

func BuildYouTubeBlock(cf *CacheFile, gameID string) YouTubeBlock {
	if cf == nil || cf.Items == nil {
		return YouTubeBlock{Status: StatusUnknown, Videos: nil}
	}
	item, ok := cf.Items[gameID]
	if !ok || item == nil {
		return YouTubeBlock{Status: StatusUnknown, Videos: nil}
	}
	normalizeCacheItem(item)
	status := item.Status
	if status == "" {
		status = StatusUnknown
	}
	if !IsPublishableStatus(status) {
		return YouTubeBlock{Status: status, Videos: nil}
	}
	return YouTubeBlock{Status: status, Videos: item.Videos}
}

func normalizeCacheFile(cf *CacheFile) {
	if cf == nil {
		return
	}
	if cf.Schema.Name == "" {
		cf.Schema = CacheSchema{Name: cacheSchemaName, Version: cacheSchemaVersion}
	}
	if cf.Schema.Version == "" {
		cf.Schema.Version = cacheSchemaVersion
	}
	if cf.Items == nil {
		cf.Items = map[string]*CacheItem{}
		return
	}
	for _, item := range cf.Items {
		normalizeCacheItem(item)
	}
}

func normalizeCacheItem(item *CacheItem) {
	if item == nil {
		return
	}
	if item.Status == "" {
		item.Status = StatusUnknown
	}
	if item.Status == StatusSelected && item.AutoScoring != nil {
		item.Status = StatusCandidate
		item.NeedsReview = true
		item.AutoScoring.NeedsReview = true
		if item.AutoScoring.Confidence == "" {
			item.AutoScoring.Confidence = confidenceFromScore(item.AutoScoring.ScoreBreakdown.Total)
		}
	}
	if item.Status == StatusCandidate {
		item.NeedsReview = true
		if item.AutoScoring != nil {
			item.AutoScoring.NeedsReview = true
			if item.AutoScoring.Confidence == "" {
				item.AutoScoring.Confidence = confidenceFromScore(item.AutoScoring.ScoreBreakdown.Total)
			}
		}
	}
}
