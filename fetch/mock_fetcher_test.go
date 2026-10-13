package fetch

import (
	"context"
	"errors"
	"testing"
)

func TestMockFetcherRecordsCallsAndUsesConfiguredFunctions(t *testing.T) {
	var f Fetcher = NewMockFetcher("mock")
	m := f.(*MockFetcher)
	if m.Name() != "mock" {
		t.Fatalf("name = %q", m.Name())
	}
	if _, err := m.Fetch(context.Background(), "https://example.com/a"); err == nil {
		t.Fatal("an unconfigured mock must fail")
	}
	m.SetFetchFunc(func(_ context.Context, url string) (FetchResult, error) {
		return FetchResult{StatusCode: 200, FinalURL: url, Body: []byte("ok")}, nil
	})
	res, err := m.Fetch(context.Background(), "https://example.com/b")
	if err != nil || res.StatusCode != 200 || string(res.Body) != "ok" {
		t.Fatalf("unexpected result %+v err=%v", res, err)
	}
	if got := m.GetFetchCalls(); m.FetchCallCount() != 2 || got[1] != "https://example.com/b" {
		t.Fatalf("unexpected call log %v", got)
	}
	boom := errors.New("boom")
	m.SetCloseFunc(func() error { return boom })
	if err := m.Close(); !errors.Is(err, boom) || m.CloseCallCount() != 1 {
		t.Fatalf("close = %v, calls=%d", err, m.CloseCallCount())
	}
}
