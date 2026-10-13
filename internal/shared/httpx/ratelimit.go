package httpx

import (
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimitRule struct {
	Limit  int
	Window time.Duration
}

type rateBucket struct {
	count     int
	windowEnd time.Time
}

type RateLimiter struct {
	mu             sync.Mutex
	rules          []routeRule
	buckets        map[string]*rateBucket
	clientIPHeader string
}

type routeRule struct {
	prefix string
	rule   RateLimitRule
}

func NewRateLimiter(rules map[string]RateLimitRule, clientIPHeader string) *RateLimiter {
	ordered := make([]routeRule, 0, len(rules))
	for prefix, rule := range rules {
		ordered = append(ordered, routeRule{prefix: prefix, rule: rule})
	}

	sort.Slice(ordered, func(i, j int) bool {
		return len(ordered[i].prefix) > len(ordered[j].prefix)
	})

	rl := &RateLimiter{
		rules:          ordered,
		buckets:        make(map[string]*rateBucket),
		clientIPHeader: clientIPHeader,
	}

	go rl.janitor()
	return rl
}

func (rl *RateLimiter) janitor() {
	for {
		time.Sleep(time.Minute)
		now := time.Now()

		rl.mu.Lock()
		for key, bucket := range rl.buckets {
			if now.After(bucket.windowEnd) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rule, ok := rl.ruleFor(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		key := rl.clientIP(r) + r.URL.Path

		rl.mu.Lock()
		bucket, exists := rl.buckets[key]
		if !exists || time.Now().After(bucket.windowEnd) {
			rl.buckets[key] = &rateBucket{
				count:     1,
				windowEnd: time.Now().Add(rule.Window),
			}
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		bucket.count++
		if bucket.count > rule.Limit {
			rl.mu.Unlock()
			w.Header().Set("Retry-After", strconv.Itoa(int(rule.Window/time.Second)))
			http.Error(w, "Muitas requisições. Tente novamente em instantes", http.StatusTooManyRequests)
			return
		}
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) ruleFor(path string) (RateLimitRule, bool) {
	for _, rr := range rl.rules {
		if strings.HasPrefix(path, rr.prefix) {
			return rr.rule, true
		}
	}
	return RateLimitRule{}, false
}

func (rl *RateLimiter) clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get(rl.clientIPHeader)); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func EdgeMiddleware(next http.Handler, corsOrigins []string, limiter *RateLimiter) http.Handler {
	return SecurityHeaders(CORS(corsOrigins)(limiter.Middleware(next)))
}
