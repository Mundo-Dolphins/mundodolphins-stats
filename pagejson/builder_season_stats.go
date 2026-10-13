package pagejson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/i18n"
)

// SeasonStatsBuilder builds a SectionedPage for season statistics pages.
type SeasonStatsBuilder struct {
	processedDir string
	now          time.Time
}

func NewSeasonStatsBuilder(processedDir string) *SeasonStatsBuilder {
	return &SeasonStatsBuilder{processedDir: processedDir, now: time.Now().UTC()}
}

func (b *SeasonStatsBuilder) SetTime(t time.Time) *SeasonStatsBuilder {
	b.now = t
	return b
}

func (b *SeasonStatsBuilder) BuildFromYear(year int) (*SectionedPage, error) {
	return b.BuildFromFile(filepath.Join(b.processedDir, fmt.Sprintf("%d.json", year)))
}

func (b *SeasonStatsBuilder) BuildFromFile(filePath string) (*SectionedPage, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read season file %s: %w", filePath, err)
	}
	var seasonData map[string]interface{}
	if err := json.Unmarshal(data, &seasonData); err != nil {
		return nil, fmt.Errorf("parse season JSON: %w", err)
	}
	return b.Build(seasonData)
}

func (b *SeasonStatsBuilder) Build(seasonData map[string]interface{}) (*SectionedPage, error) {
	year, err := extractSeasonYear(seasonData)
	if err != nil {
		return nil, err
	}

	page := NewSectionedPageWithTime(
		PageKindSeasonStats,
		fmt.Sprintf(sectionTitle("season_stats_title_fmt"), year),
		fmt.Sprintf("%d-01-01", year),
		fmt.Sprintf("%d-stats", year),
		fmt.Sprintf("historia/stats/%d", year),
		map[string]interface{}{"year": year},
		b.now,
	)

	if err := b.buildEstadisticasTemporadaSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build estadísticas de la temporada: %w", err)
	}
	if err := b.buildEstadisticasPorJugadorSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build estadísticas por jugador: %w", err)
	}
	if err := b.buildTDLogsSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build registros de touchdowns: %w", err)
	}
	if err := b.buildAdvancedStatsSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build estadísticas avanzadas: %w", err)
	}

	// Checklist coverage (season-stats page): passing/rushing/receiving/defense/special teams/scoring
	// + por jugador (passing/rushing/receiving/defense/kicking/punting/returns/scoring)
	// + TD logs (team + opponent) + advanced stats (passing/rushing/receiving/defense).
	return page, nil
}

func (b *SeasonStatsBuilder) buildEstadisticasTemporadaSection(page *SectionedPage, seasonData map[string]interface{}) error {
	h2 := sectionTitle("team_stats")
	mainTables := nestedMap(seasonData, "data", "main", "tables")

	for _, spec := range []struct {
		root      map[string]interface{}
		key       string
		label     string
		headerMap map[string]string
	}{
		{mainTables, "team_stats", sectionTitle("team_summary"), seasonMainTableHeaderMap("team_stats")},
		{mainTables, "team_conversions", sectionTitle("team_conversions"), seasonMainTableHeaderMap("team_conversions")},
	} {
		if len(spec.root) == 0 {
			continue
		}
		rows := nestedValue(spec.root, spec.key, "rows")
		if rows == nil {
			continue
		}
		converted, err := ConvertTableToRows(rows, spec.headerMap)
		if err != nil {
			return fmt.Errorf("convert %s: %w", spec.key, err)
		}
		if len(converted) > 0 {
			page.AddNestedSection(h2, spec.label, converted)
		}
	}
	return nil
}

func (b *SeasonStatsBuilder) buildEstadisticasPorJugadorSection(page *SectionedPage, seasonData map[string]interface{}) error {
	h2 := sectionTitle("player_stats")
	mainTables := nestedMap(seasonData, "data", "main", "tables")
	if len(mainTables) == 0 {
		return nil
	}

	for _, spec := range []struct {
		key   string
		label string
	}{
		{"passing", sectionTitle("passing")},
		{"rushing_and_receiving", sectionTitle("rushing_receiving")},
		{"defense", sectionTitle("defense")},
		{"kicking", sectionTitle("kicking")},
		{"punting", sectionTitle("punting")},
		{"returns", sectionTitle("returns")},
		{"scoring", sectionTitle("scoring")},
	} {
		rows := nestedValue(mainTables, spec.key, "rows")
		if rows == nil {
			continue
		}
		converted, err := ConvertTableToRows(rows, seasonMainTableHeaderMap(spec.key))
		if err != nil {
			return fmt.Errorf("convert player table %s: %w", spec.key, err)
		}
		if len(converted) > 0 {
			page.AddNestedSection(h2, spec.label, converted)
		}
	}
	return nil
}

func (b *SeasonStatsBuilder) buildTDLogsSection(page *SectionedPage, seasonData map[string]interface{}) error {
	h2 := sectionTitle("td_logs")
	mainTables := nestedMap(seasonData, "data", "main", "tables")
	if len(mainTables) == 0 {
		return nil
	}

	for _, spec := range []struct {
		key   string
		label string
	}{
		{"team_td_log", sectionTitle("team_td_log")},
		{"opp_td_log", sectionTitle("opp_td_log")},
	} {
		rows := nestedValue(mainTables, spec.key, "rows")
		if rows == nil {
			continue
		}
		converted, err := ConvertTableToRows(rows, seasonMainTableHeaderMap(spec.key))
		if err != nil {
			return fmt.Errorf("convert %s: %w", spec.key, err)
		}
		translateTDLogRows(converted, spec.key)
		if len(converted) > 0 {
			page.AddNestedSection(h2, spec.label, converted)
		}
	}
	return nil
}

func translateTDLogRows(rows []TableRow, tableKey string) {
	if len(rows) == 0 {
		return
	}
	tr := i18n.NewESTranslator()
	headerMap := seasonMainTableHeaderMap(tableKey)
	descKey := displayHeaderForKey("description", headerMap)
	typeKey := displayHeaderForKey("score_type", headerMap)

	for i := range rows {
		if rows[i].Has(descKey) {
			rows[i].Set(descKey, tr.TranslateScoringPlay(rows[i].Get(descKey)))
		}
		if rows[i].Has(typeKey) {
			rows[i].Set(typeKey, tr.TranslateScoringPlay(rows[i].Get(typeKey)))
		}
	}
}

func (b *SeasonStatsBuilder) buildAdvancedStatsSection(page *SectionedPage, seasonData map[string]interface{}) error {
	h2 := sectionTitle("advanced_stats")
	advancedTables := nestedMap(seasonData, "data", "advanced", "tables")
	if len(advancedTables) == 0 {
		return nil
	}

	for _, spec := range []struct {
		key   string
		label string
	}{
		{"passing_advanced", sectionTitle("advanced_passing")},
		{"adv_rushing", sectionTitle("advanced_rushing")},
		{"adv_receiving", sectionTitle("advanced_receiving")},
		{"adv_defense", sectionTitle("advanced_defense")},
	} {
		rows := nestedValue(advancedTables, spec.key, "rows")
		if rows == nil {
			continue
		}
		converted, err := ConvertTableToRows(rows, seasonAdvancedTableHeaderMap(spec.key))
		if err != nil {
			return fmt.Errorf("convert %s: %w", spec.key, err)
		}
		if len(converted) > 0 {
			page.AddNestedSection(h2, spec.label, converted)
		}
	}
	return nil
}
