package videos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func channelValidationServer(t *testing.T, validIDs map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels":
			id := r.URL.Query().Get("id")
			items := []map[string]any{}
			if validIDs[id] {
				items = append(items, map[string]any{
					"id":             id,
					"snippet":        map[string]any{"title": "Channel " + id},
					"contentDetails": map[string]any{"relatedPlaylists": map[string]any{"uploads": "UU_" + id}},
				})
			}
			json.NewEncoder(w).Encode(map[string]any{"items": items})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestChannelRegistry_SelectWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewChannelRegistry(filepath.Join(tmpDir, "channels.json"))

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	selected := registry.SelectForScan(reg, DefaultMinChannelVideos, DefaultMinChannelScore)
	if len(selected) != 0 {
		t.Fatalf("selected=%d want=0", len(selected))
	}
}

func TestChannelRegistry_AddsNewChannelFromAcceptedVideo(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewChannelRegistry(filepath.Join(tmpDir, "channels.json"))
	registry.now = func() time.Time { return time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC) }

	reg := &ChannelRegistryFile{}
	cf := &CacheFile{Items: map[string]*CacheItem{
		"game1": {
			Status: "selected",
			Videos: []Video{{
				ID:        "vid-1",
				ChannelID: "UCnew123",
				Channel:   "New Channel",
			}},
		},
	}}

	added, updated := registry.UpdateFromAccepted(reg, cf, map[string]bool{}, nil)
	if added != 1 || updated != 0 {
		t.Fatalf("added=%d updated=%d", added, updated)
	}
	if len(reg.Channels) != 1 {
		t.Fatalf("channels=%d want=1", len(reg.Channels))
	}
	ch := reg.Channels[0]
	if ch.ID != "UCnew123" || ch.Name != "New Channel" {
		t.Fatalf("unexpected channel: %+v", ch)
	}
	if ch.VideosCount != 1 || ch.Score != 1 {
		t.Fatalf("unexpected counters: %+v", ch)
	}
	if ch.LastSeen != "2026-03-25" {
		t.Fatalf("last_seen=%q want=2026-03-25", ch.LastSeen)
	}
}

func TestChannelRegistry_IncrementsExistingChannel(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewChannelRegistry(filepath.Join(tmpDir, "channels.json"))
	registry.now = func() time.Time { return time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC) }

	reg := &ChannelRegistryFile{Channels: []ChannelRecord{{
		ID:          "UCexisting",
		Name:        "Existing",
		VideosCount: 2,
		Score:       2,
		LastSeen:    "2026-03-20",
	}}}
	cf := &CacheFile{Items: map[string]*CacheItem{
		"game1": {
			Status: "selected",
			Videos: []Video{{
				ID:        "vid-2",
				ChannelID: "UCexisting",
				Channel:   "Existing Updated",
			}},
		},
	}}

	added, updated := registry.UpdateFromAccepted(reg, cf, map[string]bool{}, nil)
	if added != 0 || updated != 1 {
		t.Fatalf("added=%d updated=%d", added, updated)
	}
	if reg.Channels[0].VideosCount != 3 || reg.Channels[0].Score != 1 {
		t.Fatalf("unexpected counters: %+v", reg.Channels[0])
	}
	if reg.Channels[0].Name != "Existing Updated" {
		t.Fatalf("name=%q want=Existing Updated", reg.Channels[0].Name)
	}
}

func TestChannelRegistry_SelectSkipsLowScore(t *testing.T) {
	registry := NewChannelRegistry("unused")
	reg := &ChannelRegistryFile{Channels: []ChannelRecord{
		{ID: "UClow", Name: "Low", VideosCount: 1, Score: 0.2},
		{ID: "UChigh", Name: "High", VideosCount: 5, Score: 0.9},
	}}

	selected := registry.SelectForScan(reg, 2, 0.3)
	if len(selected) != 1 {
		t.Fatalf("selected=%d want=1", len(selected))
	}
	if selected[0].ID != "UChigh" {
		t.Fatalf("selected id=%q want=UChigh", selected[0].ID)
	}
}

func TestChannelRegistry_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "channels.json")
	registry := NewChannelRegistry(path)

	reg := &ChannelRegistryFile{Channels: []ChannelRecord{{
		ID:          "UC1",
		Name:        "Channel 1",
		VideosCount: 4,
		LastSeen:    "2026-03-25",
		Score:       4,
	}}}
	if err := registry.Save(reg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("channels.json should exist: %v", err)
	}

	loaded, err := registry.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Channels) != 1 || loaded.Channels[0].ID != "UC1" {
		t.Fatalf("unexpected loaded channels: %+v", loaded.Channels)
	}
}

func TestChannelRegistry_NewSearchChannelRequiresAllowlist(t *testing.T) {
	registry := NewChannelRegistry("unused")
	registry.now = func() time.Time { return time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC) }

	reg := &ChannelRegistryFile{}
	cf := &CacheFile{Items: map[string]*CacheItem{
		"game1": {
			Status:      "selected",
			Videos:      []Video{{ID: "vid-1", ChannelID: "UCsearch", Channel: "Search Channel"}},
			AutoScoring: &AutoScoringMeta{Source: "search"},
		},
	}}

	added, updated := registry.UpdateFromAccepted(reg, cf, map[string]bool{}, map[string]bool{})
	if added != 0 || updated != 0 {
		t.Fatalf("added=%d updated=%d", added, updated)
	}
	if len(reg.Channels) != 0 {
		t.Fatalf("expected channel not to be added without allowlist")
	}
}

func TestChannelRegistry_NewBootstrapChannelBypassesAllowlist(t *testing.T) {
	registry := NewChannelRegistry("unused")
	registry.now = func() time.Time { return time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC) }

	reg := &ChannelRegistryFile{}
	cf := &CacheFile{Items: map[string]*CacheItem{
		"game1": {
			Status:      "selected",
			Videos:      []Video{{ID: "vid-1", ChannelID: "UCbootstrap", Channel: "Bootstrap Channel"}},
			AutoScoring: &AutoScoringMeta{Source: "channel-scan"},
		},
	}}

	added, updated := registry.UpdateFromAccepted(reg, cf, map[string]bool{}, map[string]bool{})
	if added != 1 || updated != 0 {
		t.Fatalf("added=%d updated=%d", added, updated)
	}
	if len(reg.Channels) != 1 || reg.Channels[0].ID != "UCbootstrap" {
		t.Fatalf("unexpected channels: %+v", reg.Channels)
	}
}

func TestValidateChannelID(t *testing.T) {
	srv := channelValidationServer(t, map[string]bool{"UCvalid": true})
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	if !ValidateChannelID(context.Background(), client, "UCvalid") {
		t.Fatalf("expected valid channel id")
	}
	if ValidateChannelID(context.Background(), client, "UCinvalid") {
		t.Fatalf("expected invalid channel id")
	}
}

func TestChannelRegistry_SanitizeRemovesInvalidChannel(t *testing.T) {
	srv := channelValidationServer(t, map[string]bool{"UCvalid": true})
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	registry := NewChannelRegistry("unused")
	reg := &ChannelRegistryFile{Channels: []ChannelRecord{
		{ID: "UCvalid", Name: "Valid", VideosCount: 3, AnalyzedVideos: 3, Score: 1},
		{ID: "UCinvalid", Name: "Broken", VideosCount: 2, AnalyzedVideos: 2, Score: 1},
	}}

	stats, err := registry.Sanitize(context.Background(), reg, nil, client)
	if err != nil {
		t.Fatalf("Sanitize() error: %v", err)
	}
	if len(reg.Channels) != 1 || reg.Channels[0].ID != "UCvalid" {
		t.Fatalf("unexpected channels after sanitize: %+v", reg.Channels)
	}
	if stats.Removed != 1 {
		t.Fatalf("removed=%d want=1", stats.Removed)
	}
}

func TestChannelRegistry_SanitizeRecoversInvalidChannelFromVideos(t *testing.T) {
	srv := channelValidationServer(t, map[string]bool{"UCrecovered": true})
	defer srv.Close()

	client := youtubeapi.NewClient("test-key", srv.Client())
	client.BaseURL = srv.URL

	registry := NewChannelRegistry("unused")
	reg := &ChannelRegistryFile{Channels: []ChannelRecord{{
		ID:             "UCinvalid",
		Name:           "Recovered Channel",
		VideosCount:    2,
		AnalyzedVideos: 2,
		Score:          1,
	}}}
	cf := &CacheFile{Items: map[string]*CacheItem{
		"game1": {
			Status: "selected",
			Videos: []Video{{ID: "v1", ChannelID: "UCrecovered", Channel: "Recovered Channel"}},
		},
	}}

	stats, err := registry.Sanitize(context.Background(), reg, cf, client)
	if err != nil {
		t.Fatalf("Sanitize() error: %v", err)
	}
	if len(reg.Channels) != 1 || reg.Channels[0].ID != "UCrecovered" {
		t.Fatalf("unexpected channels after recovery: %+v", reg.Channels)
	}
	if stats.Recovered != 1 || stats.Removed != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if reg.Channels[0].Source != "video-recovery" {
		t.Fatalf("source=%q want=video-recovery", reg.Channels[0].Source)
	}
}

func TestSelectedVideoIDSet_OnlySelectedItems(t *testing.T) {
	cf := &CacheFile{Items: map[string]*CacheItem{
		"g1": {Status: "selected", Videos: []Video{{ID: "v1"}, {ID: "v2"}}},
		"g2": {Status: "none", Videos: []Video{{ID: "v3"}}},
		"g3": {Status: "selected", Videos: []Video{{ID: ""}}},
	}}

	set := selectedVideoIDSet(cf)
	if len(set) != 2 {
		t.Fatalf("selected ids=%d want=2", len(set))
	}
	if !set["v1"] || !set["v2"] {
		t.Fatalf("expected v1 and v2 in selected set")
	}
	if set["v3"] {
		t.Fatalf("did not expect v3 in selected set")
	}
}

func TestMergeChannelRecords_MergesDuplicatesAndKeepsLatest(t *testing.T) {
	input := []ChannelRecord{
		{
			ID:             "UCdup",
			Name:           "Name A",
			VideosCount:    2,
			AnalyzedVideos: 2,
			DiscoveredAt:   "2026-03-24",
			LastSeen:       "2026-03-24",
			Score:          1,
			Source:         "search",
		},
		{
			ID:             "UCdup",
			Name:           "Name B",
			VideosCount:    3,
			AnalyzedVideos: 10,
			DiscoveredAt:   "2026-03-20",
			LastSeen:       "2026-03-25",
			Score:          0.3,
			Source:         "video-recovery",
		},
		{
			ID:             "UCother",
			Name:           "Other",
			VideosCount:    1,
			AnalyzedVideos: 1,
			DiscoveredAt:   "2026-03-25",
			LastSeen:       "2026-03-25",
			Score:          1,
			Source:         "search",
		},
	}

	merged, mergedCount := mergeChannelRecords(input)
	if mergedCount != 1 {
		t.Fatalf("mergedCount=%d want=1", mergedCount)
	}
	if len(merged) != 2 {
		t.Fatalf("len(merged)=%d want=2", len(merged))
	}

	var dup ChannelRecord
	for _, ch := range merged {
		if ch.ID == "UCdup" {
			dup = ch
			break
		}
	}
	if dup.ID == "" {
		t.Fatalf("expected merged UCdup record")
	}
	if dup.VideosCount != 5 {
		t.Fatalf("videos_count=%d want=5", dup.VideosCount)
	}
	if dup.AnalyzedVideos != 10 {
		t.Fatalf("analyzed_videos=%d want=10", dup.AnalyzedVideos)
	}
	if dup.LastSeen != "2026-03-25" {
		t.Fatalf("last_seen=%q want=2026-03-25", dup.LastSeen)
	}
	if dup.DiscoveredAt != "2026-03-20" {
		t.Fatalf("discovered_at=%q want=2026-03-20", dup.DiscoveredAt)
	}
	if dup.Source != "video-recovery" {
		t.Fatalf("source=%q want=video-recovery", dup.Source)
	}
	if dup.Score <= 0 || dup.Score >= 1 {
		t.Fatalf("unexpected score=%f", dup.Score)
	}
}

func TestChannelRegistrySortChannels_ByScoreThenVideosThenID(t *testing.T) {
	registry := NewChannelRegistry("unused")
	channels := []ChannelRecord{
		{ID: "UCb", Score: 0.5, VideosCount: 2},
		{ID: "UCa", Score: 0.9, VideosCount: 1},
		{ID: "UCc", Score: 0.5, VideosCount: 3},
	}

	registry.sortChannels(channels)

	if channels[0].ID != "UCa" {
		t.Fatalf("first=%q want=UCa", channels[0].ID)
	}
	if channels[1].ID != "UCc" {
		t.Fatalf("second=%q want=UCc", channels[1].ID)
	}
	if channels[2].ID != "UCb" {
		t.Fatalf("third=%q want=UCb", channels[2].ID)
	}
}
