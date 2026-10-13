package cli

import (
	"errors"
	"os"

	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dolphins-stats",
	Short: "Generate Miami Dolphins statistics content from processed data",
	Long: `dolphins-stats generates the content of a statistics website from
versioned, already processed JSON files (seasons, games, videos and editorial
articles): Hugo data, Markdown, video matching and audits.

This library does not download or parse third-party pages. Programs that do
(downloaders, HTML parsers) are built on top of it and register their own
commands on the root command returned by Root().`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
			util.Warnf("failed to load .env: %v", err)
		}
		// Initialize logger for all commands
		util.InitLogger()
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
