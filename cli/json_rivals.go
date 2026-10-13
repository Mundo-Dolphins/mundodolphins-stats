package cli

import (
	"fmt"
	"log"

	"github.com/Mundo-Dolphins/mundodolphins-stats/hugodata"
	"github.com/spf13/cobra"
)

var jsonRivalsCmd = &cobra.Command{
	Use:   "rivals",
	Short: "Generate aggregate JSON data for the rival index",
	Long: `Generate aggregate JSON data for the rival history index from processed game JSON files.
Output is written to <output-dir>/data/historical/rivals/index.json.

Examples:
  # Generate rival history for the full available range
  dolphins-stats json rivals --from-year 1966 --to-year 2026

  # With custom directories
  dolphins-stats json rivals --processed-dir processed --output-dir output`,
	RunE: runJSONRivals,
}

var (
	jsonRivalsFromYear     int64
	jsonRivalsToYear       int64
	jsonRivalsOutputDir    string
	jsonRivalsProcessedDir string
)

func init() {
	jsonCmd.AddCommand(jsonRivalsCmd)

	jsonRivalsCmd.Flags().Int64Var(&jsonRivalsFromYear, "from-year", 1966,
		"Start season for range generation")
	jsonRivalsCmd.Flags().Int64Var(&jsonRivalsToYear, "to-year", 2026,
		"End season for range generation")
	jsonRivalsCmd.Flags().StringVar(&jsonRivalsOutputDir, "output-dir", "output",
		"Output base directory; data is written below data/historical")
	jsonRivalsCmd.Flags().StringVar(&jsonRivalsProcessedDir, "processed-dir", "processed",
		"Input directory with processed season and game JSON files")
}

func runJSONRivals(cmd *cobra.Command, args []string) error {
	fromYear := int(jsonRivalsFromYear)
	toYear := int(jsonRivalsToYear)
	log.Printf("Generating rival index for seasons %d-%d", fromYear, toYear)
	if err := hugodata.GenerateRivalIndexData(cmd.Context(), jsonRivalsProcessedDir, jsonRivalsOutputDir, fromYear, toYear); err != nil {
		return fmt.Errorf("generate rival index: %w", err)
	}
	log.Printf("Generated rival index: %s/data/historical/rivals/index.json", jsonRivalsOutputDir)
	return nil
}
