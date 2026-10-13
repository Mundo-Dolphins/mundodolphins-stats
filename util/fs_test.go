package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirAndFileExists(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "nested", "dir")

	if FileExists(target) {
		t.Fatalf("expected %s to not exist", target)
	}

	if err := EnsureDir(target); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", target)
	}

	if !FileExists(target) {
		t.Fatalf("expected %s to exist", target)
	}
}

func TestPathHelpers(t *testing.T) {
	cache := GetCachePath("cache", 2024)
	if cache != filepath.Join("cache", "2024.html") {
		t.Fatalf("GetCachePath() = %s", cache)
	}

	out := GetOutputPath("out", 2024)
	if out != filepath.Join("out", "2024.json") {
		t.Fatalf("GetOutputPath() = %s", out)
	}
}
