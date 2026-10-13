package articles

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAuditReportsExistingMissingAndDraftYears(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "2024.md"), "# Temporada real\n")
	writeTestFile(t, filepath.Join(dir, "2025.md"), "---\narticle_status: draft\ndraft: true\n---\n\n# Temporada borrador\n")

	report, err := Audit(AuditOptions{FromYear: 2024, ToYear: 2026, ArticlesDir: dir})
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if !reflect.DeepEqual(report.YearsPresent, []int{2024, 2025}) {
		t.Fatalf("unexpected present years: %#v", report.YearsPresent)
	}
	if !reflect.DeepEqual(report.YearsReal, []int{2024}) {
		t.Fatalf("unexpected real years: %#v", report.YearsReal)
	}
	if !reflect.DeepEqual(report.YearsDraft, []int{2025}) {
		t.Fatalf("unexpected draft years: %#v", report.YearsDraft)
	}
	if !reflect.DeepEqual(report.YearsMissing, []int{2026}) {
		t.Fatalf("unexpected missing years: %#v", report.YearsMissing)
	}
	if report.Present[1].Title != "Temporada borrador" {
		t.Fatalf("unexpected title extraction: %#v", report.Present[1])
	}
}

func TestAuditMissingArticlesDirTreatsRangeAsMissing(t *testing.T) {
	report, err := Audit(AuditOptions{FromYear: 2024, ToYear: 2025, ArticlesDir: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if !reflect.DeepEqual(report.YearsMissing, []int{2024, 2025}) {
		t.Fatalf("unexpected missing years: %#v", report.YearsMissing)
	}
}

func TestGenerateDraftWithMinimalData(t *testing.T) {
	articlesDir := t.TempDir()
	processedDir := t.TempDir()
	gamesDir := filepath.Join(processedDir, "games")
	writeTestFile(t, filepath.Join(processedDir, "2025.json"), seasonFixture())
	writeTestFile(t, filepath.Join(gamesDir, "2025", "202509070clt.json"), "{}")

	results, err := GenerateDrafts(DraftOptions{
		Year:         2025,
		ArticlesDir:  articlesDir,
		ProcessedDir: processedDir,
		GamesDir:     gamesDir,
	})
	if err != nil {
		t.Fatalf("GenerateDrafts failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Fatalf("unexpected results: %#v", results)
	}

	data, err := os.ReadFile(filepath.Join(articlesDir, "2025.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"article_status: draft",
		"draft: true",
		"**Récord:** 7-10-0",
		"**Entrenador:** Mike McDaniel (7-10-0)",
		"**División:** AFC East Division, posición 3",
		"**Pase:** Tua Tagovailoa (2660)",
		"**Carrera:** De'Von Achane (1350)",
		"**Recepción:** Jaylen Waddle (1015)",
		"Chop Robinson (Ronda 1, pick 21, DE, Penn St.)",
		"Calendario local: 2 partidos en la tabla de temporada.",
		"Partidos procesados localmente: 1 JSON",
		"## Pendiente de revisión editorial",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("draft missing %q:\n%s", want, text)
		}
	}
}

func TestGenerateDraftNoOverwriteByDefault(t *testing.T) {
	articlesDir := t.TempDir()
	existing := filepath.Join(articlesDir, "2025.md")
	writeTestFile(t, existing, "# Texto editorial existente\n")

	results, err := GenerateDrafts(DraftOptions{
		Year:        2025,
		ArticlesDir: articlesDir,
	})
	if err != nil {
		t.Fatalf("GenerateDrafts failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != "skipped_existing" {
		t.Fatalf("expected skipped_existing, got %#v", results)
	}
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "# Texto editorial existente\n" {
		t.Fatalf("existing article was overwritten: %s", string(data))
	}
}

func TestGenerateDraftMissingProcessedCreatesExplicitPendingDraft(t *testing.T) {
	articlesDir := t.TempDir()
	results, err := GenerateDrafts(DraftOptions{
		Year:         2026,
		ArticlesDir:  articlesDir,
		ProcessedDir: filepath.Join(t.TempDir(), "processed"),
	})
	if err != nil {
		t.Fatalf("GenerateDrafts failed: %v", err)
	}
	if len(results) != 1 || len(results[0].Warnings) == 0 {
		t.Fatalf("expected warning for missing processed JSON: %#v", results)
	}

	data, err := os.ReadFile(filepath.Join(articlesDir, "2026.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "**Datos estructurados de temporada:** pendiente: no se encontró JSON procesado local") {
		t.Fatalf("missing explicit pending source note:\n%s", text)
	}
}

func TestGenerateDraftFromLegacyTablesAndStaffFallback(t *testing.T) {
	articlesDir := t.TempDir()
	processedDir := t.TempDir()
	gamesDir := filepath.Join(processedDir, "games")
	writeTestFile(t, filepath.Join(processedDir, "2024.json"), legacySeasonFixture())
	writeTestFile(t, filepath.Join(gamesDir, "2024", "202409080mia.json"), "{}")
	writeTestFile(t, filepath.Join(gamesDir, "2024", "202501050mia.json"), "{}")

	results, err := GenerateDrafts(DraftOptions{
		Year:         2024,
		ArticlesDir:  articlesDir,
		ProcessedDir: processedDir,
		GamesDir:     gamesDir,
	})
	if err != nil {
		t.Fatalf("GenerateDrafts failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Fatalf("unexpected results: %#v", results)
	}

	data, err := os.ReadFile(filepath.Join(articlesDir, "2024.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"**Récord:** 11-6-0",
		"**Entrenador:** Coach From Staff",
		"**División:** AFC East Division, posición 2nd",
		"**Pase:** Quarterback One (4,012)",
		"**Anotación:** Kicker One (122)",
		"**Capturas:** Edge One (13.5)",
		"Receiver One (Ronda 1, pick 12, WR, Miami)",
		"Tackle One (Ronda 2, pick 40, OT, Florida)",
		"Playoffs detectados en calendario local: 1 partidos.",
		"Partidos procesados localmente: 2 JSON",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("draft missing %q:\n%s", want, text)
		}
	}
}

func TestGenerateDraftInvalidProcessedJSONCreatesPendingDraft(t *testing.T) {
	articlesDir := t.TempDir()
	processedDir := t.TempDir()
	writeTestFile(t, filepath.Join(processedDir, "2023.json"), "{bad json")

	results, err := GenerateDrafts(DraftOptions{
		Year:         2023,
		ArticlesDir:  articlesDir,
		ProcessedDir: processedDir,
	})
	if err != nil {
		t.Fatalf("GenerateDrafts failed: %v", err)
	}
	if len(results) != 1 || len(results[0].Warnings) == 0 {
		t.Fatalf("expected warning for invalid processed JSON: %#v", results)
	}
	if !strings.Contains(results[0].Warnings[0], "failed to parse processed season JSON") {
		t.Fatalf("expected parse warning in result: %#v", results[0].Warnings)
	}

	data, err := os.ReadFile(filepath.Join(articlesDir, "2023.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "**Datos estructurados de temporada:** pendiente") {
		t.Fatalf("expected pending source note in draft:\n%s", data)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func seasonFixture() string {
	return `{
  "schema": {"name": "mundodolphins.pfr.team-season", "version": "1.3.0"},
  "generated_at": "2026-01-01T00:00:00Z",
  "team": {"id": "mia", "name": "Miami Dolphins"},
  "season": 2025,
  "data": {
    "main": {
      "status": "ok",
      "info": {
        "Record": "7-10-0, 3rd in AFC East Division (Schedule and Results)",
        "Coach": "Mike McDaniel (7-10-0)"
      },
      "division_name": "AFC East Division",
      "division_rank": "3",
      "tables": {
        "passing": {
          "rows": [
            {"name_display": {"content": "Tua Tagovailoa"}, "pass_yds": 2660},
            {"name_display": {"content": "Skylar Thompson"}, "pass_yds": 210}
          ]
        },
        "rushing_and_receiving": {
          "rows": [
            {"name_display": {"content": "De'Von Achane"}, "rush_yds": 1350, "rec_yds": 488},
            {"name_display": {"content": "Jaylen Waddle"}, "rush_yds": 20, "rec_yds": 1015}
          ]
        },
        "defense": {
          "rows": [
            {"name_display": {"content": "Jordyn Brooks"}, "tackles_combined": 183, "sacks": 3.5},
            {"name_display": {"content": "Zach Sieler"}, "tackles_combined": 55, "sacks": 10}
          ]
        },
        "games": {
          "rows": [
            {"week_num": "1", "game_date": "September 7"},
            {"week_num": "2", "game_date": "September 14"}
          ]
        }
      }
    },
    "draft": {
      "derived": {
        "draftees": [
          {"round": 1, "pick_overall": 21, "player": "Chop Robinson", "position": "DE", "college": "Penn St."}
        ]
      }
    }
  }
}`
}

func legacySeasonFixture() string {
	return `{
  "schema": {"name": "mundodolphins.pfr.team-season", "version": "1.2.0"},
  "generated_at": "2026-01-01T00:00:00Z",
  "team": {"id": "mia", "name": "Miami Dolphins"},
  "season": "2024",
  "data": {
    "main": {
      "status": "ok",
      "info": {
        "record": "11-6-0, 2nd in AFC East Division (Schedule and Results)"
      },
      "staff": [
        {"role": "Head Coach", "name": {"content": "Coach From Staff"}}
      ]
    },
    "tables": {
      "passing": [
        {"player": {"content": "Quarterback One"}, "yds": "4,012"}
      ],
      "rushing_and_receiving": {
        "rows": [
          {"Player": "Running Back One", "rush_yds": 980, "rec_yds": 100},
          {"Player": "Receiver One", "rush_yds": 30, "rec_yds": 1184}
        ]
      },
      "scoring": [
        {"name_display": "Kicker One", "pts": 122}
      ],
      "defense": [
        {"name": "Edge One", "sacks": 13.5}
      ],
      "draft_picks": [
        {"Round": "2", "Pick": "40", "Player": "Tackle One", "Pos": "OT", "College": "Florida"},
        {"Round": "1", "Pick": "12", "Player": "Receiver One", "Pos": "WR", "College": "Miami"}
      ],
      "games": [
        {"Week": "1", "Date": "September 8"},
        {"Week": "Wild Card", "Date": "January 5", "game_type": "playoff"},
        {"Week": "", "Date": ""}
      ]
    }
  }
}`
}
