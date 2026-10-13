package testutil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRoundTripFunc(t *testing.T) {
	called := false
	rt := RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return NewResponse(req, http.StatusOK, "ok", map[string]string{"X-Test": "1"}), nil
	})
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	if !called {
		t.Fatalf("expected RoundTripFunc to be called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Test") != "1" {
		t.Fatalf("expected header X-Test")
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient(func(req *http.Request) (*http.Response, error) {
		return NewResponse(req, http.StatusTeapot, "teapot", nil), nil
	})
	resp, err := client.Get("http://example.com")
	if err != nil {
		t.Fatalf("client get: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("expected status 418, got %d", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "teapot" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestNewResponse(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp := NewResponse(req, http.StatusAccepted, "hello", map[string]string{"X-Test": "1"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Test") != "1" {
		t.Fatalf("expected header X-Test")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.TrimSpace(string(body)) != "hello" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}
