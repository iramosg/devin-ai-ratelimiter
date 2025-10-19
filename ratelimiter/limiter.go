package ratelimiter

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/iramosg/devin-ai-ratelimiter/storage"
)

type Storage interface {
	GetClientData(clientID string) (*storage.ClientData, bool)
	SetClientData(clientID string, data *storage.ClientData)
	IncrementRequestCount(clientID string) int
	ResetWindow(clientID string, windowStart time.Time)
	BlockClient(clientID string, blockedUntil time.Time)
	DeleteClient(clientID string)
	Clear()
}

type RateLimiter struct {
	storage         Storage
	maxRequests     int
	windowDuration  time.Duration
	blockDuration   time.Duration
	errorMessage    string
	includeJSON     bool
	logger          *slog.Logger
	logOnExceedOnly bool
}

type Option func(*RateLimiter)

func WithMaxRequests(max int) Option {
	return func(rl *RateLimiter) {
		rl.maxRequests = max
	}
}

func WithWindowDuration(duration time.Duration) Option {
	return func(rl *RateLimiter) {
		rl.windowDuration = duration
	}
}

func WithBlockDuration(duration time.Duration) Option {
	return func(rl *RateLimiter) {
		rl.blockDuration = duration
	}
}

func WithErrorMessage(message string) Option {
	return func(rl *RateLimiter) {
		rl.errorMessage = message
	}
}

func WithIncludeJSON(include bool) Option {
	return func(rl *RateLimiter) {
		rl.includeJSON = include
	}
}

func WithStorage(storage Storage) Option {
	return func(rl *RateLimiter) {
		rl.storage = storage
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(rl *RateLimiter) {
		rl.logger = logger
	}
}

func WithLogOnExceedOnly(logOnExceedOnly bool) Option {
	return func(rl *RateLimiter) {
		rl.logOnExceedOnly = logOnExceedOnly
	}
}

func New(opts ...Option) *RateLimiter {
	rl := &RateLimiter{
		storage:         storage.NewMemoryStorage(),
		maxRequests:     100,
		windowDuration:  time.Minute,
		blockDuration:   time.Minute,
		errorMessage:    "Rate limit exceeded",
		includeJSON:     true,
		logger:          slog.Default(),
		logOnExceedOnly: true,
	}

	for _, opt := range opts {
		opt(rl)
	}

	return rl
}

type Result struct {
	Allowed       bool
	RequestsMade  int
	Limit         int
	RetryAfter    time.Time
	RetryAfterSec int
	ErrorMessage  string
}

func (rl *RateLimiter) Allow(clientID string) *Result {
	now := time.Now()

	data, exists := rl.storage.GetClientData(clientID)

	if exists && !data.BlockedUntil.IsZero() && now.Before(data.BlockedUntil) {
		retryAfterSec := int(time.Until(data.BlockedUntil).Seconds())
		if retryAfterSec < 1 {
			retryAfterSec = 1
		}

		if rl.logOnExceedOnly {
			rl.logger.Info("Client blocked",
				slog.String("client_id", clientID),
				slog.Int("requests_made", data.RequestCount),
				slog.Int("limit", rl.maxRequests),
				slog.Time("retry_after", data.BlockedUntil),
			)
		}

		return &Result{
			Allowed:       false,
			RequestsMade:  data.RequestCount,
			Limit:         rl.maxRequests,
			RetryAfter:    data.BlockedUntil,
			RetryAfterSec: retryAfterSec,
			ErrorMessage:  rl.errorMessage,
		}
	}

	if exists && (!data.BlockedUntil.IsZero() && now.After(data.BlockedUntil)) {
		rl.storage.ResetWindow(clientID, now)
		exists = false
	}

	if exists && now.Sub(data.WindowStart) >= rl.windowDuration {
		rl.storage.ResetWindow(clientID, now)
		exists = false
	}

	var requestCount int
	if !exists {
		requestCount = 1
		rl.storage.SetClientData(clientID, &storage.ClientData{
			RequestCount: 1,
			WindowStart:  now,
			BlockedUntil: time.Time{},
		})
	} else {
		requestCount = rl.storage.IncrementRequestCount(clientID)
	}

	if requestCount > rl.maxRequests {
		blockedUntil := now.Add(rl.blockDuration)
		rl.storage.BlockClient(clientID, blockedUntil)

		retryAfterSec := int(rl.blockDuration.Seconds())

		if rl.logOnExceedOnly {
			rl.logger.Info("Rate limit exceeded",
				slog.String("client_id", clientID),
				slog.Int("requests_made", requestCount),
				slog.Int("limit", rl.maxRequests),
				slog.Time("retry_after", blockedUntil),
			)
		}

		return &Result{
			Allowed:       false,
			RequestsMade:  requestCount,
			Limit:         rl.maxRequests,
			RetryAfter:    blockedUntil,
			RetryAfterSec: retryAfterSec,
			ErrorMessage:  rl.errorMessage,
		}
	}

	return &Result{
		Allowed:      true,
		RequestsMade: requestCount,
		Limit:        rl.maxRequests,
	}
}

func (r *Result) FormatJSON() string {
	return fmt.Sprintf(`{"error":"%s","limit":%d,"requests_made":%d,"retry_after":"%s"}`,
		r.ErrorMessage,
		r.Limit,
		r.RequestsMade,
		r.RetryAfter.Format(time.RFC3339),
	)
}
