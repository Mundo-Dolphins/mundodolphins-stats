package videos

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSONGameLoaderLoadGames(t *testing.T) {
	dir := t.TempDir()
	data := `{"game_id":"196612180mia","data":{"week":"15","scorebox":{"teams":[{"name":{"content":"Houston Oilers"}},{"name":{"content":"Miami Dolphins"}}],"meta":{"date":"Sunday Dec 18, 1966"}}}}`
	path := filepath.Join(dir, "196612180mia.json")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	loader := NewJSONGameLoader(dir)
	games, err := loader.LoadGames(1966, 1966)
	if err != nil {
		t.Fatalf("LoadGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].AwayTeam != "Houston Oilers" || games[0].HomeTeam != "Miami Dolphins" {
		t.Fatalf("unexpected teams: %+v", games[0])
	}
	if games[0].Week != "15" {
		t.Fatalf("unexpected week: %+v", games[0])
	}
}

func TestJSONGameLoaderInvalidDir(t *testing.T) {
	loader := NewJSONGameLoader("")
	if _, err := loader.LoadGames(0, 0); err == nil {
		t.Fatalf("expected error for empty dir")
	}
}

func TestJSONGameLoaderYearFilter(t *testing.T) {
	dir := t.TempDir()
	data := `{"game_id":"196612180mia","data":{"scorebox":{"teams":[{"name":{"content":"Away"}},{"name":{"content":"Home"}}],"meta":{"date":"Sunday Dec 18, 1966"}}}}`
	path := filepath.Join(dir, "196612180mia.json")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	loader := NewJSONGameLoader(dir)
	games, err := loader.LoadGames(1967, 1968)
	if err != nil {
		t.Fatalf("LoadGames error: %v", err)
	}
	if len(games) != 0 {
		t.Fatalf("expected no games in range, got %d", len(games))
	}
}

func TestJSONGameLoaderLoadGamesFromSeasonSubdirectories(t *testing.T) {
	dir := t.TempDir()
	seasonDir := filepath.Join(dir, "2013")
	if err := os.MkdirAll(seasonDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data := `{"game_id":"201309300nor","data":{"scorebox":{"teams":[{"name":{"content":"Miami Dolphins"}},{"name":{"content":"New Orleans Saints"}}],"meta":{"date":"Monday Sep 30, 2013"}}}}`
	path := filepath.Join(seasonDir, "201309300nor.json")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	loader := NewJSONGameLoader(dir)
	games, err := loader.LoadGames(2013, 2013)
	if err != nil {
		t.Fatalf("LoadGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].GameID != "201309300nor" {
		t.Fatalf("unexpected game id: %+v", games[0])
	}
}

func TestJSONGameLoaderFillsMissingGameDataFromSeasonIndex(t *testing.T) {
	root := t.TempDir()
	gamesDir := filepath.Join(root, "games", "2026")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("mkdir games: %v", err)
	}
	writeFile(t, filepath.Join(root, "2026.json"), `{
	  "data":{"footballdb":{"game_index":{"games":[
	    {"date":"2026-09-13","away_team":"Miami Dolphins","home_team":"Las Vegas Raiders"},
	    {"date":"2026-11-08","away_team":"Detroit Lions","home_team":"Miami Dolphins"}
	  ]}}}
	}`)
	writeFile(t, filepath.Join(gamesDir, "202611080mia.json"), `{"game_id":"202611080mia","data":{"youtube":{"status":"unknown"},"tables":{}}}`)

	loader := NewJSONGameLoader(filepath.Join(root, "games"))
	games, err := loader.LoadGames(2026, 2026)
	if err != nil {
		t.Fatalf("LoadGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d: %+v", len(games), games)
	}
	if games[0].AwayTeam != "Detroit Lions" || games[0].HomeTeam != "Miami Dolphins" {
		t.Fatalf("expected fallback teams, got %+v", games[0])
	}
	if games[0].DateRaw != "2026-11-08" || games[0].Week != "9" {
		t.Fatalf("expected fallback date/week, got %+v", games[0])
	}
}

func TestJSONGameLoaderUsesLinescoreTeamsWhenScoreboxTeamsMissing(t *testing.T) {
	dir := t.TempDir()
	data := `{
	  "game_id":"197201160dal",
	  "data":{
	    "week":"Playoffs",
	    "scorebox":{"teams":null,"meta":{"date":"Sunday Jan 16, 1972"}},
	    "linescore":{"teams":[
	      {"team":{"content":"Dallas Cowboys"}},
	      {"team":{"content":"Miami Dolphins"}}
	    ]}
	  }
	}`
	writeFile(t, filepath.Join(dir, "197201160dal.json"), data)

	loader := NewJSONGameLoader(dir)
	games, err := loader.LoadGames(1971, 1972)
	if err != nil {
		t.Fatalf("LoadGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].AwayTeam != "Dallas Cowboys" || games[0].HomeTeam != "Miami Dolphins" {
		t.Fatalf("expected linescore teams, got %+v", games[0])
	}
	if games[0].Week != "Playoffs" || games[0].DateRaw != "Sunday Jan 16, 1972" {
		t.Fatalf("expected original week/date, got %+v", games[0])
	}
}

func TestJSONGameLoaderFiltersBySeasonDirectoryForJanuaryGames(t *testing.T) {
	root := t.TempDir()
	gamesDir := filepath.Join(root, "games", "2026")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatalf("mkdir games: %v", err)
	}
	writeFile(t, filepath.Join(root, "2026.json"), `{
	  "data":{"footballdb":{"game_index":{"games":[
	    {"date":"2026-09-13","away_team":"Miami Dolphins","home_team":"Las Vegas Raiders"},
	    {"date":"2027-01-10","away_team":"Miami Dolphins","home_team":"New England Patriots"}
	  ]}}}
	}`)
	writeFile(t, filepath.Join(gamesDir, "202701100nwe.json"), `{"game_id":"202701100nwe","data":{"youtube":{"status":"unknown"},"tables":{}}}`)

	loader := NewJSONGameLoader(filepath.Join(root, "games"))
	games, err := loader.LoadGames(2026, 2026)
	if err != nil {
		t.Fatalf("LoadGames error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected January game in 2026 season, got %d: %+v", len(games), games)
	}
	if games[0].GameID != "202701100nwe" || games[0].Year != 2026 {
		t.Fatalf("unexpected January game ref: %+v", games[0])
	}
	if games[0].AwayTeam != "Miami Dolphins" || games[0].HomeTeam != "New England Patriots" {
		t.Fatalf("expected fallback teams, got %+v", games[0])
	}
}

func TestParseYearFromGameID(t *testing.T) {
	if parseYearFromGameID("196612180mia") != 1966 {
		t.Fatalf("expected 1966")
	}
	if parseYearFromGameID("bad") != 0 {
		t.Fatalf("expected 0 for short id")
	}
}

func TestGameTeamsEmpty(t *testing.T) {
	var teams gameTeams
	if teams.awayName() != "" || teams.homeName() != "" {
		t.Fatalf("expected empty names")
	}
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
