// Package fetch defines how a page is downloaded. It contains only the
// contract and a mock for tests: implementations (HTTP clients, headless
// browsers, proxies) are deliberately not part of this library, so that using
// it never means shipping a scraper. Provide your own Fetcher.
package fetch

import "context"

// FetchResult represents the result of a fetch operation.
type FetchResult struct {
	StatusCode int
	FinalURL   string
	Body       []byte
	Headers    map[string]string
}

// Fetcher defines the interface for fetching data.
type Fetcher interface {
	Name() string
	Fetch(ctx context.Context, url string) (FetchResult, error)
	Close() error
}
