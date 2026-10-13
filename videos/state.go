package videos

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	stateSchemaName    = "mundodolphins.youtube.state"
	stateSchemaVersion = "1.0.0"
)

// PipelineState holds the incremental state of the auto-discovery pipeline.
type PipelineState struct {
	Schema               StateSchema       `json:"schema"`
	LastRun              time.Time         `json:"last_run"`
	ProcessedGames       []string          `json:"processed_games"`
	GamesWithVideo       []string          `json:"games_with_video"`
	GamesWithoutVideo    []string          `json:"games_without_video"`
	QuotaUsedToday       int               `json:"quota_used_today"`
	LastQuotaReset       string            `json:"last_quota_reset"`
	ChannelScanCompleted bool              `json:"channel_scan_completed"`
	ChannelsScanned      []string          `json:"channels_scanned"`
	SearchQueueRemaining []string          `json:"search_queue_remaining"`
	ChannelMatches       map[string]string `json:"channel_matches,omitempty"` // gameID → videoID from channel scan
}

// StateSchema identifies the state file version.
type StateSchema struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// StateManager handles reading and writing the pipeline state file.
type StateManager struct {
	Path    string
	nowFunc func() time.Time
}

// NewStateManager creates a state manager for the given file path.
func NewStateManager(path string) *StateManager {
	return &StateManager{Path: path, nowFunc: time.Now}
}

// Load reads the state file or returns a fresh state if it doesn't exist.
func (m *StateManager) Load() (*PipelineState, error) {
	if m.Path == "" {
		return m.newState(), nil
	}

	data, err := os.ReadFile(m.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m.newState(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state PipelineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	if state.ProcessedGames == nil {
		state.ProcessedGames = []string{}
	}
	if state.GamesWithVideo == nil {
		state.GamesWithVideo = []string{}
	}
	if state.GamesWithoutVideo == nil {
		state.GamesWithoutVideo = []string{}
	}
	if state.SearchQueueRemaining == nil {
		state.SearchQueueRemaining = []string{}
	}
	if state.ChannelsScanned == nil {
		state.ChannelsScanned = []string{}
	}
	if state.ChannelMatches == nil {
		state.ChannelMatches = map[string]string{}
	}

	return &state, nil
}

// Save writes the state to disk atomically.
func (m *StateManager) Save(state *PipelineState) error {
	if m.Path == "" {
		return fmt.Errorf("state path is empty")
	}
	if state == nil {
		return fmt.Errorf("state is nil")
	}

	state.Schema = StateSchema{Name: stateSchemaName, Version: stateSchemaVersion}
	state.LastRun = m.now()

	// Sort slices for deterministic output
	sort.Strings(state.ProcessedGames)
	sort.Strings(state.GamesWithVideo)
	sort.Strings(state.GamesWithoutVideo)
	sort.Strings(state.SearchQueueRemaining)
	sort.Strings(state.ChannelsScanned)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.Path), 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	tmpPath := m.Path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmpPath, m.Path); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

// Reset returns a fresh state, discarding all progress.
func (m *StateManager) Reset() *PipelineState {
	return m.newState()
}

// IsGameProcessed checks if a game has already been processed this run.
func (s *PipelineState) IsGameProcessed(gameID string) bool {
	for _, id := range s.ProcessedGames {
		if id == gameID {
			return true
		}
	}
	return false
}

// HasVideo checks if a game already has a video assigned.
func (s *PipelineState) HasVideo(gameID string) bool {
	for _, id := range s.GamesWithVideo {
		if id == gameID {
			return true
		}
	}
	return false
}

// MarkProcessed adds a game to the processed list.
func (s *PipelineState) MarkProcessed(gameID string) {
	if !s.IsGameProcessed(gameID) {
		s.ProcessedGames = append(s.ProcessedGames, gameID)
	}
}

// MarkWithVideo marks a game as having a video.
func (s *PipelineState) MarkWithVideo(gameID string) {
	s.MarkProcessed(gameID)
	if !s.HasVideo(gameID) {
		s.GamesWithVideo = append(s.GamesWithVideo, gameID)
	}
	// Remove from without-video list
	s.GamesWithoutVideo = removeFromSlice(s.GamesWithoutVideo, gameID)
	// Remove from search queue
	s.SearchQueueRemaining = removeFromSlice(s.SearchQueueRemaining, gameID)
}

// MarkWithoutVideo marks a game as having no video.
func (s *PipelineState) MarkWithoutVideo(gameID string) {
	s.MarkProcessed(gameID)
	found := false
	for _, id := range s.GamesWithoutVideo {
		if id == gameID {
			found = true
			break
		}
	}
	if !found {
		s.GamesWithoutVideo = append(s.GamesWithoutVideo, gameID)
	}
}

func (s *PipelineState) ReconcileWithCache(cf *CacheFile) {
	if s == nil || cf == nil {
		return
	}
	validWithVideo := map[string]bool{}
	for gameID, item := range cf.Items {
		if item == nil || len(item.Videos) == 0 {
			continue
		}
		if IsPublishableStatus(item.Status) || item.Status == StatusCandidate {
			validWithVideo[gameID] = true
		}
	}
	filtered := make([]string, 0, len(s.GamesWithVideo))
	seen := map[string]bool{}
	for _, gameID := range s.GamesWithVideo {
		if validWithVideo[gameID] && !seen[gameID] {
			filtered = append(filtered, gameID)
			seen[gameID] = true
		}
	}
	s.GamesWithVideo = filtered
}

// IsChannelScanned checks if a channel has been scanned.
func (s *PipelineState) IsChannelScanned(channelID string) bool {
	for _, id := range s.ChannelsScanned {
		if id == channelID {
			return true
		}
	}
	return false
}

// MarkChannelScanned records that a channel has been fully scanned.
func (s *PipelineState) MarkChannelScanned(channelID string) {
	if !s.IsChannelScanned(channelID) {
		s.ChannelsScanned = append(s.ChannelsScanned, channelID)
	}
}

// BuildSearchQueue creates the list of games that need search fallback.
func (s *PipelineState) BuildSearchQueue(allGameIDs []string) {
	need := make([]string, 0)
	for _, id := range allGameIDs {
		if !s.HasVideo(id) && !s.isInSearchQueue(id) {
			need = append(need, id)
		}
	}
	s.SearchQueueRemaining = need
}

func (s *PipelineState) isInSearchQueue(gameID string) bool {
	for _, id := range s.SearchQueueRemaining {
		if id == gameID {
			return true
		}
	}
	return false
}

// PopSearchQueue removes and returns the first game in the search queue.
func (s *PipelineState) PopSearchQueue() (string, bool) {
	if len(s.SearchQueueRemaining) == 0 {
		return "", false
	}
	id := s.SearchQueueRemaining[0]
	s.SearchQueueRemaining = s.SearchQueueRemaining[1:]
	return id, true
}

func (m *StateManager) now() time.Time {
	if m.nowFunc != nil {
		return m.nowFunc()
	}
	return time.Now()
}

func (m *StateManager) newState() *PipelineState {
	return &PipelineState{
		Schema:               StateSchema{Name: stateSchemaName, Version: stateSchemaVersion},
		LastRun:              m.now(),
		ProcessedGames:       []string{},
		GamesWithVideo:       []string{},
		GamesWithoutVideo:    []string{},
		QuotaUsedToday:       0,
		LastQuotaReset:       m.now().UTC().Format("2006-01-02"),
		ChannelScanCompleted: false,
		ChannelsScanned:      []string{},
		SearchQueueRemaining: []string{},
		ChannelMatches:       map[string]string{},
	}
}

func removeFromSlice(s []string, val string) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		if v != val {
			result = append(result, v)
		}
	}
	return result
}
