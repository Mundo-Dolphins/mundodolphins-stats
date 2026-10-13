package pagejson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/footballdb"
	"github.com/Mundo-Dolphins/mundodolphins-stats/nflcom"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

func TestGameBuilder_BuildFromFile(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json").SetTime(time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC))
	page, err := builder.BuildFromFile("testdata/201309300nor.json", "201309300nor")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}
	if page.Page.Kind != PageKindGame {
		t.Fatalf("unexpected kind: %s", page.Page.Kind)
	}
	if page.Page.Date != "2013-09-30" {
		t.Fatalf("expected normalized date 2013-09-30, got %q", page.Page.Date)
	}
	for _, sec := range []string{
		"Videos del partido", "Anotaciones", "Equipos",
		"Información del partido", "Oficiales", "Series ofensivas",
		"Conteo de snaps", "Estadísticas avanzadas",
	} {
		if _, ok := page.Sections[sec]; !ok {
			t.Fatalf("missing section %q", sec)
		}
	}
}

func TestGameBuilder_MinimalSectionsWithoutVideos(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json")
	gameData := map[string]interface{}{
		"game_id": "202312310mia",
		"data": map[string]interface{}{
			"scorebox": map[string]interface{}{
				"teams": []interface{}{
					map[string]interface{}{"name": map[string]interface{}{"content": "Team A", "link": "/teams/tma/2023.htm"}, "score": 20},
					map[string]interface{}{"name": map[string]interface{}{"content": "Miami Dolphins", "link": "/teams/mia/2023.htm"}, "score": 24},
				},
				"meta": map[string]interface{}{"date": "December 31, 2023"},
			},
			"youtube": map[string]interface{}{"status": "unknown", "videos": nil},
			"tables":  map[string]interface{}{},
		},
	}
	page, err := builder.Build(gameData, "202312310mia")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	videos, ok := page.Sections["Videos del partido"].([]VideoInfo)
	if !ok {
		t.Fatalf("videos section type = %T", page.Sections["Videos del partido"])
	}
	if len(videos) != 0 {
		t.Fatalf("expected empty videos, got %d", len(videos))
	}
}

func TestGameBuilder_UsesGamebookTeamStatsAndSnapCountsAsFallback(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json")
	env := boxscore.BoxscoreEnvelope{
		GameID: "202509290mia",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "New York Jets"}, Score: 21},
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Score: 27},
				},
			},
			NFLCom: &nflcom.GameData{
				GamebookData: &nflcom.GamebookData{
					TeamStats: []nflcom.GamebookTeamStat{
						{Stat: "TOTAL FIRST DOWNS", Visitor: "23", Home: "19"},
					},
					SnapCounts: []nflcom.GamebookSnapCount{
						{Team: "Miami Dolphins", Player: "K.Smith", Position: "OL", Offense: "5", OffensePct: "21%"},
					},
				},
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}

	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}

	teams := page.Sections["Equipos"].(map[string]interface{})
	dolphins := teams["Miami Dolphins"].(map[string]interface{})
	stats := dolphins["Estadísticas"].(map[string]string)
	if stats["TOTAL FIRST DOWNS"] != "19" {
		t.Fatalf("gamebook team stats fallback = %+v", stats)
	}

	snaps := page.Sections["Conteo de snaps"].(map[string]interface{})
	dolphinsSnaps := snaps["Miami Dolphins"].([]TableRow)
	if len(dolphinsSnaps) != 1 || dolphinsSnaps[0].Get("Nombre") != "K.Smith" || dolphinsSnaps[0].Get("Ofensiva") != "5" {
		t.Fatalf("gamebook snap fallback = %+v", dolphinsSnaps)
	}
}

func TestGameBuilder_UsesGamebookInfoAndOfficialsAsFallback(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json")
	env := boxscore.BoxscoreEnvelope{
		GameID: "202509290mia",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "New York Jets"}, Score: 21},
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Score: 27},
				},
			},
			NFLCom: &nflcom.GameData{
				GamebookData: &nflcom.GamebookData{
					Metadata: nflcom.GamebookMetadata{
						Date:       "Monday, 9/29/2025",
						StartTime:  "7:15 PM EDT",
						Stadium:    "Hard Rock Stadium, Miami Gardens, FL",
						Weather:    "Partly Cloudy Temp: 86 F (30.0 C) Humidity: 63%, Wind: NW 1 mph",
						Surface:    "Open Stadium on Natural Grass",
						Attendance: "65,848",
						TimeOfGame: "3:01",
					},
					Officials: []nflcom.GamebookOfficial{
						{Role: "Referee", Name: "Wrolstad, Craig (4)"},
					},
				},
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}

	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}

	info := page.Sections["Información del partido"].(TableRow)
	if info.Get("Fecha") != "29 de septiembre de 2025" || info.Get("Hora de inicio") != "7:15 PM EDT" {
		t.Fatalf("gamebook date/start fallback = %+v", info)
	}
	if info.Get("Estadio") != "Hard Rock Stadium" || info.Get("Clima") == "" || info.Get("Asistencia") != "65,848" || info.Get("Duración") != "3:01" {
		t.Fatalf("gamebook info fallback = %+v", info)
	}

	officials := page.Sections["Oficiales"].([]TableRow)
	if len(officials) != 1 || officials[0].Get("Nombre") != "Craig Wrolstad" || officials[0].Get("Posición") != "Referee" {
		t.Fatalf("gamebook officials fallback = %+v", officials)
	}
}

func TestGameBuilder_LoadsNFLComGamebookSupplementBySlug(t *testing.T) {
	processedDir := t.TempDir()
	suppPath := filepath.Join(processedDir, "nflcom", "games", "2025", "jaguars-at-dolphins-2025-pre-3.json")
	if err := os.MkdirAll(filepath.Dir(suppPath), 0755); err != nil {
		t.Fatal(err)
	}
	supp := nflcom.BuildGameEnvelope(2025, "https://www.nfl.com/games/jaguars-at-dolphins-2025-pre-3", "cache.html", nflcom.GameData{
		Slug: "jaguars-at-dolphins-2025-pre-3",
		GamebookData: &nflcom.GamebookData{
			Metadata: nflcom.GamebookMetadata{
				Weather:    "Cloudy Temp: 78 F (25.6 C) Humidity: 87%, Wind: SW 5 mph",
				Attendance: "64,808",
			},
			Officials: []nflcom.GamebookOfficial{
				{Role: "Referee", Name: "Hochuli, Shawn (83)"},
			},
		},
	})
	raw, err := json.Marshal(supp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suppPath, raw, 0644); err != nil {
		t.Fatal(err)
	}

	builder := NewGameBuilder(processedDir, "")
	env := boxscore.BoxscoreEnvelope{
		GameID: "202508230mia",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "Jacksonville Jaguars"}, Score: 6},
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Score: 14},
				},
				Meta: boxscore.ScoreboxMeta{Date: "Saturday Aug 23, 2025"},
			},
			NFLCom: &nflcom.GameData{
				Slug: "jaguars-at-dolphins-2025-pre-3",
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}

	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}
	info := page.Sections["Información del partido"].(TableRow)
	if info.Get("Clima") == "" || info.Get("Asistencia") != "64,808" {
		t.Fatalf("NFL.com supplement info was not applied: %+v", info)
	}
	officials := page.Sections["Oficiales"].([]TableRow)
	if len(officials) != 1 || officials[0].Get("Nombre") != "Shawn Hochuli" {
		t.Fatalf("NFL.com supplement officials were not applied: %+v", officials)
	}
}

func TestGameBuilder_DoesNotPublishCandidateVideos(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json")
	env := boxscore.BoxscoreEnvelope{
		GameID: "202312310mia",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{
				Status: videos.StatusCandidate,
				Videos: []videos.Video{{ID: "candidate", Title: "Candidate only"}},
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}
	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}
	videosSection, ok := page.Sections["Videos del partido"].([]VideoInfo)
	if !ok {
		t.Fatalf("videos section type = %T", page.Sections["Videos del partido"])
	}
	if len(videosSection) != 0 {
		t.Fatalf("candidate video should not be published: %+v", videosSection)
	}
}

func TestGameBuilder_PublishesEmbeddableMetadata(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json")
	embeddable := false
	env := boxscore.BoxscoreEnvelope{
		GameID: "202312310mia",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{
				Status: videos.StatusApproved,
				Videos: []videos.Video{{
					ID:              "link-only",
					Title:           "Link only",
					URL:             "https://www.youtube.com/watch?v=link-only",
					DurationSeconds: 1200,
					Embeddable:      &embeddable,
				}},
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}
	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}
	videosSection, ok := page.Sections["Videos del partido"].([]VideoInfo)
	if !ok {
		t.Fatalf("videos section type = %T", page.Sections["Videos del partido"])
	}
	if len(videosSection) != 1 || videosSection[0].Embeddable == nil || *videosSection[0].Embeddable {
		t.Fatalf("expected non-embeddable metadata in page JSON: %+v", videosSection)
	}
}

func TestGameBuilder_AddsNormalizedGameMetadata(t *testing.T) {
	builder := NewGameBuilder("", "").SetTime(time.Date(2026, 8, 14, 22, 0, 0, 0, time.UTC))
	env := boxscore.BoxscoreEnvelope{
		GameID: "202608140was",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Abbr: "mia", Score: 7},
					{Name: &boxscore.LinkContent{Content: "Washington Commanders"}, Abbr: "was", Score: 20},
				},
				Meta: boxscore.ScoreboxMeta{Date: "Friday Aug 14, 2026"},
			},
			Week:    "Preseason 1",
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			NFLCom: &nflcom.GameData{
				Gamebook: &nflcom.GamebookLink{Path: "cache/nflcom/gamebooks/2026/gamebook.pdf"},
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}

	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}
	if page.Page.Title != "Miami Dolphins vs Washington Commanders" {
		t.Fatalf("unexpected title: %q", page.Page.Title)
	}
	if page.Page.Slug != "mia-was-20260814" {
		t.Fatalf("unexpected slug: %q", page.Page.Slug)
	}
	info, ok := page.Sections["Información del partido"].(TableRow)
	if !ok {
		t.Fatalf("info section type = %T", page.Sections["Información del partido"])
	}
	if got := info.Get("Semana"); got != "Preseason 1" {
		t.Fatalf("Semana = %q, want Preseason 1", got)
	}
	if got := info.Get("Tipo de partido"); got != "preseason" {
		t.Fatalf("Tipo de partido = %q, want preseason", got)
	}
	if got := info.Get("Estado"); got != "final" {
		t.Fatalf("Estado = %q, want final", got)
	}
	if got := info.Get("Fecha"); got != "14 de agosto de 2026" {
		t.Fatalf("Fecha = %q, want translated date", got)
	}
	if got := info.Get("Gamebook oficial"); got != "assets/gamebooks/2026/202608140was.pdf" {
		t.Fatalf("Gamebook oficial = %q, want public asset path", got)
	}
}

func TestGameBuilder_NoGenericRivalPlaceholderTitle(t *testing.T) {
	builder := NewGameBuilder("", "")
	env := boxscore.BoxscoreEnvelope{
		GameID: "202609130rai",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			Tables:  map[string]boxscore.TableResult{},
		},
	}

	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}
	if page.Page.Title == "Miami Dolphins vs Rival" {
		t.Fatal("generic Rival placeholder title should not be generated")
	}
	info := page.Sections["Información del partido"].(TableRow)
	if got := info.Get("Estado"); got != "scheduled" {
		t.Fatalf("Estado = %q, want scheduled", got)
	}
	if got := info.Get("Semana"); got != "" {
		t.Fatalf("placeholder game should not get Semana, got %q", got)
	}
}

func TestGameBuilder_FileNotFound(t *testing.T) {
	builder := NewGameBuilder("testdata", "testdata/videos.json")
	if _, err := builder.BuildFromFile("testdata/nonexistent.json", "202312310xxx"); err == nil {
		t.Fatal("expected file not found error")
	}
}

func TestGameBuilder_SetTime(t *testing.T) {
	fixed := time.Date(2025, 6, 1, 10, 30, 0, 0, time.UTC)
	builder := NewGameBuilder("testdata", "testdata/videos.json").SetTime(fixed)
	page, err := builder.BuildFromFile("testdata/201309300nor.json", "201309300nor")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}
	if page.GeneratedAt != fixed.Format(time.RFC3339) {
		t.Fatalf("unexpected generated_at: %s", page.GeneratedAt)
	}
}

func TestSplitTeamStats_TranslatesStatLabels(t *testing.T) {
	tbl := boxscore.TableResult{
		Rows: []map[string]any{
			{
				"stat":      "First Downs",
				"vis_stat":  "23",
				"home_stat": "21",
			},
			{
				"stat":      "Time of Possession",
				"vis_stat":  "31:47",
				"home_stat": "28:13",
			},
		},
	}

	stats := splitTeamStats(tbl)

	if got := stats["vis"]["Primeros downs"]; got != "23" {
		t.Fatalf("expected translated vis first downs=23, got %q", got)
	}
	if got := stats["home"]["Tiempo de posesión"]; got != "28:13" {
		t.Fatalf("expected translated home time of possession=28:13, got %q", got)
	}

	if _, ok := stats["vis"]["First Downs"]; ok {
		t.Fatalf("expected original English label to be replaced")
	}
}

func TestGameBuilder_MergesNFLComStatsIntoPlayerSections(t *testing.T) {
	builder := NewGameBuilder("", "")
	env := boxscore.BoxscoreEnvelope{
		GameID: "202208130tam",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Abbr: "mia", Score: 26},
					{Name: &boxscore.LinkContent{Content: "Tampa Bay Buccaneers"}, Abbr: "tam", Score: 24},
				},
			},
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			NFLCom: &nflcom.GameData{
				Stats: &nflcom.GameStats{
					AwayTeam: map[string]any{
						"stats": []any{
							map[string]any{
								"title": "MIA PASSING",
								"table": map[string]any{
									"columns": []any{
										map[string]any{"title": "PLAYER"},
										map[string]any{"title": "CMP"},
									},
									"rows": []any{
										[]any{
											map[string]any{"text": "S. Thompson"},
											map[string]any{"text": 9},
										},
									},
								},
							},
						},
					},
					HomeTeam: map[string]any{
						"stats": []any{
							map[string]any{
								"title": "TB RUSHING",
								"table": map[string]any{
									"columns": []any{
										map[string]any{"title": "PLAYER"},
										map[string]any{"title": "YDS"},
									},
									"rows": []any{
										[]any{
											map[string]any{"text": "R. White"},
											map[string]any{"value": 18},
										},
									},
								},
							},
						},
					},
				},
			},
			Tables: map[string]boxscore.TableResult{},
		},
	}

	page, err := builder.BuildEnvelope(env)
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}
	teams, ok := page.Sections["Equipos"].(map[string]interface{})
	if !ok {
		t.Fatalf("Equipos section type = %T", page.Sections["Equipos"])
	}
	dolphins := teams["Miami Dolphins"].(map[string]interface{})
	if _, exists := dolphins["Estadisticas NFL.com"]; exists {
		t.Fatalf("NFL.com stats should be merged into Hugo-compatible player sections")
	}
	playerStats := dolphins["Estadísticas por jugador"].(map[string]interface{})
	rows := playerStats["Ofensiva"].([]TableRow)
	if len(rows) != 1 {
		t.Fatalf("offensive rows = %d, want 1", len(rows))
	}
	if got := rows[0].Get("Nombre"); got != "S. Thompson" {
		t.Fatalf("Nombre = %q, want S. Thompson", got)
	}
	if got := rows[0].Get("Comp"); got != "9" {
		t.Fatalf("Comp = %q, want 9", got)
	}
}

func TestGameBuilder_LoadsFootballDBSupplementForPreseasonFallback(t *testing.T) {
	tmp := t.TempDir()
	gameID := "202608140was"
	seasonDir := filepath.Join(tmp, "games", "2026")
	if err := os.MkdirAll(seasonDir, 0755); err != nil {
		t.Fatalf("mkdir pfr games: %v", err)
	}
	env := boxscore.BoxscoreEnvelope{
		Schema: boxscore.SchemaInfo{Name: boxscore.BoxscoreSchemaName, Version: boxscore.BoxscoreSchemaVersion},
		GameID: gameID,
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins", Link: "/teams/mia/2026.htm"}, Abbr: "mia", Score: 7},
					{Name: &boxscore.LinkContent{Content: "Washington Commanders", Link: "/teams/was/2026.htm"}, Abbr: "was", Score: 20},
				},
				Meta: boxscore.ScoreboxMeta{Date: "August 14, 2026"},
			},
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			Tables:  map[string]boxscore.TableResult{},
		},
	}
	writeJSONFile(t, filepath.Join(seasonDir, gameID+".json"), env)

	fdbDir := filepath.Join(tmp, "footballdb", "games", "2026")
	if err := os.MkdirAll(fdbDir, 0755); err != nil {
		t.Fatalf("mkdir fdb games: %v", err)
	}
	fdb := footballdb.BuildGameEnvelope(2026, "2026081404", "", "", time.Time{}, footballdb.GameData{
		GameID: "2026081404",
		Tables: map[string]footballdb.TableResult{
			"table_0": {Headers: []string{"", "1", "2", "3", "4", "total"}, Rows: []map[string]any{
				{"total": "Total"},
				{"col_0": map[string]any{"content": "Miami DolphinsMiami"}, "1": "7", "2": "0", "3": "0", "4": "0", "total": "7"},
				{"col_0": map[string]any{"content": "Washington CommandersWashington"}, "1": "0", "2": "17", "3": "0", "4": "3", "total": "20"},
			}},
			"table_1": {Headers: []string{"1st_quarter", "mia", "was"}, Rows: []map[string]any{
				{"1st_quarter": "1st Quarter", "mia": "MIA", "was": "WAS"},
				{"1st_quarter": "MIA", "mia": "7:06", "was": "De'Von Achane 1 run (Riley Patterson Kick)", "col_3": "7", "col_4": "0"},
				{"1st_quarter": "2nd Quarter", "mia": "MIA", "was": "WAS"},
				{"1st_quarter": "WAS", "mia": "4:47", "was": "FG Jake Moody 29", "col_3": "7", "col_4": "3"},
			}},
			"table_2": {Headers: []string{"", "miamimia", "washingtonwas"}, Rows: []map[string]any{
				{"col_0": "First downs", "miamimia": "10", "washingtonwas": "21"},
				{"col_0": "Total Net Yards", "miamimia": "226", "washingtonwas": "334"},
			}},
			"table_3": {Headers: []string{"", "miamimia", "washingtonwas"}, Rows: []map[string]any{
				{"col_0": "Time of Possession", "miamimia": "24:37", "washingtonwas": "35:23"},
			}},
			"table_4": {Headers: []string{"miami_dolphinsmiami", "att", "cmp", "yds", "ypa", "td", "int", "lg", "sack", "loss", "rate"}, Rows: []map[string]any{
				{"miami_dolphinsmiami": map[string]any{"content": "Cam Miller"}, "att": "11", "cmp": "3", "yds": "39", "td": "0", "int": "2", "lg": "24", "sack": "0", "loss": "0", "rate": "2.3"},
				{"miami_dolphinsmiami": "TOTAL", "att": "24", "cmp": "8", "yds": "109", "td": "0", "int": "3", "sack": "1", "loss": "0"},
			}},
			"table_6": {Headers: []string{"miami_dolphinsmiami", "att", "yds", "avg", "lg", "td", "fd"}, Rows: []map[string]any{
				{"miami_dolphinsmiami": map[string]any{"content": "De'Von Achane"}, "att": "7", "yds": "39", "td": "1", "lg": "14"},
				{"miami_dolphinsmiami": "TOTAL", "att": "28", "yds": "117", "td": "1"},
			}},
			"table_8": {Headers: []string{"miami_dolphinsmiami", "rec", "yds", "avg", "lg", "td", "fd", "tar", "yac"}, Rows: []map[string]any{
				{"miami_dolphinsmiami": map[string]any{"content": "Malik Washington"}, "rec": "3", "yds": "15", "td": "0", "lg": "8", "tar": "3"},
			}},
			"table_16": {Headers: []string{"miami_dolphinsmiami", "pat", "fg", "0_19", "20_29", "30_39", "40_49", "50", "lg", "miss", "pts"}, Rows: []map[string]any{
				{"miami_dolphinsmiami": map[string]any{"content": "Riley Patterson"}, "pat": "1/1", "fg": "0/0", "lg": "0"},
			}},
			"table_20": {Headers: []string{"miami_dolphinsmiami", "int", "yds", "avg", "lg", "td", "solo", "ast", "tot", "sack", "ydsl"}, Rows: []map[string]any{
				{"miami_dolphinsmiami": map[string]any{"content": "Jackson Woodard"}, "int": "0", "yds": "0", "lg": "0", "td": "0", "solo": "2", "ast": "5", "tot": "7", "sack": "0.0"},
			}},
		},
	})
	writeJSONFile(t, filepath.Join(fdbDir, "2026081404.json"), fdb)

	page, err := NewGameBuilder(tmp, "").BuildFromGameID(gameID)
	if err != nil {
		t.Fatalf("BuildFromGameID failed: %v", err)
	}
	anotaciones := page.Sections["Anotaciones"].(map[string]interface{})
	linescore := anotaciones["linescore"].([]TableRow)
	scoringSummary := anotaciones["scoring_summary"].([]TableRow)
	if len(linescore) != 2 {
		t.Fatalf("linescore rows = %d, want 2", len(linescore))
	}
	if got := linescore[0].Get("Equipo"); got != "Miami Dolphins" {
		t.Fatalf("linescore team = %q, want Miami Dolphins", got)
	}
	if got := linescore[0].Get("Final"); got != "7" {
		t.Fatalf("linescore final = %q, want 7", got)
	}
	if len(scoringSummary) != 2 {
		t.Fatalf("scoring_summary rows = %d, want 2", len(scoringSummary))
	}
	if got := scoringSummary[0].Get("Descripción"); got != "De'Von Achane carrera de 1 yarda (Riley Patterson kick)" {
		t.Fatalf("scoring description = %q", got)
	}
	if got := scoringSummary[1].Get("Descripción"); got != "Jake Moody field goal de 29 yardas" {
		t.Fatalf("field goal description = %q", got)
	}

	teams := page.Sections["Equipos"].(map[string]interface{})
	dolphins := teams["Miami Dolphins"].(map[string]interface{})
	teamStats := dolphins["Estadísticas"].(map[string]string)
	if got := teamStats["Primeros downs"]; got != "10" {
		t.Fatalf("Primeros downs = %q, want 10", got)
	}
	if got := teamStats["Comp-Int-Yds-TD-INT"]; got != "8-24-109-0-3" {
		t.Fatalf("passing team summary = %q", got)
	}
	playerStats := dolphins["Estadísticas por jugador"].(map[string]interface{})
	offense := playerStats["Ofensiva"].([]TableRow)
	defense := playerStats["Defensiva"].([]TableRow)
	special := playerStats["Equipos especiales"].([]TableRow)
	if len(offense) != 3 || len(defense) != 1 || len(special) != 1 {
		t.Fatalf("unexpected player rows: offense=%d defense=%d special=%d", len(offense), len(defense), len(special))
	}
	if got := offense[0].Get("Nombre"); got != "Cam Miller" {
		t.Fatalf("first offensive player = %q", got)
	}
	if got := defense[0].Get("Tackles"); got != "7" {
		t.Fatalf("defensive tackles = %q, want 7", got)
	}
	if got := special[0].Get("XPm"); got != "1" {
		t.Fatalf("special teams XPm = %q, want 1", got)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestNFLComCellTextHandlesNestedCells(t *testing.T) {
	got := nflcomCellText([]any{
		map[string]any{"text": "Jaylen"},
		map[string]any{"value": "Waddle"},
	})
	if got != "Jaylen Waddle" {
		t.Fatalf("nflcomCellText nested = %q, want Jaylen Waddle", got)
	}
}
