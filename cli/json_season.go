package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Mundo-Dolphins/mundodolphins-stats/pagejson"
	"github.com/spf13/cobra"
)

// jsonSeasonCmd generates structured JSON content for season pages
var jsonSeasonCmd = &cobra.Command{
	Use:   "season",
	Short: "Generate historical JSON content for season pages",
	Long: `Generate historical JSON content for season pages (with roster, draft, schedule) from processed season JSON files.
Output is written to <output-dir>/data/historical/seasons/<year>.json.

Examples:
  # Generate for a single season
  dolphins-stats json season --year 2025

  # Generate for a range of seasons
  dolphins-stats json season --from-year 2020 --to-year 2025

  # With custom output directory
  dolphins-stats json season --year 2025 --output-dir /tmp/pages`,
	RunE: runJSONSeason,
}

var (
	jsonSeasonYear         int64
	jsonSeasonFromYear     int64
	jsonSeasonToYear       int64
	jsonSeasonOutputDir    string
	jsonSeasonProcessedDir string
)

func init() {
	jsonCmd.AddCommand(jsonSeasonCmd)

	jsonSeasonCmd.Flags().Int64Var(&jsonSeasonYear, "year", 0,
		"Generate for specific year (0 = use range)")
	jsonSeasonCmd.Flags().Int64Var(&jsonSeasonFromYear, "from-year", 1966,
		"Start year for range generation")
	jsonSeasonCmd.Flags().Int64Var(&jsonSeasonToYear, "to-year", 2026,
		"End year for range generation")
	jsonSeasonCmd.Flags().StringVar(&jsonSeasonOutputDir, "output-dir", "output",
		"Output base directory; data is written below data/historical")
	jsonSeasonCmd.Flags().StringVar(&jsonSeasonProcessedDir, "processed-dir", "processed",
		"Input directory with processed season JSON files")
}

func runJSONSeason(cmd *cobra.Command, args []string) error {
	if jsonSeasonYear != 0 {
		// Generate for single year
		return generateJSONSeasonSingle(int(jsonSeasonYear))
	}

	// Generate for range
	return generateJSONSeasonRange(int(jsonSeasonFromYear), int(jsonSeasonToYear))
}

func generateJSONSeasonSingle(year int) error {
	builder := pagejson.NewSeasonBuilder(jsonSeasonProcessedDir)
	paths := pagejson.NewPagePaths(jsonSeasonOutputDir)

	// Load processed season file
	seasonFile := filepath.Join(jsonSeasonProcessedDir, fmt.Sprintf("%d.json", year))

	log.Printf("Generating JSON season page for %d from %s", year, seasonFile)

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
	outputPath := paths.SeasonOutputPath(year)
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory %s: %w", outputDir, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write season JSON to %s: %w", outputPath, err)
	}

	log.Printf("Generated JSON season page: %s", outputPath)
	return nil
}

func generateJSONSeasonRange(fromYear, toYear int) error {
	builder := pagejson.NewSeasonBuilder(jsonSeasonProcessedDir)
	paths := pagejson.NewPagePaths(jsonSeasonOutputDir)

	log.Printf("Generating JSON season pages for years %d-%d", fromYear, toYear)

	successCount := 0
	errorCount := 0

	for year := fromYear; year <= toYear; year++ {
		seasonFile := filepath.Join(jsonSeasonProcessedDir, fmt.Sprintf("%d.json", year))

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

		outputPath := paths.SeasonOutputPath(year)
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

	log.Printf("JSON season page generation complete: %d successful, %d errors", successCount, errorCount)
	return nil
}
