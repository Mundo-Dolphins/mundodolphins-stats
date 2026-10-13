package cli

import (
	"github.com/spf13/cobra"
)

// jsonCmd is the root command for JSON generation commands
var jsonCmd = &cobra.Command{
	Use:   "json",
	Short: "Generate historical JSON data for Hugo",
	Long: `Generate structured historical JSON for Hugo.
These commands write the same data contract used by generate hugo-data:
<output-dir>/data/historical/{seasons,season_stats,games,rivals}.

Usage:
  dolphins-stats json season [flags]
  dolphins-stats json season-stats [flags]
  dolphins-stats json games [flags]
  dolphins-stats json rivals [flags]`,
	Run: func(cmd *cobra.Command, args []string) {
		// No-op - subcommands handle the logic
	},
}

func init() {
	rootCmd.AddCommand(jsonCmd)
}
