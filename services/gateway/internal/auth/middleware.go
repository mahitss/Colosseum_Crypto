package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"qevryn/gateway/internal/httpapi"
	"qevryn/gateway/internal/requestid"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	userIDKey contextKey = "user_id"
	claimsKey contextKey = "claims"
)

// AuthMiddleware creates a middleware that validates JWT tokens and sets the user identity.
func AuthMiddleware(cfg *Config, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		tokenStr, err := ExtractTokenFromHeader(authHeader)
		if err != nil {
			// No token - let the request through, ResolveUser will handle unauthenticated case
			if err != ErrMissingToken && err != ErrInvalidAuth {
				logger.Warn("auth header parse error", slog.String("error", err.Error()))
			}
			next.ServeHTTP(w, r)
			return
		}

		// Parse and validate token
		claims, err := ParseToken(cfg, tokenStr)
		if err != nil {
			// Invalid token - reject with 401
			logger.Warn("invalid token", slog.String("error", err.Error()))
			writeAuthError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid or expired token")
			return
		}

		// Add user ID and claims to context
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, claimsKey, claims)
		ctx = requestid.With(ctx, claims.RegisteredClaims.ID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok && userID != ""
}

// ClaimsFromContext extracts the full claims from the request context.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	return claims, ok
}

// JWTUserResolver implements the httpapi.UserResolver interface using JWT tokens.
type JWTUserResolver struct {
	config *Config
	logger *slog.Logger
}

// NewJWTUserResolver creates a resolver that extracts user identity from JWT tokens.
func NewJWTUserResolver(cfg *Config, logger *slog.Logger) *JWTUserResolver {
	return &JWTUserResolver{config: cfg, logger: logger}
}

// ResolveUser implements httpapi.UserResolver.
func (r *JWTUserResolver) ResolveUser(req *http.Request) (string, error) {
	// Try to get user ID from context (set by AuthMiddleware)
	if userID, ok := UserIDFromContext(req.Context()); ok {
		return userID, nil
	}

	// Fallback: try to extract from Authorization header directly
	authHeader := req.Header.Get("Authorization")
	tokenStr, err := ExtractTokenFromHeader(authHeader)
	if err != nil {
		if err != ErrMissingToken {
			r.logger.Debug("auth header parse failed", slog.String("error", err.Error()))
		}
		return "", httpapi.ErrUnauthenticated
	}

	claims, err := ParseToken(r.config, tokenStr)
	if err != nil {
		r.logger.Debug("token validation failed", slog.String("error", err.Error()))
		return "", httpapi.ErrUnauthenticated
	}

	if strings.TrimSpace(claims.UserID) == "" {
		return "", httpapi.ErrUnauthenticated
	}

	return claims.UserID, nil
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// Need to import json
