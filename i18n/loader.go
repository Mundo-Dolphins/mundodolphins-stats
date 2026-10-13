package i18n

import (
	"embed"
	"fmt"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// Embed the translations directory
//
//go:embed translations/*.yaml
var translationsFS embed.FS

// Global translations instance with lazy loading
var (
	defaultTranslations *Translations
	transLock           sync.Mutex
)

// Translations holds all translation data
type Translations struct {
	UI             map[string]string                       `yaml:"ui"`
	GameInfo       map[string]string                       `yaml:"game_info"`
	Officials      map[string]string                       `yaml:"officials"`
	TableLegends   map[string]map[string]string            `yaml:"table_legends"`
	ColumnHeaders  map[string]string                       `yaml:"column_headers"`
	TableHeaders   map[string]map[string]map[string]string `yaml:"table_headers"`
	SectionTitles  map[string]string                       `yaml:"section_titles"`
	FieldLabels    map[string]string                       `yaml:"field_labels"`
	StatLabels     map[string]string                       `yaml:"stat_labels"`
	Months         map[string]string                       `yaml:"months"`
	Weather        map[string]interface{}                  `yaml:"weather"`
	ScoringPlays   map[string]interface{}                  `yaml:"scoring_plays"`
	CoachingStatus map[string]string                       `yaml:"coaching_status"`
	SeasonStats    map[string]map[string]string            `yaml:"season_stats"`
	PlayoffResults map[string]interface{}                  `yaml:"playoff_results"`
	SeasonPoints   map[string]string                       `yaml:"season_points"`
}

// LoadTranslations loads translations from the embedded YAML file
func LoadTranslations(language string) (*Translations, error) {
	filename := fmt.Sprintf("translations/%s.yaml", language)
	data, err := translationsFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read translations file %s: %w", filename, err)
	}

	var translations Translations
	err = yaml.Unmarshal(data, &translations)
	if err != nil {
		return nil, fmt.Errorf("failed to parse translations file: %w", err)
	}

	return &translations, nil
}

// GetDefaultTranslations returns the default Spanish translations (lazy initialized)
func GetDefaultTranslations() (*Translations, error) {
	if defaultTranslations == nil {
		transLock.Lock()
		defer transLock.Unlock()

		if defaultTranslations == nil {
			trans, err := LoadTranslations("es")
			if err != nil {
				return nil, err
			}
			defaultTranslations = trans
		}
	}
	return defaultTranslations, nil
}

// GetTableLegendTranslation retrieves a table legend translation by category and key
func (t *Translations) GetTableLegendTranslation(category, key string) (string, bool) {
	if legends, ok := t.TableLegends[category]; ok {
		if val, ok := legends[key]; ok {
			return val, true
		}
	}
	return "", false
}

// GetGameInfoTranslation retrieves a game info label translation
func (t *Translations) GetGameInfoTranslation(key string) (string, bool) {
	val, ok := t.GameInfo[key]
	return val, ok
}

// GetOfficialsTranslation retrieves an officials table translation
func (t *Translations) GetOfficialsTranslation(key string) (string, bool) {
	val, ok := t.Officials[key]
	return val, ok
}

// GetMonthTranslation retrieves a month translation
func (t *Translations) GetMonthTranslation(key string) (string, bool) {
	val, ok := t.Months[key]
	return val, ok
}

// GetCoachingStatusTranslation retrieves a coaching status translation
func (t *Translations) GetCoachingStatusTranslation(status string) (string, bool) {
	val, ok := t.CoachingStatus[status]
	return val, ok
}

// GetSeasonPointsFormat retrieves the season points format string
func (t *Translations) GetSeasonPointsFormat() string {
	if format, ok := t.SeasonPoints["format"]; ok {
		return format
	}
	return "{points} puntos. {rank}º de {total} equipos"
}

// GetTableTitles retrieves season stats table titles
func (t *Translations) GetTableTitles() map[string]string {
	if titles, ok := t.SeasonStats["table_titles"]; ok {
		return titles
	}
	return make(map[string]string)
}

// GetPlayoffTranslations retrieves playoff result translations
func (t *Translations) GetPlayoffTranslations() map[string]interface{} {
	return t.PlayoffResults
}

// GetColumnHeader retrieves a global column header translation
func (t *Translations) GetColumnHeader(key string) (string, bool) {
	if t.ColumnHeaders == nil {
		return "", false
	}
	val, ok := t.ColumnHeaders[key]
	return val, ok
}

// GetTableHeader retrieves per-table header overrides for a given context and table key
func (t *Translations) GetTableHeader(context, tableKey string) map[string]string {
	if t.TableHeaders == nil {
		return nil
	}
	if ctx, ok := t.TableHeaders[context]; ok {
		if h, ok := ctx[tableKey]; ok {
			return h
		}
	}
	return nil
}

// GetSectionTitle retrieves a section title by key, returning the key itself if not found
func (t *Translations) GetSectionTitle(key string) string {
	if t.SectionTitles == nil {
		return key
	}
	if val, ok := t.SectionTitles[key]; ok {
		return val
	}
	return key
}

// GetFieldLabel retrieves a field label by key, returning the key itself if not found
func (t *Translations) GetFieldLabel(key string) string {
	if t.FieldLabels == nil {
		return key
	}
	if val, ok := t.FieldLabels[key]; ok {
		return val
	}
	return key
}

// GetStatLabel retrieves a game stat label by key, returning the key itself if not found
func (t *Translations) GetStatLabel(key string) string {
	if t.StatLabels == nil {
		return key
	}
	if val, ok := t.StatLabels[key]; ok {
		return val
	}
	return key
}

// FlattenTableLegends converts nested table legends structure to flat map
func (t *Translations) FlattenTableLegends() map[string]string {
	result := make(map[string]string)
	if t.TableLegends == nil {
		return result
	}

	preferredOrder := []string{
		"common",
		"conversions",
		"basic_stats",
		"snap_counts",
		"drives",
		"drive_results",
		"kicking_punting",
		"expected_points",
		"passing_advanced",
		"rushing_advanced",
		"receiving_advanced",
		"defense_advanced",
	}

	seen := make(map[string]bool, len(t.TableLegends))
	for _, categoryName := range preferredOrder {
		category, ok := t.TableLegends[categoryName]
		if !ok {
			continue
		}
		seen[categoryName] = true

		keys := make([]string, 0, len(category))
		for key := range category {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			if _, exists := result[key]; !exists {
				result[key] = category[key]
			}
		}
	}

	remainingCategories := make([]string, 0, len(t.TableLegends)-len(seen))
	for categoryName := range t.TableLegends {
		if seen[categoryName] {
			continue
		}
		remainingCategories = append(remainingCategories, categoryName)
	}
	sort.Strings(remainingCategories)

	for _, categoryName := range remainingCategories {
		category := t.TableLegends[categoryName]
		keys := make([]string, 0, len(category))
		for key := range category {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			if _, exists := result[key]; !exists {
				result[key] = category[key]
			}
		}
	}
	return result
}
