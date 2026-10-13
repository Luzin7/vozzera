package httpx

import (
	"net/http"
	"net/http/httptest"
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
