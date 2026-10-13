package cli

import (
	"testing"
)

func TestRootCommandMetadata(t *testing.T) {
	if rootCmd.Use != "dolphins-stats" {
		t.Fatalf("Use = %q, want dolphins-stats", rootCmd.Use)
	}
	if rootCmd.Short == "" {
		t.Fatal("expected short description")
	}
	if rootCmd.Long == "" {
		t.Fatal("expected long description")
	}
}

func TestRootCommandRegistersCoreSubcommands(t *testing.T) {
	for _, name := range []string{"generate", "json", "videos", "articles", "audit"} {
		if _, _, err := rootCmd.Find([]string{name}); err != nil {
			t.Fatalf("expected root command to register %q: %v", name, err)
		}
	}
}

func TestRootPersistentPreRun(t *testing.T) {
	if rootCmd.PersistentPreRun == nil {
		t.Fatal("expected persistent pre-run")
	}

	rootCmd.PersistentPreRun(rootCmd, nil)
}
