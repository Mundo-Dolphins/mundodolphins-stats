package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateHugoDataWithEmptyProcessedDir tests hugo-data generation with empty processed directory
func TestGenerateHugoDataWithEmptyProcessedDir(t *testing.T) {
	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed")
	outputPath := filepath.Join(tmpDir, "output")
	os.MkdirAll(processedDir, 0755)

	rootCmd.SetArgs([]string{"generate", "hugo-data",
		"--type", "seasons",
		"--year", "1966",
		"--processed-dir", processedDir,
		"--output-dir", outputPath,
		"--include-articles=false",
	})

	// Should handle empty directory gracefully
	err := rootCmd.Execute()
	if err != nil {
		t.Logf("Expected error for empty processed dir: %v", err)
	}
}
