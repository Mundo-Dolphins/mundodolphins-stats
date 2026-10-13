package testutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// AssertGolden compares output with a golden file. Set UPDATE_GOLDEN=1 to update.
func AssertGolden(t *testing.T, path string, actual []byte) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(path, actual, 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("golden mismatch: %s", path)
	}
}
