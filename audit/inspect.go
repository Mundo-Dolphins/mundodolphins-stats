// Package audit provides tools for inspecting and analyzing processed season JSON files.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// TableInfo holds metadata about a table found in the season JSON.
type TableInfo struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	RowCount int      `json:"row_count"`
}

// SeasonInventory holds all tables discovered in a single season JSON file.
type SeasonInventory struct {
	Year   int                  `json:"year"`
	Tables map[string]TableInfo `json:"tables"`
}

// SeasonJSON represents the top-level structure of a processed season file.
type SeasonJSON struct {
	Schema struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"schema"`
	Data map[string]interface{} `json:"data"`
}

// InspectSeason loads a season JSON file and dynamically detects all tables and columns.
// It returns a SeasonInventory with metadata about each table found.
// Columns are always sorted alphabetically for deterministic output.
func InspectSeason(ctx context.Context, filePath string) (SeasonInventory, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SeasonInventory{}, fmt.Errorf("read season file %s: %w", filePath, err)
	}

	var seasonJSON SeasonJSON
	if err := json.Unmarshal(data, &seasonJSON); err != nil {
		return SeasonInventory{}, fmt.Errorf("parse season JSON: %w", err)
	}

	inventory := SeasonInventory{
		Year:   extractYear(filePath),
		Tables: make(map[string]TableInfo),
	}

	// Dynamically detect all arrays in the structure
	if seasonJSON.Data != nil {
		for sectionName, sectionData := range seasonJSON.Data {
			switch sectionData := sectionData.(type) {
			case []interface{}:
				// Direct array - treat as table
				info := inspectTable(sectionName, sectionData)
				inventory.Tables[sectionName] = info
			case map[string]interface{}:
				// Nested object - look for arrays or .tables sub-objects
				for key, val := range sectionData {
					if arr, isArray := val.([]interface{}); isArray {
						// Found an array
						info := inspectTable(key, arr)
						inventory.Tables[key] = info
					} else if key == "tables" {
						// Special handling for .tables objects
						if tablesMap, isMap := val.(map[string]interface{}); isMap {
							for tableName, tableData := range tablesMap {
								if tableObj, isMap := tableData.(map[string]interface{}); isMap {
									// Look for .rows in the table object
									if rows, hasRows := tableObj["rows"].([]interface{}); hasRows {
										info := inspectTable(tableName, rows)
										inventory.Tables[tableName] = info
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return inventory, nil
}

// inspectTable examines a single table and extracts its structure.
func inspectTable(tableName string, tableData interface{}) TableInfo {
	info := TableInfo{
		Name:    tableName,
		Columns: []string{},
	}

	// Handle array of objects (most common case)
	rows, ok := tableData.([]interface{})
	if !ok {
		return info
	}

	info.RowCount = len(rows)

	// Collect all unique columns by scanning all rows
	columnSet := make(map[string]bool)
	for _, row := range rows {
		if rowMap, ok := row.(map[string]interface{}); ok {
			for col := range rowMap {
				columnSet[col] = true
			}
		}
	}

	// Convert set to sorted slice for determinism
	for col := range columnSet {
		info.Columns = append(info.Columns, col)
	}
	sort.Strings(info.Columns)

	return info
}

// extractYear extracts the year from a file path like "processed/2024.json"
// Returns 0 if unable to parse.
func extractYear(filePath string) int {
	// Find the first number sequence that looks like a year (4 digits)
	for i := 0; i < len(filePath)-3; i++ {
		if isDigit(filePath[i]) && isDigit(filePath[i+1]) && isDigit(filePath[i+2]) && isDigit(filePath[i+3]) {
			year := 0
			for j := 0; j < 4; j++ {
				year = year*10 + int(filePath[i+j]-'0')
			}
			return year
		}
	}
	return 0
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
