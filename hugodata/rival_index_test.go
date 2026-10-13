package hugodata

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

func TestRivalIndexBuilderBuildFromDir(t *testing.T) {
	processedDir := t.TempDir()
	writeRivalTestGame(t, processedDir, 2024, rivalTestGame(
		"202409120mia",
		"September 12, 2024",
		"2",
		rivalTestTeam("buf", "Buffalo Bills", 24),
		rivalTestTeam("mia", "Miami Dolphins", 17),
	))
	writeRivalTestGame(t, processedDir, 2024, rivalTestGame(
		"202411100mia",
		"November 10, 2024",
		"10",
		rivalTestTeam("nyj", "New York Jets", 20),
		rivalTestTeam("mia", "Miami Dolphins", 20),
	))
	writeRivalTestGame(t, processedDir, 2025, rivalTestGame(
		"202601100mia",
		"Saturday Jan 10, 2026",
		"Wild Card",
		rivalTestTeam("buf", "Buffalo Bills", 28),
		rivalTestTeam("mia", "Miami Dolphins", 31),
	))

	builder := NewRivalIndexBuilder(processedDir)
	builder.SetTime(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	doc, err := builder.BuildFromDir(2024, 2025)
	if err != nil {
		t.Fatalf("BuildFromDir failed: %v", err)
	}

	data, err := EncodeRivalIndexToJSON(doc)
	if err != nil {
		t.Fatalf("EncodeRivalIndexToJSON failed: %v", err)
	}

	const expected = `{
  "schema": {
    "name": "mundodolphins.hugo.rival-index",
    "version": "1.0.0"
  },
  "generated_at": "2026-08-01T12:00:00Z",
  "data": {
    "team": {
      "id": "mia",
      "name": "Miami Dolphins"
    },
    "range": {
      "from_season": 2024,
      "to_season": 2025
    },
    "totals": {
      "opponents": 2,
      "games": 3,
      "wins": 1,
      "losses": 1,
      "ties": 1,
      "points_for": 68,
      "points_against": 72,
      "point_differential": -4,
      "regular_season_games": 2,
      "playoff_games": 1
    },
    "rivals": [
      {
        "opponent": {
          "id": "buf",
          "name": "Buffalo Bills"
        },
        "summary": {
          "games": 2,
          "wins": 1,
          "losses": 1,
          "ties": 0,
          "points_for": 48,
          "points_against": 52,
          "point_differential": -4,
          "regular_season_games": 1,
          "playoff_games": 1,
          "first_game": "2024-09-12",
          "last_game": "2026-01-10"
        },
        "by_season": [
          {
            "season": 2024,
            "games": 1,
            "wins": 0,
            "losses": 1,
            "ties": 0,
            "points_for": 17,
            "points_against": 24,
            "point_differential": -7,
            "regular_season_games": 1,
            "playoff_games": 0
          },
          {
            "season": 2025,
            "games": 1,
            "wins": 1,
            "losses": 0,
            "ties": 0,
            "points_for": 31,
            "points_against": 28,
            "point_differential": 3,
            "regular_season_games": 0,
            "playoff_games": 1
          }
        ],
        "latest_games": [
          {
            "game_id": "202601100mia",
            "season": 2025,
            "date": "2026-01-10",
            "week": "Wild Card",
            "game_type": "playoffs",
            "home_away": "home",
            "opponent_id": "buf",
            "opponent_name": "Buffalo Bills",
            "points_for": 31,
            "points_against": 28,
            "result": "W",
            "margin": 3,
            "path": "historia/partidos/buf-mia-20260110"
          },
          {
            "game_id": "202409120mia",
            "season": 2024,
            "date": "2024-09-12",
            "week": "2",
            "game_type": "regular_season",
            "home_away": "home",
            "opponent_id": "buf",
            "opponent_name": "Buffalo Bills",
            "points_for": 17,
            "points_against": 24,
            "result": "L",
            "margin": -7,
            "path": "historia/partidos/buf-mia-20240912"
          }
        ]
      },
      {
        "opponent": {
          "id": "nyj",
          "name": "New York Jets"
        },
        "summary": {
          "games": 1,
          "wins": 0,
          "losses": 0,
          "ties": 1,
          "points_for": 20,
          "points_against": 20,
          "point_differential": 0,
          "regular_season_games": 1,
          "playoff_games": 0,
          "first_game": "2024-11-10",
          "last_game": "2024-11-10"
        },
        "by_season": [
          {
            "season": 2024,
            "games": 1,
            "wins": 0,
            "losses": 0,
            "ties": 1,
            "points_for": 20,
            "points_against": 20,
            "point_differential": 0,
            "regular_season_games": 1,
            "playoff_games": 0
          }
        ],
        "latest_games": [
          {
            "game_id": "202411100mia",
            "season": 2024,
            "date": "2024-11-10",
            "week": "10",
            "game_type": "regular_season",
            "home_away": "home",
            "opponent_id": "nyj",
            "opponent_name": "New York Jets",
            "points_for": 20,
            "points_against": 20,
            "result": "T",
            "margin": 0,
            "path": "historia/partidos/nyj-mia-20241110"
          }
        ]
      }
    ]
  }
}`

	if string(data) != expected {
		t.Fatalf("unexpected rival index JSON:\n%s", data)
	}
}

func TestGenerateRivalIndexDataWritesFile(t *testing.T) {
	processedDir := t.TempDir()
	outputDir := t.TempDir()
	writeRivalTestGame(t, processedDir, 2024, rivalTestGame(
		"202409120mia",
		"September 12, 2024",
		"2",
		rivalTestTeam("buf", "Buffalo Bills", 24),
		rivalTestTeam("mia", "Miami Dolphins", 17),
	))

	err := GenerateRivalIndexData(context.Background(), processedDir, outputDir, 2024, 2024)
	if err != nil {
		t.Fatalf("GenerateRivalIndexData failed: %v", err)
	}

	path := filepath.Join(outputDir, "data", "historical", "rivals", "index.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected rival index file: %v", err)
	}
}

func TestRivalIndexUsesLatestFranchiseName(t *testing.T) {
	processedDir := t.TempDir()
	writeRivalTestGame(t, processedDir, 1972, rivalTestGame(
		"197210220mia",
		"October 22, 1972",
		"6",
		rivalTestTeam("clt", "Baltimore Colts", 0),
		rivalTestTeam("mia", "Miami Dolphins", 23),
	))
	writeRivalTestGame(t, processedDir, 2025, rivalTestGame(
		"202509070clt",
		"September 7, 2025",
		"1",
		rivalTestTeam("mia", "Miami Dolphins", 8),
		rivalTestTeam("clt", "Indianapolis Colts", 33),
	))

	doc, err := NewRivalIndexBuilder(processedDir).BuildFromDir(1972, 2025)
	if err != nil {
		t.Fatalf("BuildFromDir failed: %v", err)
	}
	if len(doc.Data.Rivals) != 1 {
		t.Fatalf("expected 1 rival, got %d", len(doc.Data.Rivals))
	}
	rival := doc.Data.Rivals[0]
	if rival.Opponent.Name != "Indianapolis Colts" {
		t.Fatalf("opponent name = %q, want Indianapolis Colts", rival.Opponent.Name)
	}
	if rival.Summary.FirstGame != "1972-10-22" || rival.Summary.LastGame != "2025-09-07" {
		t.Fatalf("unexpected first/last game: %+v", rival.Summary)
	}
}

func writeRivalTestGame(t *testing.T, processedDir string, season int, env boxscore.BoxscoreEnvelope) {
	t.Helper()
	dir := filepath.Join(processedDir, "games", strconv.Itoa(season))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, env.GameID+".json"), data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func rivalTestGame(gameID, date, week string, visitor, home boxscore.ScoreboxTeam) boxscore.BoxscoreEnvelope {
	return boxscore.BoxscoreEnvelope{
		Schema: boxscore.SchemaInfo{
			Name:    boxscore.BoxscoreSchemaName,
			Version: boxscore.BoxscoreSchemaVersion,
		},
		GameID: gameID,
		Data: boxscore.BoxscoreData{
			Scorebox: &boxscore.Scorebox{
				Teams: []boxscore.ScoreboxTeam{visitor, home},
				Meta:  boxscore.ScoreboxMeta{Date: date},
			},
			Week:    week,
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  map[string]boxscore.TableResult{},
		},
	}
}

func rivalTestTeam(id, name string, score int) boxscore.ScoreboxTeam {
	return boxscore.ScoreboxTeam{
		Abbr:  id,
		Name:  &boxscore.LinkContent{Content: name, Link: "https://www.pro-football-reference.com/teams/" + id + "/2024.htm"},
		Score: score,
	}
}
