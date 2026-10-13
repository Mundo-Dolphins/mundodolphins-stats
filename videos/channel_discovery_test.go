package videos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
)

func TestChannelDiscovery_AcceptsAndBlacklists(t *testing.T) {
	channelsCalls := 0
	playlistCalls := 0
	videosCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/channels":
			channelsCalls++
			id := r.URL.Query().Get("id")
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"id":             id,
					"snippet":        map[string]any{"title": "Channel " + id},
					"contentDetails": map[string]any{"relatedPlaylists": map[string]any{"uploads": "UU_" + id}},
				}},
			})
		case "/playlistItems":
			playlistCalls++
			pid := r.URL.Query().Get("playlistId")
			var items []map[string]any
			if pid == "UU_UCgood" {
				items = []map[string]any{
					{"snippet": map[string]any{"title": "A", "resourceId": map[string]any{"videoId": "g1"}}},
					{"snippet": map[string]any{"title": "B", "resourceId": map[string]any{"videoId": "g2"}}},
					{"snippet": map[string]any{"title": "C", "resourceId": map[string]any{"videoId": "g3"}}},
				}
			} else {
				items = []map[string]any{
					{"snippet": map[string]any{"title": "A", "resourceId": map[string]any{"videoId": "b1"}}},
					{"snippet": map[string]any{"title": "B", "resourceId": map[string]any{"videoId": "b2"}}},
					{"snippet": map[string]any{"title": "C", "resourceId": map[string]any{"videoId": "b3"}}},
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"items": items, "pageInfo": map[string]any{"totalResults": len(items)}})
		case "/videos":
			videosCalls++
			ids := splitComma(r.URL.Query().Get("id"))
			payload := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				if id == "g1" || id == "g2" || id == "g3" {
					payload = append(payload, map[string]any{
						"id": id,
						"snippet": map[string]any{
							"title":        "Miami Dolphins vs New York Jets full game 1972",
							"description":  "Full game broadcast 1972",
							"channelTitle": "Good Channel",
							"channelId":    "UCgood",
						},
						"contentDetails": map[string]any{"duration": "PT1H40M0S"},
						"statistics":     map[string]any{"viewCount": "1000"},
						"status":         map[string]any{"embeddable": true},
					})
				} else {
					payload = append(payload, map[string]any{
						"id": id,
						"snippet": map[string]any{
							"title":        "Top 10 Plays and reaction",
							"description":  "podcast recap",
							"channelTitle": "Bad Channel",
							"channelId":    "UCbad",
						},
						"contentDetails": map[string]any{"duration": "PT12M0S"},
						"statistics":     map[string]any{"viewCount": "1000"},
						"status":         map[string]any{"embeddable": true},
					})
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"items": payload})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("k", srv.Client())
	client.BaseURL = srv.URL

	tmp := t.TempDir()
	paths := NewPaths(tmp)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	cache := NewDiskCacheManager(paths.CacheDir)
	quota := NewQuotaManager(5000)

	mgr := NewChannelDiscoveryManager(paths, cache, client, quota)
	mgr.now = func() time.Time { return time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC) }

	cf := &CacheFile{Items: map[string]*CacheItem{
		"g1": {
			Status:      "selected",
			Videos:      []Video{{ID: "sv-good", ChannelID: "UCgood", Channel: "Good Channel"}},
			AutoScoring: &AutoScoringMeta{Source: "search"},
		},
		"g2": {
			Status:      "selected",
			Videos:      []Video{{ID: "sv-bad", ChannelID: "UCbad", Channel: "Bad Channel"}},
			AutoScoring: &AutoScoringMeta{Source: "search"},
		},
	}}

	reg := &ChannelRegistryFile{}
	accepted, err := mgr.DiscoverFromSearchMatches(context.Background(), cf, reg, map[string]bool{})
	if err != nil {
		t.Fatalf("DiscoverFromSearchMatches() error: %v", err)
	}

	if !accepted["UCgood"] {
		t.Fatalf("expected UCgood accepted")
	}
	if accepted["UCbad"] {
		t.Fatalf("UCbad should be rejected")
	}
	bf, err := loadJSON[ChannelBlacklistFile](paths.BlacklistFile)
	if err != nil {
		t.Fatal(err)
	}
	if bf == nil || len(bf.Channels) != 1 || bf.Channels[0].ID != "UCbad" {
		t.Fatalf("expected UCbad in blacklist, got %+v", bf)
	}

	if channelsCalls == 0 || playlistCalls == 0 || videosCalls == 0 {
		t.Fatalf("expected API calls for validation")
	}
}

func TestChannelDiscovery_DoesNotRepeatValidationWhenCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("k", srv.Client())
	client.BaseURL = srv.URL

	tmp := t.TempDir()
	paths := NewPaths(tmp)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	entry := &ChannelDiscoveryCacheEntry{
		ChannelID:   "UCcached",
		ChannelName: "Cached",
		Accepted:    true,
		Ratio:       0.8,
		ValidatedAt: time.Now(),
	}
	if err := saveJSON(filepath.Join(paths.ChannelDiscoveryCache, "UCcached.json"), entry); err != nil {
		t.Fatal(err)
	}

	mgr := NewChannelDiscoveryManager(paths, NewDiskCacheManager(paths.CacheDir), client, NewQuotaManager(5000))

	cf := &CacheFile{Items: map[string]*CacheItem{
		"g1": {
			Status:      "selected",
			Videos:      []Video{{ID: "new-video", ChannelID: "UCcached", Channel: "Cached"}},
			AutoScoring: &AutoScoringMeta{Source: "search"},
		},
	}}

	accepted, err := mgr.DiscoverFromSearchMatches(context.Background(), cf, &ChannelRegistryFile{}, map[string]bool{})
	if err != nil {
		t.Fatalf("DiscoverFromSearchMatches() error: %v", err)
	}
	if !accepted["UCcached"] {
		t.Fatalf("expected channel accepted from cache")
	}
}

func TestChannelDiscovery_SkipsWhenQuotaLow(t *testing.T) {
	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("k", srv.Client())
	client.BaseURL = srv.URL

	tmp := t.TempDir()
	paths := NewPaths(tmp)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	mgr := NewChannelDiscoveryManager(paths, NewDiskCacheManager(paths.CacheDir), client, NewQuotaManager(199))

	cf := &CacheFile{Items: map[string]*CacheItem{
		"g1": {
			Status:      "selected",
			Videos:      []Video{{ID: "new-video", ChannelID: "UCquota", Channel: "Quota Channel"}},
			AutoScoring: &AutoScoringMeta{Source: "search"},
		},
	}}

	accepted, err := mgr.DiscoverFromSearchMatches(context.Background(), cf, &ChannelRegistryFile{}, map[string]bool{})
	if err != nil {
		t.Fatalf("DiscoverFromSearchMatches() error: %v", err)
	}
	if len(accepted) != 0 {
		t.Fatalf("expected no accepted channels when quota is low")
	}
	if called != 0 {
		t.Fatalf("expected no API calls when quota is low, got %d", called)
	}
}

func TestChannelDiscovery_SkipsBlacklistedChannel(t *testing.T) {
	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := youtubeapi.NewClient("k", srv.Client())
	client.BaseURL = srv.URL

	tmp := t.TempDir()
	paths := NewPaths(tmp)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(paths.BlacklistFile, &ChannelBlacklistFile{Channels: []BlacklistedChannel{{
		ID:     "UCblacklisted",
		Name:   "Blacklisted",
		Reason: "low quality",
	}}}); err != nil {
		t.Fatal(err)
	}

	mgr := NewChannelDiscoveryManager(paths, NewDiskCacheManager(paths.CacheDir), client, NewQuotaManager(5000))

	cf := &CacheFile{Items: map[string]*CacheItem{
		"g1": {
			Status:      "selected",
			Videos:      []Video{{ID: "new-video", ChannelID: "UCblacklisted", Channel: "Blacklisted"}},
			AutoScoring: &AutoScoringMeta{Source: "search"},
		},
	}}

	accepted, err := mgr.DiscoverFromSearchMatches(context.Background(), cf, &ChannelRegistryFile{}, map[string]bool{})
	if err != nil {
		t.Fatalf("DiscoverFromSearchMatches() error: %v", err)
	}
	if len(accepted) != 0 {
		t.Fatalf("expected no accepted channels for blacklisted candidate")
	}
	if called != 0 {
		t.Fatalf("expected no API calls for blacklisted channel, got %d", called)
	}
}

func TestIsValidDiscoverySample(t *testing.T) {
	tests := []struct {
		name string
		c    youtubeapi.VideoCandidate
		want bool
	}{
		{
			name: "valid full game",
			c: youtubeapi.VideoCandidate{
				Title:       "Miami Dolphins vs Buffalo Bills Full Game 2024",
				Description: "NFL Full Game Highlights",
				Duration:    time.Hour,
			},
			want: true,
		},
		{
			name: "too short",
			c: youtubeapi.VideoCandidate{
				Title:       "Miami Dolphins vs Buffalo Bills Full Game 2024",
				Description: "Highlights",
				Duration:    5 * time.Minute, // less than 600s
			},
			want: false,
		},
		{
			name: "not full game",
			c: youtubeapi.VideoCandidate{
				Title:       "Miami Dolphins 2024 Season Preview",
				Description: "Preview of the upcoming season",
				Duration:    time.Hour,
			},
			want: false,
		},
		{
			name: "no year in text",
			c: youtubeapi.VideoCandidate{
				Title:       "Miami Dolphins vs Buffalo Bills Full Game",
				Description: "NFL Full Game Highlights",
				Duration:    time.Hour,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidDiscoverySample(tt.c)
			if got != tt.want {
				t.Errorf("isValidDiscoverySample() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveBlacklist_NilInput(t *testing.T) {
	now := time.Now()
	mgr := &ChannelDiscoveryManager{now: func() time.Time { return now }}
	err := mgr.saveBlacklist(nil)
	if err != nil {
		t.Errorf("expected nil error for nil blacklist, got: %v", err)
	}
}

func TestAddToBlacklist_NilInputs(t *testing.T) {
	now := time.Now()
	mgr := &ChannelDiscoveryManager{now: func() time.Time { return now }}
	// Calling with nil should not panic
	mgr.addToBlacklist(nil, &ChannelDiscoveryCacheEntry{ChannelID: "test"})
	mgr.addToBlacklist(&ChannelBlacklistFile{}, nil)
}
