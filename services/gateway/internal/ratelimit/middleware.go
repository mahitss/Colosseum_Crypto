package ratelimit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qevryn/gateway/internal/requestid"
)

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
	// Requests per window
	Limit int
	// Window duration
	Window time.Duration
	// Key prefix for different endpoint groups
	Prefix string
	// Whether to use sliding window log (more precise) or token bucket (faster)
	UseSlidingWindow bool
}

// DefaultRateLimitConfig returns a sensible default configuration.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Limit:            100,
		Window:           time.Minute,
		Prefix:           "api",
		UseSlidingWindow: true,
	}
}

// RateLimitMiddleware creates a middleware that enforces rate limits.
func RateLimitMiddleware(cfg *RateLimitConfig, logger *slog.Logger, next http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultRateLimitConfig()
	}

	var limiter interface {
		Allow(string) bool
	}

	if cfg.UseSlidingWindow {
		limiter = NewSlidingWindowLog(cfg.Limit, cfg.Window)
	} else {
		limiter = NewTokenBucket(cfg.Limit, float64(cfg.Limit)/cfg.Window.Seconds())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Build rate limit key: prefix + IP + path
		key := buildRateLimitKey(cfg.Prefix, r)

		allowed := limiter.Allow(key)
		if !allowed {
			requestID := requestid.From(r.Context())
			logger.Warn("rate limit exceeded",
				slog.String("request_id", requestID),
				slog.String("key", key),
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
			)
			writeRateLimitError(w, cfg.Limit, cfg.Window)
			return
		}

		// Add rate limit headers
		addRateLimitHeaders(w, cfg.Limit)

		next.ServeHTTP(w, r)
	})
}

func buildRateLimitKey(prefix string, r *http.Request) string {
	// Use X-Forwarded-For if available, otherwise RemoteAddr
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	// Take first IP if multiple
	if idx := strings.Index(ip, ","); idx != -1 {
		ip = strings.TrimSpace(ip[:idx])
	}

	// Group by path prefix (e.g., /api/v1/alert-rules, /api/v1/trades)
	path := r.URL.Path
	pathPrefix := "/"
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		pathPrefix = "/" + parts[0] + "/" + parts[1]
	}

	return prefix + ":" + ip + ":" + pathPrefix
}

func addRateLimitHeaders(w http.ResponseWriter, limit int) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	// Note: Remaining and Reset would require more state tracking
}

func writeRateLimitError(w http.ResponseWriter, limit int, window time.Duration) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "RATE_LIMIT_EXCEEDED",
			"message": "Rate limit exceeded. Please slow down.",
		},
	})
}
