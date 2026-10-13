package season_stats

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/pfr"
)

type seasonFile struct {
	Season int                `json:"season"`
	Data   pfr.SeasonDataV1_1 `json:"data"`
}

func MapToViewModel(year int, raw []byte) (SeasonStatsViewModel, error) {
	var file seasonFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return SeasonStatsViewModel{}, fmt.Errorf("parse season json: %w", err)
	}
	if file.Season == 0 {
		return SeasonStatsViewModel{}, fmt.Errorf("missing season year in json")
	}
	if year == 0 {
		year = file.Season
	}
	if file.Season != 0 && year != file.Season {
		return SeasonStatsViewModel{}, fmt.Errorf("season mismatch: input year %d json year %d", year, file.Season)
	}

	main, ok := file.Data["main"]
	if !ok {
		return SeasonStatsViewModel{}, fmt.Errorf("missing main page data")
	}
	tables := main.Tables

	passingRows := tableRows(tables, "passing")
	rushingRows := tableRows(tables, "rushing_and_receiving")
	defenseRows := tableRows(tables, "defense")
	returnsRows := tableRows(tables, "returns")
	kickingRows := tableRows(tables, "kicking")
	puntingRows := tableRows(tables, "punting")
	scoringRows := tableRows(tables, "scoring")

	games := maxGames([][]map[string]any{passingRows, rushingRows, defenseRows, returnsRows, kickingRows, puntingRows, scoringRows})
	if games == 0 {
		games = 17
	}

	teamPassing := TeamPassingStats{
		Cmp:   sumInt(passingRows, "pass_cmp"),
		Att:   sumInt(passingRows, "pass_att"),
		Yds:   sumInt(passingRows, "pass_yds"),
		TD:    sumInt(passingRows, "pass_td"),
		Int:   sumInt(passingRows, "pass_int"),
		Sacks: sumInt(passingRows, "pass_sacked"),
	}
	if teamPassing.Att > 0 {
		teamPassing.Pct = roundFloat(float64(teamPassing.Cmp)/float64(teamPassing.Att)*100, 1)
		teamPassing.YPA = roundFloat(float64(teamPassing.Yds)/float64(teamPassing.Att), 1)
	}

	teamRushing := TeamRushingStats{
		Att:  sumInt(rushingRows, "rush_att"),
		Yds:  sumInt(rushingRows, "rush_yds"),
		TD:   sumInt(rushingRows, "rush_td"),
		Long: maxInt(rushingRows, "rush_long"),
	}
	if teamRushing.Att > 0 {
		teamRushing.YPA = roundFloat(float64(teamRushing.Yds)/float64(teamRushing.Att), 2)
	}
	if games > 0 {
		teamRushing.YPG = roundFloat(float64(teamRushing.Yds)/float64(games), 1)
	}

	teamReceiving := TeamReceivingStats{
		Rec:  sumInt(rushingRows, "rec"),
		Yds:  sumInt(rushingRows, "rec_yds"),
		TD:   sumInt(rushingRows, "rec_td"),
		Long: maxInt(rushingRows, "rec_long"),
	}
	if teamReceiving.Rec > 0 {
		teamReceiving.YPR = roundFloat(float64(teamReceiving.Yds)/float64(teamReceiving.Rec), 2)
	}
	if games > 0 {
		teamReceiving.YPG = roundFloat(float64(teamReceiving.Yds)/float64(games), 1)
	}

	teamDefense := TeamDefenseStats{
		Int:     sumInt(defenseRows, "def_int"),
		IntYds:  sumInt(defenseRows, "def_int_yds"),
		IntTD:   sumInt(defenseRows, "def_int_td"),
		Sacks:   sumFloat(defenseRows, "sacks"),
		SackYds: sumInt(defenseRows, "sack_yds"),
		Tackles: sumInt(defenseRows, "tackles_solo"),
		Assists: sumInt(defenseRows, "tackles_assists"),
		Total:   sumInt(defenseRows, "tackles_combined"),
	}

	kickRetYds := sumInt(returnsRows, "kick_ret_yds")
	kickRet := sumInt(returnsRows, "kick_ret")
	puntRetYds := sumInt(returnsRows, "punt_ret_yds")
	puntRet := sumInt(returnsRows, "punt_ret")

	teamSpecial := TeamSpecialTeamsStats{
		KickRetYds:  kickRetYds,
		KickRetLong: maxInt(returnsRows, "kick_ret_long"),
		KickRetTD:   sumInt(returnsRows, "kick_ret_td"),
		PuntRetYds:  puntRetYds,
		PuntRetLong: maxInt(returnsRows, "punt_ret_long"),
		PuntRetTD:   sumInt(returnsRows, "punt_ret_td"),
		PuntYds:     sumInt(puntingRows, "punt_yds"),
		PuntLong:    maxInt(puntingRows, "punt_long"),
		PuntBlocked: sumInt(puntingRows, "punt_blocked"),
		Touchbacks:  sumInt(puntingRows, "punt_tb"),
		In20:        sumInt(puntingRows, "punt_in_20"),
	}
	punts := sumInt(puntingRows, "punt")
	punNet := sumInt(puntingRows, "punt_net_yds")
	if kickRet > 0 {
		teamSpecial.KickRetAvg = roundFloat(float64(kickRetYds)/float64(kickRet), 2)
	}
	if puntRet > 0 {
		teamSpecial.PuntRetAvg = roundFloat(float64(puntRetYds)/float64(puntRet), 2)
	}
	if punts > 0 {
		teamSpecial.PuntAvg = roundFloat(float64(teamSpecial.PuntYds)/float64(punts), 1)
		teamSpecial.NetAvg = roundFloat(float64(punNet)/float64(punts), 1)
	}

	teamScoring := TeamScoringStats{Totals: scoringTotals(scoringRows, kickingRows)}

	// Extract data for special teams and advanced stats
	conversionsRows := tableRows(tables, "team_conversions")
	teamTDLogRows := tableRows(tables, "team_td_log")
	oppTDLogRows := tableRows(tables, "opp_td_log")

	// Check for advanced stats in data.advanced section
	advanced, _ := file.Data["advanced"]
	advTables := advanced.Tables

	view := SeasonStatsViewModel{
		Year:              year,
		TeamPassing:       teamPassing,
		TeamRushing:       teamRushing,
		TeamReceiving:     teamReceiving,
		TeamDefense:       teamDefense,
		TeamSpecialTeams:  teamSpecial,
		TeamScoring:       teamScoring,
		PlayerPassing:     mapPlayerPassing(passingRows),
		PlayerRushing:     mapPlayerRushing(rushingRows),
		PlayerReceiving:   mapPlayerReceiving(rushingRows),
		PlayerDefense:     mapPlayerDefense(defenseRows),
		PlayerScoring:     mapPlayerScoring(scoringRows),
		KickingPlayers:    mapPlayerKicking(kickingRows),
		PuntingPlayers:    mapPlayerPunting(puntingRows),
		ReturnsPlayers:    mapPlayerReturns(returnsRows),
		TeamConversions:   mapTeamConversions(conversionsRows),
		TeamTDLog:         mapTDLog(teamTDLogRows),
		OppTDLog:          mapTDLog(oppTDLogRows),
		AdvancedPassing:   mapAdvancedPassing(tableRows(advTables, "passing_advanced")),
		AdvancedRushing:   mapAdvancedRushing(tableRows(advTables, "adv_rushing")),
		AdvancedReceiving: mapAdvancedReceiving(tableRows(advTables, "adv_receiving")),
		AdvancedDefense:   mapAdvancedDefense(tableRows(advTables, "adv_defense")),
		ExtraTables:       buildExtraTables(file.Data),
	}

	return view, nil
}

func tableRows(tables map[string]pfr.TableResult, name string) []map[string]any {
	if tables == nil {
		return nil
	}
	if table, ok := tables[name]; ok {
		return table.Rows
	}
	return nil
}

func mapPlayerPassing(rows []map[string]any) []PlayerPassingRow {
	players := make([]PlayerPassingRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		cmp, okCmp := toInt(row["pass_cmp"])
		att, okAtt := toInt(row["pass_att"])
		yds, okYds := toInt(row["pass_yds"])
		td, okTD := toInt(row["pass_td"])
		ints, okInt := toInt(row["pass_int"])
		sacks, _ := toInt(row["pass_sacked"])
		pct := floatFromRow(row, "pass_cmp_pct")
		if pct == 0 && okCmp && okAtt && att > 0 {
			pct = roundFloat(float64(cmp)/float64(att)*100, 1)
		}
		ypa := floatFromRow(row, "pass_yds_per_att")
		if ypa == 0 && okYds && okAtt && att > 0 {
			ypa = roundFloat(float64(yds)/float64(att), 1)
		}
		if !(okCmp || okAtt || okYds || okTD || okInt || sacks > 0) {
			continue
		}
		players = append(players, PlayerPassingRow{
			Name:  name,
			Cmp:   cmp,
			Att:   att,
			Pct:   pct,
			Yds:   yds,
			YPA:   ypa,
			TD:    td,
			Int:   ints,
			Sacks: sacks,
		})
	}
	return players
}

func mapPlayerRushing(rows []map[string]any) []PlayerRushingRow {
	players := make([]PlayerRushingRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		att, okAtt := toInt(row["rush_att"])
		yds, okYds := toInt(row["rush_yds"])
		td, okTD := toInt(row["rush_td"])
		lng, _ := toInt(row["rush_long"])
		ypg := floatFromRow(row, "rush_yds_per_g")
		ypa := floatFromRow(row, "rush_yds_per_att")
		if ypa == 0 && okYds && okAtt && att > 0 {
			ypa = roundFloat(float64(yds)/float64(att), 2)
		}
		if ypg == 0 && okYds {
			if games, okGames := toInt(row["games"]); okGames && games > 0 {
				ypg = roundFloat(float64(yds)/float64(games), 1)
			}
		}
		if !(okAtt || okYds || okTD || lng > 0) {
			continue
		}
		players = append(players, PlayerRushingRow{
			Name: name,
			Att:  att,
			Yds:  yds,
			YPA:  ypa,
			YPG:  ypg,
			Long: lng,
			TD:   td,
		})
	}
	return players
}

func mapPlayerReceiving(rows []map[string]any) []PlayerReceivingRow {
	players := make([]PlayerReceivingRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		rec, okRec := toInt(row["rec"])
		yds, okYds := toInt(row["rec_yds"])
		td, okTD := toInt(row["rec_td"])
		lng, _ := toInt(row["rec_long"])
		ypg := floatFromRow(row, "rec_yds_per_g")
		ypr := floatFromRow(row, "rec_yds_per_rec")
		if ypr == 0 && okYds && okRec && rec > 0 {
			ypr = roundFloat(float64(yds)/float64(rec), 2)
		}
		if ypg == 0 && okYds {
			if games, okGames := toInt(row["games"]); okGames && games > 0 {
				ypg = roundFloat(float64(yds)/float64(games), 1)
			}
		}
		if !(okRec || okYds || okTD || lng > 0) {
			continue
		}
		players = append(players, PlayerReceivingRow{
			Name: name,
			Rec:  rec,
			Yds:  yds,
			YPR:  ypr,
			YPG:  ypg,
			Long: lng,
			TD:   td,
		})
	}
	return players
}

func mapPlayerDefense(rows []map[string]any) []PlayerDefenseRow {
	players := make([]PlayerDefenseRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		ints, okInt := toInt(row["def_int"])
		intYds, okIntYds := toInt(row["def_int_yds"])
		intTD, okIntTD := toInt(row["def_int_td"])
		sacks := floatFromRow(row, "sacks")
		sackYds, okSackYds := toInt(row["sack_yds"])
		solo, okSolo := toInt(row["tackles_solo"])
		ast, okAst := toInt(row["tackles_assists"])
		total, okTotal := toInt(row["tackles_combined"])
		if !(okInt || okIntYds || okIntTD || sacks > 0 || okSackYds || okSolo || okAst || okTotal) {
			continue
		}
		players = append(players, PlayerDefenseRow{
			Name:    name,
			Int:     ints,
			IntYds:  intYds,
			IntTD:   intTD,
			Sacks:   sacks,
			SackYds: sackYds,
			Tackles: solo,
			Assists: ast,
			Total:   total,
		})
	}
	return players
}

func mapPlayerScoring(rows []map[string]any) []PlayerScoringRow {
	players := make([]PlayerScoringRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		totalTD, okTotal := toInt(row["total_td"])
		rushTD, okRush := toInt(row["rush_td"])
		recTD, okRec := toInt(row["rec_td"])
		krTD, okKR := toInt(row["kick_ret_td"])
		prTD, okPR := toInt(row["punt_ret_td"])
		intTD, okInt := toInt(row["def_int_td"])
		fgm, okFGM := toInt(row["fgm"])
		fga, okFGA := toInt(row["fga"])
		xpm, okXPM := toInt(row["xpm"])
		xpa, okXPA := toInt(row["xpa"])
		twoPt, okTwo := toInt(row["two_pt_md"])
		safety, okSafety := toInt(row["safety_md"])
		points, _ := toInt(row["scoring"])
		if !(okTotal || okRush || okRec || okKR || okPR || okInt || okFGM || okFGA || okXPM || okXPA || okTwo || okSafety || points > 0) {
			continue
		}
		players = append(players, PlayerScoringRow{
			Name:      name,
			TotalTD:   totalTD,
			RushTD:    rushTD,
			RecTD:     recTD,
			KickRetTD: krTD,
			PuntRetTD: prTD,
			IntRetTD:  intTD,
			FGM:       fgm,
			FGA:       fga,
			XPM:       xpm,
			XPA:       xpa,
			TwoPt:     twoPt,
			Safety:    safety,
			Points:    points,
		})
	}
	return players
}

func mapPlayerKicking(rows []map[string]any) []KickingPlayer {
	players := make([]KickingPlayer, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		fgm, okFGM := toInt(row["fgm"])
		fga, okFGA := toInt(row["fga"])
		xpm, okXPM := toInt(row["xpm"])
		xpa, okXPA := toInt(row["xpa"])
		ko, okKO := toInt(row["kickoff"])
		koYds, _ := toInt(row["kickoff_yds"])
		koTB, _ := toInt(row["kickoff_tb"])

		if !(okFGM || okFGA || okXPM || okXPA || okKO) {
			continue
		}

		kp := KickingPlayer{
			Name:  name,
			FGM:   fgm,
			FGA:   fga,
			XPM:   xpm,
			XPA:   xpa,
			KO:    ko,
			KOYds: koYds,
			KOTB:  koTB,
		}

		if fga > 0 {
			kp.FGPct = roundFloat(float64(fgm)/float64(fga)*100, 1)
		}
		if xpa > 0 {
			kp.XPPct = roundFloat(float64(xpm)/float64(xpa)*100, 1)
		}
		if ko > 0 {
			kp.KOTBPct = roundFloat(float64(koTB)/float64(ko)*100, 1)
		}

		fgLong, _ := toInt(row["fgm_long"])
		kp.FGLong = fgLong

		players = append(players, kp)
	}
	return players
}

func mapPlayerPunting(rows []map[string]any) []PuntingPlayer {
	players := make([]PuntingPlayer, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		punts, okPunts := toInt(row["punt"])
		yds, okYds := toInt(row["punt_yds"])
		long, _ := toInt(row["punt_long"])
		in20, _ := toInt(row["punt_in_20"])
		tb, _ := toInt(row["punt_tb"])
		blk, _ := toInt(row["punt_blocked"])
		netYds, _ := toInt(row["punt_net_yds"])

		if !(okPunts || okYds) {
			continue
		}

		pp := PuntingPlayer{
			Name:  name,
			Punts: punts,
			Yds:   yds,
			Long:  long,
			In20:  in20,
			TB:    tb,
			Blk:   blk,
		}

		if punts > 0 {
			pp.YdsPerPun = roundFloat(float64(yds)/float64(punts), 1)
			pp.NetYdsPerPun = roundFloat(float64(netYds)/float64(punts), 1)
		}

		players = append(players, pp)
	}
	return players
}

func mapPlayerReturns(rows []map[string]any) []ReturnsPlayer {
	players := make([]ReturnsPlayer, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}
		kret, okKRet := toInt(row["kick_ret"])
		kretYds, okKRetYds := toInt(row["kick_ret_yds"])
		kretLong, _ := toInt(row["kick_ret_long"])
		kretTD, _ := toInt(row["kick_ret_td"])
		pret, okPRet := toInt(row["punt_ret"])
		pretYds, okPRetYds := toInt(row["punt_ret_yds"])
		pretLong, _ := toInt(row["punt_ret_long"])
		pretTD, _ := toInt(row["punt_ret_td"])

		if !(okKRet || okKRetYds || okPRet || okPRetYds) {
			continue
		}

		rp := ReturnsPlayer{
			Name:     name,
			KRet:     kret,
			KRetYds:  kretYds,
			KRetLong: kretLong,
			KRetTD:   kretTD,
			PRet:     pret,
			PRetYds:  pretYds,
			PRetLong: pretLong,
			PRetTD:   pretTD,
		}

		if kret > 0 {
			rp.KRetAvg = roundFloat(float64(kretYds)/float64(kret), 2)
		}
		if pret > 0 {
			rp.PRetAvg = roundFloat(float64(pretYds)/float64(pret), 2)
		}

		players = append(players, rp)
	}
	return players
}

func mapTeamConversions(rows []map[string]any) TeamConversions {
	tc := TeamConversions{}
	if len(rows) == 0 {
		return tc
	}

	row := rows[0]

	tc.ThirdDownSuccess, _ = toInt(row["third_down_success"])
	tc.ThirdDownAtt, _ = toInt(row["third_down_att"])
	tc.FourthDownSuccess, _ = toInt(row["fourth_down_success"])
	tc.FourthDownAtt, _ = toInt(row["fourth_down_att"])
	tc.RedZoneScores, _ = toInt(row["redzone_scores"])
	tc.RedZoneAtt, _ = toInt(row["redzone_att"])

	if tc.ThirdDownAtt > 0 {
		tc.ThirdDownPct = roundFloat(float64(tc.ThirdDownSuccess)/float64(tc.ThirdDownAtt)*100, 1)
	}
	if tc.FourthDownAtt > 0 {
		tc.FourthDownPct = roundFloat(float64(tc.FourthDownSuccess)/float64(tc.FourthDownAtt)*100, 1)
	}
	if tc.RedZoneAtt > 0 {
		tc.RedZonePct = roundFloat(float64(tc.RedZoneScores)/float64(tc.RedZoneAtt)*100, 1)
	}

	return tc
}

func mapTDLog(rows []map[string]any) []TDLogEntry {
	entries := make([]TDLogEntry, 0, len(rows))
	for _, row := range rows {
		player, _ := row["player"].(string)
		quarter, _ := row["quarter"].(string)
		time, _ := row["time"].(string)
		distance, _ := row["distance"].(string)
		tdType, _ := row["td_type"].(string)
		description, _ := row["description"].(string)

		if player == "" && description == "" {
			continue
		}

		entries = append(entries, TDLogEntry{
			Player:      player,
			Quarter:     quarter,
			Time:        time,
			Distance:    distance,
			Type:        tdType,
			Description: description,
		})
	}
	return entries
}

func mapAdvancedPassing(rows []map[string]any) []AdvancedPassingRow {
	players := make([]AdvancedPassingRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}

		apr := AdvancedPassingRow{
			Name:               string(nameDisplay(row)),
			CmpAirYds:          int(floatFromRow(row, "cmp_air_yds")),
			CmpAirYdsPerAtt:    floatFromRow(row, "cmp_air_yds_per_att"),
			CmpAirYdsPerCmp:    floatFromRow(row, "cmp_air_yds_per_cmp"),
			Att:                int(floatFromRow(row, "pass_att")),
			BattedPasses:       int(floatFromRow(row, "batted_passes")),
			Blitzed:            int(floatFromRow(row, "blitzed")),
			Cmp:                int(floatFromRow(row, "pass_cmp")),
			DropPct:            floatFromRow(row, "drop_pct"),
			Drops:              int(floatFromRow(row, "drops")),
			Hits:               int(floatFromRow(row, "hits")),
			Hurried:            int(floatFromRow(row, "hurried")),
			OnTarget:           int(floatFromRow(row, "on_target")),
			OnTargetPct:        floatFromRow(row, "on_target_pct"),
			PlayAction:         int(floatFromRow(row, "play_action")),
			PlayActionPassYds:  int(floatFromRow(row, "play_action_pass_yds")),
			PoorThrowPct:       floatFromRow(row, "poor_throw_pct"),
			PoorThrows:         int(floatFromRow(row, "poor_throws")),
			Pressured:          int(floatFromRow(row, "pressured")),
			PressuredPct:       floatFromRow(row, "pressured_pct"),
			RPO:                int(floatFromRow(row, "rpo")),
			RPOPassAtt:         int(floatFromRow(row, "rpo_pass_att")),
			RPOPassYds:         int(floatFromRow(row, "rpo_pass_yds")),
			RPORushAtt:         int(floatFromRow(row, "rpo_rush_att")),
			RPORushYds:         int(floatFromRow(row, "rpo_rush_yds")),
			RPOYds:             int(floatFromRow(row, "rpo_yds")),
			Spikes:             int(floatFromRow(row, "spikes")),
			TargetYds:          int(floatFromRow(row, "target_yds")),
			TgtYdsPerAtt:       floatFromRow(row, "tgt_yds_per_att"),
			Throwaways:         int(floatFromRow(row, "throwaways")),
			YAC:                int(floatFromRow(row, "yac")),
			YACPerCmp:          floatFromRow(row, "yac_per_cmp"),
			PocketTime:         floatFromRow(row, "pocket_time"),
			Scrambles:          int(floatFromRow(row, "scrambles")),
			ScramblesYdsPerAtt: floatFromRow(row, "scrambles_yds_per_att"),
		}

		players = append(players, apr)
	}
	return players
}

func mapAdvancedRushing(rows []map[string]any) []AdvancedRushingRow {
	players := make([]AdvancedRushingRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}

		arr := AdvancedRushingRow{
			Name:                     name,
			RushAtt:                  int(floatFromRow(row, "rush_att")),
			RushBrokenTackles:        int(floatFromRow(row, "rush_broken_tackles")),
			RushBrokenTacklesPerRush: floatFromRow(row, "rush_broken_tackles_per_rush"),
			RushFirstDown:            int(floatFromRow(row, "rush_first_down")),
			RushYAC:                  int(floatFromRow(row, "rush_yac")),
			RushYACPerRush:           floatFromRow(row, "rush_yac_per_rush"),
			RushYds:                  int(floatFromRow(row, "rush_yds")),
			RushYdsBeforeContact:     int(floatFromRow(row, "rush_yds_before_contact")),
			RushYdsBCPerRush:         floatFromRow(row, "rush_yds_bc_per_rush"),
		}

		players = append(players, arr)
	}
	return players
}

func mapAdvancedReceiving(rows []map[string]any) []AdvancedReceivingRow {
	players := make([]AdvancedReceivingRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}

		arr := AdvancedReceivingRow{
			Name:                   name,
			Rec:                    int(floatFromRow(row, "rec")),
			RecADOT:                floatFromRow(row, "rec_adot"),
			RecAirYds:              int(floatFromRow(row, "rec_air_yds")),
			RecAirYdsPerRec:        floatFromRow(row, "rec_air_yds_per_rec"),
			RecBrokenTackles:       int(floatFromRow(row, "rec_broken_tackles")),
			RecBrokenTacklesPerRec: floatFromRow(row, "rec_broken_tackles_per_rec"),
			RecDropPct:             floatFromRow(row, "rec_drop_pct"),
			RecDrops:               int(floatFromRow(row, "rec_drops")),
			RecFirstDown:           int(floatFromRow(row, "rec_first_down")),
			RecPassRating:          floatFromRow(row, "rec_pass_rating"),
			RecTargetInt:           int(floatFromRow(row, "rec_target_int")),
			RecYAC:                 int(floatFromRow(row, "rec_yac")),
			RecYACPerRec:           floatFromRow(row, "rec_yac_per_rec"),
			RecYds:                 int(floatFromRow(row, "rec_yds")),
			Targets:                int(floatFromRow(row, "targets")),
		}

		players = append(players, arr)
	}
	return players
}

func mapAdvancedDefense(rows []map[string]any) []AdvancedDefenseRow {
	players := make([]AdvancedDefenseRow, 0, len(rows))
	for _, row := range rows {
		name := nameDisplay(row)
		if name == "" {
			continue
		}

		adr := AdvancedDefenseRow{
			Name:             name,
			Age:              int(floatFromRow(row, "age")),
			Blitzes:          int(floatFromRow(row, "blitzes")),
			DefAirYds:        int(floatFromRow(row, "def_air_yds")),
			DefBattedPasses:  int(floatFromRow(row, "def_batted_passes")),
			DefCmp:           int(floatFromRow(row, "def_cmp")),
			DefCmpPct:        floatFromRow(row, "def_cmp_pct"),
			DefCmpTD:         int(floatFromRow(row, "def_cmp_td")),
			DefCmpYds:        int(floatFromRow(row, "def_cmp_yds")),
			DefInt:           int(floatFromRow(row, "def_int")),
			DefPassRating:    floatFromRow(row, "def_pass_rating"),
			DefTargets:       int(floatFromRow(row, "def_targets")),
			DefTgtYdsPerAtt:  floatFromRow(row, "def_tgt_yds_per_att"),
			DefYAC:           int(floatFromRow(row, "def_yac")),
			DefYdsPerCmp:     floatFromRow(row, "def_yds_per_cmp"),
			DefYdsPerTarget:  floatFromRow(row, "def_yds_per_target"),
			Games:            int(floatFromRow(row, "games")),
			GamesStarted:     int(floatFromRow(row, "games_started")),
			Pressures:        int(floatFromRow(row, "pressures")),
			QBHurry:          int(floatFromRow(row, "qb_hurry")),
			QBKnockdown:      int(floatFromRow(row, "qb_knockdown")),
			Sacks:            floatFromRow(row, "sacks"),
			TacklesCombined:  int(floatFromRow(row, "tackles_combined")),
			TacklesMissed:    int(floatFromRow(row, "tackles_missed")),
			TacklesMissedPct: floatFromRow(row, "tackles_missed_pct"),
		}

		players = append(players, adr)
	}
	return players
}

func sumInt(rows []map[string]any, key string) int {
	sum := 0
	for _, row := range rows {
		if nameDisplay(row) == "" {
			continue
		}
		value, ok := toInt(row[key])
		if ok {
			sum += value
		}
	}
	return sum
}

func scoringTotals(scoringRows, kickingRows []map[string]any) ScoringTotals {
	totals := ScoringTotals{
		RushTD:       sumInt(scoringRows, "rush_td"),
		PassTD:       sumInt(scoringRows, "rec_td"),
		KickRetTD:    sumInt(scoringRows, "kick_ret_td"),
		PuntRetTD:    sumInt(scoringRows, "punt_ret_td"),
		IntRetTD:     sumInt(scoringRows, "def_int_td"),
		TwoPtConv:    sumInt(scoringRows, "two_pt_md"),
		Safety:       sumInt(scoringRows, "safety_md"),
		OtherTD:      sumInt(scoringRows, "other_td"),
		FumblesRecTD: sumInt(scoringRows, "fumbles_rec_td"),
	}
	totals.FieldGoalsMade = sumInt(kickingRows, "fgm")
	totals.FieldGoalsAtt = sumInt(kickingRows, "fga")
	totals.XPMade = sumInt(kickingRows, "xpm")
	totals.XPAtt = sumInt(kickingRows, "xpa")

	if totalTD := sumInt(scoringRows, "total_td"); totalTD > 0 {
		totals.Touchdowns = totalTD
	} else {
		totals.Touchdowns = totals.RushTD + totals.PassTD + totals.KickRetTD + totals.PuntRetTD + totals.IntRetTD + totals.OtherTD + totals.FumblesRecTD
	}
	return totals
}

func sumFloat(rows []map[string]any, key string) float64 {
	sum := 0.0
	for _, row := range rows {
		if nameDisplay(row) == "" {
			continue
		}
		value, ok := toFloat(row[key])
		if ok {
			sum += value
		}
	}
	return sum
}

func maxInt(rows []map[string]any, key string) int {
	max := 0
	for _, row := range rows {
		if nameDisplay(row) == "" {
			continue
		}
		value, ok := toInt(row[key])
		if ok && value > max {
			max = value
		}
	}
	return max
}

func maxGames(groups [][]map[string]any) int {
	max := 0
	for _, rows := range groups {
		for _, row := range rows {
			value, ok := toInt(row["games"])
			if ok && value > max {
				max = value
			}
		}
	}
	return max
}

func nameDisplay(row map[string]any) string {
	value, ok := row["name_display"]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case map[string]any:
		if content, ok := typed["content"].(string); ok {
			return strings.TrimSpace(content)
		}
	case map[string]string:
		if content, ok := typed["content"]; ok {
			return strings.TrimSpace(content)
		}
	case string:
		return strings.TrimSpace(typed)
	}
	return ""
}

func floatFromRow(row map[string]any, key string) float64 {
	value, ok := toFloat(row[key])
	if !ok {
		return 0
	}
	return value
}

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if math.Mod(v, 1) != 0 {
			return 0, false
		}
		return int(v), true
	case float32:
		fv := float64(v)
		if math.Mod(fv, 1) != 0 {
			return 0, false
		}
		return int(fv), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if f, err := v.Float64(); err == nil && math.Mod(f, 1) == 0 {
			return int(f), true
		}
		return 0, false
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		if strings.Contains(trimmed, ".") {
			f, err := strconv.ParseFloat(trimmed, 64)
			if err != nil || math.Mod(f, 1) != 0 {
				return 0, false
			}
			return int(f), true
		}
		i, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func roundFloat(value float64, places int) float64 {
	if places < 0 {
		return value
	}
	pow := math.Pow(10, float64(places))
	return math.Round(value*pow) / pow
}

// buildExtraTables detects and normalizes extra tables not covered by standard mapping.
// Only includes tables in ExtraTablesAllowlist and not in already-rendered core tables.
func buildExtraTables(data pfr.SeasonDataV1_1) []NormalizedTable {
	if data == nil {
		return nil
	}

	// Tables already rendered by standard mapping
	renderedTables := map[string]bool{
		"passing":               true,
		"rushing_and_receiving": true,
		"defense":               true,
		"kicking":               true,
		"punting":               true,
		"returns":               true,
		"scoring":               true,
		"team_conversions":      true,
		"team_td_log":           true,
		"opp_td_log":            true,
		"passing_advanced":      true,
		"adv_rushing":           true,
		"adv_receiving":         true,
		"adv_defense":           true,
	}

	var extras []NormalizedTable

	// Scan all sections for tables
	for _, section := range data {
		if section.Tables == nil {
			continue
		}

		for tableName, tableResult := range section.Tables {
			// Skip if already rendered
			if renderedTables[tableName] {
				continue
			}

			// Skip if not in allowlist or if explicitly skipped
			if !IsRenderableTable(tableName) {
				continue
			}

			allowedCols := GetAllowedColumns(tableName)
			if len(allowedCols) == 0 {
				// Table not in allowlist, skip
				continue
			}

			// Normalize the table
			cols, rows := NormalizeTable(tableName, tableResult.Rows, allowedCols)
			if len(rows) == 0 {
				continue
			}

			normalized := NormalizedTable{
				Name:    tableName,
				TitleES: GetTableTitleES(tableName),
				Columns: cols,
				Rows:    rows,
			}

			extras = append(extras, normalized)
		}
	}

	// Sort by table name for deterministic output
	sort.Slice(extras, func(i, j int) bool {
		return extras[i].Name < extras[j].Name
	})

	// Return nil if no extras found (for template {{- if .ExtraTables }} to work correctly)
	if len(extras) == 0 {
		return nil
	}

	return extras
}
