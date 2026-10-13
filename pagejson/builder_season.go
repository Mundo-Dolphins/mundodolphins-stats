package pagejson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/gamemd"
)

// SeasonBuilder builds a SectionedPage for a season page.
type SeasonBuilder struct {
	processedDir string
	now          time.Time
}

func NewSeasonBuilder(processedDir string) *SeasonBuilder {
	return &SeasonBuilder{processedDir: processedDir, now: time.Now().UTC()}
}

func (b *SeasonBuilder) SetTime(t time.Time) *SeasonBuilder {
	b.now = t
	return b
}

func (b *SeasonBuilder) BuildFromYear(year int) (*SectionedPage, error) {
	return b.BuildFromFile(filepath.Join(b.processedDir, fmt.Sprintf("%d.json", year)))
}

func (b *SeasonBuilder) BuildFromFile(filePath string) (*SectionedPage, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read season file %s: %w", filePath, err)
	}
	var seasonData map[string]interface{}
	if err := json.Unmarshal(data, &seasonData); err != nil {
		return nil, fmt.Errorf("parse season JSON: %w", err)
	}
	return b.Build(seasonData)
}

func (b *SeasonBuilder) Build(seasonData map[string]interface{}) (*SectionedPage, error) {
	year, err := extractSeasonYear(seasonData)
	if err != nil {
		return nil, err
	}

	page := NewSectionedPageWithTime(
		PageKindSeason,
		fmt.Sprintf(sectionTitle("season_title_fmt"), year),
		fmt.Sprintf("%d-01-01", year),
		fmt.Sprintf("%d", year),
		fmt.Sprintf("historia/%d", year),
		map[string]interface{}{"year": year},
		b.now,
	)

	if err := b.buildResumenSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build resumen: %w", err)
	}
	if err := b.buildRosterSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build roster: %w", err)
	}
	if err := b.buildScheduleSections(page, seasonData, year); err != nil {
		return nil, fmt.Errorf("build schedule sections: %w", err)
	}
	b.buildSeasonStatsLinkSection(page, year)
	if err := b.buildDraftSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build draft: %w", err)
	}
	if err := b.buildLeadersSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build leaders: %w", err)
	}
	if err := b.buildInjuriesReservesSection(page, seasonData); err != nil {
		return nil, fmt.Errorf("build injuries/reserves: %w", err)
	}

	// Checklist coverage (season page): roster, schedule regular, playoffs, season summary,
	// leaders, injuries/reserves when available. "Preseason Odds" is intentionally excluded.
	return page, nil
}

func (b *SeasonBuilder) buildResumenSection(page *SectionedPage, seasonData map[string]interface{}) error {
	main := bestSeasonSummaryPayload(seasonData)
	if len(main) == 0 {
		return nil
	}

	info := mapStringAny(main["info"])
	row := NewTableRow()

	// Division — prefer structured field, fall back to parsing Record.
	if divName, ok := main["division_name"].(string); ok && divName != "" {
		row.Set(fieldLabel("division"), divName)
	}

	// Record & rank.
	if rec, ok := info["Record"].(string); ok && rec != "" {
		record, rank := parseRecordField(rec)
		row.Set(fieldLabel("record"), record)
		if rank != "" {
			row.Set(fieldLabel("position"), rank)
		}
	}
	// Override rank from structured field if available.
	if divRank, ok := main["division_rank"].(string); ok && divRank != "" {
		row.Set(fieldLabel("position"), divRank)
	}

	// Points.
	if pf, ok := info["Points For"].(string); ok && pf != "" {
		row.Set(fieldLabel("points_for"), formatPoints(pf))
	}
	if pa, ok := info["Points Against"].(string); ok && pa != "" {
		row.Set(fieldLabel("points_against"), formatPoints(pa))
	}

	// Coach.
	if coach, ok := info["Coach"].(string); ok && coach != "" {
		row.Set(fieldLabel("head_coach"), coach)
	}

	// Front office.
	if gm := firstNonEmptyInfo(info, "General Manager", "GM"); gm != "" {
		row.Set(fieldLabel("general_manager"), gm)
	}
	if interimGM := firstNonEmptyInfo(info, "Interim General Manager"); interimGM != "" {
		row.Set(fieldLabel("interim_general_manager"), interimGM)
	}
	hasStructuredPrincipal := len(mapStringAny(main["principal"])) > 0
	if owner := firstNonEmptyInfo(info, "Owner", "Chairman", "Chairman/Managing General Partner", "Managing General Partner"); owner != "" && !hasStructuredPrincipal {
		row.Set(fieldLabel("owner"), owner)
	}

	// Stadium.
	if stadium, ok := info["Stadium"].(string); ok && stadium != "" {
		row.Set(fieldLabel("stadium"), stadium)
	}

	// Training Camp.
	if tc, ok := info["Training Camp"].(string); ok && tc != "" {
		row.Set(fieldLabel("training_camp"), tc)
	}

	// Defensive alignment.
	if da, ok := info["Defensive Alignment"].(string); ok && da != "" {
		row.Set(fieldLabel("defensive_alignment"), da)
	}

	// Offensive scheme.
	if os, ok := info["Offensive Scheme"].(string); ok && os != "" {
		row.Set(fieldLabel("offensive_scheme"), os)
	}

	// SRS.
	if srs, ok := info["SRS"].(string); ok && srs != "" {
		row.Set(fieldLabel("srs"), srs)
	}

	// Expected W-L.
	if ewl, ok := info["Expected W-L"].(string); ok && ewl != "" {
		row.Set(fieldLabel("expected_wl"), ewl)
	}

	// Principal (owner/GM).
	if principal := mapStringAny(main["principal"]); len(principal) > 0 {
		label := convertValueToString(principal["label"])
		value := convertValueToString(principal["value"])
		if label != "" && value != "" {
			row.Set(label, value)
		}
	}

	// Coaching staff.
	if staff, ok := main["staff"].([]interface{}); ok {
		for _, raw := range staff {
			m := mapStringAny(raw)
			role := convertValueToString(m["role"])
			name := convertValueToString(m["name"])
			if role != "" && name != "" {
				row.Set(role, name)
			}
		}
	}

	if len(row.Keys()) > 0 {
		page.AddSection(sectionTitle("resumen"), row)
	}
	return nil
}

func bestSeasonSummaryPayload(seasonData map[string]interface{}) map[string]interface{} {
	data := nestedMap(seasonData, "data")
	main := mapStringAny(data["main"])
	if seasonSummaryPayloadHasData(main) {
		return main
	}

	for _, key := range []string{
		"roster",
		"gamelog",
		"advanced",
		"draft",
		"injuries",
		"off_splits",
		"opp_splits",
		"travel",
		"snap_counts",
		"trades",
	} {
		candidate := mapStringAny(data[key])
		if seasonSummaryPayloadHasData(candidate) {
			return mergeSeasonSummaryPayload(main, candidate)
		}
	}

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		candidate := mapStringAny(data[key])
		if seasonSummaryPayloadHasData(candidate) {
			return mergeSeasonSummaryPayload(main, candidate)
		}
	}

	return main
}

func seasonSummaryPayloadHasData(payload map[string]interface{}) bool {
	if len(payload) == 0 {
		return false
	}
	if len(mapStringAny(payload["info"])) > 0 {
		return true
	}
	if len(mapStringAny(payload["principal"])) > 0 {
		return true
	}
	if staff, ok := payload["staff"].([]interface{}); ok && len(staff) > 0 {
		return true
	}
	return convertValueToString(payload["division_name"]) != "" ||
		convertValueToString(payload["division_rank"]) != ""
}

func mergeSeasonSummaryPayload(primary, fallback map[string]interface{}) map[string]interface{} {
	if len(primary) == 0 {
		return fallback
	}
	if len(fallback) == 0 {
		return primary
	}

	merged := make(map[string]interface{}, len(primary)+len(fallback))
	for key, value := range fallback {
		merged[key] = value
	}
	for key, value := range primary {
		if key == "info" {
			merged["info"] = mergeInfoMaps(mapStringAny(primary["info"]), mapStringAny(fallback["info"]))
			continue
		}
		if summaryValueIsEmpty(value) {
			continue
		}
		merged[key] = value
	}

	return merged
}

func mergeInfoMaps(primary, fallback map[string]interface{}) map[string]interface{} {
	if len(primary) == 0 {
		return fallback
	}
	if len(fallback) == 0 {
		return primary
	}
	merged := make(map[string]interface{}, len(primary)+len(fallback))
	for key, value := range fallback {
		merged[key] = value
	}
	for key, value := range primary {
		if convertValueToString(value) == "" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func summaryValueIsEmpty(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

func firstNonEmptyInfo(info map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := convertValueToString(info[key]); value != "" {
			return value
		}
	}
	return ""
}

// parseRecordField extracts the win-loss record and rank from a Record string
// like "4-10-0, 3rd in AFL East Division (Schedule and Results)".
func parseRecordField(rec string) (record, rank string) {
	// Split on comma to separate record from division info.
	parts := strings.SplitN(rec, ",", 2)
	record = strings.TrimSpace(parts[0])
	if len(parts) < 2 {
		return record, ""
	}
	rest := strings.TrimSpace(parts[1])
	// Extract rank: e.g. "3rd in AFL East Division ..."
	if idx := strings.Index(rest, " in "); idx > 0 {
		rank = strings.TrimSpace(rest[:idx])
	}
	return record, rank
}

// formatPoints reformats "219 (15.6/g) 9th of 9" → "219 puntos. 9º de 9 equipos".
func formatPoints(raw string) string {
	raw = strings.TrimSpace(raw)
	// Expected format: "219 (15.6/g) 9th of 9" or just "219".
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return raw
	}
	points := parts[0]

	// Find the rank part: skip past "(xx/g)" if present.
	rankIdx := -1
	for i, p := range parts {
		if strings.HasSuffix(p, ")") {
			rankIdx = i + 1
			break
		}
	}
	if rankIdx < 0 || rankIdx >= len(parts) {
		return points + " " + fieldLabel("points_text")
	}

	// rankParts should be e.g. ["9th", "of", "9"]
	rankParts := parts[rankIdx:]
	if len(rankParts) < 3 || !strings.EqualFold(rankParts[1], "of") {
		return points + " " + fieldLabel("points_text")
	}
	ordinal := rankParts[0]
	total := rankParts[2]
	// Convert English ordinal to Spanish: "9th" → "9º"
	num := strings.TrimRight(ordinal, "stndrh")
	return fmt.Sprintf(fieldLabel("points_ranked_fmt"), points, num, total)
}

func (b *SeasonBuilder) buildRosterSection(page *SectionedPage, seasonData map[string]interface{}) error {
	rows := nestedValue(seasonData, "data", "roster", "tables", "roster", "rows")
	if rows == nil {
		return nil
	}
	mapped, err := ConvertTableToRows(rows, seasonMainTableHeaderMap("roster"))
	if err != nil {
		return fmt.Errorf("convert roster table: %w", err)
	}
	if len(mapped) > 0 {
		page.AddSection(sectionTitle("roster"), mapped)
	}
	return nil
}

func (b *SeasonBuilder) buildDraftSection(page *SectionedPage, seasonData map[string]interface{}) error {
	rows := nestedValue(seasonData, "data", "draft", "tables", "draft", "rows")
	if rows == nil {
		return nil
	}
	mapped, err := ConvertTableToRows(rows, seasonMainTableHeaderMap("draft"))
	if err != nil {
		return fmt.Errorf("convert draft table: %w", err)
	}
	if len(mapped) > 0 {
		page.AddSection(sectionTitle("draft"), mapped)
	}
	return nil
}

func (b *SeasonBuilder) buildSeasonStatsLinkSection(page *SectionedPage, year int) {
	page.AddSection(sectionTitle("season_stats_link"), []map[string]string{
		{
			"title": sectionTitle("season_stats_link"),
			"id":    fmt.Sprintf("%d", year),
			"url":   fmt.Sprintf("/historia/stats/%d", year),
		},
	})
}

func (b *SeasonBuilder) buildScheduleSections(page *SectionedPage, seasonData map[string]interface{}, year int) error {
	rowsRaw := nestedValue(seasonData, "data", "main", "tables", "games", "rows")
	byGameID := make(map[string]GameLink)
	byID := make(map[string]GameLink)

	if rowsRaw != nil {
		rows, ok := rowsRaw.([]interface{})
		if !ok {
			return fmt.Errorf("main.games.rows has unexpected type %T", rowsRaw)
		}
		for _, raw := range rows {
			m := mapStringAny(raw)
			gl, ok := seasonGameLinkFromMainGamesRow(m)
			if !ok {
				continue
			}
			addSeasonGameLink(byGameID, byID, gl)
		}
	}

	loaded, err := b.loadSeasonGameLinks(year)
	if err != nil {
		return err
	}
	for _, gl := range loaded {
		addSeasonGameLink(byGameID, byID, gl)
	}

	all := make([]GameLink, 0, len(byGameID)+len(byID))
	seenIDs := map[string]bool{}
	for key, gl := range byGameID {
		all = append(all, gl)
		seenIDs[key] = true
	}
	for key, gl := range byID {
		if seenIDs[key] {
			continue
		}
		all = append(all, gl)
	}
	sort.Slice(all, func(i, j int) bool {
		return compareSeasonGameLinks(all[i], all[j])
	})

	regular := make([]GameLink, 0)
	playoffs := make([]GameLink, 0)
	for _, gl := range all {
		switch gl.GameType {
		case "regular_season":
			regular = append(regular, gl)
		case "postseason":
			playoffs = append(playoffs, gl)
		}
	}

	if len(all) > 0 {
		page.AddSection(sectionTitle("season_games"), all)
	}
	if len(regular) > 0 {
		page.AddSection(sectionTitle("regular_season_games"), regular)
	}
	if len(playoffs) > 0 {
		page.AddSection(sectionTitle("playoffs"), playoffs)
	}
	return nil
}

func (b *SeasonBuilder) buildLeadersSection(page *SectionedPage, seasonData map[string]interface{}) error {
	mainTables := nestedMap(seasonData, "data", "main", "tables")
	if len(mainTables) == 0 {
		return nil
	}

	leaders := map[string]interface{}{}
	for _, spec := range []struct {
		key   string
		label string
	}{
		{"passing", sectionTitle("passing")},
		{"rushing_and_receiving", sectionTitle("rushing_receiving")},
		{"defense", sectionTitle("defense")},
		{"kicking", sectionTitle("kicking")},
		{"returns", sectionTitle("returns")},
		{"scoring", sectionTitle("scoring")},
	} {
		rows := nestedValue(mainTables, spec.key, "rows")
		if rows == nil {
			continue
		}
		converted, err := ConvertTableToRows(rows, seasonMainTableHeaderMap(spec.key))
		if err != nil {
			return fmt.Errorf("convert leaders table %s: %w", spec.key, err)
		}
		if len(converted) > 0 {
			leaders[spec.label] = converted
		}
	}

	if len(leaders) > 0 {
		page.AddSection(sectionTitle("leaders"), leaders)
	}
	return nil
}

func (b *SeasonBuilder) buildInjuriesReservesSection(page *SectionedPage, seasonData map[string]interface{}) error {
	injTables := nestedMap(seasonData, "data", "injuries", "tables")
	if len(injTables) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	for _, spec := range []struct {
		key   string
		label string
	}{
		{"team_injuries", sectionTitle("injuries")},
		{"team_injuries_totals", sectionTitle("injuries_summary")},
	} {
		rows := nestedValue(injTables, spec.key, "rows")
		if rows == nil {
			continue
		}
		converted, err := ConvertTableToRows(rows, nil)
		if err != nil {
			return fmt.Errorf("convert injuries table %s: %w", spec.key, err)
		}
		if len(converted) > 0 {
			out[spec.label] = converted
		}
	}
	if len(out) > 0 {
		page.AddSection(sectionTitle("injuries_reserves"), out)
	}
	return nil
}

func addSeasonGameLink(byGameID map[string]GameLink, byID map[string]GameLink, gl GameLink) {
	if gl.GameID != "" {
		byGameID[gl.GameID] = gl
		return
	}
	if gl.ID != "" {
		byID[gl.ID] = gl
	}
}

func (b *SeasonBuilder) loadSeasonGameLinks(year int) ([]GameLink, error) {
	if strings.TrimSpace(b.processedDir) == "" {
		return nil, nil
	}
	pattern := filepath.Join(b.processedDir, "games", fmt.Sprintf("%d", year), "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("find processed games for %d: %w", year, err)
	}
	out := make([]GameLink, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read processed game %s: %w", file, err)
		}
		var env boxscore.BoxscoreEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("parse processed game %s: %w", file, err)
		}
		if env.GameID == "" {
			env.GameID = strings.TrimSuffix(filepath.Base(file), ".json")
		}
		gl, ok := seasonGameLinkFromBoxscoreEnvelope(env)
		if ok {
			out = append(out, gl)
		}
	}
	return out, nil
}

func seasonGameLinkFromMainGamesRow(row map[string]interface{}) (GameLink, bool) {
	box := mapStringAny(row["boxscore_word"])
	boxURL := convertValueToString(box["link"])
	if boxURL == "" {
		return GameLink{}, false // bye week or no loaded game page
	}
	dateISO, err := dateISOFromBoxscoreLink(boxURL)
	if err != nil {
		return GameLink{}, false
	}
	gameID := gameIDFromBoxscoreLink(boxURL)

	oppName := convertValueToString(row["opp"])
	if oppName == "" {
		return GameLink{}, false
	}
	miamiAway := strings.TrimSpace(convertValueToString(row["game_location"])) == "@"

	visitor := oppName
	home := "Miami Dolphins"
	if miamiAway {
		visitor = "Miami Dolphins"
		home = oppName
	}
	url := fmt.Sprintf("/historia/partidos/%s/", gamemd.BuildSlug(visitor, home, dateISO))
	id := pathLastSegment(url)
	week := normalizeGameWeek(convertValueToString(row["week_num"]))
	gameType := gameTypeFromWeek(week)
	if gameType == "" {
		return GameLink{}, false
	}

	return GameLink{
		Title:    fmt.Sprintf("Miami Dolphins vs %s", oppName),
		ID:       id,
		GameID:   gameID,
		Fecha:    formatDDMMYYYY(dateISO),
		Date:     dateISO,
		URL:      url,
		Stage:    seasonGameStage(week, gameType),
		Week:     week,
		GameType: gameType,
		Status:   "final",
	}, true
}

func seasonGameLinkFromBoxscoreEnvelope(env boxscore.BoxscoreEnvelope) (GameLink, bool) {
	gameID := strings.TrimSpace(env.GameID)
	week := normalizeGameWeek(env.Data.Week)
	gameType := gameTypeFromWeek(week)
	if gameID == "" || week == "" || gameType == "" {
		return GameLink{}, false
	}
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return GameLink{}, false
	}

	visitorName := linkContent(env.Data.Scorebox.Teams[0].Name)
	homeName := linkContent(env.Data.Scorebox.Teams[1].Name)
	if !isRealTeamName(visitorName) || !isRealTeamName(homeName) {
		return GameLink{}, false
	}

	dateISO := normalizeGameDate(env.Data.Scorebox.Meta.Date, gameID)
	if dateISO == "" {
		return GameLink{}, false
	}
	slug := gamemd.BuildSlug(visitorName, homeName, dateISO)
	return GameLink{
		Title:    fmt.Sprintf("%s vs %s", visitorName, homeName),
		ID:       slug,
		GameID:   gameID,
		Fecha:    formatDDMMYYYY(dateISO),
		Date:     dateISO,
		URL:      fmt.Sprintf("/historia/partidos/%s/", slug),
		Stage:    seasonGameStage(week, gameType),
		Week:     week,
		GameType: gameType,
		Status:   gameStatusFromEnvelope(env),
	}, true
}

func isRealTeamName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && !strings.EqualFold(name, "Rival")
}

func seasonGameStage(week, gameType string) string {
	switch gameType {
	case "preseason":
		n := strings.TrimSpace(strings.TrimPrefix(week, "Preseason"))
		if n != "" {
			return "Pretemporada " + n
		}
		return "Pretemporada"
	case "regular_season":
		return "Semana " + strings.TrimSpace(week)
	case "postseason":
		return "Playoffs"
	default:
		return week
	}
}

func compareSeasonGameLinks(a, b GameLink) bool {
	ai, bi := gameSortKey(a), gameSortKey(b)
	if ai != bi {
		return ai < bi
	}
	if a.Date != b.Date {
		return a.Date < b.Date
	}
	return a.ID < b.ID
}

func gameSortKey(gl GameLink) int {
	switch gl.GameType {
	case "preseason":
		return gameWeekNumber(gl.Week)
	case "regular_season":
		return 100 + gameWeekNumber(gl.Week)
	case "postseason":
		return 200 + gameWeekNumber(gl.Week)
	default:
		return 300
	}
}

func gameWeekNumber(week string) int {
	fields := strings.Fields(strings.TrimSpace(week))
	raw := week
	if len(fields) > 0 {
		raw = fields[len(fields)-1]
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err == nil {
		return n
	}
	return 0
}

func extractSeasonYear(seasonData map[string]interface{}) (int, error) {
	season, ok := seasonData["season"]
	if !ok {
		return 0, fmt.Errorf("missing 'season' field")
	}
	switch v := season.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		var year int
		if _, err := fmt.Sscanf(v, "%d", &year); err != nil {
			return 0, fmt.Errorf("invalid season format: %s", v)
		}
		return year, nil
	default:
		return 0, fmt.Errorf("unexpected season type: %T", season)
	}
}

func nestedMap(root map[string]interface{}, path ...string) map[string]interface{} {
	cur := any(root)
	for _, key := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = m[key]
	}
	m, _ := cur.(map[string]interface{})
	return m
}

func nestedValue(root interface{}, path ...string) interface{} {
	cur := root
	for _, key := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func mapStringAny(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func extractString(m map[string]interface{}, key string) string {
	return convertValueToString(m[key])
}

func dateISOFromBoxscoreLink(link string) (string, error) {
	// /boxscores/202509070clt.htm -> 2025-09-07
	idx := strings.Index(link, "/boxscores/")
	if idx < 0 {
		return "", fmt.Errorf("invalid boxscore link: %s", link)
	}
	base := strings.TrimPrefix(link[idx:], "/boxscores/")
	if len(base) < 8 {
		return "", fmt.Errorf("boxscore link too short: %s", link)
	}
	raw := base[:8]
	return fmt.Sprintf("%s-%s-%s", raw[0:4], raw[4:6], raw[6:8]), nil
}

func gameIDFromBoxscoreLink(link string) string {
	idx := strings.Index(link, "/boxscores/")
	if idx < 0 {
		return ""
	}
	base := strings.TrimPrefix(link[idx:], "/boxscores/")
	base = strings.TrimSuffix(base, ".htm")
	if slash := strings.Index(base, "/"); slash >= 0 {
		base = base[:slash]
	}
	return base
}

func formatDDMMYYYY(dateISO string) string {
	parts := strings.Split(dateISO, "-")
	if len(parts) != 3 {
		return dateISO
	}
	return fmt.Sprintf("%s-%s-%s", parts[2], parts[1], parts[0])
}

func pathLastSegment(url string) string {
	trimmed := strings.Trim(url, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}

func SortSections(sections map[string]interface{}) []string {
	order := []string{
		sectionTitle("resumen"),
		sectionTitle("roster"),
		sectionTitle("season_games"),
		sectionTitle("regular_season_games"),
		sectionTitle("playoffs"),
		sectionTitle("draft"),
		sectionTitle("season_stats_link"),
		sectionTitle("leaders"),
		sectionTitle("injuries_reserves"),
		sectionTitle("season_stats_link"),
	}
	keys := make([]string, 0, len(sections))
	for k := range sections {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ii, jj := indexOf(order, keys[i]), indexOf(order, keys[j])
		if ii >= 0 && jj >= 0 {
			return ii < jj
		}
		if ii >= 0 {
			return true
		}
		if jj >= 0 {
			return false
		}
		return keys[i] < keys[j]
	})
	return keys
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
