package i18n

import (
	"fmt"
	"regexp"
	"strings"
)

type ESTranslator struct {
	labels       map[string]string
	months       map[string]string
	translations *Translations
}

func NewESTranslator() *ESTranslator {
	// Load translations from YAML
	translations, err := LoadTranslations("es")
	if err != nil {
		// Fallback to embedded translations if loading fails
		return &ESTranslator{
			labels: map[string]string{
				"Week": "Semana", "N/A": "N/D",
				"outdoors": "Al aire libre", "grass": "Césped",
				"retractable roof (closed)": "Techo retráctil (cerrado)",
				"retractable roof (open)":   "Techo retráctil (abierto)",
				"fieldturf":                 "Césped artificial",
				"a_turf":                    "Césped artificial",
			},
			months: map[string]string{
				"Jan": "enero", "January": "enero", "Feb": "febrero", "February": "febrero",
				"Mar": "marzo", "March": "marzo", "Apr": "abril", "April": "abril",
				"May": "mayo", "Jun": "junio", "June": "junio",
				"Jul": "julio", "July": "julio", "Aug": "agosto", "August": "agosto",
				"Sep": "septiembre", "September": "septiembre",
				"Oct": "octubre", "October": "octubre",
				"Nov": "noviembre", "November": "noviembre",
				"Dec": "diciembre", "December": "diciembre",
			},
		}
	}

	t := &ESTranslator{
		translations: translations,
		labels:       make(map[string]string),
		months:       make(map[string]string),
	}

	// Populate labels from translations, with key variants
	t.labels = buildLabelVariants(translations.UI)

	// Populate months from translations, with key variants
	t.months = buildMonthVariants(translations.Months)

	return t
}

// buildLabelVariants creates multiple key variants for label lookups
func buildLabelVariants(ui map[string]string) map[string]string {
	result := make(map[string]string)

	for key, val := range ui {
		// Add the original key
		result[key] = val

		// Add lowercase version
		result[strings.ToLower(key)] = val

		// Add capitalized version (first letter uppercase, rest lowercase)
		if len(key) > 0 {
			capitalized := strings.ToUpper(string(key[0])) + strings.ToLower(key[1:])
			result[capitalized] = val
		}

		// Add version with spaces instead of underscores
		keyWithSpaces := strings.ReplaceAll(key, "_", " ")
		result[keyWithSpaces] = val

		// Add capitalized version with spaces
		if len(keyWithSpaces) > 0 {
			capitalizedWithSpaces := strings.ToUpper(string(keyWithSpaces[0])) + strings.ToLower(keyWithSpaces[1:])
			result[capitalizedWithSpaces] = val
		}

		// Add version with parentheses (e.g., "retractable_roof_closed" -> "retractable roof (closed)")
		if strings.Contains(key, "roof") {
			keyWithParens := strings.ReplaceAll(key, "_", " ")
			if strings.Contains(keyWithParens, "closed") {
				keyWithParens = strings.ReplaceAll(keyWithParens, " closed", " (closed)")
				result[keyWithParens] = val
				// Add capitalized version
				if len(keyWithParens) > 0 {
					capitalizedWithParens := strings.ToUpper(string(keyWithParens[0])) + strings.ToLower(keyWithParens[1:])
					result[capitalizedWithParens] = val
				}
			} else if strings.Contains(keyWithParens, "open") {
				keyWithParens = strings.ReplaceAll(keyWithParens, " open", " (open)")
				result[keyWithParens] = val
				// Add capitalized version
				if len(keyWithParens) > 0 {
					capitalizedWithParens := strings.ToUpper(string(keyWithParens[0])) + strings.ToLower(keyWithParens[1:])
					result[capitalizedWithParens] = val
				}
			}
		}
	}

	return result
}

// buildMonthVariants creates multiple key variants for month lookups
func buildMonthVariants(months map[string]string) map[string]string {
	result := make(map[string]string)

	for key, val := range months {
		// Add the original key
		result[key] = val

		// Add lowercase version
		result[strings.ToLower(key)] = val

		// Add uppercase version
		result[strings.ToUpper(key)] = val

		// Add capitalized version (first letter uppercase, rest lowercase)
		if len(key) > 0 {
			capitalized := strings.ToUpper(string(key[0])) + strings.ToLower(key[1:])
			result[capitalized] = val
		}
	}

	return result
}

func (t *ESTranslator) T(key string) string {
	if val, ok := t.labels[key]; ok {
		return val
	}
	return key
}

func (t *ESTranslator) TranslateTeamName(name string) string {
	return name
}

func (t *ESTranslator) TranslateMonth(month string) string {
	if val, ok := t.months[month]; ok {
		return val
	}
	return strings.ToLower(month)
}

func (t *ESTranslator) TranslateDay(dayNum int) string {
	return fmt.Sprintf("%d", dayNum)
}

func (t *ESTranslator) TranslateWeather(text string) string {
	result := strings.TrimSpace(text)
	if result == "" {
		return text
	}

	replacements := []struct {
		pattern string
		value   string
	}{
		{`(?i)\brelative humidity\b`, "humedad relativa"},
		{`(?i)\bgusts up to\b`, "rafagas de hasta"},
		{`(?i)\bdegrees?\b`, "grados"},
		{`(?i)\bhumidity\b`, "humedad"},
		{`(?i)\bwind\b`, "viento"},
	}
	for _, item := range replacements {
		re := regexp.MustCompile(item.pattern)
		result = re.ReplaceAllString(result, item.value)
	}

	return result
}

func (t *ESTranslator) TranslateScoringPlay(text string) string {
	result := text

	// PRIMERO: Patrones complejos que reordenan la frase completa
	// Para carreras: "N yard rush" -> "carrera de N yardas"
	re := regexp.MustCompile(`(\d+)\s+yards?\s+(?:rushing|run|rush)\b`)
	result = re.ReplaceAllString(result, "carrera de $1 yardas")

	// Para field goals: "N yard field goal" -> "field goal de N yardas"
	re = regexp.MustCompile(`(\d+)\s+yards?\s+field goal\b`)
	result = re.ReplaceAllString(result, "field goal de $1 yardas")

	// Para pases: "N yard pass from QB" -> "pase de N yardas de QB"
	re = regexp.MustCompile(`(\d+)\s+yards?\s+pass from\s+(\S+)\b`)
	result = re.ReplaceAllString(result, "pase de $1 yardas de $2")

	// SEGUNDO: Si aún hay yardas sin procesar, convertirlas al plural
	re = regexp.MustCompile(`(\d+)\s+yards?\b`)
	result = re.ReplaceAllString(result, "$1 yardas")

	// TERCERO: Reemplazar frases comunes (ORDEN IMPORTANTE - más específicas primero)
	// Conversiones de puntos extra
	result = strings.ReplaceAll(result, "two point pase successful", "conversión de dos puntos")
	result = strings.ReplaceAll(result, "two point pass successful", "conversión de dos puntos")
	result = strings.ReplaceAll(result, "run failed", "conversión fallida")
	result = strings.ReplaceAll(result, "pass failed", "conversión fallida")
	result = strings.ReplaceAll(result, "tackled in end zone", "placado en la end zone")

	result = strings.ReplaceAll(result, "kickoff return", "retorno de kickoff")
	result = strings.ReplaceAll(result, "kick return", "retorno de patada")
	result = strings.ReplaceAll(result, "interception return", "retorno de interception")
	result = strings.ReplaceAll(result, "pass from", "pase de")
	result = strings.ReplaceAll(result, "pass to", "pase a")
	result = strings.ReplaceAll(result, "pass", "pase")
	result = strings.ReplaceAll(result, "extra point", "punto extra")
	result = strings.ReplaceAll(result, "rush", "carrera")
	result = strings.ReplaceAll(result, "running", "carrera")
	result = strings.ReplaceAll(result, "touchdown", "touchdown")

	// CUARTO: Corregir singular/plural de yardas
	result = strings.ReplaceAll(result, "de 1 yardas", "de 1 yarda")

	return result
}
