package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetCachePath returns the cache file path for a given year
func GetCachePath(cacheDir string, year int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%d.html", year))
}

// GetOutputPath returns the output JSON path for a given year
func GetOutputPath(outDir string, year int) string {
	return filepath.Join(outDir, fmt.Sprintf("%d.json", year))
}
