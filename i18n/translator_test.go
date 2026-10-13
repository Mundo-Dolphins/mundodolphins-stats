package i18n

import (
	"strings"
	"testing"
)

func TestESTranslator_T(t *testing.T) {
	tr := NewESTranslator()
	if tr.T("Week") != "Semana" {
		t.Error("Week should translate to Semana")
	}
	if tr.T("retractable roof (closed)") != "Techo retráctil (cerrado)" {
		t.Error("retractable roof (closed) should translate to Techo retráctil (cerrado)")
	}
	if tr.T("fieldturf") != "Césped artificial" {
		t.Error("fieldturf should translate to Césped artificial")
	}
	if tr.T("a_turf") != "Césped artificial" {
		t.Error("a_turf should translate to Césped artificial")
	}
}

func TestESTranslator_TranslateMonth(t *testing.T) {
	tr := NewESTranslator()
	if tr.TranslateMonth("Sep") != "septiembre" {
		t.Error("Sep should translate to septiembre")
	}
}

func TestESTranslator_TranslateScoringPlay(t *testing.T) {
	tr := NewESTranslator()
	result := tr.TranslateScoringPlay("10 yard pass from QB")
	if !strings.Contains(result, "yarda") {
		t.Error("Should translate yard to yarda")
	}
}

func TestESTranslator_TranslateWeather(t *testing.T) {
	tr := NewESTranslator()
	result := tr.TranslateWeather("76 degrees, relative humidity 61%, wind 16 mph")

	if !strings.Contains(result, "grados") {
		t.Error("degrees should translate to grados")
	}
	if !strings.Contains(result, "humedad relativa") {
		t.Error("relative humidity should translate to humedad relativa")
	}
	if !strings.Contains(result, "viento") {
		t.Error("wind should translate to viento")
	}
}

func TestGetDefaultTranslations(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}
	if trans == nil {
		t.Error("Expected translations, got nil")
	}

	// Verify it's cached
	trans2, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed on second call: %v", err)
	}
	if trans != trans2 {
		t.Error("GetDefaultTranslations should return cached instance")
	}
}

func TestTranslations_GetGameInfoTranslation(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	// Test existing translation
	val, ok := trans.GetGameInfoTranslation("won_toss")
	if !ok || val != "Sorteo ganado por" {
		t.Errorf("Expected 'Sorteo ganado por', got %q (ok=%v)", val, ok)
	}

	// Test missing translation
	_, ok2 := trans.GetGameInfoTranslation("nonexistent")
	if ok2 {
		t.Error("Expected missing translation to return false")
	}
}

func TestTranslations_GetOfficialsTranslation(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	// Test existing translations
	val, ok := trans.GetOfficialsTranslation("name")
	if !ok || val != "Nombre" {
		t.Errorf("Expected 'Nombre', got %q (ok=%v)", val, ok)
	}

	val2, ok2 := trans.GetOfficialsTranslation("ref_pos")
	if !ok2 || val2 != "Posición" {
		t.Errorf("Expected 'Posición', got %q (ok=%v)", val2, ok2)
	}
}

func TestTranslations_GetCoachingStatusTranslation(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	tests := []struct {
		status   string
		expected string
		wantOk   bool
	}{
		{"fired", "Despedido", true},
		{"resigned", "Renunció", true},
		{"retired", "Retirado", true},
		{"interim", "Interino", true},
		{"hired", "Contratado", true},
		{"nonexistent", "", false},
	}

	for _, tt := range tests {
		val, ok := trans.GetCoachingStatusTranslation(tt.status)
		if ok != tt.wantOk {
			t.Errorf("GetCoachingStatusTranslation(%q): ok=%v, want %v", tt.status, ok, tt.wantOk)
		}
		if ok && val != tt.expected {
			t.Errorf("GetCoachingStatusTranslation(%q): got %q, want %q", tt.status, val, tt.expected)
		}
	}
}

func TestTranslations_GetSeasonPointsFormat_WithFormat(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	format := trans.GetSeasonPointsFormat()
	if format == "" {
		t.Error("GetSeasonPointsFormat should return non-empty format")
	}
	if !strings.Contains(format, "{points}") {
		t.Errorf("Format should contain {points}: %q", format)
	}
	if !strings.Contains(format, "{rank}") {
		t.Errorf("Format should contain {rank}: %q", format)
	}
	if !strings.Contains(format, "{total}") {
		t.Errorf("Format should contain {total}: %q", format)
	}
}

func TestTranslations_GetSeasonPointsFormat_Fallback(t *testing.T) {
	// Create a translation with empty SeasonPoints to test fallback
	trans := &Translations{
		SeasonPoints: make(map[string]string),
	}

	format := trans.GetSeasonPointsFormat()
	if format != "{points} puntos. {rank}º de {total} equipos" {
		t.Errorf("Expected fallback format, got %q", format)
	}
}

func TestTranslations_GetTableTitles_HasTitles(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	titles := trans.GetTableTitles()
	if titles == nil {
		t.Error("GetTableTitles should not return nil")
	}
	if len(titles) == 0 {
		t.Error("GetTableTitles should return non-empty map")
	}

	// Verify some expected titles exist
	if roster, ok := titles["roster"]; !ok {
		t.Error("Expected 'roster' title")
	} else if roster == "" {
		t.Error("roster title should not be empty")
	}
}

func TestTranslations_GetTableTitles_Fallback(t *testing.T) {
	// Create a translation with empty SeasonStats to test fallback
	trans := &Translations{
		SeasonStats: make(map[string]map[string]string),
	}

	titles := trans.GetTableTitles()
	if titles == nil {
		t.Error("GetTableTitles should not return nil even with empty SeasonStats")
	}
}

func TestTranslations_GetPlayoffTranslations(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	playoffs := trans.GetPlayoffTranslations()
	if playoffs == nil {
		t.Error("Expected playoff translations, got nil")
	}
}

func TestTranslations_FlattenTableLegends(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	flattened := trans.FlattenTableLegends()
	if len(flattened) == 0 {
		t.Error("Expected flattened table legends, got empty map")
	}

	// Verify some expected legends
	if _, ok := flattened["player"]; !ok {
		t.Error("Expected 'player' legend")
	}
	if _, ok := flattened["position"]; !ok {
		t.Error("Expected 'position' legend")
	}
}

func TestLoadTranslations_ValidLanguage(t *testing.T) {
	trans, err := LoadTranslations("es")
	if err != nil {
		t.Fatalf("LoadTranslations failed: %v", err)
	}
	if trans == nil {
		t.Error("Expected translations, got nil")
	}
}

func TestLoadTranslations_InvalidLanguage(t *testing.T) {
	trans, err := LoadTranslations("invalid_language")
	if err == nil {
		t.Error("Expected error for invalid language")
	}
	if trans != nil {
		t.Error("Expected nil translations for invalid language")
	}
}

func TestTranslations_GetTableLegendTranslation(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	// Test existing legend in common category
	val, ok := trans.GetTableLegendTranslation("common", "player")
	if !ok || val != "Jugador" {
		t.Errorf("Expected 'Jugador', got %q (ok=%v)", val, ok)
	}

	// Test missing category
	_, ok2 := trans.GetTableLegendTranslation("nonexistent", "player")
	if ok2 {
		t.Error("Expected missing category to return false")
	}

	// Test missing key in existing category
	_, ok3 := trans.GetTableLegendTranslation("common", "nonexistent")
	if ok3 {
		t.Error("Expected missing key to return false")
	}
}

func TestESTranslator_T_EdgeCases(t *testing.T) {
	tr := NewESTranslator()

	// Test uppercase with underscore conversion
	if tr.T("na") != "N/D" {
		t.Errorf("Expected 'N/D', got %q", tr.T("na"))
	}

	// Test fallback for unknown key
	unknown := tr.T("unknown_key_xyz")
	if unknown != "unknown_key_xyz" {
		t.Errorf("Expected fallback to original key, got %q", unknown)
	}

	// Test grass translation
	if tr.T("grass") != "Césped" {
		t.Errorf("Expected 'Césped', got %q", tr.T("grass"))
	}

	// Test outdoors translation
	if tr.T("outdoors") != "Al aire libre" {
		t.Errorf("Expected 'Al aire libre', got %q", tr.T("outdoors"))
	}
}

func TestESTranslator_TranslateMonth_AllMonths(t *testing.T) {
	tr := NewESTranslator()

	tests := []struct {
		input    string
		expected string
	}{
		{"Jan", "enero"},
		{"Feb", "febrero"},
		{"Mar", "marzo"},
		{"Apr", "abril"},
		{"May", "mayo"},
		{"Jun", "junio"},
		{"Jul", "julio"},
		{"Aug", "agosto"},
		{"Sep", "septiembre"},
		{"Oct", "octubre"},
		{"Nov", "noviembre"},
		{"Dec", "diciembre"},
	}

	for _, tt := range tests {
		result := tr.TranslateMonth(tt.input)
		if result != tt.expected {
			t.Errorf("TranslateMonth(%q): got %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestESTranslator_TranslateDay(t *testing.T) {
	tr := NewESTranslator()

	tests := []struct {
		input    int
		expected string
	}{
		{1, "1"},
		{2, "2"},
		{7, "7"},
		{14, "14"},
	}

	for _, tt := range tests {
		result := tr.TranslateDay(tt.input)
		if result != tt.expected {
			t.Errorf("TranslateDay(%d): got %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestESTranslator_TranslateTeamName(t *testing.T) {
	tr := NewESTranslator()

	// TranslateTeamName should return the name unchanged
	teamName := "Miami Dolphins"
	result := tr.TranslateTeamName(teamName)
	if result != teamName {
		t.Errorf("TranslateTeamName(%q): got %q, want %q", teamName, result, teamName)
	}
}

func TestTranslations_GetMonthTranslation(t *testing.T) {
	trans, err := GetDefaultTranslations()
	if err != nil {
		t.Fatalf("GetDefaultTranslations failed: %v", err)
	}

	// Test existing month (lowercase)
	val, ok := trans.GetMonthTranslation("jan")
	if !ok || val != "enero" {
		t.Errorf("Expected 'enero', got %q (ok=%v)", val, ok)
	}

	// Test full month name
	val2, ok2 := trans.GetMonthTranslation("january")
	if !ok2 || val2 != "enero" {
		t.Errorf("Expected 'enero', got %q (ok=%v)", val2, ok2)
	}

	// Test missing month
	_, ok3 := trans.GetMonthTranslation("xyz")
	if ok3 {
		t.Error("Expected missing month to return false")
	}
}
