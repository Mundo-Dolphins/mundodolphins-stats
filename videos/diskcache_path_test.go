package videos

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchCachePathStaysInsideTheCacheDir(t *testing.T) {
	base := t.TempDir()
	m := &DiskCacheManager{BaseDir: base}
	for _, id := range []string{"202412210mia", "../../etc/passwd", "../x", "a/b"} {
		p := m.searchCachePath(id)
		rel, err := filepath.Rel(filepath.Join(base, "search"), p)
		if err != nil || strings.HasPrefix(rel, "..") || strings.ContainsRune(rel, filepath.Separator) {
			t.Errorf("searchCachePath(%q) = %q escapes the search dir (rel=%q)", id, p, rel)
		}
	}
}
