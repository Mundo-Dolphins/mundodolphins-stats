package season_stats

import (
	"fmt"
	"sort"
	"strings"
)

// NormalizedTable represents a table normalized for presentation rendering.
type NormalizedTable struct {
	Name    string              // Table name (machine-readable)
	TitleES string              // Spanish title for display
	Columns []string            // Column names in render order
	Rows    []map[string]string // Row data, all values as strings
}

// NormalizeTable converts raw JSON table data to presentation-ready format.
//
// Parameters:
// - tableName: name of the table (used for looking up allowed columns)
// - rows: array of row objects from JSON (map[string]any)
// - allowedCols: preferred column order (nil = use all columns found)
//
// Returns:
// - columns: final column list to render (subset of allowedCols, only columns with data)
// - outRows: rows with all values converted to strings
//
// Rules:
// - Respect allowedCols order
// - Skip empty columns (all rows have empty/nil values)
// - Convert complex types (LinkValue, RichTextWithLinks, numbers) consistently
// - Deterministic iteration over maps (sort keys)
func NormalizeTable(
	tableName string,
	rows []map[string]any,
	allowedCols []string,
) (columns []string, outRows []map[string]string) {
	if len(rows) == 0 {
		return []string{}, []map[string]string{}
	}

	// Determine which columns to use
	var colsToUse []string
	if len(allowedCols) > 0 {
		// Use allowed columns that actually exist in rows
		colsToUse = filterExistingColumns(rows, allowedCols)
	} else {
		// Use all columns found in first row (deterministically sorted)
		colsToUse = getAllColumnsFromRows(rows)
	}

	// Skip completely empty columns
	colsToUse = filterEmptyColumns(rows, colsToUse)

	// Convert rows
	for _, rowMap := range rows {
		outRow := make(map[string]string)
		for _, col := range colsToUse {
			val := rowMap[col]
			outRow[col] = valueToString(val)
		}
		outRows = append(outRows, outRow)
	}

	return colsToUse, outRows
}

// filterExistingColumns keeps only columns that appear in at least one row.
func filterExistingColumns(rows []map[string]any, allowedCols []string) []string {
	existingCols := make(map[string]bool)
	for _, row := range rows {
		for _, col := range allowedCols {
			if _, exists := row[col]; exists {
				existingCols[col] = true
			}
		}
	}

	var result []string
	for _, col := range allowedCols {
		if existingCols[col] {
			result = append(result, col)
		}
	}
	return result
}

// getAllColumnsFromRows extracts all unique columns from rows, sorted for determinism.
func getAllColumnsFromRows(rows []map[string]any) []string {
	colSet := make(map[string]bool)
	for _, row := range rows {
		for col := range row {
			colSet[col] = true
		}
	}

	var cols []string
	for col := range colSet {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

// filterEmptyColumns removes columns where all values are empty/nil.
func filterEmptyColumns(rows []map[string]any, cols []string) []string {
	var result []string

	for _, col := range cols {
		hasData := false
		for _, row := range rows {
			if val, exists := row[col]; exists && val != nil && val != "" {
				// Check if it's an empty string representation
				valStr := valueToString(val)
				if valStr != "" && valStr != "0" && strings.TrimSpace(valStr) != "" {
					hasData = true
					break
				}
			}
		}
		if hasData {
			result = append(result, col)
		}
	}

	return result
}

// valueToString converts any JSON value to a string representation.
// Handles:
// - nil -> ""
// - bool -> "yes"/"no"
// - numbers -> decimal notation
// - strings -> as-is
// - LinkValue -> display text with link metadata
// - RichTextWithLinks -> combined text from links
// - maps/arrays -> JSON representation
func valueToString(val any) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v

	case float64:
		// Check if it's a whole number
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.2f", v)

	case bool:
		if v {
			return "yes"
		}
		return "no"

	case int:
		return fmt.Sprintf("%d", v)

	case int64:
		return fmt.Sprintf("%d", v)

	case map[string]any:
		// LinkValue: {content: "text", link: "/url"} -> "text (/url)"
		if content, hasContent := v["content"].(string); hasContent {
			if link, hasLink := v["link"].(string); hasLink {
				if link != "" && link != "." {
					return fmt.Sprintf("%s [%s]", content, link)
				}
			}
			return content
		}

		// RichTextWithLinks: {content: "...", links: [...]}
		if content, hasContent := v["content"].(string); hasContent {
			if links, hasLinks := v["links"].([]any); hasLinks && len(links) > 0 {
				return content
			}
			return content
		}

		// Fallback for unknown maps
		return fmt.Sprintf("%v", v)

	case []any:
		// Convert array to comma-separated string
		var items []string
		for _, item := range v {
			items = append(items, valueToString(item))
		}
		return strings.Join(items, ", ")

	default:
		return fmt.Sprintf("%v", v)
	}
}

// HumanizeTableName converts table_name to "Table Name" for display.
func HumanizeTableName(tableName string) string {
	parts := strings.Split(tableName, "_")
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, strings.ToUpper(string(part[0]))+strings.ToLower(part[1:]))
		}
	}
	return strings.Join(result, " ")
}
