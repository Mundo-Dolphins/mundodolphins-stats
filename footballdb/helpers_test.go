package footballdb

import "time"

// fixedTime returns a deterministic time for tests.
func fixedTime() time.Time {
	return time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
}
