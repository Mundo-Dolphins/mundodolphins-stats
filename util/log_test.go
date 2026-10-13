package util

import (
	"log"
	"testing"
)

func TestInitLoggerSetsFlags(t *testing.T) {
	prev := log.Flags()
	defer log.SetFlags(prev)

	log.SetFlags(0)
	InitLogger()

	if log.Flags() != log.LstdFlags {
		t.Fatalf("expected log flags %d, got %d", log.LstdFlags, log.Flags())
	}
}
