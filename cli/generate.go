package cli

import "github.com/spf13/cobra"

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate output artifacts",
	Long:  "Generate output artifacts from processed JSON files",
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
