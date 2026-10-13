package youtubeapi

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestChunkIDs(t *testing.T) {
	chunks := chunkIDs([]string{"a", "b", "c"}, 2)
	if len(chunks) != 2 {
		t.Fatalf("unexpected chunk len: %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 1 {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
}

func TestParseISO8601Duration(t *testing.T) {
	d, err := parseISO8601Duration("PT1H2M3S")
	if err != nil {
		t.Fatalf("parseISO8601Duration error: %v", err)
	}
	if d != (time.Hour + 2*time.Minute + 3*time.Second) {
		t.Fatalf("unexpected duration: %v", d)
	}
	if _, err := parseISO8601Duration("bad"); err == nil {
		t.Fatalf("expected error for bad duration")
	}
}

func TestParseAPIErrorFallbackBody(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("plain error"))}
	err := parseAPIError(resp)
	apiErr, ok := err.(*GoogleAPIError)
	if !ok {
		t.Fatalf("expected GoogleAPIError, got %T", err)
	}
	if apiErr.Message != "plain error" {
		t.Fatalf("unexpected message: %q", apiErr.Message)
	}
}

func TestClientValidate(t *testing.T) {
	if err := (*Client)(nil).validate(); err == nil {
		t.Fatalf("expected error for nil client")
	}
	c := NewClient("", nil)
	if err := c.validate(); err == nil {
		t.Fatalf("expected error for empty api key")
	}
}

func TestGoogleAPIErrorErrorString(t *testing.T) {
	err := (&GoogleAPIError{StatusCode: 403, Reasons: []string{"quotaExceeded"}, Message: "quota"}).Error()
	if !strings.Contains(err, "quotaExceeded") {
		t.Fatalf("expected reason in error string")
	}
}
