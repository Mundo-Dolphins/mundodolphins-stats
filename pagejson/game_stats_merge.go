package pagejson

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/footballdb"
	"github.com/Mundo-Dolphins/mundodolphins-stats/i18n"
)

func mergeStringStats(primary map[string]string, fallbacks ...map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range primary {
		out[k] = v
	}
	for _, fallback := range fallbacks {
		for k, v := range fallback {
			if strings.TrimSpace(out[k]) == "" && strings.TrimSpace(v) != "" {
				out[k] = v
			}
		}
	}
	return out
}

func mergePlayerStatRows(primary []TableRow, fallbacks ...[]TableRow) []TableRow {
	out := cloneTableRows(primary)
	index := indexTableRows(out)

	for fallbackIdx, fallback := range fallbacks {
		appendUnmatched := fallbackIdx == 0 || len(out) == 0
		for _, row := range fallback {
			match := findTableRowIndex(index, row)
			if match >= 0 {
				fillTableRow(&out[match], row)
				index = indexTableRows(out)
				continue
			}
			if appendUnmatched {
				out = append(out, row)
				index = indexTableRows(out)
			}
		}
	}
	if out == nil {
		return []TableRow{}
	}
	return out
}

func cloneTableRows(rows []TableRow) []TableRow {
	out := make([]TableRow, 0, len(rows))
	for _, row := range rows {
		next := NewTableRow()
		for _, key := range row.Keys() {
			next.Set(key, row.Get(key))
		}
		out = append(out, next)
	}
	return out
}

func fillTableRow(dst *TableRow, src TableRow) {
	if dst == nil {
		return
	}
	for _, key := range src.Keys() {
		if strings.TrimSpace(dst.Get(key)) == "" && strings.TrimSpace(src.Get(key)) != "" {
			dst.Set(key, src.Get(key))
		}
	}
}

func indexTableRows(rows []TableRow) map[string]int {
	index := map[string]int{}
	for i, row := range rows {
		for _, key := range tableRowMatchKeys(row) {
			if _, exists := index[key]; !exists {
				index[key] = i
			}
		}
	}
	return index
}

func findTableRowIndex(index map[string]int, row TableRow) int {
	for _, key := range tableRowMatchKeys(row) {
		if idx, ok := index[key]; ok {
			return idx
		}
	}
	return -1
}

func tableRowMatchKeys(row TableRow) []string {
	name := row.Get(fieldLabel("player"))
	if name == "" {
		name = row.Get("PLAYER")
	}
	if name == "" {
		name = row.Get("Player")
	}
	if name == "" {
		return nil
	}
	keys := []string{compactToken(name)}
	if initials := playerInitialKey(name); initials != "" {
		keys = append(keys, initials)
	}
	return keys
}

func playerInitialKey(name string) string {
	parts := strings.Fields(strings.ReplaceAll(name, ".", ""))
	if len(parts) < 2 {
		return ""
	}
	return compactToken(fmt.Sprintf("%c %s", []rune(parts[0])[0], parts[len(parts)-1]))
}

func compactToken(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func buildNFLComPlayerStats(env boxscore.BoxscoreEnvelope) map[string]map[string][]TableRow {
	out := emptyPlayerStatsBySide()
	if env.Data.NFLCom == nil || env.Data.NFLCom.Stats == nil {
		return out
	}
	for _, cfg := range []struct {
		side string
		team map[string]any
	}{
		{"vis", env.Data.NFLCom.Stats.AwayTeam},
		{"home", env.Data.NFLCom.Stats.HomeTeam},
	} {
		abbr := nflComTeamAbbr(cfg.team)
		rowsByTable := map[string][]map[string]any{}
		stats, ok := cfg.team["stats"].([]any)
		if !ok {
			continue
		}
		for _, item := range stats {
			stat, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title := convertValueToString(stat["title"])
			tableKey, normalized := normalizeNFLComStatRows(title, abbr, stat)
			if tableKey == "" || len(normalized) == 0 {
				continue
			}
			rowsByTable[tableKey] = mergeRawPlayerRows(rowsByTable[tableKey], normalized)
		}
		out[cfg.side][sectionTitle("offensive")] = rawRowsToTableRows(rowsByTable["player_offense"], gameTableHeaderMap("player_offense"))
		out[cfg.side][sectionTitle("defensive")] = rawRowsToTableRows(rowsByTable["player_defense"], gameTableHeaderMap("player_defense"))
		out[cfg.side][sectionTitle("special_teams")] = mergePlayerTables(
			rawRowsToTableRows(rowsByTable["kicking"], gameTableHeaderMap("kicking")),
			rawRowsToTableRows(rowsByTable["returns"], gameTableHeaderMap("returns")),
		)
	}
	return out
}

func normalizeNFLComStatRows(title string, teamAbbr string, stat map[string]any) (string, []map[string]any) {
	table, ok := stat["table"].(map[string]any)
	if !ok {
		return "", nil
	}
	columns, ok := table["columns"].([]any)
	if !ok || len(columns) == 0 {
		return "", nil
	}
	columnKeys := make([]string, len(columns))
	tableKey := ""
	for idx, col := range columns {
		header := nflcomCellText(col)
		if strings.EqualFold(header, "PLAYER") || strings.TrimSpace(header) == "" {
			columnKeys[idx] = "player"
			continue
		}
		nextTableKey, pfrKey := nflComPFRColumnMapping(title, header)
		if nextTableKey != "" {
			tableKey = nextTableKey
		}
		columnKeys[idx] = pfrKey
	}
	if tableKey == "" {
		return "", nil
	}
	rawRows, ok := table["rows"].([]any)
	if !ok || len(rawRows) == 0 {
		return "", nil
	}
	out := make([]map[string]any, 0, len(rawRows))
	for _, rawRow := range rawRows {
		cells, ok := rawRow.([]any)
		if !ok {
			continue
		}
		row := map[string]any{"team": strings.ToUpper(teamAbbr)}
		for idx, key := range columnKeys {
			if key == "" || idx >= len(cells) {
				continue
			}
			value := nflcomCellText(cells[idx])
			if strings.EqualFold(key, "player") && strings.EqualFold(value, "TEAM") {
				row = nil
				break
			}
			row[key] = value
		}
		if len(row) > 1 && strings.TrimSpace(convertValueToString(row["player"])) != "" {
			out = append(out, row)
		}
	}
	return tableKey, out
}

func nflComPFRColumnMapping(nflTitle string, columnTitle string) (string, string) {
	category := normalizeNFLComStatCategory(nflTitle)
	switch category {
	case "PASSING":
		return "player_offense", mapString(columnTitle, map[string]string{
			"CMP": "pass_cmp", "ATT": "pass_att", "YDS": "pass_yds", "TD": "pass_td", "INT": "pass_int", "LONG": "pass_long", "SACKS": "pass_sacked", "RATING": "pass_rating",
		})
	case "RUSHING":
		return "player_offense", mapString(columnTitle, map[string]string{
			"ATT": "rush_att", "YDS": "rush_yds", "TD": "rush_td", "LONG": "rush_long", "FUM": "fumbles",
		})
	case "RECEIVING":
		return "player_offense", mapString(columnTitle, map[string]string{
			"REC": "rec", "YDS": "rec_yds", "TD": "rec_td", "TGTS": "targets", "LONG": "rec_long", "YAC": "rec_yac",
		})
	case "FUMBLES":
		return "player_offense", mapString(columnTitle, map[string]string{
			"FUM": "fumbles", "LOST": "fumbles_lost",
		})
	case "INTERCEPTIONS":
		return "player_defense", mapString(columnTitle, map[string]string{
			"INT": "def_int", "INT YDS": "def_int_yds", "INT TD": "def_int_td", "LONG": "def_int_long",
		})
	case "DEFENSE":
		return "player_defense", mapString(columnTitle, map[string]string{
			"TOT": "tackles_combined", "SOLO": "tackles_solo", "SACKS": "sacks", "TFL": "tackles_loss", "PD": "pass_defended", "INT": "def_int", "QBP": "qb_hits", "FF": "fumbles_forced", "FR": "fumbles_rec",
		})
	case "KICKING":
		return "kicking", mapString(columnTitle, map[string]string{
			"FGM": "fgm", "FGA": "fga", "XP": "xpm", "LONG": "fg_long",
		})
	case "KICKOFF RETURNS":
		return "returns", mapString(columnTitle, map[string]string{
			"RET": "kick_ret", "AVG": "kick_ret_yds_per_ret", "TD": "kick_ret_td", "LONG": "kick_ret_long",
		})
	case "PUNTING":
		return "kicking", mapString(columnTitle, map[string]string{
			"PUNTS": "punt", "AVG": "punt_yds_per_punt", "IN 20": "punt_in_20", "LONG": "punt_long",
		})
	case "PUNT RETURNS":
		return "returns", mapString(columnTitle, map[string]string{
			"RET": "punt_ret", "AVG": "punt_ret_yds_per_ret", "TD": "punt_ret_td", "LONG": "punt_ret_long",
		})
	default:
		return "", ""
	}
}

func normalizeNFLComStatCategory(title string) string {
	title = strings.ToUpper(strings.TrimSpace(title))
	parts := strings.Fields(title)
	if len(parts) <= 1 {
		return title
	}
	return strings.Join(parts[1:], " ")
}

func mapString(key string, values map[string]string) string {
	return values[strings.ToUpper(strings.TrimSpace(key))]
}

func nflComTeamAbbr(team map[string]any) string {
	for _, key := range []string{"abbreviation", "abbr"} {
		if value := convertValueToString(team[key]); value != "" {
			return value
		}
	}
	return ""
}

func buildGamebookTeamStats(env boxscore.BoxscoreEnvelope) map[string]map[string]string {
	out := map[string]map[string]string{"vis": {}, "home": {}}
	if env.Data.NFLCom == nil || env.Data.NFLCom.GamebookData == nil {
		return out
	}
	for _, stat := range env.Data.NFLCom.GamebookData.TeamStats {
		label := strings.TrimSpace(stat.Stat)
		if label == "" {
			continue
		}
		setIfBlank(out["vis"], label, stat.Visitor)
		setIfBlank(out["home"], label, stat.Home)
	}
	return out
}

func gamebookSnapCountRows(env boxscore.BoxscoreEnvelope, teamName string) []TableRow {
	if env.Data.NFLCom == nil || env.Data.NFLCom.GamebookData == nil {
		return nil
	}
	rawRows := make([]map[string]any, 0)
	for _, snap := range env.Data.NFLCom.GamebookData.SnapCounts {
		if !gamebookTeamMatches(snap.Team, teamName) {
			continue
		}
		rawRows = append(rawRows, map[string]any{
			"player":        snap.Player,
			"pos":           snap.Position,
			"offense":       snap.Offense,
			"off_pct":       snap.OffensePct,
			"defense":       snap.Defense,
			"def_pct":       snap.DefensePct,
			"special_teams": snap.SpecialTeams,
			"st_pct":        snap.SpecialTeamsPct,
		})
	}
	return rawRowsToTableRows(rawRows, gameTableHeaderMap("snap_counts"))
}

func gamebookTeamMatches(source, target string) bool {
	source = compactToken(source)
	target = compactToken(target)
	if source == "" || target == "" {
		return false
	}
	return source == target || strings.Contains(source, target) || strings.Contains(target, source)
}

func buildFootballDBLinescore(fdb *footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope) []TableRow {
	if fdb == nil || env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return nil
	}
	table, ok := fdb.Data.Tables["table_0"]
	if !ok || len(table.Rows) == 0 {
		return nil
	}
	out := make([]TableRow, 0, 2)
	for _, side := range []string{"vis", "home"} {
		sourceRow := footballDBLinescoreRow(table, env, side)
		if sourceRow == nil {
			continue
		}
		row := NewTableRow()
		row.Set(fieldLabel("team"), scoreboxSideName(env, side))
		for _, q := range []string{"1", "2", "3", "4"} {
			row.Set(q, zeroIfEmpty(convertValueToString(sourceRow[q])))
		}
		row.Set(fieldLabel("final"), scoreOrZero(convertValueToString(sourceRow["total"])))
		out = append(out, row)
	}
	return out
}

func footballDBLinescoreRow(table footballdb.TableResult, env boxscore.BoxscoreEnvelope, side string) map[string]any {
	for _, row := range table.Rows {
		name := convertValueToString(row["col_0"])
		if name == "" || strings.EqualFold(name, "Total") {
			continue
		}
		if footballDBHasTeam([]string{name}, scoreboxSideName(env, side), scoreboxSideAbbr(env, side)) {
			return row
		}
	}
	return nil
}

func buildFootballDBScoringSummary(fdb *footballdb.Envelope[footballdb.GameData], sb *boxscore.Scorebox) []TableRow {
	if fdb == nil || sb == nil || len(sb.Teams) < 2 {
		return nil
	}
	table, ok := fdb.Data.Tables["table_1"]
	if !ok || len(table.Headers) < 3 || len(table.Rows) == 0 {
		return nil
	}
	tr := i18n.NewESTranslator()
	quarterKey := table.Headers[0]
	timeKey := table.Headers[1]
	descKey := table.Headers[2]
	currentQuarter := ""
	out := make([]TableRow, 0, len(table.Rows))
	for _, row := range table.Rows {
		first := convertValueToString(row[quarterKey])
		if first == "" {
			continue
		}
		if quarter := footballDBQuarterNumber(first); quarter != "" {
			currentQuarter = quarter
			continue
		}
		desc := convertValueToString(row[descKey])
		if desc == "" {
			continue
		}
		item := NewTableRow()
		item.Set(fieldLabel("quarter"), zeroIfEmpty(currentQuarter))
		if tm := convertValueToString(row[timeKey]); tm != "" {
			item.Set(fieldLabel("time"), tm)
		}
		item.Set(fieldLabel("team"), footballDBScoringTeamName(first, sb))
		item.Set(fieldLabel("description"), tr.TranslateScoringPlay(normalizeFootballDBScoringDescription(desc)))
		item.Set(fieldLabel("score_visitor"), scoreOrZero(convertValueToString(row["col_3"])))
		item.Set(fieldLabel("score_home"), scoreOrZero(convertValueToString(row["col_4"])))
		out = append(out, item)
	}
	return out
}

func footballDBQuarterNumber(label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	for _, suffix := range []string{"st quarter", "nd quarter", "rd quarter", "th quarter"} {
		if strings.HasSuffix(lower, suffix) {
			n := strings.TrimSuffix(lower, suffix)
			n = strings.TrimSpace(n)
			if _, err := strconv.Atoi(n); err == nil {
				return n
			}
		}
	}
	return ""
}

func footballDBScoringTeamName(raw string, sb *boxscore.Scorebox) string {
	token := compactToken(raw)
	for _, team := range sb.Teams {
		abbr := compactToken(team.Abbr)
		name := compactToken(linkContent(team.Name))
		if token != "" && ((abbr != "" && token == abbr) || (name != "" && strings.Contains(name, token))) {
			return linkContent(team.Name)
		}
	}
	return raw
}

var (
	footballDBFGScoringPattern   = regexp.MustCompile(`^FG\s+(.+)\s+(\d+)$`)
	footballDBRunScoringPattern  = regexp.MustCompile(`^(.+?)\s+(\d+)\s+run(\b.*)$`)
	footballDBPassScoringPattern = regexp.MustCompile(`^(.+?)\s+(\d+)\s+pass\s+from\s+(.+?)(\s+\(|$)`)
)

func normalizeFootballDBScoringDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	desc = footballDBFGScoringPattern.ReplaceAllString(desc, "$1 $2 yard field goal")
	desc = footballDBRunScoringPattern.ReplaceAllString(desc, "$1 $2 yard run$3")
	desc = footballDBPassScoringPattern.ReplaceAllString(desc, "$1 $2 yard pass from $3$4")
	desc = strings.ReplaceAll(desc, " Kick)", " kick)")
	desc = strings.ReplaceAll(desc, " Kick Failed)", " kick failed)")
	return desc
}

func buildFootballDBTeamStats(fdb *footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope) map[string]map[string]string {
	out := map[string]map[string]string{"vis": {}, "home": {}}
	if fdb == nil {
		return out
	}
	for _, table := range sortedFootballDBTables(fdb.Data.Tables) {
		if !isFootballDBTeamStatsTable(table) {
			continue
		}
		for _, row := range table.Rows {
			label := convertValueToString(row["col_0"])
			if label == "" {
				continue
			}
			if translated := footballDBTeamStatLabel(label); translated != "" {
				label = translated
			} else {
				continue
			}
			for _, side := range []string{"vis", "home"} {
				key := footballDBTeamStatColumn(table.Headers, env, side)
				if key == "" {
					continue
				}
				if value := convertValueToString(row[key]); value != "" {
					out[side][label] = value
				}
			}
		}
	}
	for _, side := range []string{"vis", "home"} {
		fillFootballDBDerivedTeamStats(out[side], fdb, env, side)
	}
	return out
}

func fillFootballDBDerivedTeamStats(stats map[string]string, fdb *footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope, side string) {
	if passing := footballDBTotalRow(fdb, env, side, "passing"); passing != nil {
		setIfBlank(stats, "Comp-Int-Yds-TD-INT", strings.Join([]string{
			convertValueToString(passing["cmp"]),
			convertValueToString(passing["att"]),
			convertValueToString(passing["yds"]),
			convertValueToString(passing["td"]),
			convertValueToString(passing["int"]),
		}, "-"))
		setIfBlank(stats, "Sacks-yardas perdidas", strings.Join([]string{
			convertValueToString(passing["sack"]),
			convertValueToString(passing["loss"]),
		}, "-"))
	}
	if rushing := footballDBTotalRow(fdb, env, side, "rushing"); rushing != nil {
		setIfBlank(stats, "Carreras-yardas-TD", strings.Join([]string{
			convertValueToString(rushing["att"]),
			convertValueToString(rushing["yds"]),
			convertValueToString(rushing["td"]),
		}, "-"))
	}
	if strings.TrimSpace(stats["Pérdidas de balón"]) == "" {
		turnovers := footballDBTurnovers(fdb, env, side)
		if turnovers != "" {
			stats["Pérdidas de balón"] = turnovers
		}
	}
}

func footballDBTurnovers(fdb *footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope, side string) string {
	total := 0
	found := false
	if passing := footballDBTotalRow(fdb, env, side, "passing"); passing != nil {
		if n, ok := atoiString(passing["int"]); ok {
			total += n
			found = true
		}
	}
	if fumbles := footballDBTotalRow(fdb, env, side, "fumbles"); fumbles != nil {
		if n, ok := atoiString(fumbles["lost"]); ok {
			total += n
			found = true
		}
	}
	if !found {
		return ""
	}
	return strconv.Itoa(total)
}

func footballDBTeamStatLabel(label string) string {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "first downs":
		return "Primeros downs"
	case "total net yards":
		return "Yardas totales"
	case "net yards passing":
		return "Yardas netas por pase"
	case "penalties - yards":
		return "Penalizaciones-yardas"
	case "fumbles - lost":
		return "Fumbles-perdidos"
	case "third downs":
		return "Conv. en tercer down"
	case "fourth downs":
		return "Conv. en cuarto down"
	case "time of possession":
		return "Tiempo de posesión"
	default:
		return ""
	}
}

func buildFootballDBPlayerStats(fdb *footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope) map[string]map[string][]TableRow {
	out := emptyPlayerStatsBySide()
	if fdb == nil {
		return out
	}
	raw := map[string]map[string][]map[string]any{"vis": {}, "home": {}}
	for _, table := range sortedFootballDBTables(fdb.Data.Tables) {
		side := footballDBTableSide(table, env)
		if side == "" {
			continue
		}
		category := footballDBTableCategory(table)
		if category == "" {
			continue
		}
		rows := normalizeFootballDBPlayerRows(table, category, strings.ToUpper(scoreboxSideAbbr(env, side)))
		if len(rows) == 0 {
			continue
		}
		switch category {
		case "passing", "rushing", "receiving", "offense_fumbles":
			raw[side]["player_offense"] = mergeRawPlayerRows(raw[side]["player_offense"], rows)
		case "defense", "defense_fumbles":
			raw[side]["player_defense"] = mergeRawPlayerRows(raw[side]["player_defense"], rows)
		case "kicking", "punting":
			raw[side]["kicking"] = mergeRawPlayerRows(raw[side]["kicking"], rows)
		case "kick_returns", "punt_returns":
			raw[side]["returns"] = mergeRawPlayerRows(raw[side]["returns"], rows)
		}
	}
	for _, side := range []string{"vis", "home"} {
		out[side][sectionTitle("offensive")] = rawRowsToTableRows(raw[side]["player_offense"], gameTableHeaderMap("player_offense"))
		out[side][sectionTitle("defensive")] = rawRowsToTableRows(raw[side]["player_defense"], gameTableHeaderMap("player_defense"))
		out[side][sectionTitle("special_teams")] = mergePlayerTables(
			rawRowsToTableRows(raw[side]["kicking"], gameTableHeaderMap("kicking")),
			rawRowsToTableRows(raw[side]["returns"], gameTableHeaderMap("returns")),
		)
	}
	return out
}

func emptyPlayerStatsBySide() map[string]map[string][]TableRow {
	return map[string]map[string][]TableRow{
		"vis":  {},
		"home": {},
	}
}

type footballDBTable struct {
	key string
	footballdb.TableResult
}

func sortedFootballDBTables(tables map[string]footballdb.TableResult) []footballDBTable {
	out := make([]footballDBTable, 0, len(tables))
	for key, table := range tables {
		out = append(out, footballDBTable{key: key, TableResult: table})
	}
	sort.Slice(out, func(i, j int) bool {
		return footballDBTableNumber(out[i].key) < footballDBTableNumber(out[j].key)
	})
	return out
}

var footballDBTableNumberPattern = regexp.MustCompile(`\d+`)

func footballDBTableNumber(key string) int {
	match := footballDBTableNumberPattern.FindString(key)
	if match == "" {
		return 0
	}
	n, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return n
}

func isFootballDBTeamStatsTable(table footballDBTable) bool {
	if len(table.Headers) < 3 {
		return false
	}
	if table.Headers[0] != "" {
		return false
	}
	for _, row := range table.Rows {
		if convertValueToString(row["col_0"]) != "" {
			return true
		}
	}
	return false
}

func footballDBTeamStatColumn(headers []string, env boxscore.BoxscoreEnvelope, side string) string {
	abbr := compactToken(scoreboxSideAbbr(env, side))
	name := compactToken(scoreboxSideName(env, side))
	for _, header := range headers {
		if header == "" {
			continue
		}
		token := compactToken(header)
		if abbr != "" && strings.Contains(token, abbr) {
			return header
		}
		if name != "" && strings.Contains(token, name) {
			return header
		}
	}
	return ""
}

func footballDBTableSide(table footballDBTable, env boxscore.BoxscoreEnvelope) string {
	if len(table.Headers) == 0 {
		return ""
	}
	playerKey := table.Headers[0]
	if playerKey == "" {
		return ""
	}
	token := compactToken(playerKey)
	for _, side := range []string{"vis", "home"} {
		abbr := compactToken(scoreboxSideAbbr(env, side))
		name := compactToken(scoreboxSideName(env, side))
		if abbr != "" && strings.Contains(token, abbr) {
			return side
		}
		if name != "" && strings.Contains(token, name) {
			return side
		}
	}
	return ""
}

func footballDBTableCategory(table footballDBTable) string {
	headers := map[string]bool{}
	for _, header := range table.Headers {
		headers[header] = true
	}
	n := footballDBTableNumber(table.key)
	switch {
	case headers["cmp"] && headers["rate"]:
		return "passing"
	case headers["rec"] && headers["tar"]:
		return "receiving"
	case headers["att"] && headers["fd"] && !headers["cmp"]:
		return "rushing"
	case headers["pat"] && headers["fg"]:
		return "kicking"
	case headers["punts"] && headers["net"]:
		return "punting"
	case headers["solo"] && headers["ast"] && headers["tot"] && headers["sack"]:
		return "defense"
	case headers["fum"] && headers["lost"]:
		return "offense_fumbles"
	case headers["num"] && headers["fc"] && n >= 10 && n <= 11:
		return "kick_returns"
	case headers["num"] && headers["fc"] && n >= 12 && n <= 13:
		return "punt_returns"
	default:
		return ""
	}
}

func normalizeFootballDBPlayerRows(table footballDBTable, category string, teamAbbr string) []map[string]any {
	if len(table.Headers) == 0 {
		return nil
	}
	playerKey := table.Headers[0]
	out := make([]map[string]any, 0, len(table.Rows))
	for _, row := range table.Rows {
		player := convertValueToString(row[playerKey])
		if player == "" || strings.EqualFold(player, "TOTAL") {
			continue
		}
		next := map[string]any{"player": player, "team": teamAbbr}
		switch category {
		case "passing":
			setMapped(next, row, map[string]string{"cmp": "pass_cmp", "att": "pass_att", "yds": "pass_yds", "td": "pass_td", "int": "pass_int", "lg": "pass_long", "sack": "pass_sacked", "loss": "pass_sacked_yds", "rate": "pass_rating"})
		case "rushing":
			setMapped(next, row, map[string]string{"att": "rush_att", "yds": "rush_yds", "td": "rush_td", "lg": "rush_long"})
		case "receiving":
			setMapped(next, row, map[string]string{"rec": "rec", "yds": "rec_yds", "td": "rec_td", "lg": "rec_long", "tar": "targets", "yac": "rec_yac"})
		case "offense_fumbles":
			setMapped(next, row, map[string]string{"fum": "fumbles", "lost": "fumbles_lost"})
		case "defense":
			setMapped(next, row, map[string]string{"int": "def_int", "yds": "def_int_yds", "lg": "def_int_long", "td": "def_int_td", "solo": "tackles_solo", "ast": "tackles_assists", "tot": "tackles_combined", "sack": "sacks"})
		case "defense_fumbles":
			setMapped(next, row, map[string]string{"ff": "fumbles_forced", "rec": "fumbles_rec", "yds": "fumbles_rec_yds", "td": "fumbles_rec_td"})
		case "kicking":
			splitMadeAttempt(next, convertValueToString(row["pat"]), "xpm", "xpa")
			splitMadeAttempt(next, convertValueToString(row["fg"]), "fgm", "fga")
			setMapped(next, row, map[string]string{"lg": "fg_long"})
		case "punting":
			setMapped(next, row, map[string]string{"punts": "punt", "yds": "punt_yds", "avg": "punt_yds_per_punt", "lg": "punt_long"})
		case "kick_returns":
			setMapped(next, row, map[string]string{"num": "kick_ret", "yds": "kick_ret_yds", "avg": "kick_ret_yds_per_ret", "lg": "kick_ret_long", "td": "kick_ret_td"})
		case "punt_returns":
			setMapped(next, row, map[string]string{"num": "punt_ret", "yds": "punt_ret_yds", "avg": "punt_ret_yds_per_ret", "lg": "punt_ret_long", "td": "punt_ret_td"})
		}
		out = append(out, next)
	}
	return out
}

func setMapped(dst map[string]any, src map[string]any, mapping map[string]string) {
	for from, to := range mapping {
		if value := convertValueToString(src[from]); value != "" {
			dst[to] = value
		}
	}
}

func splitMadeAttempt(dst map[string]any, raw string, madeKey string, attemptKey string) {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	if len(parts) != 2 {
		return
	}
	dst[madeKey] = parts[0]
	dst[attemptKey] = parts[1]
}

func mergeRawPlayerRows(primary []map[string]any, incoming []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(primary)+len(incoming))
	index := map[string]int{}
	for _, row := range primary {
		next := cloneRawRow(row)
		index[compactToken(convertValueToString(next["player"]))] = len(out)
		out = append(out, next)
	}
	for _, row := range incoming {
		key := compactToken(convertValueToString(row["player"]))
		if idx, ok := index[key]; ok {
			for k, v := range row {
				if strings.TrimSpace(convertValueToString(out[idx][k])) == "" && strings.TrimSpace(convertValueToString(v)) != "" {
					out[idx][k] = v
				}
			}
			continue
		}
		index[key] = len(out)
		out = append(out, cloneRawRow(row))
	}
	return out
}

func cloneRawRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	return out
}

func rawRowsToTableRows(rows []map[string]any, headerMap map[string]string) []TableRow {
	if len(rows) == 0 {
		return []TableRow{}
	}
	converted, err := ConvertTableToRows(rows, headerMap)
	if err != nil {
		return []TableRow{}
	}
	return converted
}

func footballDBTotalRow(fdb *footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope, side string, category string) map[string]any {
	for _, table := range sortedFootballDBTables(fdb.Data.Tables) {
		if footballDBTableSide(table, env) != side || footballDBTableCategory(table) != category {
			continue
		}
		if len(table.Headers) == 0 {
			continue
		}
		playerKey := table.Headers[0]
		for _, row := range table.Rows {
			if strings.EqualFold(convertValueToString(row[playerKey]), "TOTAL") {
				return row
			}
		}
	}
	return nil
}

func setIfBlank(stats map[string]string, key string, value string) {
	if strings.Trim(value, "- ") == "" {
		return
	}
	if strings.TrimSpace(stats[key]) == "" {
		stats[key] = value
	}
}

func atoiString(value any) (int, bool) {
	s := strings.TrimSpace(convertValueToString(value))
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.Split(s, ".")[0])
	return n, err == nil
}

func scoreboxSideName(env boxscore.BoxscoreEnvelope, side string) string {
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return ""
	}
	idx := 0
	if side == "home" {
		idx = 1
	}
	return linkContent(env.Data.Scorebox.Teams[idx].Name)
}

func scoreboxSideAbbr(env boxscore.BoxscoreEnvelope, side string) string {
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return ""
	}
	idx := 0
	if side == "home" {
		idx = 1
	}
	return env.Data.Scorebox.Teams[idx].Abbr
}
