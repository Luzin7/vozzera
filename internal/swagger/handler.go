package swagger

import (
	"crypto/subtle"
	"embed"
	"io/fs"
	"net/http"
)

type HandlerConfig struct {
	Enabled  bool
	User     string
	Password string
}

const swaggerCSP = "default-src 'none'; script-src 'self' https://unpkg.com 'unsafe-inline'; style-src 'self' https://unpkg.com 'unsafe-inline'; img-src 'self' data: https://unpkg.com; font-src 'self' data: https://unpkg.com; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

//go:embed all:static
var staticFS embed.FS

//go:embed openapi.yaml
var openapiYAML []byte

func RegisterHandlers(mux *http.ServeMux, cfg HandlerConfig) {
	if !cfg.Enabled {
		registerNotFound(mux)
		return
	}

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	guard := func(next http.Handler) http.Handler {
		return withSwaggerCSP(basicAuth(cfg)(next))
	}
	ui := guard(http.StripPrefix("/swagger", http.FileServer(http.FS(sub))))

	mux.Handle("GET /swagger", ui)
	mux.Handle("GET /swagger/", ui)
	mux.Handle("GET /swagger/openapi.yaml", guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapiYAML)
	})))
}

func registerNotFound(mux *http.ServeMux) {
	notFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.Handle("GET /swagger", notFound)
	mux.Handle("GET /swagger/", notFound)
	mux.Handle("GET /swagger/openapi.yaml", notFound)
}

func withSwaggerCSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", swaggerCSP)
		next.ServeHTTP(w, r)
	})
}

func basicAuth(cfg HandlerConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, password, ok := r.BasicAuth()
			if !ok || !credentialsMatch(user, password, cfg) {
				w.Header().Set("WWW-Authenticate", `Basic realm="swagger"`)
				http.Error(w, "Não autorizado", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func credentialsMatch(user, password string, cfg HandlerConfig) bool {
	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.User))
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password))
	return userMatch&passwordMatch == 1
}
