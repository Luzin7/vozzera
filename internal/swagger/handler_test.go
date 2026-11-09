package swagger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testUser     = "swagger-user"
	testPassword = "swagger-password"
)

func serveSwagger(cfg HandlerConfig, method, path, user, password string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	RegisterHandlers(mux, cfg)

	req := httptest.NewRequest(method, path, nil)
	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestRegisterHandlers_DisabledReturnsNotFound(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "ui_without_slash", path: "/swagger"},
		{name: "ui", path: "/swagger/"},
		{name: "openapi", path: "/swagger/openapi.yaml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serveSwagger(HandlerConfig{Enabled: false}, http.MethodGet, tc.path, "", "")

			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
			}
		})
	}
}

func TestRegisterHandlers_EnabledRequiresBasicAuth(t *testing.T) {
	cases := []struct {
		name     string
		user     string
		password string
	}{
		{name: "without_credentials", user: "", password: ""},
		{name: "wrong_password", user: testUser, password: "wrong"},
		{name: "wrong_user", user: "wrong", password: testPassword},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serveSwagger(enabledConfig(), http.MethodGet, "/swagger", tc.user, tc.password)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
			if got := w.Header().Get("WWW-Authenticate"); got != `Basic realm="swagger"` {
				t.Errorf("WWW-Authenticate = %q, want %q", got, `Basic realm="swagger"`)
			}
		})
	}
}

func TestRegisterHandlers_EnabledWithCredentialsServesUIWithOwnCSP(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{name: "ui_without_slash", path: "/swagger"},
		{name: "ui", path: "/swagger/"},
		{name: "openapi", path: "/swagger/openapi.yaml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := serveSwagger(enabledConfig(), http.MethodGet, tc.path, testUser, testPassword)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			csp := w.Header().Get("Content-Security-Policy")
			for _, allowed := range []string{"https://unpkg.com", "'unsafe-inline'"} {
				if !strings.Contains(csp, allowed) {
					t.Errorf("Content-Security-Policy = %q, want it to contain %q", csp, allowed)
				}
			}
		})
	}
}

func enabledConfig() HandlerConfig {
	return HandlerConfig{Enabled: true, User: testUser, Password: testPassword}
}
