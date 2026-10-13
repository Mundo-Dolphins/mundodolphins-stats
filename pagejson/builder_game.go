package pagejson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/footballdb"
	"github.com/Mundo-Dolphins/mundodolphins-stats/gamemd"
	"github.com/Mundo-Dolphins/mundodolphins-stats/i18n"
	"github.com/Mundo-Dolphins/mundodolphins-stats/nflcom"
	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
	videospkg "github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

// GamebookParser extracts the data of an official game book PDF.
//
// The library ships no implementation: reading a PDF published by a third
// party is the caller's responsibility. Without a parser, the game book data
// that is already stored in the processed JSON is used as is.
type GamebookParser interface {
	ParseGamebook(path string) (*nflcom.GamebookData, error)
}

// GameBuilder builds a SectionedPage for a game page.
type GameBuilder struct {
	gamebookParser     GamebookParser
	processedDir       string
	videosCache        string
	videosCacheLoaded  bool
	videosCacheFile    *videospkg.CacheFile
	videosCacheLoadErr error
	now                time.Time
}

func NewGameBuilder(processedDir, videosCache string) *GameBuilder {
	return &GameBuilder{processedDir: processedDir, videosCache: videosCache, now: time.Now().UTC()}
}

// SetGamebookParser sets the parser used to read game book PDFs.
func (g *GameBuilder) SetGamebookParser(p GamebookParser) *GameBuilder {
	g.gamebookParser = p
	return g
}

func (g *GameBuilder) SetTime(t time.Time) *GameBuilder {
	g.now = t
	return g
}

func (g *GameBuilder) BuildFromGameID(gameID string) (*SectionedPage, error) {
	if len(gameID) < 8 {
		return nil, fmt.Errorf("invalid game id %q", gameID)
	}
	season, err := util.NFLSeasonFromGameID(gameID)
	if err != nil {
		return nil, fmt.Errorf("determine season for game %s: %w", gameID, err)
	}
	return g.BuildFromFile(filepath.Join(g.processedDir, "games", fmt.Sprintf("%d", season), gameID+".json"), gameID)
}

func (g *GameBuilder) BuildFromFile(filePath, gameID string) (*SectionedPage, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read game file %s: %w", filePath, err)
	}
	var env boxscore.BoxscoreEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		var raw map[string]interface{}
		if rawErr := json.Unmarshal(data, &raw); rawErr != nil {
			return nil, fmt.Errorf("parse game JSON: %w", err)
		}
		return g.Build(raw, gameID)
	}
	if env.GameID == "" {
		env.GameID = gameID
	}
	return g.BuildEnvelope(env)
}

func (g *GameBuilder) Build(gameData map[string]interface{}, gameID string) (*SectionedPage, error) {
	raw, err := json.Marshal(gameData)
	if err != nil {
		return nil, fmt.Errorf("marshal input game data: %w", err)
	}
	var env boxscore.BoxscoreEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return g.buildLooseMap(gameData, gameID), nil
	}
	if env.GameID == "" {
		env.GameID = gameID
	}
	return g.BuildEnvelope(env)
}

func (g *GameBuilder) BuildEnvelope(env boxscore.BoxscoreEnvelope) (*SectionedPage, error) {
	if err := g.applyVideosCache(&env); err != nil {
		return nil, fmt.Errorf("apply videos cache: %w", err)
	}
	g.applyNFLComSupplement(&env)

	var visitor, home boxscore.ScoreboxTeam
	var visitorName, homeName string
	gameDate := normalizeGameDate("", env.GameID)
	if env.Data.Scorebox != nil {
		gameDate = normalizeGameDate(env.Data.Scorebox.Meta.Date, env.GameID)
		if len(env.Data.Scorebox.Teams) > 0 {
			visitor = env.Data.Scorebox.Teams[0]
			visitorName = linkContent(visitor.Name)
		}
		if len(env.Data.Scorebox.Teams) > 1 {
			home = env.Data.Scorebox.Teams[1]
			homeName = linkContent(home.Name)
		}
	}
	slug := env.GameID
	title := fmt.Sprintf(sectionTitle("game_title_fmt"), env.GameID)
	if gameDate != "" && visitorName != "" && homeName != "" {
		slug = gamemd.BuildSlug(visitorName, homeName, gameDate)
		title = fmt.Sprintf("%s vs %s", visitorName, homeName)
	}

	page := NewSectionedPageWithTime(
		PageKindGame,
		title,
		gameDate,
		slug,
		fmt.Sprintf("historia/partidos/%s", slug),
		map[string]interface{}{"game_id": env.GameID},
		g.now,
	)

	if err := g.buildInfoSection(page, env); err != nil {
		return nil, fmt.Errorf("build info: %w", err)
	}
	if err := g.buildVideosSection(page, env); err != nil {
		return nil, fmt.Errorf("build videos: %w", err)
	}
	supp := g.loadGameStatSupplement(env)
	if err := g.buildAnotacionesSection(page, env, supp); err != nil {
		return nil, fmt.Errorf("build anotaciones: %w", err)
	}
	if err := g.buildEquiposSection(page, env, supp); err != nil {
		return nil, fmt.Errorf("build equipos: %w", err)
	}
	if err := g.buildOficialesSection(page, env); err != nil {
		return nil, fmt.Errorf("build oficiales: %w", err)
	}
	if err := g.buildDrivesSection(page, env); err != nil {
		return nil, fmt.Errorf("build drives: %w", err)
	}
	if err := g.buildSnapCountsSection(page, env); err != nil {
		return nil, fmt.Errorf("build snap counts: %w", err)
	}
	if err := g.buildAdvancedStatsSection(page, env); err != nil {
		return nil, fmt.Errorf("build advanced stats: %w", err)
	}

	// Checklist coverage (game page): info + linescore + scoring summary + team stats + player stats + videos
	// + officials + drives + snap counts + advanced stats.
	return page, nil
}

func (g *GameBuilder) applyNFLComSupplement(env *boxscore.BoxscoreEnvelope) {
	if g == nil || env == nil || env.Data.NFLCom == nil || env.Data.NFLCom.GamebookData != nil {
		return
	}
	supp := g.loadNFLComGame(*env)
	if supp != nil {
		mergeNFLComGameData(env.Data.NFLCom, supp)
	}
	if env.Data.NFLCom.GamebookData != nil {
		return
	}
	if env.Data.NFLCom.Gamebook != nil && g.gamebookParser != nil {
		if gamebook, err := g.gamebookParser.ParseGamebook(env.Data.NFLCom.Gamebook.Path); err == nil && gamebook != nil {
			env.Data.NFLCom.GamebookData = gamebook
		}
	}
}

func (g *GameBuilder) loadNFLComGame(env boxscore.BoxscoreEnvelope) *nflcom.GameData {
	if strings.TrimSpace(g.processedDir) == "" || env.Data.NFLCom == nil {
		return nil
	}
	season, err := util.NFLSeasonFromGameID(env.GameID)
	if err != nil {
		return nil
	}
	slug := strings.TrimSpace(env.Data.NFLCom.Slug)
	if slug == "" {
		slug = nflcom.SlugFromGameURL(env.Data.NFLCom.URL)
	}
	if slug == "" || slug == "unknown" {
		return nil
	}
	path := filepath.Join(g.processedDir, "nflcom", "games", fmt.Sprintf("%d", season), slug+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var envelope nflcom.Envelope[nflcom.GameData]
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	return &envelope.Data
}

func mergeNFLComGameData(dst *nflcom.GameData, src *nflcom.GameData) {
	if dst == nil || src == nil {
		return
	}
	if dst.Slug == "" {
		dst.Slug = src.Slug
	}
	if dst.URL == "" {
		dst.URL = src.URL
	}
	if dst.Gamebook == nil {
		dst.Gamebook = src.Gamebook
	}
	if dst.GamebookData == nil {
		dst.GamebookData = src.GamebookData
	}
	if dst.Stats == nil {
		dst.Stats = src.Stats
	}
	if len(dst.Links) == 0 {
		dst.Links = src.Links
	}
	if dst.PageTitle == "" {
		dst.PageTitle = src.PageTitle
	}
	if len(dst.RawHeadings) == 0 {
		dst.RawHeadings = src.RawHeadings
	}
}

func (g *GameBuilder) applyVideosCache(env *boxscore.BoxscoreEnvelope) error {
	if g == nil || env == nil {
		return nil
	}
	if strings.TrimSpace(g.videosCache) == "" || strings.TrimSpace(env.GameID) == "" {
		return nil
	}

	cf, err := g.loadVideosCache()
	if err != nil {
		return err
	}
	if cf == nil || cf.Items == nil {
		return nil
	}
	if _, ok := cf.Items[env.GameID]; !ok {
		return nil
	}

	env.Data.YouTube = videospkg.BuildYouTubeBlock(cf, env.GameID)
	return nil
}

func (g *GameBuilder) loadVideosCache() (*videospkg.CacheFile, error) {
	if g.videosCacheLoaded {
		return g.videosCacheFile, g.videosCacheLoadErr
	}
	g.videosCacheLoaded = true
	g.videosCacheFile, g.videosCacheLoadErr = videospkg.NewFileCache(strings.TrimSpace(g.videosCache)).Load()
	return g.videosCacheFile, g.videosCacheLoadErr
}

func (g *GameBuilder) buildLooseMap(gameData map[string]interface{}, gameID string) *SectionedPage {
	date := normalizeGameDate("", gameID)
	title := fmt.Sprintf(sectionTitle("game_title_fmt"), gameID)
	if scorebox := nestedMap(gameData, "data", "scorebox"); len(scorebox) > 0 {
		if teams, ok := scorebox["teams"].([]interface{}); ok && len(teams) >= 2 {
			a := convertValueToString(mapStringAny(teams[0])["name"])
			b := convertValueToString(mapStringAny(teams[1])["name"])
			if a != "" && b != "" {
				title = fmt.Sprintf("%s vs %s", a, b)
			}
		}
	}
	page := NewSectionedPageWithTime(
		PageKindGame,
		title,
		date,
		gameID,
		fmt.Sprintf("historia/partidos/%s", gameID),
		map[string]interface{}{"game_id": gameID},
		g.now,
	)
	page.AddSection(sectionTitle("game_info"), NewTableRow())
	page.AddSection(sectionTitle("game_videos"), []VideoInfo{})
	page.AddSection(sectionTitle("anotaciones"), map[string]interface{}{"linescore": []TableRow{}, "scoring_summary": []string{}})
	page.AddSection(sectionTitle("equipos"), map[string]interface{}{})
	page.AddSection(sectionTitle("oficiales"), []TableRow{})
	page.AddSection(sectionTitle("series"), map[string]interface{}{})
	page.AddSection(sectionTitle("snap_counts"), map[string]interface{}{})
	page.AddSection(sectionTitle("advanced_stats"), map[string]interface{}{})
	return page
}

func (g *GameBuilder) buildVideosSection(page *SectionedPage, env boxscore.BoxscoreEnvelope) error {
	videos := make([]VideoInfo, 0)
	if !videospkg.IsPublishableStatus(env.Data.YouTube.Status) {
		page.AddSection(sectionTitle("game_videos"), videos)
		return nil
	}
	for _, v := range env.Data.YouTube.Videos {
		videos = append(videos, VideoInfo{
			YouTubeID:       v.ID,
			Title:           v.Title,
			URL:             v.URL,
			DurationSeconds: v.DurationSeconds,
			Embeddable:      v.Embeddable,
		})
	}
	page.AddSection(sectionTitle("game_videos"), videos)
	return nil
}

func (g *GameBuilder) buildAnotacionesSection(page *SectionedPage, env boxscore.BoxscoreEnvelope, supp gameStatSupplement) error {
	anotaciones := map[string]interface{}{}
	if env.Data.Linescore != nil && len(env.Data.Linescore.Teams) >= 2 {
		lineRows := make([]TableRow, 0, len(env.Data.Linescore.Teams))
		for _, t := range env.Data.Linescore.Teams {
			row := NewTableRow()
			row.Set(fieldLabel("team"), linkContent(t.Team))
			for _, q := range []string{"1", "2", "3", "4"} {
				row.Set(q, fmt.Sprintf("%d", t.Quarters[q]))
			}
			row.Set(fieldLabel("final"), fmt.Sprintf("%d", t.Final))
			lineRows = append(lineRows, row)
		}
		anotaciones["linescore"] = lineRows
	}
	if _, ok := anotaciones["linescore"]; !ok {
		if rows := buildFootballDBLinescore(supp.footballDB, env); len(rows) > 0 {
			anotaciones["linescore"] = rows
		}
	}

	if scoring, ok := env.Data.Tables["scoring"]; ok {
		summary := buildScoringSummary(scoring, env.Data.Scorebox)
		if len(summary) > 0 {
			anotaciones["scoring_summary"] = summary
		}
	}
	if _, ok := anotaciones["scoring_summary"]; !ok {
		if rows := buildFootballDBScoringSummary(supp.footballDB, env.Data.Scorebox); len(rows) > 0 {
			anotaciones["scoring_summary"] = rows
		}
	}

	if len(anotaciones) == 0 {
		anotaciones["linescore"] = []TableRow{}
		anotaciones["scoring_summary"] = []TableRow{}
	}
	page.AddSection(sectionTitle("anotaciones"), anotaciones)
	return nil
}

type gameStatSupplement struct {
	footballDB *footballdb.Envelope[footballdb.GameData]
}

func (g *GameBuilder) loadGameStatSupplement(env boxscore.BoxscoreEnvelope) gameStatSupplement {
	var supp gameStatSupplement
	if g == nil || strings.TrimSpace(g.processedDir) == "" || strings.TrimSpace(env.GameID) == "" {
		return supp
	}
	if fdb := g.loadFootballDBGame(env); fdb != nil {
		supp.footballDB = fdb
	}
	return supp
}

func (g *GameBuilder) loadFootballDBGame(env boxscore.BoxscoreEnvelope) *footballdb.Envelope[footballdb.GameData] {
	season, err := util.NFLSeasonFromGameID(env.GameID)
	if err != nil {
		return nil
	}
	dir := filepath.Join(g.processedDir, "footballdb", "games", fmt.Sprintf("%d", season))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	datePrefix := ""
	if len(env.GameID) >= 8 {
		datePrefix = env.GameID[:8]
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if datePrefix != "" && !strings.HasPrefix(entry.Name(), datePrefix) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var fdb footballdb.Envelope[footballdb.GameData]
		if err := json.Unmarshal(raw, &fdb); err != nil {
			continue
		}
		if footballDBGameMatchesEnvelope(fdb, env) {
			return &fdb
		}
	}
	return nil
}

func footballDBGameMatchesEnvelope(fdb footballdb.Envelope[footballdb.GameData], env boxscore.BoxscoreEnvelope) bool {
	if len(env.GameID) >= 8 && fdb.GameID != "" && !strings.HasPrefix(fdb.GameID, env.GameID[:8]) {
		return false
	}
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return true
	}
	scoreTeams := footballDBScoreboxTeams(fdb)
	if len(scoreTeams) == 0 {
		return true
	}
	for _, team := range env.Data.Scorebox.Teams[:2] {
		if !footballDBHasTeam(scoreTeams, linkContent(team.Name), team.Abbr) {
			return false
		}
	}
	return true
}

func footballDBScoreboxTeams(fdb footballdb.Envelope[footballdb.GameData]) []string {
	table, ok := fdb.Data.Tables["table_0"]
	if !ok {
		return nil
	}
	out := make([]string, 0, 2)
	for _, row := range table.Rows {
		name := convertValueToString(row["col_0"])
		if name == "" || strings.EqualFold(name, "Total") {
			continue
		}
		out = append(out, name)
	}
	return out
}

func footballDBHasTeam(candidates []string, fullName string, abbr string) bool {
	wantName := compactToken(fullName)
	wantAbbr := compactToken(abbr)
	for _, candidate := range candidates {
		got := compactToken(candidate)
		if wantName != "" && strings.Contains(got, wantName) {
			return true
		}
		if wantAbbr != "" && strings.Contains(got, wantAbbr) {
			return true
		}
	}
	return false
}

func (g *GameBuilder) buildEquiposSection(page *SectionedPage, env boxscore.BoxscoreEnvelope, supp gameStatSupplement) error {
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		page.AddSection(sectionTitle("equipos"), map[string]interface{}{})
		return nil
	}
	teamStatsTable, hasTeamStats := env.Data.Tables["team_stats"]
	teamStatsBySide := map[string]map[string]string{}
	if hasTeamStats {
		teamStatsBySide = splitTeamStats(teamStatsTable)
	}
	footballDBTeamStats := buildFootballDBTeamStats(supp.footballDB, env)
	gamebookTeamStats := buildGamebookTeamStats(env)
	footballDBPlayers := buildFootballDBPlayerStats(supp.footballDB, env)
	nflComPlayers := buildNFLComPlayerStats(env)

	teamCodes := resolveTeamCodes(env)
	abbrToSide := map[string]string{"visitor": "vis", "home": "home"}
	equipos := map[string]interface{}{}
	for i, team := range env.Data.Scorebox.Teams {
		side := "visitor"
		if i == 1 {
			side = "home"
		}
		teamName := linkContent(team.Name)
		teamCode := teamCodes[abbrToSide[side]]
		teamObj := map[string]interface{}{
			fieldLabel("coach"):        linkContent(team.Coach),
			fieldLabel("points_label"): team.Score,
			fieldLabel("record_label"): team.Record,
		}
		if stats := teamStatsBySide[abbrToSide[side]]; len(stats) > 0 {
			teamObj[fieldLabel("statistics")] = mergeStringStats(stats, footballDBTeamStats[abbrToSide[side]], gamebookTeamStats[abbrToSide[side]])
		} else {
			teamObj[fieldLabel("statistics")] = mergeStringStats(nil, footballDBTeamStats[abbrToSide[side]], gamebookTeamStats[abbrToSide[side]])
		}
		offenseRows := mergePlayerStatRows(
			filterPlayerTableByTeam("player_offense", env.Data.Tables["player_offense"], teamCode),
			footballDBPlayers[abbrToSide[side]][sectionTitle("offensive")],
			nflComPlayers[abbrToSide[side]][sectionTitle("offensive")],
		)
		defenseRows := mergePlayerStatRows(
			filterPlayerTableByTeam("player_defense", env.Data.Tables["player_defense"], teamCode),
			footballDBPlayers[abbrToSide[side]][sectionTitle("defensive")],
			nflComPlayers[abbrToSide[side]][sectionTitle("defensive")],
		)
		specialTeamsRows := mergePlayerStatRows(
			mergePlayerTables(
				filterPlayerTableByTeam("kicking", env.Data.Tables["kicking"], teamCode),
				filterPlayerTableByTeam("returns", env.Data.Tables["returns"], teamCode),
			),
			footballDBPlayers[abbrToSide[side]][sectionTitle("special_teams")],
			nflComPlayers[abbrToSide[side]][sectionTitle("special_teams")],
		)
		teamObj[fieldLabel("player_statistics")] = map[string]interface{}{
			sectionTitle("offensive"):     offenseRows,
			sectionTitle("defensive"):     defenseRows,
			sectionTitle("special_teams"): specialTeamsRows,
		}
		equipos[teamName] = teamObj
	}
	page.AddSection(sectionTitle("equipos"), equipos)
	return nil
}

func nflcomCellText(value any) string {
	switch v := value.(type) {
	case map[string]any:
		if text := convertValueToString(v["title"]); text != "" {
			return text
		}
		if text := convertValueToString(v["text"]); text != "" {
			return text
		}
		if text := convertValueToString(v["value"]); text != "" {
			return text
		}
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := nflcomCellText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	}
	return convertValueToString(value)
}

// resolveTeamCodes determines the player-table team abbreviation for visitor and
// home by cross-referencing starter names with player_offense/player_defense rows.
// Falls back to the scorebox URL-based code when starters are unavailable.
func resolveTeamCodes(env boxscore.BoxscoreEnvelope) map[string]string {
	codes := map[string]string{"vis": "", "home": ""}
	if env.Data.Scorebox == nil || len(env.Data.Scorebox.Teams) < 2 {
		return codes
	}
	// Collect all player names indexed by their table team code.
	playerTeamIndex := map[string]string{} // player name → team code
	for _, tblKey := range []string{"player_offense", "player_defense"} {
		tbl := env.Data.Tables[tblKey]
		for _, row := range tbl.Rows {
			pname := convertValueToString(row["player"])
			tc := convertValueToString(row["team"])
			if pname != "" && tc != "" {
				playerTeamIndex[pname] = tc
			}
		}
	}
	// For each side, look up a starter in the player index.
	for _, pair := range []struct {
		side       string
		starterKey string
		sbIdx      int
	}{
		{"vis", "vis_starters", 0},
		{"home", "home_starters", 1},
	} {
		starters := env.Data.Tables[pair.starterKey]
		for _, row := range starters.Rows {
			pname := convertValueToString(row["player"])
			if tc, ok := playerTeamIndex[pname]; ok {
				codes[pair.side] = tc
				break
			}
		}
		// Fallback: extract from scorebox URL.
		if codes[pair.side] == "" {
			codes[pair.side] = scoreboxTeamCode(env.Data.Scorebox.Teams[pair.sbIdx])
		}
	}
	return codes
}

func splitTeamStats(tbl boxscore.TableResult) map[string]map[string]string {
	out := map[string]map[string]string{"vis": {}, "home": {}}
	labels := gameTableHeaderMap("team_stats")
	for _, row := range tbl.Rows {
		label := convertValueToString(row["stat"])
		if label == "" {
			continue
		}
		if translated, ok := labels[label]; ok && strings.TrimSpace(translated) != "" {
			label = translated
		}
		out["vis"][label] = convertValueToString(row["vis_stat"])
		out["home"][label] = convertValueToString(row["home_stat"])
	}
	return out
}

func filterPlayerTableByTeam(tableKey string, tbl boxscore.TableResult, teamCode string) []TableRow {
	if len(tbl.Rows) == 0 {
		return []TableRow{}
	}
	filtered := make([]interface{}, 0)
	for _, row := range tbl.Rows {
		if strings.EqualFold(convertValueToString(row["team"]), teamCode) {
			filtered = append(filtered, row)
		}
	}
	if len(filtered) == 0 {
		return []TableRow{}
	}
	rows, err := ConvertTableToRows(filtered, gameTableHeaderMap(tableKey))
	if err != nil {
		return []TableRow{}
	}
	return rows
}

func mergePlayerTables(parts ...[]TableRow) []TableRow {
	out := make([]TableRow, 0)
	for _, p := range parts {
		out = append(out, p...)
	}
	if len(out) == 0 {
		return []TableRow{}
	}
	return out
}

func buildScoringSummary(tbl boxscore.TableResult, sb *boxscore.Scorebox) []TableRow {
	if len(tbl.Rows) == 0 {
		return []TableRow{}
	}
	tr := i18n.NewESTranslator()
	// Build short name → full name map from scorebox teams.
	fullNames := map[string]string{}
	if sb != nil {
		for _, t := range sb.Teams {
			if t.Name != nil {
				full := t.Name.Content
				parts := strings.Fields(full)
				if len(parts) > 0 {
					fullNames[strings.ToLower(parts[len(parts)-1])] = full
				}
			}
		}
	}
	out := make([]TableRow, 0, len(tbl.Rows))
	currentQuarter := ""
	for _, row := range tbl.Rows {
		q := convertValueToString(row["quarter"])
		if q != "" {
			currentQuarter = q
		}
		teamShort := convertValueToString(row["team"])
		teamFull := teamShort
		if full, ok := fullNames[strings.ToLower(teamShort)]; ok {
			teamFull = full
		}
		desc := convertValueToString(row["description"])
		descES := tr.TranslateScoringPlay(desc)
		vis := scoreOrZero(convertValueToString(row["vis_team_score"]))
		home := scoreOrZero(convertValueToString(row["home_team_score"]))
		tm := convertValueToString(row["time"])

		r := NewTableRow()
		r.Set(fieldLabel("quarter"), zeroIfEmpty(currentQuarter))
		if tm != "" {
			r.Set(fieldLabel("time"), tm)
		}
		r.Set(fieldLabel("team"), teamFull)
		r.Set(fieldLabel("description"), descES)
		r.Set(fieldLabel("score_visitor"), vis)
		r.Set(fieldLabel("score_home"), home)
		out = append(out, r)
	}
	return out
}

func scoreOrZero(v string) string {
	if strings.TrimSpace(v) == "" {
		return "0"
	}
	return v
}

func zeroIfEmpty(v string) string {
	if strings.TrimSpace(v) == "" {
		return "?"
	}
	return v
}

func linkContent(l *boxscore.LinkContent) string {
	if l == nil {
		return ""
	}
	return l.Content
}

func scoreboxTeamCode(team boxscore.ScoreboxTeam) string {
	if team.Name == nil {
		return ""
	}
	link := team.Name.Link
	parts := strings.Split(strings.Trim(link, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "teams" && len(parts[i+1]) >= 3 {
			return strings.ToUpper(parts[i+1][:3])
		}
	}
	return ""
}

func normalizeGameDate(raw, gameID string) string {
	for _, layout := range []string{"Monday Jan 2, 2006", "January 2, 2006", "Mon Jan 2, 2006"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(gameID) >= 8 {
		return fmt.Sprintf("%s-%s-%s", gameID[:4], gameID[4:6], gameID[6:8])
	}
	return ""
}

// buildInfoSection builds the "Información del partido" section from game_info table and scorebox meta.
func (g *GameBuilder) buildInfoSection(page *SectionedPage, env boxscore.BoxscoreEnvelope) error {
	tr := i18n.NewESTranslator()
	info := NewTableRow()

	// Week
	week := normalizeGameWeek(env.Data.Week)
	if week != "" {
		info.Set(fieldLabel("week"), week)
	}
	if gameType := gameTypeFromWeek(week); gameType != "" {
		info.Set(fieldLabel("game_type"), gameType)
	}
	info.Set(fieldLabel("status"), gameStatusFromEnvelope(env))

	// Date from scorebox meta
	dateAdded := false
	if env.Data.Scorebox != nil {
		meta := env.Data.Scorebox.Meta
		if meta.Date != "" {
			info.Set(fieldLabel("date"), translateDate(meta.Date, tr))
			dateAdded = true
		}
		if meta.Stadium != nil {
			info.Set(fieldLabel("stadium"), meta.Stadium.Content)
		}
	}
	if !dateAdded {
		if dateISO := normalizeGameDate("", env.GameID); dateISO != "" {
			info.Set(fieldLabel("date"), translateISODate(dateISO, tr))
		}
	}

	// Extract roof, surface, weather from game_info table
	if gi, ok := env.Data.Tables["game_info"]; ok {
		for _, row := range gi.Rows {
			label := strings.ToLower(convertValueToString(row["info"]))
			value := convertValueToString(row["stat"])
			if value == "" {
				continue
			}
			switch {
			case strings.Contains(label, "roof"):
				info.Set(fieldLabel("stadium_type"), tr.T(value))
			case strings.Contains(label, "surface"):
				info.Set(fieldLabel("playing_surface"), tr.T(value))
			case strings.Contains(label, "weather"):
				info.Set(fieldLabel("weather"), tr.TranslateWeather(value))
			}
		}
	}
	applyGamebookInfoFallback(&info, env, tr)

	if env.Data.NFLCom != nil && env.Data.NFLCom.Gamebook != nil {
		gamebook := strings.TrimSpace(env.Data.NFLCom.Gamebook.Path)
		if gamebook == "" {
			gamebook = strings.TrimSpace(env.Data.NFLCom.Gamebook.URL)
		}
		if gamebook != "" {
			info.Set(fieldLabel("gamebook"), publicGamebookPath(env.GameID, gamebook))
		}
	}

	page.AddSection(sectionTitle("game_info"), info)
	return nil
}

func applyGamebookInfoFallback(info *TableRow, env boxscore.BoxscoreEnvelope, tr i18n.Translator) {
	if info == nil || env.Data.NFLCom == nil || env.Data.NFLCom.GamebookData == nil {
		return
	}
	meta := env.Data.NFLCom.GamebookData.Metadata
	setTableRowIfBlank(info, fieldLabel("date"), translateGamebookDate(meta.Date, tr))
	setTableRowIfBlank(info, fieldLabel("start_time"), meta.StartTime)
	setTableRowIfBlank(info, fieldLabel("stadium"), gamebookStadiumName(meta.Stadium))
	setTableRowIfBlank(info, fieldLabel("stadium_type"), gamebookStadiumType(meta.Surface, tr))
	setTableRowIfBlank(info, fieldLabel("playing_surface"), gamebookPlayingSurface(meta.Surface, tr))
	setTableRowIfBlank(info, fieldLabel("weather"), tr.TranslateWeather(normalizeGamebookWeather(meta.Weather)))
	setTableRowIfBlank(info, fieldLabel("attendance"), meta.Attendance)
	setTableRowIfBlank(info, fieldLabel("duration"), meta.TimeOfGame)
}

func setTableRowIfBlank(row *TableRow, key string, value string) {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(row.Get(key)) != "" {
		return
	}
	row.Set(key, value)
}

func gamebookStadiumName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(raw, ",")[0])
}

func gamebookStadiumType(raw string, tr i18n.Translator) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "outdoor"), strings.Contains(lower, "open stadium"):
		return tr.T("outdoors")
	case strings.Contains(lower, "retractable") && strings.Contains(lower, "closed"):
		return tr.T("retractable_roof_closed")
	case strings.Contains(lower, "retractable") && strings.Contains(lower, "open"):
		return tr.T("retractable_roof_open")
	default:
		return ""
	}
}

func gamebookPlayingSurface(raw string, tr i18n.Translator) string {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "natural grass"), strings.Contains(lower, "grass"):
		return tr.T("grass")
	case strings.Contains(lower, "fieldturf"):
		return tr.T("fieldturf")
	case strings.Contains(lower, "a-turf"):
		return tr.T("a_turf")
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeGamebookWeather(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	re := regexp.MustCompile(`(?i)^(.*?)\s+Temp:\s*([0-9]+).*?Humidity:\s*([0-9]+%).*?Wind:\s*(.*?)\s*$`)
	if m := re.FindStringSubmatch(raw); len(m) == 5 {
		parts := []string{}
		if condition := strings.Trim(strings.TrimSpace(m[1]), ","); condition != "" {
			parts = append(parts, condition)
		}
		parts = append(parts, fmt.Sprintf("%s degrees", strings.TrimSpace(m[2])))
		parts = append(parts, fmt.Sprintf("relative humidity %s", strings.TrimSpace(m[3])))
		parts = append(parts, fmt.Sprintf("wind %s", strings.TrimSpace(m[4])))
		return strings.Join(parts, ", ")
	}
	return raw
}

func publicGamebookPath(gameID, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(filepath.ToSlash(raw), "assets/") {
		return filepath.ToSlash(raw)
	}
	season, err := util.NFLSeasonFromGameID(gameID)
	if err != nil {
		return filepath.ToSlash(raw)
	}
	clean := strings.Split(raw, "?")[0]
	ext := filepath.Ext(clean)
	if ext == "" {
		ext = ".pdf"
	}
	return filepath.ToSlash(filepath.Join("assets", "gamebooks", fmt.Sprintf("%d", season), filepath.Base(gameID+ext)))
}

func normalizeGameWeek(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "preseason") {
		parts := strings.Fields(s)
		if len(parts) >= 2 {
			return "Preseason " + parts[len(parts)-1]
		}
		return "Preseason"
	}
	if isPostseasonWeekLabel(s) {
		return "Playoffs"
	}
	return s
}

func gameTypeFromWeek(week string) string {
	s := strings.TrimSpace(week)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "preseason"):
		return "preseason"
	case isPostseasonWeekLabel(s):
		return "postseason"
	case isNumericString(s):
		return "regular_season"
	default:
		return ""
	}
}

func gameStatusFromEnvelope(env boxscore.BoxscoreEnvelope) string {
	if env.Data.Scorebox != nil && len(env.Data.Scorebox.Teams) >= 2 {
		return "final"
	}
	return "scheduled"
}

func isPostseasonWeekLabel(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "playoff") ||
		strings.Contains(lower, "wild card") ||
		strings.Contains(lower, "division") ||
		strings.Contains(lower, "conference") ||
		strings.Contains(lower, "super bowl")
}

func isNumericString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// translateDate converts "Sunday Sep 6, 1981" to "6 de septiembre de 1981".
func translateDate(dateStr string, tr i18n.Translator) string {
	re := regexp.MustCompile(`(?i)\w+\s+(\w+)\s+(\d+),\s+(\d+)`)
	m := re.FindStringSubmatch(dateStr)
	if len(m) < 4 {
		return dateStr
	}
	monthES := tr.TranslateMonth(m[1])
	return fmt.Sprintf("%s de %s de %s", m[2], monthES, m[3])
}

func translateISODate(dateISO string, tr i18n.Translator) string {
	t, err := time.Parse("2006-01-02", dateISO)
	if err != nil {
		return dateISO
	}
	return fmt.Sprintf("%d de %s de %d", t.Day(), tr.TranslateMonth(t.Month().String()), t.Year())
}

func translateGamebookDate(dateStr string, tr i18n.Translator) string {
	dateStr = strings.TrimSpace(dateStr)
	for _, layout := range []string{"Monday, 1/2/2006", "Monday, 01/02/2006"} {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return fmt.Sprintf("%d de %s de %d", t.Day(), tr.TranslateMonth(t.Month().String()), t.Year())
		}
	}
	return translateDate(dateStr, tr)
}

// buildOficialesSection builds the "Oficiales" section from the officials table.
func (g *GameBuilder) buildOficialesSection(page *SectionedPage, env boxscore.BoxscoreEnvelope) error {
	offTbl, ok := env.Data.Tables["officials"]
	if !ok || len(offTbl.Rows) == 0 {
		page.AddSection(sectionTitle("oficiales"), gamebookOfficialRows(env))
		return nil
	}
	rows, err := ConvertTableToRows(offTbl.Rows, gameTableHeaderMap("officials"))
	if err != nil {
		return fmt.Errorf("convert officials table: %w", err)
	}
	page.AddSection(sectionTitle("oficiales"), rows)
	return nil
}

func gamebookOfficialRows(env boxscore.BoxscoreEnvelope) []TableRow {
	if env.Data.NFLCom == nil || env.Data.NFLCom.GamebookData == nil {
		return []TableRow{}
	}
	rows := make([]TableRow, 0, len(env.Data.NFLCom.GamebookData.Officials))
	for _, official := range env.Data.NFLCom.GamebookData.Officials {
		if strings.TrimSpace(official.Name) == "" && strings.TrimSpace(official.Role) == "" {
			continue
		}
		row := NewTableRow()
		row.Set(fieldLabel("name"), normalizeGamebookOfficialName(official.Name))
		row.Set(fieldLabel("position"), official.Role)
		rows = append(rows, row)
	}
	return rows
}

func normalizeGamebookOfficialName(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = regexp.MustCompile(`\s*\([0-9]+\)\s*$`).ReplaceAllString(raw, "")
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1] + " " + parts[0])
	}
	return raw
}

// buildDrivesSection builds the "Series ofensivas" section with nested visitor/home drives.
func (g *GameBuilder) buildDrivesSection(page *SectionedPage, env boxscore.BoxscoreEnvelope) error {
	drives := map[string]interface{}{}

	visitorName := sectionTitle("visitor")
	homeName := sectionTitle("home")
	if env.Data.Scorebox != nil {
		if len(env.Data.Scorebox.Teams) > 0 {
			visitorName = linkContent(env.Data.Scorebox.Teams[0].Name)
		}
		if len(env.Data.Scorebox.Teams) > 1 {
			homeName = linkContent(env.Data.Scorebox.Teams[1].Name)
		}
	}

	for _, spec := range []struct {
		key  string
		name string
	}{
		{"vis_drives", visitorName},
		{"home_drives", homeName},
	} {
		tbl, ok := env.Data.Tables[spec.key]
		if !ok || len(tbl.Rows) == 0 {
			continue
		}
		rows, err := ConvertTableToRows(tbl.Rows, gameTableHeaderMap("drives"))
		if err != nil {
			return fmt.Errorf("convert %s: %w", spec.key, err)
		}
		if len(rows) > 0 {
			drives[spec.name] = rows
		}
	}

	page.AddSection(sectionTitle("series"), drives)
	return nil
}

// buildSnapCountsSection builds the "Conteo de snaps" section with nested visitor/home snap counts.
func (g *GameBuilder) buildSnapCountsSection(page *SectionedPage, env boxscore.BoxscoreEnvelope) error {
	snaps := map[string]interface{}{}

	visitorName := sectionTitle("visitor")
	homeName := sectionTitle("home")
	if env.Data.Scorebox != nil {
		if len(env.Data.Scorebox.Teams) > 0 {
			visitorName = linkContent(env.Data.Scorebox.Teams[0].Name)
		}
		if len(env.Data.Scorebox.Teams) > 1 {
			homeName = linkContent(env.Data.Scorebox.Teams[1].Name)
		}
	}

	for _, spec := range []struct {
		key  string
		name string
	}{
		{"vis_snap_counts", visitorName},
		{"home_snap_counts", homeName},
	} {
		tbl, ok := env.Data.Tables[spec.key]
		if !ok || len(tbl.Rows) == 0 {
			rows := gamebookSnapCountRows(env, spec.name)
			if len(rows) > 0 {
				snaps[spec.name] = rows
			}
			continue
		}
		rows, err := ConvertTableToRows(tbl.Rows, gameTableHeaderMap("snap_counts"))
		if err != nil {
			return fmt.Errorf("convert %s: %w", spec.key, err)
		}
		if len(rows) > 0 {
			snaps[spec.name] = rows
		}
	}

	page.AddSection(sectionTitle("snap_counts"), snaps)
	return nil
}

// buildAdvancedStatsSection builds the "Estadísticas avanzadas" section with all advanced tables.
func (g *GameBuilder) buildAdvancedStatsSection(page *SectionedPage, env boxscore.BoxscoreEnvelope) error {
	adv := map[string]interface{}{}

	for _, spec := range []struct {
		key   string
		label string
	}{
		{"passing_advanced", sectionTitle("advanced_passing")},
		{"rushing_advanced", sectionTitle("advanced_rushing")},
		{"receiving_advanced", sectionTitle("advanced_receiving")},
		{"defense_advanced", sectionTitle("advanced_defense")},
		{"expected_points", sectionTitle("expected_points")},
	} {
		tbl, ok := env.Data.Tables[spec.key]
		if !ok || len(tbl.Rows) == 0 {
			continue
		}
		rows, err := ConvertTableToRows(tbl.Rows, gameTableHeaderMap(spec.key))
		if err != nil {
			return fmt.Errorf("convert advanced table %s: %w", spec.key, err)
		}
		if len(rows) > 0 {
			adv[spec.label] = rows
		}
	}

	page.AddSection(sectionTitle("advanced_stats"), adv)
	return nil
}
