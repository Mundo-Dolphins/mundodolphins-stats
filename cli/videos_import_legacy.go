package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
	"github.com/spf13/cobra"
)

var videosImportLegacyCmd = &cobra.Command{
	Use:   "import-legacy",
	Short: "Import legacy videos.json into cache",
	RunE:  runVideosImportLegacy,
}

var (
	videosLegacyFile        string
	videosLegacyCacheFile   string
	videosLegacyDryRun      bool
	videosLegacyOnlyMissing bool
	videosLegacyOverwrite   string
	videosLegacyGamesDir    string
)

var (
	videosLegacyImporterFactory = func() *videos.Importer { return &videos.Importer{Now: time.Now} }
	videosLegacyIndexBuilder    = func(dir string) (videos.GameIndex, error) { return videos.BuildPFRGameIndexFromDir(dir) }
)

func init() {
	videosImportLegacyCmd.Flags().StringVar(&videosLegacyFile, "legacy-file", "", "Legacy videos.json path")
	videosImportLegacyCmd.Flags().StringVar(&videosLegacyCacheFile, "cache-file", "cache/videos/videos.json", "Cache file path")
	videosImportLegacyCmd.Flags().BoolVar(&videosLegacyDryRun, "dry-run", false, "Do not write cache, only print summary")
	videosImportLegacyCmd.Flags().BoolVar(&videosLegacyOnlyMissing, "only-missing", true, "Only add missing items")
	videosImportLegacyCmd.Flags().StringVar(&videosLegacyOverwrite, "overwrite-status", "", "Overwrite status only when current status matches this value")
	videosImportLegacyCmd.Flags().StringVar(&videosLegacyGamesDir, "games-dir", "processed/games", "Directory with processed games JSON")
}

func runVideosImportLegacy(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(videosLegacyFile) == "" {
		return fmt.Errorf("legacy-file is required")
	}
	idx, err := videosLegacyIndexBuilder(videosLegacyGamesDir)
	if err != nil {
		return err
	}
	importer := videosLegacyImporterFactory()

	opts := videos.ImportOptions{
		DryRun:          videosLegacyDryRun,
		OnlyMissing:     videosLegacyOnlyMissing,
		OverwriteStatus: videosLegacyOverwrite,
	}

	summary, err := importer.ImportLegacy(context.Background(), videosLegacyFile, videosLegacyCacheFile, idx, opts)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Added: %d\n", summary.Added)
	fmt.Fprintf(out, "Skipped existing: %d\n", summary.SkippedExisting)
	fmt.Fprintf(out, "Unmatched dates: %d\n", summary.UnmatchedDates)
	fmt.Fprintf(out, "Duplicates: %d\n", summary.Duplicates)
	if summary.UnmatchedDates > 0 && len(summary.Unmatched) > 0 {
		fmt.Fprintf(out, "Unmatched (first %d): %s\n", len(summary.Unmatched), strings.Join(summary.Unmatched, ", "))
		fmt.Fprintf(out, "Index dir: %s\n", videosLegacyGamesDir)
	}
	if videosLegacyDryRun {
		fmt.Fprintln(out, "Dry run: cache not written")
	}
	return nil
}
