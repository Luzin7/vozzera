package httpx

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_CSPAndHSTSOnEveryStatus(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{name: "ok", status: http.StatusOK},
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "forbidden", status: http.StatusForbidden},
		{name: "too_many_requests", status: http.StatusTooManyRequests},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
			req.Header.Set("X-Forwarded-Proto", "https")

			w := serve(handler, req)

			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if got := w.Header().Get("Content-Security-Policy"); got != securityCSP {
				t.Errorf("Content-Security-Policy = %q, want %q", got, securityCSP)
			}
			if got := w.Header().Get("Strict-Transport-Security"); got != securityHSTS {
				t.Errorf("Strict-Transport-Security = %q, want %q", got, securityHSTS)
			}
		})
	}
}

func TestSecurityHeaders_HSTSOnlyOverHTTPS(t *testing.T) {
	cases := []struct {
		name  string
		build func() *http.Request
		want  string
	}{
		{
			name: "plain_http",
			build: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "http://api.example.com/api/rooms", nil)
			},
			want: "",
		},
		{
			name: "forwarded_proto_https",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "http://api.example.com/api/rooms", nil)
				r.Header.Set("X-Forwarded-Proto", "https")
				return r
			},
			want: securityHSTS,
		},
		{
			name: "tls_connection",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "https://api.example.com/api/rooms", nil)
				r.TLS = &tls.ConnectionState{}
				return r
			},
			want: securityHSTS,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(SecurityHeaders(okHandler()), tc.build())

			if got := w.Header().Get("Strict-Transport-Security"); got != tc.want {
				t.Errorf("Strict-Transport-Security = %q, want %q", got, tc.want)
			}
		})
	}
}
