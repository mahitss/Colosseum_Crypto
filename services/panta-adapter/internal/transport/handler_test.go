package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qevryn/panta-adapter/internal/client"
	"qevryn/panta-adapter/internal/config"
)

const adapterTestKey = "pk_test_never_return_or_log_this"

func TestMarketsRoutesAndPantaDiagnostic(t *testing.T) {
	var observedKey, observedRequestID string
	panta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedKey = r.Header.Get("X-Api-Key")
		observedRequestID = r.Header.Get("X-Request-Id")
		switch r.URL.Path {
		case "/api/v1/markets/":
			if r.URL.Query().Get("limit") != "1" || r.URL.Query().Get("status") != "primary" {
				t.Errorf("unexpected list query: %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"items":[{"marketId":"11111111111111111111111111111111","title":"A real documented shape","volumeUsdc":"20.00","yesPrice":null,"noPrice":null}],"nextCursor":null}`)
		case "/api/v1/markets/11111111111111111111111111111111/":
			_, _ = io.WriteString(w, `{"marketId":"11111111111111111111111111111111","title":"A real documented shape","volumeUsdc":"20.00","yesPrice":"0.52","noPrice":"0.48"}`)
		case "/api/v1/account/":
			_, _ = io.WriteString(w, `{"userId":"usr_test","status":"active","canCreateMarkets":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer panta.Close()

	cfg := config.Config{BaseURL: panta.URL + "/api/v1/", APIKey: adapterTestKey, Timeout: time.Second}
	apiClient, err := client.New(cfg.BaseURL, cfg.APIKey, cfg.Timeout, client.Options{Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	service := NewMarketService(apiClient)
	handler := NewHandler(cfg, apiClient, service)

	listResponse := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/markets/?limit=1&status=primary", nil)
	listRequest.Header.Set("X-Request-Id", "request-from-gateway")
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var list map[string]json.RawMessage
	if err := json.Unmarshal(listResponse.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if _, exists := list["items"]; !exists {
		t.Fatalf("adapter response missing items: %s", listResponse.Body.String())
	}
	if strings.Contains(listResponse.Body.String(), adapterTestKey) {
		t.Fatal("API key leaked into market response")
	}
	if observedKey != adapterTestKey || observedRequestID != "request-from-gateway" {
		t.Fatalf("key or request ID not sent to Panta: keyPresent=%t requestID=%q", observedKey == adapterTestKey, observedRequestID)
	}
	if listResponse.Header().Get("X-Request-Id") != "request-from-gateway" {
		t.Fatal("adapter did not echo the request ID")
	}

	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/markets/11111111111111111111111111111111/", nil))
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), `"yesPrice":"0.52"`) {
		t.Fatalf("unexpected detail response: status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}

	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, httptest.NewRequest(http.MethodGet, "/health", nil))
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", healthResponse.Code, healthResponse.Body.String())
	}

	pantaHealthResponse := httptest.NewRecorder()
	handler.ServeHTTP(pantaHealthResponse, httptest.NewRequest(http.MethodGet, "/health/panta", nil))
	if pantaHealthResponse.Code != http.StatusOK || strings.Contains(pantaHealthResponse.Body.String(), adapterTestKey) {
		t.Fatalf("unsafe or failed Panta health response: %s", pantaHealthResponse.Body.String())
	}
}

func TestHealthRejectsInvalidConfigurationWithoutLeakingKey(t *testing.T) {
	cfg := config.Config{BaseURL: "https://panta.example/api/v1/", Timeout: time.Second}
	handler := NewHandler(cfg, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), adapterTestKey) {
		t.Fatalf("unexpected health response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMalformedPantaResponseIsMappedToSafeError(t *testing.T) {
	panta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"marketId":"secret-internal-id"}]}`)
	}))
	defer panta.Close()
	cfg := config.Config{BaseURL: panta.URL, APIKey: adapterTestKey, Timeout: time.Second}
	apiClient, err := client.New(cfg.BaseURL, cfg.APIKey, cfg.Timeout, client.Options{})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(cfg, apiClient, NewMarketService(apiClient))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/markets/", nil))
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "secret-internal-id") || strings.Contains(response.Body.String(), adapterTestKey) {
		t.Fatalf("unsafe or unexpected error: status=%d body=%s", response.Code, response.Body.String())
	}
}

