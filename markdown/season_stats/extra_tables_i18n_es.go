package season_stats

import (
	"github.com/Mundo-Dolphins/mundodolphins-stats/i18n"
)

// GetTableTitleES returns the Spanish title for a table.
// Falls back to humanized name if not in translations.
func GetTableTitleES(tableName string) string {
	trans, err := i18n.GetDefaultTranslations()
	if err != nil {
		return HumanizeTableName(tableName)
	}

	titles := trans.GetTableTitles()
	if title, exists := titles[tableName]; exists {
		return title
	}
	return HumanizeTableName(tableName)
}
