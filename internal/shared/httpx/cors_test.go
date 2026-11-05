package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const corsAllowedOrigin = "https://app.example.com"

func serveCORS(method, origin string) *httptest.ResponseRecorder {
	handler := CORS([]string{corsAllowedOrigin})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(method, "/api/rooms", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestCORS_AllowedOriginActualResponseExposesHeaders(t *testing.T) {
	w := serveCORS(http.MethodGet, corsAllowedOrigin)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	want := map[string]string{
		"Access-Control-Allow-Origin":      corsAllowedOrigin,
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Expose-Headers":    "X-Next-Before",
	}
	for header, value := range want {
		if got := w.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestCORS_PreflightReturnsNoContentWithAllowHeaders(t *testing.T) {
	w := serveCORS(http.MethodOptions, corsAllowedOrigin)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	want := map[string]string{
		"Access-Control-Allow-Origin":  corsAllowedOrigin,
		"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}
	for header, value := range want {
		if got := w.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestCORS_DisallowedOriginIsForbiddenWithoutCORSHeaders(t *testing.T) {
	w := serveCORS(http.MethodGet, "https://evil.example.com")

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}
