package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/iramosg/devin-ai-ratelimiter/ratelimiter"
)

type ClientIDExtractor func(*http.Request) string

func DefaultClientIDExtractor(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

type RateLimiterMiddleware struct {
	limiter           *ratelimiter.RateLimiter
	clientIDExtractor ClientIDExtractor
	includeJSON       bool
}

type MiddlewareOption func(*RateLimiterMiddleware)

func WithClientIDExtractor(extractor ClientIDExtractor) MiddlewareOption {
	return func(m *RateLimiterMiddleware) {
		m.clientIDExtractor = extractor
	}
}

func WithIncludeJSON(include bool) MiddlewareOption {
	return func(m *RateLimiterMiddleware) {
		m.includeJSON = include
	}
}

func NewRateLimiterMiddleware(limiter *ratelimiter.RateLimiter, opts ...MiddlewareOption) *RateLimiterMiddleware {
	m := &RateLimiterMiddleware{
		limiter:           limiter,
		clientIDExtractor: DefaultClientIDExtractor,
		includeJSON:       true,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := m.clientIDExtractor(r)

		result := m.limiter.Allow(clientID)

		if !result.Allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", result.RetryAfterSec))

			if m.includeJSON {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(result.FormatJSON()))
			} else {
				w.WriteHeader(http.StatusTooManyRequests)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *RateLimiterMiddleware) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := m.clientIDExtractor(r)

		result := m.limiter.Allow(clientID)

		if !result.Allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", result.RetryAfterSec))

			if m.includeJSON {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(result.FormatJSON()))
			} else {
				w.WriteHeader(http.StatusTooManyRequests)
			}
			return
		}

		next(w, r)
	}
}
