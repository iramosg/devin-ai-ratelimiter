# Rate Limiter for Go REST APIs

A flexible, high-performance rate limiter middleware for Go REST APIs with configurable options and modular architecture.

## Features

- **IP-based rate limiting** with support for custom client identification
- **Configurable time windows** and request limits
- **Automatic client blocking** when limits are exceeded
- **HTTP 429 responses** with `Retry-After` header
- **Optional JSON error responses** with detailed information
- **Thread-safe** with atomic operations for concurrent requests
- **Modular architecture** supporting different storage backends
- **Structured logging** using Go's `slog` package
- **Easy integration** with any HTTP framework

## Installation

```bash
go get github.com/iramosg/devin-ai-ratelimiter
```

## Quick Start

```go
package main

import (
    "net/http"
    "time"
    
    "github.com/iramosg/devin-ai-ratelimiter/middleware"
    "github.com/iramosg/devin-ai-ratelimiter/ratelimiter"
)

func main() {
    // Create rate limiter with default settings (100 req/min)
    limiter := ratelimiter.New()
    
    // Create middleware
    rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(limiter)
    
    // Apply to your handler
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })
    
    handler := rateLimiterMiddleware.Handler(mux)
    
    http.ListenAndServe(":8080", handler)
}
```

## Configuration

### Rate Limiter Options

The rate limiter uses the **options pattern** for configuration. All parameters are optional and have sensible defaults.

#### Available Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithMaxRequests(int)` | Maximum number of requests allowed in the time window | 100 |
| `WithWindowDuration(time.Duration)` | Time window for counting requests | 1 minute |
| `WithBlockDuration(time.Duration)` | How long to block a client after exceeding the limit | 1 minute |
| `WithErrorMessage(string)` | Custom error message for rate limit responses | "Rate limit exceeded" |
| `WithIncludeJSON(bool)` | Whether to include JSON body in error responses | true |
| `WithStorage(Storage)` | Custom storage backend | Memory storage |
| `WithLogger(*slog.Logger)` | Custom logger instance | Default logger |
| `WithLogOnExceedOnly(bool)` | Only log when rate limit is exceeded | true |

#### Example with Custom Configuration

```go
limiter := ratelimiter.New(
    ratelimiter.WithMaxRequests(50),
    ratelimiter.WithWindowDuration(30 * time.Second),
    ratelimiter.WithBlockDuration(2 * time.Minute),
    ratelimiter.WithErrorMessage("Too many requests, please slow down"),
    ratelimiter.WithIncludeJSON(true),
)
```

### Middleware Options

The middleware also supports configuration options:

| Option | Description | Default |
|--------|-------------|---------|
| `WithClientIDExtractor(func)` | Custom function to extract client ID from request | IP-based extractor |
| `WithIncludeJSON(bool)` | Whether to include JSON body in error responses | true |

#### Custom Client ID Extraction

By default, the middleware extracts the client IP from the request. You can customize this to use API keys, user tokens, or any other identifier:

```go
// Use API key from header
customExtractor := func(r *http.Request) string {
    return r.Header.Get("X-API-Key")
}

rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(
    limiter,
    middleware.WithClientIDExtractor(customExtractor),
)
```

## Response Format

### When Rate Limit is Exceeded

**HTTP Status:** `429 Too Many Requests`

**Headers:**
```
Retry-After: 60
Content-Type: application/json
```

**Body (when JSON is enabled):**
```json
{
  "error": "Rate limit exceeded",
  "limit": 100,
  "requests_made": 120,
  "retry_after": "2025-02-06T14:30:00.000Z"
}
```

The `Retry-After` header contains the number of seconds until the client can make requests again.

The `retry_after` field in the JSON body uses ISO 8601 format.

## Architecture

The rate limiter is built with a modular architecture:

```
┌─────────────────────────────────────┐
│      HTTP Middleware Layer          │
│  (Extracts client ID, applies       │
│   rate limiting, formats response)  │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│      Rate Limiter Core              │
│  (Business logic, window tracking,  │
│   blocking logic)                   │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│      Storage Layer                  │
│  (Memory, Redis, etc.)              │
└─────────────────────────────────────┘
```

### Storage Backends

Currently supported:
- **Memory Storage** (default) - In-memory storage using Go maps with mutex protection

Future support planned:
- **Redis** - For distributed rate limiting across multiple instances
- **Custom backends** - Implement the `Storage` interface

## Usage Examples

### Example 1: Basic Usage with Default Settings

```go
limiter := ratelimiter.New()
rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(limiter)

mux := http.NewServeMux()
mux.HandleFunc("/api/data", dataHandler)

http.ListenAndServe(":8080", rateLimiterMiddleware.Handler(mux))
```

### Example 2: Strict Rate Limiting

```go
// Allow only 5 requests per 10 seconds, block for 5 minutes
limiter := ratelimiter.New(
    ratelimiter.WithMaxRequests(5),
    ratelimiter.WithWindowDuration(10 * time.Second),
    ratelimiter.WithBlockDuration(5 * time.Minute),
)

rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(limiter)
```

### Example 3: API Key-Based Rate Limiting

```go
limiter := ratelimiter.New(
    ratelimiter.WithMaxRequests(1000),
    ratelimiter.WithWindowDuration(time.Hour),
)

apiKeyExtractor := func(r *http.Request) string {
    apiKey := r.Header.Get("X-API-Key")
    if apiKey == "" {
        return "anonymous"
    }
    return apiKey
}

rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(
    limiter,
    middleware.WithClientIDExtractor(apiKeyExtractor),
)
```

### Example 4: Custom Logger

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

limiter := ratelimiter.New(
    ratelimiter.WithLogger(logger),
    ratelimiter.WithLogOnExceedOnly(false), // Log all requests
)
```

### Example 5: Without JSON Response Body

```go
limiter := ratelimiter.New(
    ratelimiter.WithIncludeJSON(false),
)

rateLimiterMiddleware := middleware.NewRateLimiterMiddleware(
    limiter,
    middleware.WithIncludeJSON(false),
)
```

## Running the Example

A complete example API is provided in the `examples/memory` directory.

### Using Docker (Recommended)

1. **Start the development container:**
   ```bash
   docker compose up -d
   ```

2. **Access the container:**
   ```bash
   docker compose exec ratelimiter-dev sh
   ```

3. **Run the example server:**
   ```bash
   go run examples/memory/memory.go
   ```

4. **In another terminal, access the container again:**
   ```bash
   docker compose exec ratelimiter-dev sh
   ```

5. **Test with hey (load testing tool):**
   ```bash
   # Send 150 requests with 1 concurrent connection
   # Expected: 100 requests with 200 OK, 50 with 429 Too Many Requests
   hey -n 150 -c 1 http://localhost:8080/
   
   # Send 200 requests with 50 concurrent connections
   # Expected: 100 requests with 200 OK, 100 with 429 Too Many Requests
   hey -n 200 -c 50 http://localhost:8080/
   
   # Send requests for 10 seconds with 10 concurrent connections
   # Expected: First ~100 requests succeed, rest are blocked
   hey -z 10s -c 10 http://localhost:8080/
   ```

6. **Or test with curl:**
   ```bash
   # Make multiple requests to trigger rate limiting
   for i in {1..110}; do
     echo "Request $i:"
     curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/
   done
   ```

### Expected Behavior

With the default configuration (100 requests per minute):
- The first 100 requests will return `200 OK`
- The 101st request will return `429 Too Many Requests` with a `Retry-After` header
- Subsequent requests will continue to be blocked for 1 minute
- After the block duration expires, requests will be allowed again

### Example Output

**Successful request:**
```
HTTP/1.1 200 OK
Content-Type: application/json

{
  "message": "Hello! This endpoint is protected by rate limiting.",
  "timestamp": "2025-02-06T14:25:30.123Z",
  "client_ip": "172.18.0.1"
}
```

**Rate limited request:**
```
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 58

{
  "error": "Rate limit exceeded",
  "limit": 100,
  "requests_made": 101,
  "retry_after": "2025-02-06T14:26:30.000Z"
}
```

## Testing

Run all tests:

```bash
# Inside the Docker container
go test ./...

# With verbose output
go test -v ./...

# With coverage
go test -cover ./...

# Run specific package tests
go test ./storage
go test ./ratelimiter
go test ./middleware
```

## Project Structure

```
.
├── ratelimiter/
│   ├── limiter.go          # Core rate limiter logic
│   └── limiter_test.go     # Rate limiter tests
├── middleware/
│   ├── http.go             # HTTP middleware implementation
│   └── http_test.go        # Middleware tests
├── storage/
│   ├── memory.go           # In-memory storage implementation
│   └── memory_test.go      # Storage tests
├── examples/
│   └── memory/
│       └── memory.go       # Example API server
├── Dockerfile              # Development environment
├── docker-compose.yaml     # Docker Compose configuration
├── go.mod                  # Go module definition
├── go.sum                  # Go dependencies
└── README.md               # This file
```

## Performance Considerations

- **Thread-safe operations:** All storage operations use mutex locks to ensure consistency
- **Atomic counting:** Request counts are incremented atomically to handle concurrent requests
- **Efficient lookups:** O(1) lookup time for client data using hash maps
- **Memory management:** Old client data can be cleaned up periodically (future enhancement)

## Future Enhancements

- Redis storage adapter for distributed rate limiting
- Automatic cleanup of expired client data
- Rate limiting by user agent or custom headers
- Different rate limits for different endpoints
- Sliding window algorithm option
- Token bucket algorithm option
- Metrics and monitoring integration

## Contributing

Contributions are welcome! Please ensure:
- All tests pass before submitting a PR
- New features include comprehensive tests
- Code follows Go best practices
- Documentation is updated accordingly

## License

MIT License

## Author

Created by Igor Ramos Gonçalves (@iramosg)
