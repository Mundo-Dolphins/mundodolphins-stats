package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunVideosImportLegacyMissingFile(t *testing.T) {
	prevLegacyFile := videosLegacyFile
	defer func() { videosLegacyFile = prevLegacyFile }()

	videosLegacyFile = ""
	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})

	if err := runVideosImportLegacy(cmd, nil); err == nil {
		t.Fatalf("expected error for missing legacy-file")
	}
}
