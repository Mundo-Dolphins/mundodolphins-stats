package pagejson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

func TestSeasonBuilder_BuildFromFile_2025(t *testing.T) {
	builder := NewSeasonBuilder("testdata").SetTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	page, err := builder.BuildFromFile("testdata/2025.json")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}

	if page.Schema.Name != SchemaName || page.Schema.Version != SchemaVersion {
		t.Fatalf("unexpected schema: %+v", page.Schema)
	}
	if page.Page.Kind != PageKindSeason {
		t.Fatalf("unexpected kind: %s", page.Page.Kind)
	}
	for _, sec := range []string{"Resumen", "Roster", "Partidos de la temporada regular", "Jugadores seleccionados en el draft", "Estadísticas de la temporada", "Líderes", "Lesiones y reservas"} {
		if _, ok := page.Sections[sec]; !ok {
			t.Fatalf("missing expected section %q", sec)
		}
	}

	roster, ok := page.Sections["Roster"].([]TableRow)
	if !ok || len(roster) == 0 {
		t.Fatalf("Roster invalid type/empty: %T", page.Sections["Roster"])
	}
	if roster[0].Get("Nombre") == "" {
		t.Fatal("roster rows should include Nombre")
	}
}

func TestSeasonBuilder_BuildFromYear(t *testing.T) {
	builder := NewSeasonBuilder("testdata").SetTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}
	if page.Page.Params["year"] != 2025 {
		t.Fatalf("expected year=2025, got %v", page.Page.Params["year"])
	}
}

func TestSeasonBuilder_ScheduleRegularSeasonStructuredLinks(t *testing.T) {
	builder := NewSeasonBuilder("testdata")
	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	section, ok := page.Sections["Partidos de la temporada regular"]
	if !ok {
		t.Fatal("missing schedule section")
	}
	items, ok := section.([]GameLink)
	if !ok {
		t.Fatalf("schedule section type = %T, want []GameLink", section)
	}
	if len(items) == 0 {
		t.Fatal("schedule section empty")
	}
	first := items[0]
	if first.Title == "" || first.URL == "" || first.ID == "" || first.Fecha == "" {
		t.Fatalf("invalid first schedule item: %+v", first)
	}
	if got := pathLastSegment(first.URL); got != first.ID {
		t.Fatalf("id must match last URL segment: id=%q url=%q", first.ID, first.URL)
	}
	if first.Fecha != "07-09-2025" {
		t.Fatalf("expected first date 07-09-2025, got %q", first.Fecha)
	}
}

func TestSeasonBuilder_SeasonGamesIncludesLoadedPreseasonAndRegularSchedule(t *testing.T) {
	processedDir := t.TempDir()
	gamesDir := filepath.Join(processedDir, "games", "2025")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("MkdirAll games dir: %v", err)
	}
	writeSeasonBuilderGame(t, gamesDir, boxscore.BoxscoreEnvelope{
		GameID: "202508100chi",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Score: 24},
					{Name: &boxscore.LinkContent{Content: "Chicago Bears"}, Score: 24},
				},
				Meta: boxscore.ScoreboxMeta{Date: "Sunday Aug 10, 2025"},
			},
			Week:    "Preseason 1",
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			Tables:  map[string]boxscore.TableResult{},
		},
	})
	writeSeasonBuilderGame(t, gamesDir, boxscore.BoxscoreEnvelope{
		GameID: "202508160det",
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{
					{Name: &boxscore.LinkContent{Content: "Miami Dolphins"}, Score: 24},
					{Name: &boxscore.LinkContent{Content: "Detroit Lions"}, Score: 17},
				},
				Meta: boxscore.ScoreboxMeta{Date: "Saturday Aug 16, 2025"},
			},
			Week:    "Preseason 2",
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			Tables:  map[string]boxscore.TableResult{},
		},
	})
	writeSeasonBuilderGame(t, gamesDir, boxscore.BoxscoreEnvelope{
		GameID: "202508300mia",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: videos.StatusUnknown},
			Tables:  map[string]boxscore.TableResult{},
		},
	})

	seasonData := map[string]interface{}{
		"season": float64(2025),
		"data": map[string]interface{}{
			"main": map[string]interface{}{
				"tables": map[string]interface{}{
					"games": map[string]interface{}{
						"rows": []interface{}{
							map[string]interface{}{
								"week_num":      float64(1),
								"game_location": "@",
								"opp":           map[string]interface{}{"content": "Indianapolis Colts"},
								"boxscore_word": map[string]interface{}{"content": "boxscore", "link": "/boxscores/202509070clt.htm"},
							},
						},
					},
				},
			},
		},
	}

	builder := NewSeasonBuilder(processedDir)
	page, err := builder.Build(seasonData)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	all, ok := page.Sections["Partidos de la temporada"].([]GameLink)
	if !ok {
		t.Fatalf("season games section type = %T", page.Sections["Partidos de la temporada"])
	}
	if len(all) != 3 {
		t.Fatalf("season games len = %d, want 3: %+v", len(all), all)
	}
	if all[0].Week != "Preseason 1" || all[0].GameType != "preseason" || all[0].Status != "final" {
		t.Fatalf("first game metadata = %+v", all[0])
	}
	if all[1].Week != "Preseason 2" {
		t.Fatalf("second game = %+v, want Preseason 2", all[1])
	}
	if all[2].Week != "1" || all[2].GameType != "regular_season" {
		t.Fatalf("third game = %+v, want regular week 1", all[2])
	}
	for _, item := range all {
		if item.Title == "Miami Dolphins vs Rival" {
			t.Fatalf("placeholder title leaked into season list: %+v", item)
		}
	}

	legacy, ok := page.Sections["Partidos de la temporada regular"].([]GameLink)
	if !ok {
		t.Fatalf("legacy regular section type = %T", page.Sections["Partidos de la temporada regular"])
	}
	if len(legacy) != 1 || legacy[0].GameType != "regular_season" {
		t.Fatalf("legacy regular section should contain only regular season games: %+v", legacy)
	}
}

func writeSeasonBuilderGame(t *testing.T, dir string, env boxscore.BoxscoreEnvelope) {
	t.Helper()
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal game %s: %v", env.GameID, err)
	}
	if err := os.WriteFile(filepath.Join(dir, env.GameID+".json"), data, 0644); err != nil {
		t.Fatalf("WriteFile game %s: %v", env.GameID, err)
	}
}

func TestSeasonBuilder_FileNotFound(t *testing.T) {
	builder := NewSeasonBuilder("testdata")
	if _, err := builder.BuildFromYear(9999); err == nil {
		t.Fatal("expected error for missing season file")
	}
}

func TestSortSections_Order(t *testing.T) {
	sections := map[string]interface{}{"Playoffs": nil, "Resumen": nil, "Jugadores seleccionados en el draft": nil, "Roster": nil}
	sorted := SortSections(sections)
	if len(sorted) < 4 || sorted[0] != "Resumen" {
		t.Fatalf("unexpected sort order: %#v", sorted)
	}
}

func TestFormatDDMMYYYY(t *testing.T) {
	if got := formatDDMMYYYY("2025-09-07"); got != "07-09-2025" {
		t.Fatalf("unexpected formatted date: %s", got)
	}
}

func TestExtractSeasonYear(t *testing.T) {
	got, err := extractSeasonYear(map[string]interface{}{"season": float64(2025)})
	if err != nil || got != 2025 {
		t.Fatalf("extractSeasonYear failed: year=%d err=%v", got, err)
	}
}

func TestDateISOFromBoxscoreLink(t *testing.T) {
	got, err := dateISOFromBoxscoreLink("/boxscores/202509070clt.htm")
	if err != nil {
		t.Fatalf("dateISOFromBoxscoreLink error: %v", err)
	}
	if got != "2025-09-07" {
		t.Fatalf("unexpected date iso: %s", got)
	}
}

func TestParseRecordField(t *testing.T) {
	tests := []struct {
		input      string
		wantRecord string
		wantRank   string
	}{
		{"4-10-0, 3rd in AFL East Division (Schedule and Results)", "4-10-0", "3rd"},
		{"8-9-0, 2nd in AFC East Division (Schedule and Results)", "8-9-0", "2nd"},
		{"14-3-0, 1st in AFC East Division (Schedule and Results)", "14-3-0", "1st"},
		{"7-10-0", "7-10-0", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rec, rank := parseRecordField(tt.input)
			if rec != tt.wantRecord {
				t.Errorf("record = %q, want %q", rec, tt.wantRecord)
			}
			if rank != tt.wantRank {
				t.Errorf("rank = %q, want %q", rank, tt.wantRank)
			}
		})
	}
}

func TestFormatPoints(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"219 (15.6/g) 9th of 9", "219 puntos. 9º de 9 equipos"},
		{"407 (29.1/g) 8th of 9", "407 puntos. 8º de 9 equipos"},
		{"347 (20.4/g) 25th of 32", "347 puntos. 25º de 32 equipos"},
		{"345 (20.3/g) 22nd of 32", "345 puntos. 22º de 32 equipos"},
		{"364 (21.4/g) 10th of 32", "364 puntos. 10º de 32 equipos"},
		{"100", "100 puntos"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatPoints(tt.input)
			if got != tt.want {
				t.Errorf("formatPoints(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildResumenSection_MapOutput(t *testing.T) {
	builder := NewSeasonBuilder("testdata")
	page := NewSectionedPage(PageKindSeason, "Test", "2025-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": float64(1967),
		"data": map[string]interface{}{
			"main": map[string]interface{}{
				"info": map[string]interface{}{
					"Coach":               "George Wilson (4-10-0)",
					"Record":              "4-10-0, 3rd in AFL East Division (Schedule and Results)",
					"Points For":          "219 (15.6/g) 9th of 9",
					"Points Against":      "407 (29.1/g) 8th of 9",
					"Stadium":             "Orange Bowl",
					"Defensive Alignment": "4-3",
					"Training Camp":       "Saint Andrew's School (Boca Raton, Florida)",
					"Expected W-L":        "2.6-11.4",
					"SRS":                 "-12.18 (9th of 9), SOS: 1.25",
				},
			},
		},
	}

	if err := builder.buildResumenSection(page, seasonData); err != nil {
		t.Fatalf("buildResumenSection failed: %v", err)
	}

	resumen, ok := page.Sections["Resumen"]
	if !ok {
		t.Fatal("Resumen section not found")
	}

	row, ok := resumen.(TableRow)
	if !ok {
		t.Fatalf("Resumen should be TableRow, got %T", resumen)
	}

	if row.Get("Head Coach") != "George Wilson (4-10-0)" {
		t.Errorf("Head Coach = %q", row.Get("Head Coach"))
	}
	if row.Get("Récord") != "4-10-0" {
		t.Errorf("Récord = %q", row.Get("Récord"))
	}
	if row.Get("Posición") != "3rd" {
		t.Errorf("Posición = %q", row.Get("Posición"))
	}
	if row.Get("Puntos a favor") != "219 puntos. 9º de 9 equipos" {
		t.Errorf("Puntos a favor = %q", row.Get("Puntos a favor"))
	}
	if row.Get("Estadio") != "Orange Bowl" {
		t.Errorf("Estadio = %q", row.Get("Estadio"))
	}
}

func TestBuildResumenSection_FallsBackWhenMainInfoIsEmpty(t *testing.T) {
	builder := NewSeasonBuilder("testdata")
	page := NewSectionedPage(PageKindSeason, "Test", "2016-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": float64(2016),
		"data": map[string]interface{}{
			"main": map[string]interface{}{
				"info": map[string]interface{}{},
			},
			"roster": map[string]interface{}{
				"division_name": "AFC East Division",
				"division_rank": "2",
				"info": map[string]interface{}{
					"Coach":           "Adam Gase (10-6-0)",
					"General Manager": "Chris Grier",
					"Record":          "10-6-0, 2nd in AFC East Division (Schedule and Results)",
					"Stadium":         "Hard Rock Stadium",
				},
			},
		},
	}

	if err := builder.buildResumenSection(page, seasonData); err != nil {
		t.Fatalf("buildResumenSection failed: %v", err)
	}

	resumen, ok := page.Sections["Resumen"]
	if !ok {
		t.Fatal("Resumen section not found")
	}

	row, ok := resumen.(TableRow)
	if !ok {
		t.Fatalf("Resumen should be TableRow, got %T", resumen)
	}

	if row.Get("Head Coach") != "Adam Gase (10-6-0)" {
		t.Errorf("Head Coach = %q", row.Get("Head Coach"))
	}
	if row.Get("General Manager") != "Chris Grier" {
		t.Errorf("General Manager = %q", row.Get("General Manager"))
	}
	if row.Get("Récord") != "10-6-0" {
		t.Errorf("Récord = %q", row.Get("Récord"))
	}
	if row.Get("Posición") != "2" {
		t.Errorf("Posición = %q", row.Get("Posición"))
	}
	if row.Get("Estadio") != "Hard Rock Stadium" {
		t.Errorf("Estadio = %q", row.Get("Estadio"))
	}
}
