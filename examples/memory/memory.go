package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/iramosg/devin-ai-ratelimiter/middleware"
	"github.com/iramosg/devin-ai-ratelimiter/ratelimiter"
)

type Response struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	ClientIP  string    `json:"client_ip"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	limiter := ratelimiter.New(
		ratelimiter.WithMaxRequests(100),
		ratelimiter.WithWindowDuration(time.Minute),
		ratelimiter.WithBlockDuration(time.Minute),
		ratelimiter.WithLogger(logger),
		ratelimiter.WithLogOnExceedOnly(true),
	)

	rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(limiter)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clientIP := middleware.DefaultClientIDExtractor(r)

		response := Response{
			Message:   "Hello! This endpoint is protected by rate limiting.",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		})
	})

	handler := rateLimiterMiddleware.Handler(mux)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting server on %s", server.Addr)
	log.Printf("Rate limit: 100 requests per minute")
	log.Printf("Block duration: 1 minute")
	log.Println("Try accessing http://localhost:8080/")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
