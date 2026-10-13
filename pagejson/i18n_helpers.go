package pagejson

import "github.com/Mundo-Dolphins/mundodolphins-stats/i18n"

// sectionTitle returns the translated section title for the given key.
func sectionTitle(key string) string {
	tr, err := i18n.GetDefaultTranslations()
	if err != nil || tr == nil {
		return key
	}
	return tr.GetSectionTitle(key)
}

// fieldLabel returns the translated field label for the given key.
func fieldLabel(key string) string {
	tr, err := i18n.GetDefaultTranslations()
	if err != nil || tr == nil {
		return key
	}
	return tr.GetFieldLabel(key)
}

// tableHeaderFromI18n returns per-table header overrides from the i18n YAML.
func tableHeaderFromI18n(context, tableKey string) map[string]string {
	tr, err := i18n.GetDefaultTranslations()
	if err != nil || tr == nil {
		return nil
	}
	return tr.GetTableHeader(context, tableKey)
}
