package cli

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	articleops "github.com/Mundo-Dolphins/mundodolphins-stats/articles"
	"github.com/spf13/cobra"
)

var articlesCmd = &cobra.Command{
	Use:   "articles",
	Short: "Audit and generate season editorial articles",
	Long:  "Audit source season articles and generate deterministic Spanish draft articles from local structured data.",
}

var articlesMissingCmd = &cobra.Command{
	Use:   "missing",
	Short: "List present, draft, real, and missing season articles",
	RunE:  runArticlesMissing,
}

var articlesDraftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Generate deterministic draft season articles",
	RunE:  runArticlesDraft,
}

var (
	articlesMissingFromYear int
	articlesMissingToYear   int
	articlesMissingDir      string
	articlesDraftYear       int
	articlesDraftFromYear   int
	articlesDraftToYear     int
	articlesDraftDir        string
	articlesDraftProcessed  string
	articlesDraftGamesDir   string
	articlesDraftForce      bool
)

func init() {
	rootCmd.AddCommand(articlesCmd)

	articlesMissingCmd.Flags().IntVar(&articlesMissingFromYear, "from", 1966, "Start season year")
	articlesMissingCmd.Flags().IntVar(&articlesMissingToYear, "to", 2026, "End season year")
	articlesMissingCmd.Flags().StringVar(&articlesMissingDir, "articles-dir", "articles", "Directory with source season articles markdown")
	articlesCmd.AddCommand(articlesMissingCmd)

	articlesDraftCmd.Flags().IntVar(&articlesDraftYear, "year", 0, "Single season year to draft")
	articlesDraftCmd.Flags().IntVar(&articlesDraftFromYear, "from", 1966, "Start season year when --year is not set")
	articlesDraftCmd.Flags().IntVar(&articlesDraftToYear, "to", 2026, "End season year when --year is not set")
	articlesDraftCmd.Flags().StringVar(&articlesDraftDir, "articles-dir", "articles", "Directory where draft articles are written")
	articlesDraftCmd.Flags().StringVar(&articlesDraftProcessed, "processed-dir", "processed", "Directory with processed season JSON")
	articlesDraftCmd.Flags().StringVar(&articlesDraftGamesDir, "games-dir", "", "Directory with processed game JSON; defaults to <processed-dir>/games")
	articlesDraftCmd.Flags().BoolVar(&articlesDraftForce, "force", false, "Overwrite existing articles")
	articlesCmd.AddCommand(articlesDraftCmd)
}

func runArticlesMissing(cmd *cobra.Command, args []string) error {
	report, err := articleops.Audit(articleops.AuditOptions{
		FromYear:    articlesMissingFromYear,
		ToYear:      articlesMissingToYear,
		ArticlesDir: articlesMissingDir,
	})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal articles audit: %w", err)
	}
	cmd.Println(string(data))
	return nil
}

func runArticlesDraft(cmd *cobra.Command, args []string) error {
	results, err := articleops.GenerateDrafts(articleops.DraftOptions{
		Year:         articlesDraftYear,
		FromYear:     articlesDraftFromYear,
		ToYear:       articlesDraftToYear,
		ArticlesDir:  articlesDraftDir,
		ProcessedDir: articlesDraftProcessed,
		GamesDir:     articlesDraftGamesDir,
		Force:        articlesDraftForce,
	})
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal draft results: %w", err)
	}
	cmd.Println(string(data))
	return nil
}

func generateSeasonArticle(year int, articlesDir, outDir, gamesDir string) (string, error) {
	sourcePath := filepath.Join(articlesDir, fmt.Sprintf("%d.md", year))
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Season article not found for year=%d path=%s; skipping", year, sourcePath)
			return "", nil
		}
		return "", fmt.Errorf("read article: %w", err)
	}
	title, body := extractArticleTitleAndBody(string(raw), year)
	date := seasonStartDateFromGames(year, gamesDir)
	footer := fmt.Sprintf("[Volver](/historia/%d)", year)
	if !strings.Contains(body, footer) {
		body = strings.TrimRight(body, "\n\r\t ")
		body = body + "\n\n" + footer
	}

	content := fmt.Sprintf("---\ntitle: %q\ndate: %s\ndraft: false\n---\n\n%s", title, date, body)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("create articles output dir: %w", err)
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%d.md", year))
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write article output: %w", err)
	}
	return outPath, nil
}

func extractArticleTitleAndBody(markdown string, year int) (string, string) {
	lines := strings.Split(markdown, "\n")
	title := fmt.Sprintf("%d", year)
	titleIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			title = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			titleIdx = i
		}
		break
	}
	if titleIdx >= 0 {
		lines = append([]string{}, lines[titleIdx+1:]...)
	}
	body := strings.TrimLeft(strings.Join(lines, "\n"), "\n\r\t ")
	return title, body
}

func seasonStartDateFromGames(year int, gamesDir string) string {
	pattern := filepath.Join(gamesDir, fmt.Sprintf("%d*.json", year))
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		return fmt.Sprintf("%d-01-01", year)
	}
	dates := make([]string, 0, len(files))
	for _, f := range files {
		base := filepath.Base(f)
		if len(base) < 8 {
			continue
		}
		raw := base[:8]
		if _, err := strconv.Atoi(raw); err != nil {
			continue
		}
		dates = append(dates, raw)
	}
	if len(dates) == 0 {
		return fmt.Sprintf("%d-01-01", year)
	}
	sort.Strings(dates)
	d := dates[0]
	return fmt.Sprintf("%s-%s-%s", d[:4], d[4:6], d[6:8])
}
