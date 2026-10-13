package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mundo-Dolphins/mundodolphins-stats/audit"
	"github.com/spf13/cobra"
)

var auditSeasonCmd = &cobra.Command{
	Use:   "season",
	Short: "Audit processed season JSON files",
	Long: `Audit processed season JSON files to detect tables, columns, and unused fields.

Generates a JSON report with a complete inventory of what exists.`,
	RunE: runAuditSeason,
}

var (
	auditYear      int
	auditFromYear  int
	auditToYear    int
	auditOutputDir string
	auditCacheDir  string
	auditCompareTo int
)

func init() {
	auditCmd.AddCommand(auditSeasonCmd)

	auditSeasonCmd.Flags().IntVar(&auditYear, "year", 0, "Single year to audit (e.g., 2024)")
	auditSeasonCmd.Flags().IntVar(&auditFromYear, "from-year", 1966, "Starting year for range audit")
	auditSeasonCmd.Flags().IntVar(&auditToYear, "to-year", 2025, "Ending year for range audit")
	auditSeasonCmd.Flags().StringVar(&auditCacheDir, "cache-dir", "processed", "Directory with processed season JSON")
	auditSeasonCmd.Flags().StringVar(&auditOutputDir, "output-dir", "", "Output directory for individual reports (default: stdout)")
	auditSeasonCmd.Flags().IntVar(&auditCompareTo, "compare-with", 0, "Compare with specific year to show evolution")
}

func runAuditSeason(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cacheDir := auditCacheDir

	// Determine which years to audit
	yearsToAudit := []int{}
	if auditYear > 0 {
		yearsToAudit = []int{auditYear}
	} else {
		for y := auditFromYear; y <= auditToYear; y++ {
			yearsToAudit = append(yearsToAudit, y)
		}
	}

	// Process each year
	var allReports []audit.AuditReport
	var previousInventory audit.SeasonInventory

	for _, year := range yearsToAudit {
		filePath := filepath.Join(cacheDir, fmt.Sprintf("%d.json", year))

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Printf("WARN: Season file not found: %d\n", year)
			continue
		}

		// Inspect the season
		inv, err := audit.InspectSeason(ctx, filePath)
		if err != nil {
			fmt.Printf("ERROR: Failed to inspect season %d: %v\n", year, err)
			continue
		}

		// Build report
		report := audit.BuildReport(inv)
		allReports = append(allReports, report)

		// Output report
		if auditOutputDir != "" {
			outputFile := filepath.Join(auditOutputDir, fmt.Sprintf("%d_audit.json", year))
			if err := outputReportToFile(outputFile, report); err != nil {
				fmt.Printf("ERROR: Failed to write audit report for %d: %v\n", year, err)
				continue
			}
			fmt.Printf("INFO: Audit report written for %d: %s\n", year, outputFile)
		}

		// If compare-with is set and this is the first year, store it
		if auditCompareTo == 0 && len(yearsToAudit) > 1 && previousInventory.Year == 0 {
			previousInventory = inv
		}

		// If compare-with is set, generate diff
		if auditCompareTo > 0 && year == yearsToAudit[0] {
			previousInventory = inv
		}
	}

	// Output to stdout if no output dir specified, or a summary
	if auditOutputDir == "" && len(allReports) > 0 {
		// Output in compact form
		data, err := json.MarshalIndent(allReports, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal reports: %w", err)
		}
		fmt.Println(string(data))
	} else if auditOutputDir != "" {
		fmt.Printf("INFO: Audit complete - %d years processed, output: %s\n", len(allReports), auditOutputDir)
	}

	return nil
}

// outputReportToFile writes an audit report to a JSON file.
func outputReportToFile(filePath string, report audit.AuditReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}

	return nil
}
