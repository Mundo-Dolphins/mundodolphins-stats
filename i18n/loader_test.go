package i18n

import (
	"testing"
)

func TestLoaderLoadTranslations_InvalidLanguage(t *testing.T) {
	_, err := LoadTranslations("xx")
	if err == nil {
		t.Fatalf("expected error for unknown language")
	}
}

func TestLoaderGetDefaultTranslations(t *testing.T) {
	defaultTranslations = nil

	first, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations first call failed: %v", err)
	}
	if first == nil {
		t.Fatalf("expected non-nil translations")
	}

	second, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations second call failed: %v", err)
	}
	if second == nil {
		t.Fatalf("expected non-nil translations")
	}
	if first != second {
		t.Fatalf("expected cached singleton instance")
	}
}

func TestTranslations_GettersAndFallbacks(t *testing.T) {
	tr := &Translations{
		GameInfo:       map[string]string{"roof": "Techo"},
		Officials:      map[string]string{"referee": "Arbitro"},
		Months:         map[string]string{"Sep": "Septiembre"},
		CoachingStatus: map[string]string{"interim": "Interino"},
		ColumnHeaders:  map[string]string{"player": "Jugador"},
		TableHeaders: map[string]map[string]map[string]string{
			"game": {
				"team_stats": {"yds": "Yardas"},
			},
		},
		SectionTitles: map[string]string{"summary": "Resumen"},
		FieldLabels:   map[string]string{"attendance": "Asistencia"},
		StatLabels:    map[string]string{"first_downs": "Primeros downs"},
		SeasonPoints:  map[string]string{"format": "{points} puntos"},
		SeasonStats: map[string]map[string]string{
			"table_titles": {"offense": "Ofensiva"},
		},
		PlayoffResults: map[string]interface{}{"won_wc": "Comodin"},
		TableLegends: map[string]map[string]string{
			"common": {"yds": "Yardas"},
		},
	}

	if v, ok := tr.GetGameInfoTranslation("roof"); !ok || v != "Techo" {
		t.Fatalf("unexpected game info translation: %q ok=%v", v, ok)
	}
	if _, ok := tr.GetGameInfoTranslation("missing"); ok {
		t.Fatalf("expected missing game info key")
	}

	if v, ok := tr.GetOfficialsTranslation("referee"); !ok || v != "Arbitro" {
		t.Fatalf("unexpected officials translation: %q ok=%v", v, ok)
	}
	if v, ok := tr.GetMonthTranslation("Sep"); !ok || v != "Septiembre" {
		t.Fatalf("unexpected month translation: %q ok=%v", v, ok)
	}
	if v, ok := tr.GetCoachingStatusTranslation("interim"); !ok || v != "Interino" {
		t.Fatalf("unexpected coaching status translation: %q ok=%v", v, ok)
	}

	if v := tr.GetSeasonPointsFormat(); v != "{points} puntos" {
		t.Fatalf("unexpected season points format: %q", v)
	}
	if tr2 := (&Translations{}); tr2.GetSeasonPointsFormat() != "{points} puntos. {rank}º de {total} equipos" {
		t.Fatalf("expected default season points fallback")
	}

	if v, ok := tr.GetColumnHeader("player"); !ok || v != "Jugador" {
		t.Fatalf("unexpected column header: %q ok=%v", v, ok)
	}
	if _, ok := (&Translations{}).GetColumnHeader("player"); ok {
		t.Fatalf("expected false for nil column headers")
	}

	headers := tr.GetTableHeader("game", "team_stats")
	if headers == nil || headers["yds"] != "Yardas" {
		t.Fatalf("unexpected table header map: %#v", headers)
	}
	if v := tr.GetTableHeader("game", "missing"); v != nil {
		t.Fatalf("expected nil for missing table header")
	}
	if v := (&Translations{}).GetTableHeader("game", "team_stats"); v != nil {
		t.Fatalf("expected nil table header with nil source map")
	}

	if v := tr.GetSectionTitle("summary"); v != "Resumen" {
		t.Fatalf("unexpected section title: %q", v)
	}
	if v := (&Translations{}).GetSectionTitle("unknown"); v != "unknown" {
		t.Fatalf("expected section title fallback key, got %q", v)
	}

	if v := tr.GetFieldLabel("attendance"); v != "Asistencia" {
		t.Fatalf("unexpected field label: %q", v)
	}
	if v := (&Translations{}).GetFieldLabel("missing"); v != "missing" {
		t.Fatalf("expected field label fallback key, got %q", v)
	}

	if v := tr.GetStatLabel("first_downs"); v != "Primeros downs" {
		t.Fatalf("unexpected stat label: %q", v)
	}
	if v := (&Translations{}).GetStatLabel("missing"); v != "missing" {
		t.Fatalf("expected stat label fallback key, got %q", v)
	}

	titles := tr.GetTableTitles()
	if titles["offense"] != "Ofensiva" {
		t.Fatalf("unexpected table title map: %#v", titles)
	}
	if len((&Translations{}).GetTableTitles()) != 0 {
		t.Fatalf("expected empty map for missing table titles")
	}

	po := tr.GetPlayoffTranslations()
	if po["won_wc"] != "Comodin" {
		t.Fatalf("unexpected playoff translations: %#v", po)
	}

	if v, ok := tr.GetTableLegendTranslation("common", "yds"); !ok || v != "Yardas" {
		t.Fatalf("unexpected table legend translation: %q ok=%v", v, ok)
	}
	if _, ok := tr.GetTableLegendTranslation("common", "missing"); ok {
		t.Fatalf("expected false for missing legend key")
	}
}

func TestFlattenTableLegends_OrderAndFallback(t *testing.T) {
	tr := &Translations{
		TableLegends: map[string]map[string]string{
			"misc": {
				"dup": "misc-dup",
				"x":   "misc-x",
			},
			"common": {
				"dup": "common-dup",
				"a":   "common-a",
			},
		},
	}

	flat := tr.FlattenTableLegends()
	if flat["a"] != "common-a" {
		t.Fatalf("expected key from preferred category common")
	}
	if flat["dup"] != "common-dup" {
		t.Fatalf("expected preferred category to win on duplicates, got %q", flat["dup"])
	}
	if flat["x"] != "misc-x" {
		t.Fatalf("expected key from remaining category, got %q", flat["x"])
	}

	empty := (&Translations{}).FlattenTableLegends()
	if len(empty) != 0 {
		t.Fatalf("expected empty map for nil legends")
	}
}
