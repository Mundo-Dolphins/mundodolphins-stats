package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExtractArticleTitleAndBody_WithHeading(t *testing.T) {
	md := "# Temporada 2025\n\nLinea 1\nLinea 2\n"
	title, body := extractArticleTitleAndBody(md, 2025)

	if title != "Temporada 2025" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(body, "Linea 1") || !strings.Contains(body, "Linea 2") {
		t.Fatalf("unexpected body: %q", body)
	}
	if strings.HasPrefix(body, "#") {
		t.Fatalf("body should not include heading: %q", body)
	}
}

func TestExtractArticleTitleAndBody_NoHeadingFallback(t *testing.T) {
	md := "\n\nContenido sin encabezado\n"
	title, body := extractArticleTitleAndBody(md, 2025)

	if title != "2025" {
		t.Fatalf("expected fallback title=year, got %q", title)
	}
	if body != "Contenido sin encabezado\n" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestSeasonStartDateFromGames_DefaultWhenNoFiles(t *testing.T) {
	got := seasonStartDateFromGames(2025, t.TempDir())
	if got != "2025-01-01" {
		t.Fatalf("unexpected fallback date: %q", got)
	}
}

func TestSeasonStartDateFromGames_FirstDateFromFiles(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"202509290mia.json",
		"202509070clt.json",
		"202512280mia.json",
		"not_a_game.json",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile failed for %s: %v", name, err)
		}
	}

	got := seasonStartDateFromGames(2025, dir)
	if got != "2025-09-07" {
		t.Fatalf("unexpected season start date: %q", got)
	}
}

func TestGenerateSeasonArticle_NotFoundReturnsEmpty(t *testing.T) {
	articlesDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	gamesDir := t.TempDir()

	out, err := generateSeasonArticle(2025, articlesDir, outDir, gamesDir)
	if err != nil {
		t.Fatalf("expected nil error for missing source article: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty out path for missing source article, got %q", out)
	}
}

func TestGenerateSeasonArticle_WritesFrontmatterAndFooter(t *testing.T) {
	articlesDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	gamesDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(gamesDir, "202509070clt.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile game failed: %v", err)
	}

	src := "# Dolphins 2025\n\nResumen de temporada\n"
	if err := os.WriteFile(filepath.Join(articlesDir, "2025.md"), []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile source article failed: %v", err)
	}

	outPath, err := generateSeasonArticle(2025, articlesDir, outDir, gamesDir)
	if err != nil {
		t.Fatalf("generateSeasonArticle failed: %v", err)
	}
	if outPath == "" {
		t.Fatalf("expected non-empty out path")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile output failed: %v", err)
	}
	text := string(data)

	if !strings.Contains(text, `title: "Dolphins 2025"`) {
		t.Fatalf("missing title frontmatter: %s", text)
	}
	if !strings.Contains(text, "date: 2025-09-07") {
		t.Fatalf("missing computed date from games: %s", text)
	}
	if !strings.Contains(text, "[Volver](/historia/2025)") {
		t.Fatalf("missing footer backlink: %s", text)
	}
}

func TestGenerateSeasonArticle_DoesNotDuplicateFooter(t *testing.T) {
	articlesDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	gamesDir := t.TempDir()

	src := "# Dolphins 2025\n\nTexto\n\n[Volver](/historia/2025)\n"
	if err := os.WriteFile(filepath.Join(articlesDir, "2025.md"), []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile source article failed: %v", err)
	}

	outPath, err := generateSeasonArticle(2025, articlesDir, outDir, gamesDir)
	if err != nil {
		t.Fatalf("generateSeasonArticle failed: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile output failed: %v", err)
	}
	text := string(data)

	if strings.Count(text, "[Volver](/historia/2025)") != 1 {
		t.Fatalf("footer should appear exactly once: %s", text)
	}
}

func TestRunArticlesMissingPrintsAuditJSON(t *testing.T) {
	restore := saveArticleFlagState()
	defer restore()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "2025.md"), []byte("# Temporada 2025\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	articlesMissingFromYear = 2025
	articlesMissingToYear = 2026
	articlesMissingDir = dir

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "missing"}
	cmd.SetOut(&out)

	if err := runArticlesMissing(cmd, nil); err != nil {
		t.Fatalf("runArticlesMissing failed: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, `"years_present": [
    2025
  ]`) {
		t.Fatalf("missing present year in output: %s", text)
	}
	if !strings.Contains(text, `"years_missing": [
    2026
  ]`) {
		t.Fatalf("missing missing year in output: %s", text)
	}
}

func TestRunArticlesMissingReturnsInvalidRangeError(t *testing.T) {
	restore := saveArticleFlagState()
	defer restore()

	articlesMissingFromYear = 2026
	articlesMissingToYear = 2025
	articlesMissingDir = t.TempDir()

	cmd := &cobra.Command{Use: "missing"}
	if err := runArticlesMissing(cmd, nil); err == nil {
		t.Fatalf("expected invalid range error")
	}
}

func TestRunArticlesDraftPrintsResultAndWritesDraft(t *testing.T) {
	restore := saveArticleFlagState()
	defer restore()

	articlesDir := t.TempDir()
	processedDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(processedDir, "2025.json"), []byte(`{
		"season": 2025,
		"data": {
			"main": {
				"info": {"Record": "7-10-0", "Coach": "Mike McDaniel"},
				"tables": {}
			}
		}
	}`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	articlesDraftYear = 2025
	articlesDraftFromYear = 1966
	articlesDraftToYear = 2026
	articlesDraftDir = articlesDir
	articlesDraftProcessed = processedDir
	articlesDraftGamesDir = filepath.Join(processedDir, "games")
	articlesDraftForce = false

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "draft"}
	cmd.SetOut(&out)

	if err := runArticlesDraft(cmd, nil); err != nil {
		t.Fatalf("runArticlesDraft failed: %v", err)
	}
	if !strings.Contains(out.String(), `"status": "created"`) {
		t.Fatalf("missing created status in output: %s", out.String())
	}

	data, err := os.ReadFile(filepath.Join(articlesDir, "2025.md"))
	if err != nil {
		t.Fatalf("ReadFile draft failed: %v", err)
	}
	if !strings.Contains(string(data), "article_status: draft") {
		t.Fatalf("generated article is not marked as draft: %s", string(data))
	}
}

func TestRunArticlesDraftReturnsInvalidRangeError(t *testing.T) {
	restore := saveArticleFlagState()
	defer restore()

	articlesDraftYear = 0
	articlesDraftFromYear = 2026
	articlesDraftToYear = 2025
	articlesDraftDir = t.TempDir()
	articlesDraftProcessed = t.TempDir()

	cmd := &cobra.Command{Use: "draft"}
	if err := runArticlesDraft(cmd, nil); err == nil {
		t.Fatalf("expected invalid range error")
	}
}

func saveArticleFlagState() func() {
	oldMissingFromYear := articlesMissingFromYear
	oldMissingToYear := articlesMissingToYear
	oldMissingDir := articlesMissingDir
	oldDraftYear := articlesDraftYear
	oldDraftFromYear := articlesDraftFromYear
	oldDraftToYear := articlesDraftToYear
	oldDraftDir := articlesDraftDir
	oldDraftProcessed := articlesDraftProcessed
	oldDraftGamesDir := articlesDraftGamesDir
	oldDraftForce := articlesDraftForce

	return func() {
		articlesMissingFromYear = oldMissingFromYear
		articlesMissingToYear = oldMissingToYear
		articlesMissingDir = oldMissingDir
		articlesDraftYear = oldDraftYear
		articlesDraftFromYear = oldDraftFromYear
		articlesDraftToYear = oldDraftToYear
		articlesDraftDir = oldDraftDir
		articlesDraftProcessed = oldDraftProcessed
		articlesDraftGamesDir = oldDraftGamesDir
		articlesDraftForce = oldDraftForce
	}
}
