package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunAuditSeasonSingleYearOutput(t *testing.T) {
	prevYear := auditYear
	prevFrom := auditFromYear
	prevTo := auditToYear
	prevOutput := auditOutputDir
	prevCache := auditCacheDir
	prevCompare := auditCompareTo
	defer func() {
		auditYear = prevYear
		auditFromYear = prevFrom
		auditToYear = prevTo
		auditOutputDir = prevOutput
		auditCacheDir = prevCache
		auditCompareTo = prevCompare
	}()

	cacheDir := t.TempDir()
	outputDir := t.TempDir()

	auditYear = 1966
	auditFromYear = 1966
	auditToYear = 1966
	auditOutputDir = outputDir
	auditCacheDir = cacheDir
	auditCompareTo = 0

	jsonContent := `{"schema":{"name":"mundodolphins.pfr.team-season","version":"1.2.0"},"data":{"main":{"tables":{"passing":{"rows":[{"pass_yds":"1"}]}}}}}`
	if err := os.WriteFile(filepath.Join(cacheDir, "1966.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write season json: %v", err)
	}

	if err := runAuditSeason(auditSeasonCmd, nil); err != nil {
		t.Fatalf("runAuditSeason error: %v", err)
	}

	outputPath := filepath.Join(outputDir, "1966_audit.json")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected audit report file: %v", err)
	}
}

func TestRunAuditSeasonMissingFile(t *testing.T) {
	prevYear := auditYear
	prevFrom := auditFromYear
	prevTo := auditToYear
	prevOutput := auditOutputDir
	prevCache := auditCacheDir
	prevCompare := auditCompareTo
	defer func() {
		auditYear = prevYear
		auditFromYear = prevFrom
		auditToYear = prevTo
		auditOutputDir = prevOutput
		auditCacheDir = prevCache
		auditCompareTo = prevCompare
	}()

	cacheDir := t.TempDir()

	auditYear = 1967
	auditFromYear = 1967
	auditToYear = 1967
	auditOutputDir = ""
	auditCacheDir = cacheDir
	auditCompareTo = 0

	if err := runAuditSeason(auditSeasonCmd, nil); err != nil {
		t.Fatalf("runAuditSeason should skip missing file without error: %v", err)
	}
}
