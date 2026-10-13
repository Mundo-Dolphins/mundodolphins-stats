package videos

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebHandlerRejectsForeignHostAndOrigin(t *testing.T) {
	h := (&WebServer{Service: &Service{}}).Handler()
	cases := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"loopback ip", "127.0.0.1:8787", "", http.StatusOK},
		{"localhost", "localhost:8787", "http://localhost:8787", http.StatusOK},
		{"ipv6 loopback", "[::1]:8787", "", http.StatusOK},
		{"dns rebinding host", "evil.example.com:8787", "", http.StatusForbidden},
		{"other lan host", "192.168.1.20:8787", "", http.StatusForbidden},
		{"cross-site origin", "127.0.0.1:8787", "https://evil.example.com", http.StatusForbidden},
		{"origin with other port", "127.0.0.1:8787", "http://127.0.0.1:9999", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// newLocalRequest builds a request as a browser on the same machine would send
// it to the review UI (httptest uses example.com as Host by default).
func newLocalRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = "127.0.0.1:8787"
	return req
}
