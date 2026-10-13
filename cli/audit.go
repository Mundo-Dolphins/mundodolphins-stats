package cli

import (
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit processed season data",
	Long:  `Tools for auditing and inspecting processed season JSON files`,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}
