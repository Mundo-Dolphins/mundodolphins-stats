package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Mundo-Dolphins/mundodolphins-stats/pagejson"
	"github.com/spf13/cobra"
)

// jsonSeasonStatsCmd generates structured JSON content for season statistics
var jsonSeasonStatsCmd = &cobra.Command{
	Use:   "season-stats",
	Short: "Generate historical JSON content for season statistics",
	Long: `Generate historical JSON content for season statistics from processed season JSON files.
Output is written to <output-dir>/data/historical/season_stats/<year>.json.

Examples:
  # Generate for a single season
  dolphins-stats json season-stats --year 2025

  # Generate for a range of seasons
  dolphins-stats json season-stats --from-year 2020 --to-year 2025

  # With custom output directory
  dolphins-stats json season-stats --year 2025 --output-dir /tmp/pages`,
	RunE: runJSONSeasonStats,
}

var (
	jsonSeasonStatsYear         int64
	jsonSeasonStatsFromYear     int64
	jsonSeasonStatsToYear       int64
	jsonSeasonStatsOutputDir    string
	jsonSeasonStatsProcessedDir string
)

func init() {
	jsonCmd.AddCommand(jsonSeasonStatsCmd)

	jsonSeasonStatsCmd.Flags().Int64Var(&jsonSeasonStatsYear, "year", 0,
		"Generate for specific year (0 = use range)")
	jsonSeasonStatsCmd.Flags().Int64Var(&jsonSeasonStatsFromYear, "from-year", 1966,
		"Start year for range generation")
	jsonSeasonStatsCmd.Flags().Int64Var(&jsonSeasonStatsToYear, "to-year", 2026,
		"End year for range generation")
	jsonSeasonStatsCmd.Flags().StringVar(&jsonSeasonStatsOutputDir, "output-dir", "output",
		"Output base directory; data is written below data/historical")
	jsonSeasonStatsCmd.Flags().StringVar(&jsonSeasonStatsProcessedDir, "processed-dir", "processed",
		"Input directory with processed season JSON files")
}

func runJSONSeasonStats(cmd *cobra.Command, args []string) error {
	if jsonSeasonStatsYear != 0 {
		// Generate for single year
		return generateJSONSeasonStatsSingle(int(jsonSeasonStatsYear))
	}

	// Generate for range
	return generateJSONSeasonStatsRange(int(jsonSeasonStatsFromYear), int(jsonSeasonStatsToYear))
}

func generateJSONSeasonStatsSingle(year int) error {
	builder := pagejson.NewSeasonStatsBuilder(jsonSeasonStatsProcessedDir)
	paths := pagejson.NewPagePaths(jsonSeasonStatsOutputDir)

	// Load processed season file
	seasonFile := filepath.Join(jsonSeasonStatsProcessedDir, fmt.Sprintf("%d.json", year))

	log.Printf("Generating JSON season stats for %d from %s", year, seasonFile)

	doc, err := builder.BuildFromFile(seasonFile)
	if err != nil {
		return fmt.Errorf("build season JSON for %d: %w", year, err)
	}

	// Encode to JSON
	data, err := pagejson.EncodeToJSON(doc)
	if err != nil {
		return fmt.Errorf("encode season JSON for %d: %w", year, err)
	}

	// Write to output file
	outputPath := paths.SeasonStatsOutputPath(year)
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory %s: %w", outputDir, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write season JSON to %s: %w", outputPath, err)
	}

	log.Printf("Generated JSON season stats: %s", outputPath)
	return nil
}

func generateJSONSeasonStatsRange(fromYear, toYear int) error {
	builder := pagejson.NewSeasonStatsBuilder(jsonSeasonStatsProcessedDir)
	paths := pagejson.NewPagePaths(jsonSeasonStatsOutputDir)

	log.Printf("Generating JSON season stats for years %d-%d", fromYear, toYear)

	successCount := 0
	errorCount := 0

	for year := fromYear; year <= toYear; year++ {
		seasonFile := filepath.Join(jsonSeasonStatsProcessedDir, fmt.Sprintf("%d.json", year))

		doc, err := builder.BuildFromFile(seasonFile)
		if err != nil {
			log.Printf("Warning: failed to generate season JSON for %d: %v", year, err)
			errorCount++
			continue
		}

		data, err := pagejson.EncodeToJSON(doc)
		if err != nil {
			log.Printf("Warning: failed to encode season JSON for %d: %v", year, err)
			errorCount++
			continue
		}

		outputPath := paths.SeasonStatsOutputPath(year)
		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Printf("Warning: failed to create directory for %d: %v", year, err)
			errorCount++
			continue
		}

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			log.Printf("Warning: failed to write season JSON for %d: %v", year, err)
			errorCount++
			continue
		}

		successCount++
	}

	log.Printf("JSON season stats generation complete: %d successful, %d errors", successCount, errorCount)
	return nil
}
