package season_stats

// SkipTables contains table names that should never be rendered.
var SkipTables = map[string]bool{
	"preseason_odds": true, // Not relevant, explicitly excluded
}

// ExtraTablesAllowlist maps table names to their preferred column order.
// Only columns present in both the JSON and this list will be rendered.
// The order here defines the column rendering order.
var ExtraTablesAllowlist = map[string][]string{
	// Roster/Personnel
	"roster": {
		"name_display", "pos", "age", "ht", "wt", "college",
	},
	"starters": {
		"week", "player_off", "player_def",
	},

	// Snap Counts
	"snap_counts": {
		"week", "player", "pos", "snaps_offense", "snaps_defense",
	},

	// Injuries
	"team_injuries": {
		"week", "player", "pos", "nfl_status", "notes",
	},
	"team_injuries_totals": {
		"player", "pos", "games_missed",
	},

	// Draft
	"draft": {
		"round", "pick", "player", "pos", "college", "notes",
	},

	// Splits (Offensive)
	"down_splits": {
		"down", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"distance_splits": {
		"distance", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"down_distance_splits": {
		"down_distance", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"qtr_splits": {
		"quarter", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"situation_splits": {
		"situation", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"field_position_splits": {
		"field_position", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"game_location_splits": {
		"location", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"pt_diff_splits": {
		"point_diff", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"snap_type_splits": {
		"snap_type", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"blitz_type_splits": {
		"blitz_type", "plays", "yds", "yds_per_play", "td", "fum",
	},
	"yds_split_splits": {
		"yds_range", "plays", "yds", "yds_per_play", "td", "fum",
	},

	// Travel/Map
	"map_table": {
		"week", "opponent", "location", "distance",
	},

	// Games (detailed log)
	"games": {
		"week", "day", "date", "time", "opponent", "result", "pts_off", "pts_def", "yds_off", "yds_def",
	},
}

// IsRenderableTable checks if a table should be rendered.
// Returns false if the table is in SkipTables.
func IsRenderableTable(tableName string) bool {
	return !SkipTables[tableName]
}

// GetAllowedColumns returns the preferred column order for a table.
func GetAllowedColumns(tableName string) []string {
	return ExtraTablesAllowlist[tableName]
}
