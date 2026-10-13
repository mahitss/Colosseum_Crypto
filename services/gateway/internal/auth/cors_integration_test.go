package auth_test

// The CORS-then-authentication integration test lives in the auth package's
// external test package because internal/httpapi cannot import internal/auth
// (auth's middleware already imports httpapi for the UserResolver seam), which
// would create an import cycle. Here both packages can be composed freely.

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qevryn/gateway/internal/auth"
	"qevryn/gateway/internal/httpapi"
)

func TestCORSDoesNotBypassAuthentication(t *testing.T) {
	jwtConfig := &auth.Config{
		Secret: "cors-test-secret", Expiry: time.Minute,
		Issuer: "prophet-gateway", Audience: []string{"prophet-api"},
	}
	token, err := auth.GenerateToken(jwtConfig, "user-1", "user@example.com", "user")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	corsMiddleware, err := httpapi.NewCORSMiddleware([]string{"http://localhost:3000"},
		slog.New(slog.NewTextHandler(&strings.Builder{}, nil)))
	if err != nil {
		t.Fatalf("NewCORSMiddleware: %v", err)
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(auth.AuthMiddleware(jwtConfig,
		slog.New(slog.NewTextHandler(&strings.Builder{}, nil)), inner))

	// Authenticated request from an allowed origin: passes with CORS headers.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/watchlists", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid token from allowed origin must reach handler, got %d", recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("CORS headers must survive auth, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}

	// Invalid token: rejected by the auth middleware with CORS headers intact.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/watchlists", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Authorization", "Bearer not-a-token")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token must stay 401 after CORS, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "UNAUTHENTICATED") {
		t.Fatalf("expected UNAUTHENTICATED body, got %s", recorder.Body.String())
	}

	// Unauthenticated request from an allowed origin: the middleware lets it
	// through and the route's user-resolution decides, exactly as without CORS.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("public route must remain reachable with CORS applied, got %d", recorder.Code)
	}
}
