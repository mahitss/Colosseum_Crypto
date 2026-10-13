package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
)

// CORS middleware.
//
// The browser frontend is served by Next.js on a different origin than the Go
// gateway (http://localhost:3000 versus http://localhost:8080 in development),
// so every client-side data fetch is cross-origin and needs explicit approval
// from this middleware. Without it the browser blocks every response before
// the React code ever sees it, which is exactly what the first E2E audit
// observed: pages rendered shells with zero counts while the backend held
// real data.
//
// Policy decisions, kept deliberately narrow:
//
//   - Allowed origins are an exact-match allowlist. The list comes from
//     CORS_ALLOWED_ORIGINS (comma separated); when it is unset the middleware
//     falls back to the documented local-development origins only
//     (http://localhost:3000 and http://127.0.0.1:3000). An arbitrary request
//     origin is never reflected back.
//   - The wildcard origin is rejected at construction time. The application
//     authenticates with Authorization headers rather than cookies, but a
//     wildcard combined with credentialed requests is the classic CORS
//     misconfiguration, so it is refused outright instead of relying on the
//     caller remembering not to set Access-Control-Allow-Credentials.
//   - Preflight OPTIONS requests are answered by this middleware alone, before
//     authentication and before any route handler, because a preflight cannot
//     carry credentials by definition.
//   - CORS is transport policy, not access control. It is applied outside the
//     auth middleware and never grants or denies anything the gateway would
//     not otherwise do: authentication and authorization behave identically
//     with or without CORS headers present.
const (
	corsAllowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Authorization, Content-Type, Accept, X-Request-Id"
	corsMaxAgeSeconds  = "600"
)

// DefaultLocalOrigins are the origins the local development frontend is served
// from when CORS_ALLOWED_ORIGINS is not configured. Production deployments
// must set CORS_ALLOWED_ORIGINS to their real origins.
var DefaultLocalOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}

// NewCORSMiddleware validates the allowlist and returns middleware that adds
// CORS headers for allowed origins and answers preflight requests. An entry
// of "*" is rejected rather than expanded.
func NewCORSMiddleware(allowedOrigins []string, logger *slog.Logger) (func(http.Handler) http.Handler, error) {
	normalized := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, errWildcardOrigin
		}
		normalized[origin] = struct{}{}
	}
	if len(normalized) == 0 {
		normalized = make(map[string]struct{}, len(DefaultLocalOrigins))
		for _, origin := range DefaultLocalOrigins {
			normalized[origin] = struct{}{}
		}
		if logger != nil {
			logger.Warn("CORS_ALLOWED_ORIGINS is not set; allowing only the local development origins",
				slog.Any("origins", DefaultLocalOrigins))
		}
	}
	if logger != nil {
		logger.Info("CORS policy configured",
			slog.Any("allowed_origins", normalized))
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			_, allowed := normalized[origin]

			if r.Method == http.MethodOptions {
				if origin != "" && r.Header.Get("Access-Control-Request-Method") != "" {
					// A preflight request. Answer it entirely here, before
					// authentication: preflights cannot carry credentials and
					// no route should ever see them.
					if allowed {
						writeCORSHeaders(w, origin)
						w.WriteHeader(http.StatusNoContent)
						return
					}
					// A preflight from a disallowed origin gets no CORS
					// headers, which the browser reports as a blocked
					// request. Reply 204 without policy so the failure is
					// clearly attributable to the missing headers.
					w.WriteHeader(http.StatusNoContent)
					return
				}
				// OPTIONS without preflight headers is not a CORS request
				// (some health checkers use bare OPTIONS); pass it through.
				next.ServeHTTP(w, r)
				return
			}

			if allowed && origin != "" {
				// The Vary header keeps any intermediary cache from reusing
				// this response for a different Origin.
				w.Header().Add("Vary", "Origin")
				writeCORSHeaders(w, origin)
			} else {
				w.Header().Add("Vary", "Origin")
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func writeCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
	w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
	w.Header().Set("Access-Control-Max-Age", corsMaxAgeSeconds)
}

type corsPolicyError string

func (e corsPolicyError) Error() string { return string(e) }

const errWildcardOrigin = corsPolicyError("CORS_ALLOWED_ORIGINS must not contain '*': configure explicit origins")
