package httpx

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const testOrigin = "https://vozzera.vercel.app"

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func loginRequest(method, clientIP string) *http.Request {
	r := httptest.NewRequest(method, "/api/login", nil)
	r.RemoteAddr = "10.0.0.1:4321"
	r.Header.Set("CF-Connecting-IP", clientIP)
	r.Header.Set("Origin", testOrigin)
	return r
}

func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func oneLoginPerMinute(clientIPHeader string) *RateLimiter {
	return NewRateLimiter(map[string]RateLimitRule{
		"/api/login": {Limit: 1, Window: time.Minute},
	}, clientIPHeader)
}

func TestRateLimiter_HeaderSeparatesClientsBehindSameProxy(t *testing.T) {
	h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

	first := serve(h, loginRequest(http.MethodPost, "203.0.113.1"))
	other := serve(h, loginRequest(http.MethodPost, "203.0.113.2"))
	again := serve(h, loginRequest(http.MethodPost, "203.0.113.1"))

	if first.Code != http.StatusOK || other.Code != http.StatusOK {
		t.Errorf("códigos = %d e %d, want 200 e 200", first.Code, other.Code)
	}
	if again.Code != http.StatusTooManyRequests {
		t.Errorf("repetição do mesmo cliente = %d, want 429", again.Code)
	}
}

func TestRateLimiter_EmptyHeaderConfigUsesRemoteAddr(t *testing.T) {
	h := oneLoginPerMinute("").Middleware(okHandler())

	serve(h, loginRequest(http.MethodPost, "203.0.113.1"))
	second := serve(h, loginRequest(http.MethodPost, "203.0.113.2"))

	if second.Code != http.StatusTooManyRequests {
		t.Errorf("segunda requisição do mesmo RemoteAddr = %d, want 429", second.Code)
	}
}

func TestRateLimiter_RetryAfterInSeconds(t *testing.T) {
	h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

	serve(h, loginRequest(http.MethodPost, "203.0.113.1"))
	limited := serve(h, loginRequest(http.MethodPost, "203.0.113.1"))

	if got := limited.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want %q", got, "60")
	}
}

func TestRateLimiter_InvalidHeaderFallsBackToRemoteAddr(t *testing.T) {
	h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

	serve(h, loginRequest(http.MethodPost, "nao-e-ip"))
	second := serve(h, loginRequest(http.MethodPost, "outro-valor-forjado"))

	if second.Code != http.StatusTooManyRequests {
		t.Errorf("segundo valor forjado do mesmo RemoteAddr = %d, want 429", second.Code)
	}
}

func TestRateLimiter_NonPublicIPInHeaderFallsBackToRemoteAddr(t *testing.T) {
	cases := []struct {
		name  string
		first string
		other string
	}{
		{name: "rfc1918_class_a", first: "10.0.0.5", other: "10.1.1.1"},
		{name: "rfc1918_class_b", first: "172.16.0.1", other: "172.20.0.1"},
		{name: "rfc1918_class_c", first: "192.168.1.1", other: "192.168.1.2"},
		{name: "loopback_ipv4", first: "127.0.0.1", other: "127.0.0.2"},
		{name: "loopback_ipv6", first: "::1", other: "::2"},
		{name: "link_local_unicast_ipv4", first: "169.254.1.1", other: "169.254.1.2"},
		{name: "link_local_unicast_ipv6", first: "fe80::1", other: "fe80::2"},
		{name: "unspecified", first: "0.0.0.0", other: "::"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

			first := serve(h, loginRequest(http.MethodPost, tc.first))
			second := serve(h, loginRequest(http.MethodPost, tc.other))

			if first.Code != http.StatusOK {
				t.Errorf("primeira requisição = %d, want 200", first.Code)
			}
			if second.Code != http.StatusTooManyRequests {
				t.Errorf("segunda requisição com outro IP não público no header = %d, want 429", second.Code)
			}
		})
	}
}

func TestRateLimiter_NonPublicHeaderLogsOnce(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)

	h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

	serve(h, loginRequest(http.MethodPost, "10.0.0.5"))
	serve(h, loginRequest(http.MethodPost, "10.0.0.5"))

	withoutHeader := loginRequest(http.MethodPost, "")
	withoutHeader.Header.Del("CF-Connecting-IP")
	serve(h, withoutHeader)
	serve(h, loginRequest(http.MethodPost, "nao-e-ip"))

	if got := strings.Count(logged.String(), "\n"); got != 1 {
		t.Errorf("linhas de log = %d, want 1; log = %q", got, logged.String())
	}
}

func TestRateLimiter_IgnoresXForwardedFor(t *testing.T) {
	h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

	first := loginRequest(http.MethodPost, "203.0.113.1")
	first.Header.Set("X-Forwarded-For", "198.51.100.1")
	second := loginRequest(http.MethodPost, "203.0.113.1")
	second.Header.Set("X-Forwarded-For", "198.51.100.2")
	withoutHeader := loginRequest(http.MethodPost, "")
	withoutHeader.Header.Del("CF-Connecting-IP")
	withoutHeader.Header.Set("X-Forwarded-For", "198.51.100.3")
	withoutHeaderOtherXFF := loginRequest(http.MethodPost, "")
	withoutHeaderOtherXFF.Header.Del("CF-Connecting-IP")
	withoutHeaderOtherXFF.Header.Set("X-Forwarded-For", "203.0.113.9")

	serve(h, first)
	if got := serve(h, second); got.Code != http.StatusTooManyRequests {
		t.Errorf("segunda requisição da mesma CF-Connecting-IP com outro X-Forwarded-For = %d, want 429", got.Code)
	}
	serve(h, withoutHeader)
	if got := serve(h, withoutHeaderOtherXFF); got.Code != http.StatusTooManyRequests {
		t.Errorf("segunda requisição sem CF-Connecting-IP com outro X-Forwarded-For = %d, want 429", got.Code)
	}
}

func TestRateLimiter_MissingHeaderFallsBackToRemoteAddr(t *testing.T) {
	h := oneLoginPerMinute("CF-Connecting-IP").Middleware(okHandler())

	serve(h, loginRequest(http.MethodPost, ""))
	second := serve(h, loginRequest(http.MethodPost, ""))

	if second.Code != http.StatusTooManyRequests {
		t.Errorf("segunda requisição sem header do mesmo RemoteAddr = %d, want 429", second.Code)
	}
}

func TestRateLimiter_RetryAfterIsTimeLeftInWindow(t *testing.T) {
	limiter := NewRateLimiter(map[string]RateLimitRule{
		"/api/login": {Limit: 1, Window: 2 * time.Second},
	}, "CF-Connecting-IP")
	h := limiter.Middleware(okHandler())

	serve(h, loginRequest(http.MethodPost, "203.0.113.1"))
	time.Sleep(1100 * time.Millisecond)
	limited := serve(h, loginRequest(http.MethodPost, "203.0.113.1"))

	if got := limited.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q", got, "1")
	}
}

func TestEdgeMiddleware_PreflightDoesNotConsumeLimit(t *testing.T) {
	h := EdgeMiddleware(okHandler(), []string{testOrigin}, oneLoginPerMinute("CF-Connecting-IP"))

	serve(h, loginRequest(http.MethodOptions, "203.0.113.1"))
	login := serve(h, loginRequest(http.MethodPost, "203.0.113.1"))

	if login.Code != http.StatusOK {
		t.Errorf("login depois do preflight = %d, want 200", login.Code)
	}
}

func TestEdgeMiddleware_TooManyRequestsCarriesCORS(t *testing.T) {
	h := EdgeMiddleware(okHandler(), []string{testOrigin}, oneLoginPerMinute("CF-Connecting-IP"))

	serve(h, loginRequest(http.MethodPost, "203.0.113.1"))
	limited := serve(h, loginRequest(http.MethodPost, "203.0.113.1"))

	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("código = %d, want 429", limited.Code)
	}
	if got := limited.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, testOrigin)
	}
}
