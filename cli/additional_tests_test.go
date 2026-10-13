package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestVideoDiscoverWithYear tests video discovery
func TestVideoDiscoverWithYear(t *testing.T) {
	videosDiscoverCmd.SetArgs([]string{
		"--year", "2024",
		"--non-interactive",
	})
	_ = videosDiscoverCmd.Execute()
}

// TestInitCommandsInRoot tests that init functions register commands properly
func TestInitCommandsInRoot(t *testing.T) {
	// Verify that commands are registered
	if rootCmd == nil {
		t.Fatalf("rootCmd is nil")
	}

	// Check that we have subcommands
	if len(rootCmd.Commands()) == 0 {
		t.Fatalf("rootCmd has no subcommands")
	}

	// Verify known commands exist
	want := map[string]bool{"generate": false, "json": false, "videos": false, "articles": false, "audit": false}
	for _, cmd := range rootCmd.Commands() {
		if _, ok := want[cmd.Use]; ok {
			want[cmd.Use] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s command not found", name)
		}
	}
}

// TestGenerateHugoDataRangeFromAdditional tests Hugo data generation for a range
func TestGenerateHugoDataRangeFromAdditional(t *testing.T) {
	tmpDir := t.TempDir()
	processedDir := filepath.Join(tmpDir, "processed")
	outputDir := filepath.Join(tmpDir, "output")

	os.MkdirAll(processedDir, 0755)
	os.MkdirAll(outputDir, 0755)

	// Create JSON files for a range
	for year := 1970; year <= 1972; year++ {
		jsonContent := fmt.Sprintf("{\"season\": %d, \"data\": {\"main\": {\"status\": \"ok\", \"tables\": {}}}}", year)
		os.WriteFile(filepath.Join(processedDir, fmt.Sprintf("%d.json", year)), []byte(jsonContent), 0644)
	}

	// Generate Hugo season data for a specific range
	generateHugoDataCmd.SetArgs([]string{
		"--type", "seasons",
		"--from-year", "1970",
		"--to-year", "1972",
		"--processed-dir", processedDir,
		"--output-dir", outputDir,
		"--include-articles=false",
	})
	_ = generateHugoDataCmd.Execute()
}
