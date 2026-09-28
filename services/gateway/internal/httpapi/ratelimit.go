package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ratelimitMiddleware wraps a handler with rate limiting.
func ratelimitMiddleware(limiter interface{ Allow(string) bool }, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build rate limit key: IP + path
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}

		// Group by path prefix
		path := r.URL.Path
		pathPrefix := "/"
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 {
			pathPrefix = "/" + parts[0] + "/" + parts[1]
		}

		key := "api:" + ip + ":" + pathPrefix

		// Use type assertion to call Allow
		if l, ok := limiter.(interface{ Allow(string) bool }); ok {
			if !l.Allow(key) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Limit", "rate-limited")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Rate limit exceeded. Please slow down.",
					},
				})
				return
			}
		}

		next.ServeHTTP(w, r)
	}
}