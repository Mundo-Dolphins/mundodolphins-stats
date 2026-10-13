package videos

import (
	"fmt"
	"sync"
	"time"
)

// QuotaCosts defines the cost of each YouTube API operation.
const (
	QuotaCostSearch        = 100
	QuotaCostPlaylistItems = 1
	QuotaCostVideosList    = 1
	QuotaCostChannelsList  = 1
)

// QuotaManager tracks YouTube API quota usage with a daily budget.
type QuotaManager struct {
	mu            sync.Mutex
	used          int
	max           int
	lastResetDate string // "2006-01-02"
	nowFunc       func() time.Time
}

// NewQuotaManager creates a quota manager with the given daily max.
func NewQuotaManager(maxQuota int) *QuotaManager {
	return &QuotaManager{
		max:     maxQuota,
		nowFunc: time.Now,
	}
}

// ResetIfNewDay resets the counter if the date has changed since the last reset.
// Returns true if a reset occurred.
func (q *QuotaManager) ResetIfNewDay() bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	today := q.today()
	if q.lastResetDate != today {
		q.used = 0
		q.lastResetDate = today
		return true
	}
	return false
}

// TryConsume attempts to consume the given cost. Returns true if budget was available.
func (q *QuotaManager) TryConsume(cost int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.used+cost > q.max {
		return false
	}
	q.used += cost
	return true
}

// ForceConsume records consumption without checking the budget (for post-hoc tracking).
func (q *QuotaManager) ForceConsume(cost int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.used += cost
}

// Used returns the current quota used.
func (q *QuotaManager) Used() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.used
}

// Max returns the quota limit.
func (q *QuotaManager) Max() int {
	return q.max
}

// Remaining returns how much quota is left.
func (q *QuotaManager) Remaining() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.max - q.used
}

// IsExhausted returns true if no more quota is available.
func (q *QuotaManager) IsExhausted() bool {
	return q.Remaining() <= 0
}

// CanAfford checks if the given cost fits within the remaining budget.
func (q *QuotaManager) CanAfford(cost int) bool {
	return q.Remaining() >= cost
}

// String returns a human-readable summary like "used 8932/9000".
func (q *QuotaManager) String() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return fmt.Sprintf("used %d/%d", q.used, q.max)
}

// SetUsed restores previously tracked usage (e.g. from state.json).
func (q *QuotaManager) SetUsed(used int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.used = used
}

// SetLastResetDate sets the last reset date (for state restoration).
func (q *QuotaManager) SetLastResetDate(date string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.lastResetDate = date
}

// LastResetDate returns the last date the quota was reset.
func (q *QuotaManager) LastResetDate() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lastResetDate
}

func (q *QuotaManager) today() string {
	now := q.nowFunc
	if now == nil {
		now = time.Now
	}
	return now().UTC().Format("2006-01-02")
}
