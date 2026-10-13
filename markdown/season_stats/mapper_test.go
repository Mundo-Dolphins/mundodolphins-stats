package season_stats

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mundo-Dolphins/mundodolphins-stats/pfr"
)

func TestMapToViewModel_2022(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("fixtures", "2022.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	view, err := MapToViewModel(2022, data)
	if err != nil {
		t.Fatalf("MapToViewModel: %v", err)
	}

	if view.Year != 2022 {
		t.Fatalf("expected year 2022, got %d", view.Year)
	}

	passing := view.TeamPassing
	if passing.Cmp != 368 || passing.Att != 584 || passing.Yds != 4765 || passing.TD != 30 || passing.Int != 15 || passing.Sacks != 34 {
		t.Fatalf("unexpected passing totals: %+v", passing)
	}
	assertFloat(t, passing.Pct, 63.0, "passing pct")
	assertFloat(t, passing.YPA, 8.2, "passing ypa")

	rushing := view.TeamRushing
	if rushing.Att != 191 || rushing.Yds != 949 || rushing.TD != 4 || rushing.Long != 67 {
		t.Fatalf("unexpected rushing totals: %+v", rushing)
	}
	assertFloat(t, rushing.YPA, 4.97, "rushing ypa")
	assertFloat(t, rushing.YPG, 55.8, "rushing ypg")

	receiving := view.TeamReceiving
	if receiving.Rec != 225 || receiving.Yds != 3268 || receiving.TD != 17 || receiving.Long != 84 {
		t.Fatalf("unexpected receiving totals: %+v", receiving)
	}
	assertFloat(t, receiving.YPR, 14.52, "receiving ypr")
	assertFloat(t, receiving.YPG, 192.2, "receiving ypg")

	defense := view.TeamDefense
	if defense.Int != 2 || defense.IntYds != 64 || defense.IntTD != 0 || defense.SackYds != 0 || defense.Tackles != 147 || defense.Assists != 53 || defense.Total != 200 {
		t.Fatalf("unexpected defense totals: %+v", defense)
	}
	assertFloat(t, defense.Sacks, 5.5, "defense sacks")

	special := view.TeamSpecialTeams
	if special.KickRetYds != 20 || special.KickRetLong != 12 || special.KickRetTD != 0 {
		t.Fatalf("unexpected kickoff returns: %+v", special)
	}
	assertFloat(t, special.KickRetAvg, 10, "kick return avg")
	if special.PuntRetYds != 14 || special.PuntRetLong != 8 || special.PuntRetTD != 0 {
		t.Fatalf("unexpected punt returns: %+v", special)
	}
	assertFloat(t, special.PuntRetAvg, 2.33, "punt return avg")
	if special.PuntYds != 2828 || special.PuntLong != 66 || special.PuntBlocked != 0 || special.Touchbacks != 2 || special.In20 != 28 {
		t.Fatalf("unexpected punting totals: %+v", special)
	}
	assertFloat(t, special.PuntAvg, 46.4, "punt avg")
	assertFloat(t, special.NetAvg, 40.6, "punt net avg")

	scoring := view.TeamScoring.Totals
	if scoring.Touchdowns != 17 || scoring.RushTD != 1 || scoring.PassTD != 15 || scoring.KickRetTD != 0 || scoring.PuntRetTD != 0 || scoring.IntRetTD != 0 {
		t.Fatalf("unexpected scoring totals: %+v", scoring)
	}
	if scoring.FieldGoalsMade != 26 || scoring.FieldGoalsAtt != 32 || scoring.XPMade != 41 || scoring.XPAtt != 44 {
		t.Fatalf("unexpected kicking totals: %+v", scoring)
	}
	if scoring.TwoPtConv != 0 || scoring.Safety != 0 || scoring.OtherTD != 0 || scoring.FumblesRecTD != 1 {
		t.Fatalf("unexpected misc scoring totals: %+v", scoring)
	}
}

func TestHelpers_ToInt_ToFloat(t *testing.T) {
	cases := []struct {
		name      string
		value     any
		wantInt   int
		okInt     bool
		wantFloat float64
		okFloat   bool
	}{
		{name: "nil", value: nil, okInt: false, okFloat: false},
		{name: "empty string", value: "", okInt: false, okFloat: false},
		{name: "int", value: 7, wantInt: 7, okInt: true, wantFloat: 7, okFloat: true},
		{name: "float integer", value: 7.0, wantInt: 7, okInt: true, wantFloat: 7, okFloat: true},
		{name: "float decimal", value: 7.5, okInt: false, wantFloat: 7.5, okFloat: true},
		{name: "string int", value: "12", wantInt: 12, okInt: true, wantFloat: 12, okFloat: true},
		{name: "string float", value: "12.5", okInt: false, wantFloat: 12.5, okFloat: true},
		{name: "json number", value: json.Number("33"), wantInt: 33, okInt: true, wantFloat: 33, okFloat: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotInt, okInt := toInt(tc.value)
			if okInt != tc.okInt || (okInt && gotInt != tc.wantInt) {
				t.Fatalf("toInt(%v) = %d, %v", tc.value, gotInt, okInt)
			}
			gotFloat, okFloat := toFloat(tc.value)
			if okFloat != tc.okFloat || (okFloat && math.Abs(gotFloat-tc.wantFloat) > 0.0001) {
				t.Fatalf("toFloat(%v) = %f, %v", tc.value, gotFloat, okFloat)
			}
		})
	}
}

func assertFloat(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("%s: expected %.3f got %.3f", label, want, got)
	}
}
func TestMapPlayerKicking(t *testing.T) {
	rows := []map[string]any{
		{
			"name_display": map[string]any{"content": "Jason Sanders"},
			"fgm":          float64(26),
			"fga":          float64(32),
			"fgm_long":     float64(49),
			"xpm":          float64(41),
			"xpa":          float64(44),
			"kickoff":      float64(43),
			"kickoff_yds":  float64(2679),
			"kickoff_tb":   float64(2),
		},
	}

	players := mapPlayerKicking(rows)
	if len(players) != 1 {
		t.Fatalf("expected 1 kicker, got %d", len(players))
	}

	p := players[0]
	if p.Name != "Jason Sanders" || p.FGM != 26 || p.FGA != 32 || p.FGLong != 49 {
		t.Fatalf("unexpected kicker data: %+v", p)
	}
	assertFloat(t, p.FGPct, 81.30, "kicking FG%")
	assertFloat(t, p.XPPct, 93.20, "kicking XP%")
}

func TestMapPlayerPunting(t *testing.T) {
	rows := []map[string]any{
		{
			"name_display": map[string]any{"content": "Jason Moore"},
			"punt":         float64(48),
			"punt_yds":     float64(2328),
			"punt_long":    float64(63),
			"punt_in_20":   float64(14),
			"punt_tb":      float64(2),
			"punt_blocked": float64(0),
			"punt_net_yds": float64(2100),
		},
	}

	players := mapPlayerPunting(rows)
	if len(players) != 1 {
		t.Fatalf("expected 1 punter, got %d", len(players))
	}

	p := players[0]
	if p.Name != "Jason Moore" || p.Punts != 48 || p.Yds != 2328 {
		t.Fatalf("unexpected punter data: %+v", p)
	}
	assertFloat(t, p.YdsPerPun, 48.5, "punting yards per punt")
}

func TestMapPlayerReturns(t *testing.T) {
	rows := []map[string]any{
		{
			"name_display":  map[string]any{"content": "Jaylen Waddle"},
			"kick_ret":      float64(1),
			"kick_ret_yds":  float64(25),
			"kick_ret_long": float64(25),
			"kick_ret_td":   float64(0),
			"punt_ret":      float64(0),
			"punt_ret_yds":  float64(0),
			"punt_ret_long": float64(0),
			"punt_ret_td":   float64(0),
		},
	}

	players := mapPlayerReturns(rows)
	if len(players) != 1 {
		t.Fatalf("expected 1 returner, got %d", len(players))
	}

	p := players[0]
	if p.Name != "Jaylen Waddle" || p.KRet != 1 || p.KRetYds != 25 {
		t.Fatalf("unexpected returner data: %+v", p)
	}
}

func TestMapTeamConversions(t *testing.T) {
	rows := []map[string]any{
		{
			"third_down_success":  float64(95),
			"third_down_att":      float64(234),
			"fourth_down_success": float64(12),
			"fourth_down_att":     float64(22),
			"redzone_scores":      float64(47),
			"redzone_att":         float64(59),
		},
	}

	tc := mapTeamConversions(rows)
	if tc.ThirdDownSuccess != 95 || tc.ThirdDownAtt != 234 {
		t.Fatalf("unexpected conversions data: %+v", tc)
	}
	assertFloat(t, tc.ThirdDownPct, 40.60, "3rd down conversion %")
}

func TestMapTDLog(t *testing.T) {
	rows := []map[string]any{
		{
			"player":      "Mike McDaniel",
			"quarter":     "1",
			"time":        "8:30",
			"distance":    "5",
			"td_type":     "Pass",
			"description": "Tua Tagovailoa to Jaylen Waddle",
		},
	}

	entries := mapTDLog(rows)
	if len(entries) != 1 {
		t.Fatalf("expected 1 TD log entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Player != "Mike McDaniel" || e.Quarter != "1" || e.Type != "Pass" {
		t.Fatalf("unexpected TD log entry: %+v", e)
	}
}

func TestNameDisplay(t *testing.T) {
	tests := []struct {
		name string
		row  map[string]any
		want string
	}{
		{"string value", map[string]any{"name_display": "Tua Tagovailoa"}, "Tua Tagovailoa"},
		{"map content", map[string]any{"name_display": map[string]any{"content": "Tyreek Hill"}}, "Tyreek Hill"},
		{"nil value", map[string]any{"name_display": nil}, ""},
		{"missing key", map[string]any{}, ""},
		{"empty string", map[string]any{"name_display": ""}, ""},
		{"map with empty content", map[string]any{"name_display": map[string]any{"content": ""}}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nameDisplay(tt.row)
			if got != tt.want {
				t.Errorf("nameDisplay(%v) = %q, want %q", tt.row, got, tt.want)
			}
		})
	}
}

func TestTableRows(t *testing.T) {
	tables := map[string]pfr.TableResult{
		"passing": {Rows: []map[string]any{{"name": "Player1"}}},
	}
	rows := tableRows(tables, "passing")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	rows = tableRows(tables, "nonexistent")
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for nonexistent table, got %d", len(rows))
	}

	rows = tableRows(nil, "passing")
	if rows != nil {
		t.Fatalf("expected nil for nil tables, got %v", rows)
	}
}
