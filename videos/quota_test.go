package videos

import (
	"testing"
	"time"
)

func TestNewQuotaManager(t *testing.T) {
	q := NewQuotaManager(9000)
	if q.Max() != 9000 {
		t.Errorf("Max() = %d, want 9000", q.Max())
	}
	if q.Used() != 0 {
		t.Errorf("Used() = %d, want 0", q.Used())
	}
	if q.Remaining() != 9000 {
		t.Errorf("Remaining() = %d, want 9000", q.Remaining())
	}
}

func TestQuotaManager_TryConsume(t *testing.T) {
	q := NewQuotaManager(200)

	if !q.TryConsume(100) {
		t.Error("TryConsume(100) should succeed with budget 200")
	}
	if q.Used() != 100 {
		t.Errorf("Used() = %d, want 100", q.Used())
	}

	if !q.TryConsume(100) {
		t.Error("TryConsume(100) should succeed at exact limit")
	}
	if q.Used() != 200 {
		t.Errorf("Used() = %d, want 200", q.Used())
	}

	if q.TryConsume(1) {
		t.Error("TryConsume(1) should fail when exhausted")
	}
	if q.Used() != 200 {
		t.Errorf("Used() should remain 200 after failed consume, got %d", q.Used())
	}
}

func TestQuotaManager_IsExhausted(t *testing.T) {
	q := NewQuotaManager(100)
	if q.IsExhausted() {
		t.Error("should not be exhausted initially")
	}
	q.TryConsume(100)
	if !q.IsExhausted() {
		t.Error("should be exhausted after consuming full budget")
	}
}

func TestQuotaManager_CanAfford(t *testing.T) {
	q := NewQuotaManager(150)
	if !q.CanAfford(100) {
		t.Error("should afford 100 with budget 150")
	}
	if !q.CanAfford(150) {
		t.Error("should afford 150 at exact limit")
	}
	if q.CanAfford(151) {
		t.Error("should not afford 151 with budget 150")
	}
}

func TestQuotaManager_ForceConsume(t *testing.T) {
	q := NewQuotaManager(100)
	q.ForceConsume(150) // Goes over budget
	if q.Used() != 150 {
		t.Errorf("Used() = %d, want 150 (force consume ignores budget)", q.Used())
	}
}

func TestQuotaManager_ResetIfNewDay(t *testing.T) {
	day1 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)

	q := NewQuotaManager(9000)
	q.nowFunc = func() time.Time { return day1 }

	// First call sets the date
	q.ResetIfNewDay()
	q.TryConsume(5000)
	if q.Used() != 5000 {
		t.Errorf("Used() = %d, want 5000", q.Used())
	}

	// Same day: no reset
	if q.ResetIfNewDay() {
		t.Error("should not reset on same day")
	}
	if q.Used() != 5000 {
		t.Errorf("Used() should remain 5000, got %d", q.Used())
	}

	// New day: should reset
	q.nowFunc = func() time.Time { return day2 }
	if !q.ResetIfNewDay() {
		t.Error("should reset on new day")
	}
	if q.Used() != 0 {
		t.Errorf("Used() should be 0 after reset, got %d", q.Used())
	}
}

func TestQuotaManager_SetUsed(t *testing.T) {
	q := NewQuotaManager(9000)
	q.SetUsed(3000)
	if q.Used() != 3000 {
		t.Errorf("Used() = %d, want 3000", q.Used())
	}
	if q.Remaining() != 6000 {
		t.Errorf("Remaining() = %d, want 6000", q.Remaining())
	}
}

func TestQuotaManager_String(t *testing.T) {
	q := NewQuotaManager(9000)
	q.SetUsed(1234)
	got := q.String()
	want := "used 1234/9000"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestQuotaCosts(t *testing.T) {
	// Verify cost constants match YouTube API v3 documentation
	if QuotaCostSearch != 100 {
		t.Errorf("QuotaCostSearch = %d, want 100", QuotaCostSearch)
	}
	if QuotaCostPlaylistItems != 1 {
		t.Errorf("QuotaCostPlaylistItems = %d, want 1", QuotaCostPlaylistItems)
	}
	if QuotaCostVideosList != 1 {
		t.Errorf("QuotaCostVideosList = %d, want 1", QuotaCostVideosList)
	}
	if QuotaCostChannelsList != 1 {
		t.Errorf("QuotaCostChannelsList = %d, want 1", QuotaCostChannelsList)
	}
}
