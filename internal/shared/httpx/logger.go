package httpx

import (
	"bufio"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.ResponseWriter.WriteHeader(statusCode)
}

func (rec *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rec.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("o ResponseWriter subjacente não suporta a interface Hijacker")
	}
	return hijacker.Hijack()
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		log.Printf("[HTTP] %s %s | Status: %d | %v", r.Method, r.URL.Path, rec.statusCode, duration)
	})
}
