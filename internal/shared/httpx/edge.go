package httpx

import "net/http"

func EdgeMiddleware(next http.Handler, corsOrigins []string, limiter *RateLimiter) http.Handler {
	return SecurityHeaders(CORS(corsOrigins)(limiter.Middleware(next)))
}
