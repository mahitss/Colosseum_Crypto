package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func corsTestChain(t *testing.T, origins []string) http.Handler {
	t.Helper()
	middleware, err := NewCORSMiddleware(origins, slog.New(slog.NewTextHandler(&strings.Builder{}, nil)))
	if err != nil {
		t.Fatalf("NewCORSMiddleware: %v", err)
	}
	return middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
}

func TestCORSMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	handler := corsTestChain(t, []string{"http://localhost:3000"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("allowed origin not reflected: %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestCORSMiddlewareRejectsDisallowedOrigin(t *testing.T) {
	handler := corsTestChain(t, []string{"http://localhost:3000"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://evil.example")
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed origin must not receive a CORS allow header, got %q",
			recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddlewareAnswersPreflight(t *testing.T) {
	handler := corsTestChain(t, []string{"http://localhost:3000"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/watchlists", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight must be answered 204, got %d", recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("preflight missing allow-origin: %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(recorder.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("preflight must advertise POST: %q", recorder.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCORSMiddlewareAdvertisesRequiredHeaders(t *testing.T) {
	handler := corsTestChain(t, nil) // exercises the default local origins too
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/watchlists", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "GET")
	handler.ServeHTTP(recorder, request)
	advertised := recorder.Header().Get("Access-Control-Allow-Headers")
	for _, required := range []string{"Authorization", "Content-Type", "X-Request-Id"} {
		if !strings.Contains(advertised, required) {
			t.Fatalf("Access-Control-Allow-Headers must include %s, got %q", required, advertised)
		}
	}
}

func TestCORSMiddlewarePassesThroughRequestsWithoutOrigin(t *testing.T) {
	handler := corsTestChain(t, []string{"http://localhost:3000"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("no Origin must mean no CORS headers, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("request without Origin must reach the handler, got %d", recorder.Code)
	}
}

func TestCORSMiddlewareRejectsWildcardConfiguration(t *testing.T) {
	if _, err := NewCORSMiddleware([]string{"*"}, nil); err == nil {
		t.Fatal("wildcard origin must be rejected at configuration time")
	}
}
