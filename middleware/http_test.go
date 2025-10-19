package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iramosg/devin-ai-ratelimiter/ratelimiter"
)

func TestDefaultClientIDExtractor_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	clientID := DefaultClientIDExtractor(req)

	if clientID != "192.168.1.1" {
		t.Errorf("Expected clientID '192.168.1.1', got '%s'", clientID)
	}
}

func TestDefaultClientIDExtractor_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")

	clientID := DefaultClientIDExtractor(req)

	if clientID != "10.0.0.1" {
		t.Errorf("Expected clientID '10.0.0.1', got '%s'", clientID)
	}
}

func TestDefaultClientIDExtractor_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Real-IP", "10.0.0.5")

	clientID := DefaultClientIDExtractor(req)

	if clientID != "10.0.0.5" {
		t.Errorf("Expected clientID '10.0.0.5', got '%s'", clientID)
	}
}

func TestDefaultClientIDExtractor_Priority(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("X-Real-IP", "10.0.0.5")

	clientID := DefaultClientIDExtractor(req)

	if clientID != "10.0.0.1" {
		t.Errorf("Expected X-Forwarded-For to take priority, got '%s'", clientID)
	}
}

func TestNewRateLimiterMiddleware_DefaultOptions(t *testing.T) {
	limiter := ratelimiter.New()
	middleware := NewRateLimiterMiddleware(limiter)

	if middleware.limiter == nil {
		t.Error("Expected limiter to be set")
	}
	if middleware.clientIDExtractor == nil {
		t.Error("Expected clientIDExtractor to be set")
	}
	if !middleware.includeJSON {
		t.Error("Expected includeJSON to be true by default")
	}
}

func TestNewRateLimiterMiddleware_WithOptions(t *testing.T) {
	limiter := ratelimiter.New()
	customExtractor := func(r *http.Request) string {
		return "custom-id"
	}

	middleware := NewRateLimiterMiddleware(
		limiter,
		WithClientIDExtractor(customExtractor),
		WithIncludeJSON(false),
	)

	if middleware.includeJSON {
		t.Error("Expected includeJSON to be false")
	}

	req := httptest.NewRequest("GET", "/", nil)
	clientID := middleware.clientIDExtractor(req)
	if clientID != "custom-id" {
		t.Errorf("Expected custom extractor to return 'custom-id', got '%s'", clientID)
	}
}

func TestHandler_AllowedRequest(t *testing.T) {
	limiter := ratelimiter.New(ratelimiter.WithMaxRequests(10))
	middleware := NewRateLimiterMiddleware(limiter)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", rec.Body.String())
	}
}

func TestHandler_BlockedRequest(t *testing.T) {
	limiter := ratelimiter.New(
		ratelimiter.WithMaxRequests(2),
		ratelimiter.WithBlockDuration(time.Minute),
	)
	middleware := NewRateLimiterMiddleware(limiter)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header to be set")
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("Expected JSON response body")
	}
}

func TestHandler_WithoutJSON(t *testing.T) {
	limiter := ratelimiter.New(ratelimiter.WithMaxRequests(1))
	middleware := NewRateLimiterMiddleware(limiter, WithIncludeJSON(false))

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType == "application/json" {
		t.Error("Expected Content-Type to not be set to application/json")
	}

	body := rec.Body.String()
	if body != "" {
		t.Errorf("Expected empty body, got '%s'", body)
	}
}

func TestHandlerFunc_AllowedRequest(t *testing.T) {
	limiter := ratelimiter.New(ratelimiter.WithMaxRequests(10))
	middleware := NewRateLimiterMiddleware(limiter)

	handlerFunc := middleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handlerFunc(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", rec.Body.String())
	}
}

func TestHandlerFunc_BlockedRequest(t *testing.T) {
	limiter := ratelimiter.New(ratelimiter.WithMaxRequests(1))
	middleware := NewRateLimiterMiddleware(limiter)

	handlerFunc := middleware.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rec := httptest.NewRecorder()
	handlerFunc(rec, req)

	rec = httptest.NewRecorder()
	handlerFunc(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rec.Code)
	}
}

func TestHandler_MultipleClients(t *testing.T) {
	limiter := ratelimiter.New(ratelimiter.WithMaxRequests(2))
	middleware := NewRateLimiterMiddleware(limiter)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.2:12345"

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req1)
		if rec.Code != http.StatusOK {
			t.Errorf("Client 1 request %d should be allowed", i+1)
		}
	}

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusTooManyRequests {
		t.Error("Client 1 should be blocked")
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Error("Client 2 should still be allowed")
	}
}

func TestHandler_CustomClientIDExtractor(t *testing.T) {
	limiter := ratelimiter.New(ratelimiter.WithMaxRequests(1))

	customExtractor := func(r *http.Request) string {
		return r.Header.Get("X-API-Key")
	}

	middleware := NewRateLimiterMiddleware(limiter, WithClientIDExtractor(customExtractor))

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-API-Key", "key1")

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-API-Key", "key2")

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Error("First request with key1 should be allowed")
	}

	rec1Again := httptest.NewRecorder()
	handler.ServeHTTP(rec1Again, req1)
	if rec1Again.Code != http.StatusTooManyRequests {
		t.Error("Second request with key1 should be blocked")
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Error("First request with key2 should be allowed")
	}
}
