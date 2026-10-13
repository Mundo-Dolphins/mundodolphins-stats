package fetch

import (
	"context"
	"errors"
)

// MockFetcher is a test double that implements the Fetcher interface.
// It allows tests to simulate different scenarios without requiring actual HTTP or browser operations.
type MockFetcher struct {
	name         string
	fetchFunc    func(ctx context.Context, url string) (FetchResult, error)
	closeFunc    func() error
	fetchCallLog []string
	closeCallLog []string
}

// NewMockFetcher creates a new MockFetcher with the given name.
func NewMockFetcher(name string) *MockFetcher {
	return &MockFetcher{
		name:         name,
		fetchCallLog: make([]string, 0),
		closeCallLog: make([]string, 0),
	}
}

// Name returns the mock fetcher's name.
func (m *MockFetcher) Name() string {
	return m.name
}

// Fetch executes the configured fetch function or returns an error if not configured.
func (m *MockFetcher) Fetch(ctx context.Context, url string) (FetchResult, error) {
	m.fetchCallLog = append(m.fetchCallLog, url)

	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, url)
	}

	return FetchResult{}, errors.New("mock fetcher: fetch function not configured")
}

// Close executes the configured close function or returns nil if not configured.
func (m *MockFetcher) Close() error {
	m.closeCallLog = append(m.closeCallLog, "close")

	if m.closeFunc != nil {
		return m.closeFunc()
	}

	return nil
}

// SetFetchFunc configures the function to be called when Fetch is invoked.
func (m *MockFetcher) SetFetchFunc(f func(ctx context.Context, url string) (FetchResult, error)) {
	m.fetchFunc = f
}

// SetCloseFunc configures the function to be called when Close is invoked.
func (m *MockFetcher) SetCloseFunc(f func() error) {
	m.closeFunc = f
}

// FetchCallCount returns the number of times Fetch was called.
func (m *MockFetcher) FetchCallCount() int {
	return len(m.fetchCallLog)
}

// CloseCallCount returns the number of times Close was called.
func (m *MockFetcher) CloseCallCount() int {
	return len(m.closeCallLog)
}

// GetFetchCalls returns all URLs that were passed to Fetch.
func (m *MockFetcher) GetFetchCalls() []string {
	return m.fetchCallLog
}
