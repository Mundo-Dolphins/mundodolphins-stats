package util

import (
	"log"
)

// InitLogger initializes the standard logger
func InitLogger() {
	// Use a simple timestamp format for CLI output.
	log.SetFlags(log.LstdFlags)
}

// Warnf logs a warning message using the standard logger.
func Warnf(format string, args ...any) {
	log.Printf("WARN: "+format, args...)
}
