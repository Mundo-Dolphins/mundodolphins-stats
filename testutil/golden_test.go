package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssertGoldenMatches(t *testing.T) {
	tmpDir := t.TempDir()
	goldenPath := filepath.Join(tmpDir, "test.golden")
	if err := os.WriteFile(goldenPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}

	AssertGolden(t, goldenPath, []byte("ok"))
}

func TestAssertGoldenUpdateModeWritesFile(t *testing.T) {
	t.Setenv("UPDATE_GOLDEN", "1")
	tmpDir := t.TempDir()
	goldenPath := filepath.Join(tmpDir, "nested", "test.golden")

	AssertGolden(t, goldenPath, []byte("updated"))

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read updated golden: %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}
