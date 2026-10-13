//go:generate go run github.com/golang/mock/mockgen -source=../youtubeapi/client.go -package mocks -output mocks/client.go

package mocks

import "github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"

// ClientMock is a mock implementation for testing.
type ClientMock struct {
	lastQuery string
	results   []youtubeapi.SearchResult
	err       error
}

func (m *ClientMock) Search(query string) ([]youtubeapi.SearchResult, error) {
	m.lastQuery = query
	return m.results, m.err
}

// Reset clears the mock state.
func (m *ClientMock) Reset() {
	m.lastQuery = ""
	m.results = nil
	m.err = nil
}

// Query returns the last query that was mocked.
func (m *ClientMock) Query() string { return m.lastQuery }

func NewClient(_ ...string) *ClientMock {
	return &ClientMock{}
}
