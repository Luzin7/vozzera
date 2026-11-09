package httpx

import "net/http"

const (
	securityCSP  = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	securityHSTS = "max-age=31536000; includeSubDomains"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", securityCSP)
		if requestIsHTTPS(r) {
			w.Header().Set("Strict-Transport-Security", securityHSTS)
		}

		next.ServeHTTP(w, r)
	})
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
