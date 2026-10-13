package cli

import (
	"encoding/json"
	"fmt"

	"github.com/Mundo-Dolphins/mundodolphins-stats/audit"
	"github.com/spf13/cobra"
)

var auditStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Audit global local data readiness",
	Long:  "Build a deterministic offline manifest of processed seasons, games, articles, videos, output data, and JSON schemas.",
	RunE:  runAuditStatus,
}

var (
	auditStatusProcessedDir       string
	auditStatusGamesDir           string
	auditStatusArticlesDir        string
	auditStatusVideosCache        string
	auditStatusOutputDir          string
	auditStatusIncludeGeneratedAt bool
)

func init() {
	auditCmd.AddCommand(auditStatusCmd)

	auditStatusCmd.Flags().StringVar(&auditStatusProcessedDir, "processed-dir", "processed", "Directory with processed season JSON")
	auditStatusCmd.Flags().StringVar(&auditStatusGamesDir, "games-dir", "processed/games", "Directory with processed game JSON")
	auditStatusCmd.Flags().StringVar(&auditStatusArticlesDir, "articles-dir", "articles", "Directory with source season articles")
	auditStatusCmd.Flags().StringVar(&auditStatusVideosCache, "videos-cache", "cache/videos/videos.json", "Videos cache JSON file")
	auditStatusCmd.Flags().StringVar(&auditStatusOutputDir, "output", "output/data/historical", "Hugo historical data directory")
	auditStatusCmd.Flags().BoolVar(&auditStatusIncludeGeneratedAt, "include-generated-at", false, "Include current timestamp in the manifest")
}

func runAuditStatus(cmd *cobra.Command, args []string) error {
	manifest, err := audit.BuildStatusManifest(audit.StatusOptions{
		ProcessedDir:       auditStatusProcessedDir,
		GamesDir:           auditStatusGamesDir,
		ArticlesDir:        auditStatusArticlesDir,
		VideosCacheFile:    auditStatusVideosCache,
		OutputDir:          auditStatusOutputDir,
		IncludeGeneratedAt: auditStatusIncludeGeneratedAt,
	})
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal audit status manifest: %w", err)
	}
	cmd.Println(string(data))
	return nil
}
