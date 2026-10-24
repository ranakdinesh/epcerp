package httpmiddleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Limiter uses a bounded sliding-window map. Replicas require edge rate limiting.
type Limiter struct {
	mu          sync.Mutex
	requests    map[string][]time.Time
	maximum     int
	window      time.Duration
	capacity    int
	clock       func() time.Time
	nextCleanup time.Time
}

func NewLimiter(maximum int, window time.Duration) *Limiter {
	if maximum < 1 || window <= 0 {
		panic("invalid rate limiter configuration")
	}
	return &Limiter{requests: map[string][]time.Time{}, maximum: maximum, window: window, capacity: 10000, clock: time.Now}
}
func (l *Limiter) Allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	cutoff := now.Add(-l.window)
	if !now.Before(l.nextCleanup) {
		for key, timestamps := range l.requests {
			if len(timestamps) == 0 || !timestamps[len(timestamps)-1].After(cutoff) {
				delete(l.requests, key)
			}
		}
		l.nextCleanup = now.Add(l.window)
	}
	timestamps, exists := l.requests[client]
	if !exists && len(l.requests) >= l.capacity {
		return false
	}
	first := 0
	for first < len(timestamps) && !timestamps[first].After(cutoff) {
		first++
	}
	if first > 0 {
		copy(timestamps, timestamps[first:])
		timestamps = timestamps[:len(timestamps)-first]
	}
	if len(timestamps) >= l.maximum {
		l.requests[client] = timestamps
		return false
	}
	l.requests[client] = append(timestamps, now)
	return true
}
func RateLimit(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && (r.URL.Path == "/api/v1/health/live" || r.URL.Path == "/api/v1/health/ready") {
				next.ServeHTTP(w, r)
				return
			}
			// Ignore X-Forwarded-For until a reviewed trusted-proxy policy is implemented.
			client := r.RemoteAddr
			if host, _, err := net.SplitHostPort(client); err == nil {
				client = host
			}
			if client == "" {
				client = "unknown"
			}
			if !limiter.Allow(client) {
				seconds := int64((limiter.window + time.Second - 1) / time.Second)
				w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
				reject(w, r, 429, "Too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
