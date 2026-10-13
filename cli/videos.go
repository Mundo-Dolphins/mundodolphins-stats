package cli

import "github.com/spf13/cobra"

var videosCmd = &cobra.Command{
	Use:   "videos",
	Short: "Discover and manage YouTube videos for games",
}

func init() {
	videosCmd.AddCommand(videosDiscoverCmd)
	videosCmd.AddCommand(videosImportLegacyCmd)
	rootCmd.AddCommand(videosCmd)
}
