package videos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestStateManager_LoadFreshState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	mgr := NewStateManager(path)
	state, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if state.Schema.Name != stateSchemaName {
		t.Errorf("Schema.Name = %q, want %q", state.Schema.Name, stateSchemaName)
	}
	if len(state.ProcessedGames) != 0 {
		t.Errorf("ProcessedGames should be empty, got %d", len(state.ProcessedGames))
	}
	if state.ChannelScanCompleted {
		t.Error("ChannelScanCompleted should be false initially")
	}
}

func TestStateManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	fixed := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	mgr := NewStateManager(path)
	mgr.nowFunc = func() time.Time { return fixed }

	state := mgr.Reset()
	state.MarkWithVideo("game1")
	state.MarkWithoutVideo("game2")
	state.MarkChannelScanned("ch1")
	state.ChannelMatches["game1"] = "vid1"

	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("state file should exist after save")
	}

	// Reload
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !loaded.HasVideo("game1") {
		t.Error("game1 should have video after reload")
	}
	if loaded.HasVideo("game2") {
		t.Error("game2 should not have video")
	}
	if !loaded.IsChannelScanned("ch1") {
		t.Error("ch1 should be scanned after reload")
	}
	if loaded.ChannelMatches["game1"] != "vid1" {
		t.Errorf("ChannelMatches[game1] = %q, want vid1", loaded.ChannelMatches["game1"])
	}
}

func TestStateManager_Reset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	mgr := NewStateManager(path)

	state := mgr.Reset()
	state.MarkWithVideo("game1")
	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	fresh := mgr.Reset()
	if fresh.HasVideo("game1") {
		t.Error("fresh state should not have game1")
	}
	if len(fresh.ProcessedGames) != 0 {
		t.Error("fresh state should have empty processed games")
	}
}

func TestPipelineState_MarkProcessed(t *testing.T) {
	s := &PipelineState{
		ProcessedGames:       []string{},
		GamesWithVideo:       []string{},
		GamesWithoutVideo:    []string{},
		ChannelsScanned:      []string{},
		ChannelMatches:       map[string]string{},
		SearchQueueRemaining: []string{},
	}

	s.MarkProcessed("g1")
	if !s.IsGameProcessed("g1") {
		t.Error("g1 should be processed")
	}

	// Duplicate should not add again
	s.MarkProcessed("g1")
	count := 0
	for _, id := range s.ProcessedGames {
		if id == "g1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("g1 appears %d times, want 1", count)
	}
}

func TestPipelineState_MarkWithVideo(t *testing.T) {
	s := &PipelineState{
		ProcessedGames:       []string{},
		GamesWithVideo:       []string{},
		GamesWithoutVideo:    []string{"g1"}, // Was previously without video
		ChannelsScanned:      []string{},
		ChannelMatches:       map[string]string{},
		SearchQueueRemaining: []string{"g1"},
	}

	s.MarkWithVideo("g1")

	if !s.HasVideo("g1") {
		t.Error("g1 should have video")
	}
	if !s.IsGameProcessed("g1") {
		t.Error("g1 should be processed")
	}
	// Should be removed from without-video and search queue
	for _, id := range s.GamesWithoutVideo {
		if id == "g1" {
			t.Error("g1 should be removed from GamesWithoutVideo")
		}
	}
	for _, id := range s.SearchQueueRemaining {
		if id == "g1" {
			t.Error("g1 should be removed from SearchQueueRemaining")
		}
	}
}

func TestPipelineState_MarkWithoutVideoDeduplicatesAndReconcileFilters(t *testing.T) {
	s := &PipelineState{
		ProcessedGames:    []string{},
		GamesWithVideo:    []string{"valid", "valid", "candidate", "empty", "none", "missing"},
		GamesWithoutVideo: []string{},
	}

	s.MarkWithoutVideo("none")
	s.MarkWithoutVideo("none")
	if !s.IsGameProcessed("none") {
		t.Fatal("none should be marked processed")
	}
	if !reflect.DeepEqual(s.GamesWithoutVideo, []string{"none"}) {
		t.Fatalf("GamesWithoutVideo=%v, want [none]", s.GamesWithoutVideo)
	}

	s.ReconcileWithCache(nil)
	((*PipelineState)(nil)).ReconcileWithCache(&CacheFile{})

	s.ReconcileWithCache(&CacheFile{Items: map[string]*CacheItem{
		"valid":     {Status: StatusApproved, Videos: []Video{{ID: "v1"}}},
		"candidate": {Status: StatusCandidate, Videos: []Video{{ID: "v2"}}},
		"empty":     {Status: StatusApproved},
		"none":      {Status: StatusNone, Videos: []Video{{ID: "v3"}}},
	}})
	if !reflect.DeepEqual(s.GamesWithVideo, []string{"valid", "candidate"}) {
		t.Fatalf("GamesWithVideo=%v, want [valid candidate]", s.GamesWithVideo)
	}
}

func TestPipelineState_BuildSearchQueue(t *testing.T) {
	s := &PipelineState{
		ProcessedGames:       []string{},
		GamesWithVideo:       []string{"g1"},
		GamesWithoutVideo:    []string{},
		ChannelsScanned:      []string{},
		ChannelMatches:       map[string]string{},
		SearchQueueRemaining: []string{},
	}

	allGames := []string{"g1", "g2", "g3"}
	s.BuildSearchQueue(allGames)

	if len(s.SearchQueueRemaining) != 2 {
		t.Errorf("search queue should have 2 items, got %d", len(s.SearchQueueRemaining))
	}
}

func TestPipelineState_PopSearchQueue(t *testing.T) {
	s := &PipelineState{
		SearchQueueRemaining: []string{"g1", "g2"},
	}

	id, ok := s.PopSearchQueue()
	if !ok || id != "g1" {
		t.Errorf("PopSearchQueue() = %q, %v; want g1, true", id, ok)
	}

	id, ok = s.PopSearchQueue()
	if !ok || id != "g2" {
		t.Errorf("PopSearchQueue() = %q, %v; want g2, true", id, ok)
	}

	_, ok = s.PopSearchQueue()
	if ok {
		t.Error("PopSearchQueue() should return false when empty")
	}
}

func TestPipelineState_ChannelScanned(t *testing.T) {
	s := &PipelineState{
		ChannelsScanned: []string{},
	}

	if s.IsChannelScanned("ch1") {
		t.Error("ch1 should not be scanned initially")
	}
	s.MarkChannelScanned("ch1")
	if !s.IsChannelScanned("ch1") {
		t.Error("ch1 should be scanned after marking")
	}
	// Dedup
	s.MarkChannelScanned("ch1")
	count := 0
	for _, id := range s.ChannelsScanned {
		if id == "ch1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ch1 appears %d times, want 1", count)
	}
}

func TestStateManager_EmptyPath(t *testing.T) {
	mgr := NewStateManager("")
	state, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() with empty path: %v", err)
	}
	if state == nil {
		t.Fatal("state should not be nil")
	}
}

func TestStateManager_SaveErrors(t *testing.T) {
	mgr := NewStateManager("")
	if err := mgr.Save(&PipelineState{}); err == nil {
		t.Fatal("expected error for empty state path")
	}

	mgr = NewStateManager(filepath.Join(t.TempDir(), "state.json"))
	if err := mgr.Save(nil); err == nil {
		t.Fatal("expected error for nil state")
	}
}

func TestStateManager_LoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := NewStateManager(path).Load()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestStateManager_LoadLegacyStateInitializesMissingCollections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema":{"name":"mundodolphins.youtube.state","version":"1.0.0"}}`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	state, err := NewStateManager(path).Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.ProcessedGames == nil || state.GamesWithVideo == nil || state.GamesWithoutVideo == nil ||
		state.SearchQueueRemaining == nil || state.ChannelsScanned == nil || state.ChannelMatches == nil {
		t.Fatalf("legacy state collections should be initialized: %+v", state)
	}
}

func TestPipelineState_SortedOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	mgr := NewStateManager(path)

	state := mgr.Reset()
	state.ProcessedGames = []string{"z", "a", "m"}
	state.GamesWithVideo = []string{"z", "a"}

	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.ProcessedGames[0] != "a" {
		t.Errorf("ProcessedGames should be sorted, got %v", loaded.ProcessedGames)
	}
	if loaded.GamesWithVideo[0] != "a" {
		t.Errorf("GamesWithVideo should be sorted, got %v", loaded.GamesWithVideo)
	}
}

func TestPipelineState_SaveSortsAllSlicesDeterministically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	mgr := NewStateManager(path)
	mgr.nowFunc = func() time.Time { return time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC) }

	state := mgr.Reset()
	state.ProcessedGames = []string{"202512210mia", "196609020mia"}
	state.GamesWithVideo = []string{"b", "a"}
	state.GamesWithoutVideo = []string{"d", "c"}
	state.SearchQueueRemaining = []string{"g2", "g1"}
	state.ChannelsScanned = []string{"UCz", "UCa"}

	if err := mgr.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var saved PipelineState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	assertSorted := func(name string, got, want []string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s=%v want %v", name, got, want)
		}
	}
	assertSorted("ProcessedGames", saved.ProcessedGames, []string{"196609020mia", "202512210mia"})
	assertSorted("GamesWithVideo", saved.GamesWithVideo, []string{"a", "b"})
	assertSorted("GamesWithoutVideo", saved.GamesWithoutVideo, []string{"c", "d"})
	assertSorted("SearchQueueRemaining", saved.SearchQueueRemaining, []string{"g1", "g2"})
	assertSorted("ChannelsScanned", saved.ChannelsScanned, []string{"UCa", "UCz"})
}
