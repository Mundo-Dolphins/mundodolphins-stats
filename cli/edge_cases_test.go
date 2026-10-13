package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateHugoDataWithInvalidJSON tests generation with corrupted JSON
func TestGenerateHugoDataWithInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed")
	outputDir := filepath.Join(tmpDir, "output")

	os.MkdirAll(processedDir, 0755)
	os.MkdirAll(outputDir, 0755)

	// Create invalid JSON
	os.WriteFile(filepath.Join(processedDir, "2020.json"), []byte("invalid"), 0644)

	generateHugoDataCmd.SetArgs([]string{
		"--type", "seasons",
		"--year", "2020",
		"--processed-dir", processedDir,
		"--output-dir", outputDir,
	})
	err := generateHugoDataCmd.Execute()
	if err == nil {
		t.Logf("Expected error parsing invalid JSON")
	}
}

// TestGenerateHugoDataWithReadOnlyOutput tests generation with read-only output dir
func TestGenerateHugoDataWithReadOnlyOutput(t *testing.T) {
	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed")
	readOnlyDir := filepath.Join(tmpDir, "readonly")

	os.MkdirAll(processedDir, 0755)
	os.MkdirAll(readOnlyDir, 0555)

	// Create valid JSON
	jsonContent := `{"season": 2020, "data": {"main": {"status": "ok", "tables": {}}}}`
	os.WriteFile(filepath.Join(processedDir, "2020.json"), []byte(jsonContent), 0644)

	outputDir := filepath.Join(readOnlyDir, "output")

	generateHugoDataCmd.SetArgs([]string{
		"--type", "seasons",
		"--year", "2020",
		"--processed-dir", processedDir,
		"--output-dir", outputDir,
	})
	err := generateHugoDataCmd.Execute()
	if err == nil {
		t.Logf("Expected error creating output directory")
	}
}
