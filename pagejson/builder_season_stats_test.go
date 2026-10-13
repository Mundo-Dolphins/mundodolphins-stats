package pagejson

import (
	"testing"
	"time"
)

func TestSeasonStatsBuilder_BuildFromFile_2025(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata").SetTime(time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC))

	page, err := builder.BuildFromFile("testdata/2025.json")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}

	// Verify page info
	if page.Page.Kind != PageKindSeasonStats {
		t.Errorf("Expected kind=%s, got=%s", PageKindSeasonStats, page.Page.Kind)
	}

	if page.Page.Title != "Estadísticas Temporada 2025" {
		t.Errorf("Expected title='Estadísticas Temporada 2025', got=%q", page.Page.Title)
	}

	// Verify schema
	if page.Schema.Name != SchemaName {
		t.Errorf("Expected schema name=%s, got=%s", SchemaName, page.Schema.Name)
	}

	if page.Schema.Version != SchemaVersion {
		t.Errorf("Expected schema version=%s, got=%s", SchemaVersion, page.Schema.Version)
	}

	// Verify sections exist (may be empty if no data)
	if page.Sections == nil {
		t.Error("Expected Sections map not to be nil")
	}
}

func TestSeasonStatsBuilder_BuildFromYear(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")

	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	if page.Page.Title != "Estadísticas Temporada 2025" {
		t.Errorf("Expected title='Estadísticas Temporada 2025', got=%q", page.Page.Title)
	}

	// Check params
	if year, ok := page.Page.Params["year"].(int); !ok || year != 2025 {
		t.Errorf("Expected params.year=2025, got=%v", page.Page.Params["year"])
	}
}

func TestSeasonStatsBuilder_NestedSections(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata").SetTime(time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC))

	page, err := builder.BuildFromFile("testdata/2025.json")
	if err != nil {
		t.Fatalf("BuildFromFile failed: %v", err)
	}

	// Check if nested sections exist (H2 → H3 structure)
	// "Estadísticas de la temporada" should be an H2 with nested H3s
	estadisticasSection, ok := page.Sections["Estadísticas de la temporada"]
	if !ok {
		// It's OK if section doesn't exist (no data in fixture)
		t.Logf("Estadísticas de la temporada section not found (may be empty in fixture)")
		return
	}

	// If the section exists, it should be a map (nested structure)
	if _, ok := estadisticasSection.(map[string]interface{}); !ok {
		t.Errorf("Estadísticas de la temporada should be a map[string]interface{} for nested sections, got %T", estadisticasSection)
	}
}

func TestSeasonStatsBuilder_FileNotFound(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")

	_, err := builder.BuildFromFile("testdata/nonexistent.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestSeasonStatsBuilder_SetTime(t *testing.T) {
	fixedTime := time.Date(2025, 6, 1, 10, 30, 0, 0, time.UTC)
	builder := NewSeasonStatsBuilder("testdata").SetTime(fixedTime)

	page, err := builder.BuildFromYear(2025)
	if err != nil {
		t.Fatalf("BuildFromYear failed: %v", err)
	}

	expectedTime := fixedTime.Format(time.RFC3339)
	if page.GeneratedAt != expectedTime {
		t.Errorf("Expected GeneratedAt=%s, got=%s", expectedTime, page.GeneratedAt)
	}
}

func TestNewSeasonStatsBuilder(t *testing.T) {
	builder := NewSeasonStatsBuilder("/test/path")

	if builder.processedDir != "/test/path" {
		t.Errorf("Expected processedDir=/test/path, got=%s", builder.processedDir)
	}

	if builder.now.IsZero() {
		t.Error("Expected now to be initialized, got zero time")
	}
}

// Test the buildEstadisticasTemporadaSection specifically
func TestSeasonStatsBuilder_BuildEstadisticasTemporadaSection(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")
	page := NewSectionedPage(PageKindSeasonStats, "Test", "2025-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": 2025,
		"data": map[string]interface{}{
			"main": map[string]interface{}{
				"tables": map[string]interface{}{
					"team_stats": map[string]interface{}{
						"rows": []interface{}{
							map[string]interface{}{"stat": "Points For", "value": "300"},
						},
					},
				},
			},
		},
	}

	err := builder.buildEstadisticasTemporadaSection(page, seasonData)
	if err != nil {
		t.Fatalf("buildEstadisticasTemporadaSection failed: %v", err)
	}

	estadisticas, ok := page.Sections["Estadísticas de la temporada"]
	if !ok {
		t.Error("Estadísticas de la temporada section not found")
		return
	}

	estadisticasMap, ok := estadisticas.(map[string]interface{})
	if !ok {
		t.Errorf("Estadísticas de la temporada should be a map, got %T", estadisticas)
		return
	}

	if _, ok := estadisticasMap["Resumen del equipo"]; !ok {
		t.Error("Resumen del equipo subsection not found")
	}
}

func TestSeasonStatsBuilder_BuildAdvancedStatsSection(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")
	page := NewSectionedPage(PageKindSeasonStats, "Test", "2025-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": 2025,
		"data": map[string]interface{}{
			"advanced": map[string]interface{}{
				"tables": map[string]interface{}{
					"passing_advanced": map[string]interface{}{
						"rows": []interface{}{
							map[string]interface{}{"name_display": "Test Player", "pass_yds": 100},
						},
					},
				},
			},
		},
	}

	err := builder.buildAdvancedStatsSection(page, seasonData)
	if err != nil {
		t.Fatalf("buildAdvancedStatsSection failed: %v", err)
	}

	section, ok := page.Sections["Estadísticas avanzadas"]
	if !ok {
		t.Error("Estadísticas avanzadas section not found")
		return
	}

	sectionMap, ok := section.(map[string]interface{})
	if !ok {
		t.Errorf("Estadísticas avanzadas should be a map, got %T", section)
		return
	}

	if _, ok := sectionMap["Pase avanzado"]; !ok {
		t.Error("Pase avanzado subsection not found")
	}
}

func TestSeasonStatsBuilder_BuildTDLogsSection(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")
	page := NewSectionedPage(PageKindSeasonStats, "Test", "2025-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": 2025,
		"data": map[string]interface{}{
			"main": map[string]interface{}{
				"tables": map[string]interface{}{
					"team_td_log": map[string]interface{}{
						"rows": []interface{}{
							map[string]interface{}{
								"game_date":   "2025-09-07",
								"quarter":     "1",
								"score_type":  "pass",
								"description": "15 yard pass from T.Tagovailoa",
							},
						},
					},
				},
			},
		},
	}

	err := builder.buildTDLogsSection(page, seasonData)
	if err != nil {
		t.Fatalf("buildTDLogsSection failed: %v", err)
	}

	section, ok := page.Sections["Registros de touchdowns"]
	if !ok {
		t.Error("Registros de touchdowns section not found")
		return
	}

	sectionMap, ok := section.(map[string]interface{})
	if !ok {
		t.Errorf("Registros de touchdowns should be a map, got %T", section)
		return
	}

	if _, ok := sectionMap["Del equipo"]; !ok {
		t.Error("Del equipo subsection not found")
	}

	rows, ok := sectionMap["Del equipo"].([]TableRow)
	if !ok || len(rows) == 0 {
		t.Fatalf("Del equipo should contain rows, got %T", sectionMap["Del equipo"])
	}

	if got := rows[0].Get("Tipo"); got != "pase" {
		t.Errorf("expected translated Tipo='pase', got=%q", got)
	}
	if got := rows[0].Get("Descripción"); got != "pase de 15 yardas de T.Tagovailoa" {
		t.Errorf("expected translated Descripción, got=%q", got)
	}
}

// Test the buildEstadisticasPorJugadorSection specifically
func TestSeasonStatsBuilder_BuildEstadisticasPorJugadorSection(t *testing.T) {
	builder := NewSeasonStatsBuilder("testdata")
	page := NewSectionedPage(PageKindSeasonStats, "Test", "2025-01-01", "test", "/test", nil)

	seasonData := map[string]interface{}{
		"season": 2025,
		"data": map[string]interface{}{
			"advanced": map[string]interface{}{
				"tables": map[string]interface{}{},
			},
		},
	}

	err := builder.buildEstadisticasPorJugadorSection(page, seasonData)
	if err != nil {
		t.Fatalf("buildEstadisticasPorJugadorSection failed: %v", err)
	}

	// Section may or may not exist depending on data
	// Just verify no error occurred
}
