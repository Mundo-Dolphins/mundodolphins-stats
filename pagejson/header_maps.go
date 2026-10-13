package pagejson

func seasonMainTableHeaderMap(tableKey string) map[string]string {
	return tableHeaderFromI18n("season", tableKey)
}

func seasonAdvancedTableHeaderMap(tableKey string) map[string]string {
	return tableHeaderFromI18n("season_advanced", tableKey)
}

func gameTableHeaderMap(tableKey string) map[string]string {
	return tableHeaderFromI18n("game", tableKey)
}
