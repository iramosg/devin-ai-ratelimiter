package ratelimiter

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/iramosg/devin-ai-ratelimiter/storage"
)

func TestNew_DefaultValues(t *testing.T) {
	rl := New()

	if rl.maxRequests != 100 {
		t.Errorf("Expected default maxRequests 100, got %d", rl.maxRequests)
	}
	if rl.windowDuration != time.Minute {
		t.Errorf("Expected default windowDuration 1 minute, got %v", rl.windowDuration)
	}
	if rl.blockDuration != time.Minute {
		t.Errorf("Expected default blockDuration 1 minute, got %v", rl.blockDuration)
	}
	if rl.errorMessage != "Rate limit exceeded" {
		t.Errorf("Expected default errorMessage 'Rate limit exceeded', got %s", rl.errorMessage)
	}
	if !rl.includeJSON {
		t.Error("Expected default includeJSON to be true")
	}
	if !rl.logOnExceedOnly {
		t.Error("Expected default logOnExceedOnly to be true")
	}
	if rl.storage == nil {
		t.Error("Expected storage to be initialized")
	}
}

func TestNew_WithOptions(t *testing.T) {
	customStorage := storage.NewMemoryStorage()
	customLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	rl := New(
		WithMaxRequests(50),
		WithWindowDuration(30*time.Second),
		WithBlockDuration(2*time.Minute),
		WithErrorMessage("Custom error"),
		WithIncludeJSON(false),
		WithStorage(customStorage),
		WithLogger(customLogger),
		WithLogOnExceedOnly(false),
	)

	if rl.maxRequests != 50 {
		t.Errorf("Expected maxRequests 50, got %d", rl.maxRequests)
	}
	if rl.windowDuration != 30*time.Second {
		t.Errorf("Expected windowDuration 30s, got %v", rl.windowDuration)
	}
	if rl.blockDuration != 2*time.Minute {
		t.Errorf("Expected blockDuration 2m, got %v", rl.blockDuration)
	}
	if rl.errorMessage != "Custom error" {
		t.Errorf("Expected errorMessage 'Custom error', got %s", rl.errorMessage)
	}
	if rl.includeJSON {
		t.Error("Expected includeJSON to be false")
	}
	if rl.logOnExceedOnly {
		t.Error("Expected logOnExceedOnly to be false")
	}
}

func TestAllow_FirstRequest(t *testing.T) {
	rl := New(WithMaxRequests(10))
	clientID := "test-client"

	result := rl.Allow(clientID)

	if !result.Allowed {
		t.Error("Expected first request to be allowed")
	}
	if result.RequestsMade != 1 {
		t.Errorf("Expected RequestsMade 1, got %d", result.RequestsMade)
	}
	if result.Limit != 10 {
		t.Errorf("Expected Limit 10, got %d", result.Limit)
	}
}

func TestAllow_WithinLimit(t *testing.T) {
	rl := New(WithMaxRequests(5))
	clientID := "test-client"

	for i := 1; i <= 5; i++ {
		result := rl.Allow(clientID)
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i)
		}
		if result.RequestsMade != i {
			t.Errorf("Expected RequestsMade %d, got %d", i, result.RequestsMade)
		}
	}
}

func TestAllow_ExceedLimit(t *testing.T) {
	rl := New(WithMaxRequests(3), WithBlockDuration(time.Minute))
	clientID := "test-client"

	for i := 1; i <= 3; i++ {
		result := rl.Allow(clientID)
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	result := rl.Allow(clientID)
	if result.Allowed {
		t.Error("Request exceeding limit should not be allowed")
	}
	if result.RequestsMade != 4 {
		t.Errorf("Expected RequestsMade 4, got %d", result.RequestsMade)
	}
	if result.ErrorMessage != "Rate limit exceeded" {
		t.Errorf("Expected error message, got %s", result.ErrorMessage)
	}
	if result.RetryAfterSec <= 0 {
		t.Error("Expected RetryAfterSec to be positive")
	}
}

func TestAllow_BlockedClient(t *testing.T) {
	rl := New(WithMaxRequests(2), WithBlockDuration(100*time.Millisecond))
	clientID := "test-client"

	rl.Allow(clientID)
	rl.Allow(clientID)
	result := rl.Allow(clientID)

	if result.Allowed {
		t.Error("Client should be blocked after exceeding limit")
	}

	result = rl.Allow(clientID)
	if result.Allowed {
		t.Error("Blocked client should remain blocked")
	}

	time.Sleep(150 * time.Millisecond)

	result = rl.Allow(clientID)
	if !result.Allowed {
		t.Error("Client should be unblocked after block duration")
	}
}

func TestAllow_WindowReset(t *testing.T) {
	rl := New(WithMaxRequests(3), WithWindowDuration(100*time.Millisecond))
	clientID := "test-client"

	rl.Allow(clientID)
	rl.Allow(clientID)
	rl.Allow(clientID)

	time.Sleep(150 * time.Millisecond)

	result := rl.Allow(clientID)
	if !result.Allowed {
		t.Error("Request should be allowed after window reset")
	}
	if result.RequestsMade != 1 {
		t.Errorf("Expected RequestsMade 1 after window reset, got %d", result.RequestsMade)
	}
}

func TestAllow_MultipleClients(t *testing.T) {
	rl := New(WithMaxRequests(2))

	result1 := rl.Allow("client1")
	result2 := rl.Allow("client2")

	if !result1.Allowed || !result2.Allowed {
		t.Error("Both clients should be allowed independently")
	}

	rl.Allow("client1")
	rl.Allow("client1")

	result1 = rl.Allow("client1")
	result2 = rl.Allow("client2")

	if result1.Allowed {
		t.Error("client1 should be blocked")
	}
	if !result2.Allowed {
		t.Error("client2 should still be allowed")
	}
}

func TestResult_FormatJSON(t *testing.T) {
	retryAfter := time.Date(2025, 2, 6, 14, 30, 0, 0, time.UTC)
	result := &Result{
		Allowed:      false,
		RequestsMade: 120,
		Limit:        100,
		RetryAfter:   retryAfter,
		ErrorMessage: "Rate limit exceeded",
	}

	json := result.FormatJSON()

	expectedJSON := `{"error":"Rate limit exceeded","limit":100,"requests_made":120,"retry_after":"2025-02-06T14:30:00Z"}`
	if json != expectedJSON {
		t.Errorf("Expected JSON:\n%s\nGot:\n%s", expectedJSON, json)
	}
}

func TestAllow_ConcurrentRequests(t *testing.T) {
	rl := New(WithMaxRequests(100))
	clientID := "concurrent-client"

	done := make(chan bool)
	allowedCount := 0
	blockedCount := 0

	for i := 0; i < 150; i++ {
		go func() {
			result := rl.Allow(clientID)
			if result.Allowed {
				allowedCount++
			} else {
				blockedCount++
			}
			done <- true
		}()
	}

	for i := 0; i < 150; i++ {
		<-done
	}

	if allowedCount > 100 {
		t.Errorf("Expected at most 100 allowed requests, got %d", allowedCount)
	}
	if blockedCount < 50 {
		t.Errorf("Expected at least 50 blocked requests, got %d", blockedCount)
	}
}

func TestAllow_RetryAfterCalculation(t *testing.T) {
	blockDuration := 5 * time.Second
	rl := New(WithMaxRequests(1), WithBlockDuration(blockDuration))
	clientID := "test-client"

	rl.Allow(clientID)
	result := rl.Allow(clientID)

	if result.Allowed {
		t.Error("Second request should be blocked")
	}

	if result.RetryAfterSec < 1 || result.RetryAfterSec > 5 {
		t.Errorf("Expected RetryAfterSec between 1 and 5, got %d", result.RetryAfterSec)
	}

	if result.RetryAfter.IsZero() {
		t.Error("RetryAfter should not be zero")
	}

	expectedRetryAfter := time.Now().Add(blockDuration)
	diff := result.RetryAfter.Sub(expectedRetryAfter).Abs()
	if diff > time.Second {
		t.Errorf("RetryAfter time is off by more than 1 second: %v", diff)
	}
}

func TestAllow_ZeroBlockDuration(t *testing.T) {
	rl := New(WithMaxRequests(1), WithBlockDuration(0))
	clientID := "test-client"

	rl.Allow(clientID)
	result := rl.Allow(clientID)

	if result.Allowed {
		t.Error("Request should be blocked even with zero block duration")
	}
}

func TestAllow_CustomErrorMessage(t *testing.T) {
	customMessage := "Too many requests, please slow down"
	rl := New(WithMaxRequests(1), WithErrorMessage(customMessage))
	clientID := "test-client"

	rl.Allow(clientID)
	result := rl.Allow(clientID)

	if result.ErrorMessage != customMessage {
		t.Errorf("Expected error message '%s', got '%s'", customMessage, result.ErrorMessage)
	}
}
