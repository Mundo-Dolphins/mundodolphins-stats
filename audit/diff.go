package audit

import (
	"sort"
)

// SeasonDiff holds the differences between two season inventories.
type SeasonDiff struct {
	TablesOnlyInA             []string              `json:"tables_only_in_a"`
	TablesOnlyInB             []string              `json:"tables_only_in_b"`
	FieldsOnlyInA             map[string][]string   `json:"fields_only_in_a"`
	FieldsOnlyInB             map[string][]string   `json:"fields_only_in_b"`
	ColumnDiffsInCommonTables map[string]ColumnDiff `json:"column_diffs_in_common_tables"`
}

// ColumnDiff holds column differences for a table that exists in both seasons.
type ColumnDiff struct {
	ColumnsOnlyInA []string `json:"columns_only_in_a"`
	ColumnsOnlyInB []string `json:"columns_only_in_b"`
}

// DiffSeasons compares two season inventories and returns differences.
// Results are sorted alphabetically for deterministic output.
func DiffSeasons(a, b SeasonInventory) SeasonDiff {
	diff := SeasonDiff{
		FieldsOnlyInA:             make(map[string][]string),
		FieldsOnlyInB:             make(map[string][]string),
		ColumnDiffsInCommonTables: make(map[string]ColumnDiff),
	}

	// Find tables only in A or only in B
	aSet := make(map[string]bool)
	bSet := make(map[string]bool)

	for tableName := range a.Tables {
		aSet[tableName] = true
	}
	for tableName := range b.Tables {
		bSet[tableName] = true
	}

	for tableName := range aSet {
		if !bSet[tableName] {
			diff.TablesOnlyInA = append(diff.TablesOnlyInA, tableName)
		}
	}

	for tableName := range bSet {
		if !aSet[tableName] {
			diff.TablesOnlyInB = append(diff.TablesOnlyInB, tableName)
		}
	}

	// Sort for determinism
	sort.Strings(diff.TablesOnlyInA)
	sort.Strings(diff.TablesOnlyInB)

	// Find field differences in common tables
	for tableName := range aSet {
		if !bSet[tableName] {
			continue // table not in both
		}

		aTable := a.Tables[tableName]
		bTable := b.Tables[tableName]

		aColSet := make(map[string]bool)
		bColSet := make(map[string]bool)

		for _, col := range aTable.Columns {
			aColSet[col] = true
		}
		for _, col := range bTable.Columns {
			bColSet[col] = true
		}

		var colsOnlyInA, colsOnlyInB []string

		for col := range aColSet {
			if !bColSet[col] {
				colsOnlyInA = append(colsOnlyInA, col)
			}
		}

		for col := range bColSet {
			if !aColSet[col] {
				colsOnlyInB = append(colsOnlyInB, col)
			}
		}

		if len(colsOnlyInA) > 0 || len(colsOnlyInB) > 0 {
			sort.Strings(colsOnlyInA)
			sort.Strings(colsOnlyInB)
			diff.ColumnDiffsInCommonTables[tableName] = ColumnDiff{
				ColumnsOnlyInA: colsOnlyInA,
				ColumnsOnlyInB: colsOnlyInB,
			}
		}
	}

	return diff
}
