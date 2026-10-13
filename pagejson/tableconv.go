package pagejson

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/Mundo-Dolphins/mundodolphins-stats/i18n"
)

// ConvertTableToRows converts a table from processed JSON format to []TableRow
// Each row becomes a map where keys are column headers and values are cell contents as strings
func ConvertTableToRows(tableData interface{}, headerMapping map[string]string) ([]TableRow, error) {
	// tableData can be:
	// - []any where each element is map[string]any (most common from PFR tables)
	// - []map[string]any
	// - []map[string]string

	switch v := tableData.(type) {
	case []interface{}:
		return convertSliceOfInterface(v, headerMapping)
	case []map[string]interface{}:
		return convertSliceOfMapInterface(v, headerMapping)
	case []map[string]string:
		return convertSliceOfMapString(v, headerMapping)
	default:
		return nil, fmt.Errorf("unsupported table data type: %T", tableData)
	}
}

// convertSliceOfInterface handles []interface{} where each element is a map
func convertSliceOfInterface(data []interface{}, headerMapping map[string]string) ([]TableRow, error) {
	rows := make([]TableRow, 0, len(data))

	for i, item := range data {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("row %d is not a map: %T", i, item)
		}

		row := NewTableRow()
		for _, key := range sortedKeys(itemMap) {
			value := itemMap[key]
			// Get display header name (use mapping if provided, otherwise use key)
			displayHeader := displayHeaderForKey(key, headerMapping)

			// Convert value to string
			strValue := convertValueToString(value)
			strValue = translateCellValue(strValue, headerMapping)
			row.Set(displayHeader, strValue)
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// convertSliceOfMapInterface handles []map[string]interface{}
func convertSliceOfMapInterface(data []map[string]interface{}, headerMapping map[string]string) ([]TableRow, error) {
	rows := make([]TableRow, 0, len(data))

	for _, itemMap := range data {
		row := NewTableRow()
		for _, key := range sortedKeys(itemMap) {
			value := itemMap[key]
			displayHeader := displayHeaderForKey(key, headerMapping)

			strValue := convertValueToString(value)
			strValue = translateCellValue(strValue, headerMapping)
			row.Set(displayHeader, strValue)
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// convertSliceOfMapString handles []map[string]string (already strings)
func convertSliceOfMapString(data []map[string]string, headerMapping map[string]string) ([]TableRow, error) {
	rows := make([]TableRow, 0, len(data))

	for _, itemMap := range data {
		row := NewTableRow()
		for _, key := range sortedKeys(itemMap) {
			value := itemMap[key]
			displayHeader := displayHeaderForKey(key, headerMapping)

			row.Set(displayHeader, translateCellValue(value, headerMapping))
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// convertValueToString converts any value to string representation
func convertValueToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		// Check if it's a whole number
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.2f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case map[string]interface{}:
		// This might be a LinkValue structure {"content": "...", "link": "..."}
		if content, ok := v["content"]; ok {
			return convertValueToString(content)
		}
		// Otherwise, return empty or a JSON representation
		return ""
	default:
		// For other types, use fmt.Sprintf
		return fmt.Sprintf("%v", v)
	}
}

// NormalizeHeaders ensures unique column headers by appending numbers to duplicates
// Example: if "Columna" appears twice, second becomes "Columna_2"
func NormalizeHeaders(headers []string) []string {
	seen := make(map[string]int)
	normalized := make([]string, len(headers))

	for i, header := range headers {
		// If header is empty, use "Columna"
		if strings.TrimSpace(header) == "" {
			header = "Columna"
		}

		// Check if we've seen this header before
		count, exists := seen[header]
		if exists {
			// Append number to make it unique
			count++
			normalized[i] = fmt.Sprintf("%s_%d", header, count)
			seen[header] = count
		} else {
			normalized[i] = header
			seen[header] = 1
		}
	}

	return normalized
}

// ConvertTableWithHeaders converts a table with explicit header list
// This is useful when the table in processed JSON has separate header and rows arrays
func ConvertTableWithHeaders(headers []string, rows [][]interface{}) ([]TableRow, error) {
	// Normalize headers to ensure uniqueness
	normalizedHeaders := NormalizeHeaders(headers)

	result := make([]TableRow, 0, len(rows))

	for rowIdx, row := range rows {
		if len(row) != len(normalizedHeaders) {
			return nil, fmt.Errorf("row %d has %d cells but expected %d (headers count)", rowIdx, len(row), len(normalizedHeaders))
		}

		tableRow := NewTableRow()
		for colIdx, cell := range row {
			header := normalizedHeaders[colIdx]
			tableRow.Set(header, convertValueToString(cell))
		}

		result = append(result, tableRow)
	}

	return result, nil
}

func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return string(keys[i]) < string(keys[j]) })
	return keys
}

func displayHeaderForKey(key string, headerMapping map[string]string) string {
	if headerMapping != nil {
		if mapped, exists := headerMapping[key]; exists && strings.TrimSpace(mapped) != "" {
			return mapped
		}
	}
	if mapped, ok := columnHeaderFromI18n(key); ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return humanizeHeaderKey(key)
}

var (
	columnHeadersOnce sync.Once
	columnHeadersMap  map[string]string
)

// columnHeaderFromI18n checks ColumnHeaders (authoritative) and falls back to
// FlattenTableLegends for backward-compat keys only present there.
func columnHeaderFromI18n(key string) (string, bool) {
	columnHeadersOnce.Do(func() {
		columnHeadersMap = map[string]string{}
		tr, err := i18n.GetDefaultTranslations()
		if err != nil || tr == nil {
			return
		}
		// Start with table legends (backward compat)
		columnHeadersMap = tr.FlattenTableLegends()
		// Override with column_headers (authoritative source)
		for k, v := range tr.ColumnHeaders {
			columnHeadersMap[k] = v
		}
	})
	v, ok := columnHeadersMap[key]
	return v, ok
}

func humanizeHeaderKey(key string) string {
	if strings.TrimSpace(key) == "" {
		return "Columna"
	}
	s := strings.ReplaceAll(key, "_", " ")
	parts := strings.Fields(s)
	for i, p := range parts {
		l := strings.ToLower(p)
		if n, err := strconv.Atoi(l); err == nil {
			parts[i] = strconv.Itoa(n)
			continue
		}
		switch l {
		case "qb", "rb", "wr", "te", "td", "int", "xp", "fg", "yds", "yd":
			parts[i] = strings.ToUpper(l)
		case "pct":
			parts[i] = "%"
		default:
			parts[i] = titleWord(l)
		}
	}
	return strings.Join(parts, " ")
}

// translateCellValue translates a table cell value using the same table-specific
// mapping used for headers when a direct match exists.
// This enables YAML-only localization of row labels like team_stats.stat values.
func translateCellValue(value string, headerMapping map[string]string) string {
	if strings.TrimSpace(value) == "" || headerMapping == nil {
		return value
	}
	if mapped, ok := headerMapping[value]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return value
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
