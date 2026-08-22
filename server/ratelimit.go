// This file implements a simple per-client-IP rate limiter middleware using
// golang.org/x/time/rate's token bucket, the standard Go rate-limiting
// primitive. Each distinct remote IP gets its own limiter, created lazily
// and reclaimed by a periodic sweep of idle entries so the limiter map does
// not grow unbounded under many distinct clients.
package server

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiterIdleTTL is how long a per-IP limiter may sit unused before a
// sweep reclaims it.
const rateLimiterIdleTTL = 10 * time.Minute

type clientLimiter struct {
	limiter    *rate.Limiter
	lastSeenAt time.Time
}

// ipRateLimiter tracks one token-bucket limiter per client IP.
type ipRateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientLimiter
	rps      rate.Limit
	burst    int
	stopSwap chan struct{}
}

// newIPRateLimiter builds a limiter allowing rps requests/second, per
// client IP, with the given burst. A background goroutine periodically
// evicts limiters that have been idle longer than rateLimiterIdleTTL; call
// the returned stop function to end it (used in tests).
func newIPRateLimiter(rps float64, burst int) (*ipRateLimiter, func()) {
	l := &ipRateLimiter{
		clients:  map[string]*clientLimiter{},
		rps:      rate.Limit(rps),
		burst:    burst,
		stopSwap: make(chan struct{}),
	}
	go l.sweepLoop()
	return l, func() { close(l.stopSwap) }
}

func (l *ipRateLimiter) sweepLoop() {
	ticker := time.NewTicker(rateLimiterIdleTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.sweep()
		case <-l.stopSwap:
			return
		}
	}
}

func (l *ipRateLimiter) sweep() {
	cutoff := time.Now().Add(-rateLimiterIdleTTL)
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, c := range l.clients {
		if c.lastSeenAt.Before(cutoff) {
			delete(l.clients, ip)
		}
	}
}

func (l *ipRateLimiter) allow(clientIP string) bool {
	l.mu.Lock()
	c, ok := l.clients[clientIP]
	if !ok {
		c = &clientLimiter{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.clients[clientIP] = c
	}
	c.lastSeenAt = time.Now()
	limiter := c.limiter
	l.mu.Unlock()
	return limiter.Allow()
}

// middleware wraps next, rejecting requests over the per-IP rate with
// 429 Too Many Requests once the client's token bucket is exhausted.
func (l *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the request's client IP from RemoteAddr, falling back
// to the raw value if it isn't a host:port pair.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
