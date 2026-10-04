package trading

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pantaTestServer(t *testing.T, status int, payload string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Validate auth header is present.
		if r.Header.Get("X-Api-Key") == "" {
			t.Error("expected X-Api-Key header")
		}
		// Validate trailing slash on path.
		if !strings.HasSuffix(r.URL.Path, "/") {
			t.Errorf("expected trailing slash on path, got %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
}

func newPantaTestClient(t *testing.T, url string) *PantaHTTPClient {
	t.Helper()
	c, err := NewPantaHTTPClient(url, "test-api-key", 2*time.Second)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return c
}

func TestQuotePrimaryBuySuccess(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{
		"market_id":"m1","side":"YES","amount_usdc":"10.50",
		"price_per_share":"0.95","total_cost":"10.50","shares_received":"11.05",
		"quote_reference":"qr_1","expires_at":"2026-12-31T00:00:00Z"
	}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	quote, err := c.QuotePrimaryBuy(context.Background(), "m1", "YES", "10.50", "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quote.QuoteReference != "qr_1" {
		t.Errorf("expected qr_1, got %s", quote.QuoteReference)
	}
	if quote.PricePerShare != "0.95" {
		t.Errorf("expected 0.95, got %s", quote.PricePerShare)
	}
}

func TestQuotePrimaryBuyUnauthorized(t *testing.T) {
	server := pantaTestServer(t, http.StatusUnauthorized, `{}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.QuotePrimaryBuy(context.Background(), "m1", "YES", "10.50", "7xWallet")
	if err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected authentication error, got %v", err)
	}
}

func TestQuotePrimaryBuyRateLimited(t *testing.T) {
	server := pantaTestServer(t, http.StatusTooManyRequests, `{}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.QuotePrimaryBuy(context.Background(), "m1", "YES", "10.50", "7xWallet")
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestQuotePrimaryBuyBadRequest(t *testing.T) {
	server := pantaTestServer(t, http.StatusBadRequest, `{}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.QuotePrimaryBuy(context.Background(), "m1", "YES", "10.50", "7xWallet")
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("expected rejected error, got %v", err)
	}
}

func TestQuotePrimaryBuyServerError(t *testing.T) {
	server := pantaTestServer(t, http.StatusInternalServerError, `{}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.QuotePrimaryBuy(context.Background(), "m1", "YES", "10.50", "7xWallet")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestQuotePrimaryBuyMalformedResponse(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `not-json-at-all{`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.QuotePrimaryBuy(context.Background(), "m1", "YES", "10.50", "7xWallet")
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed error, got %v", err)
	}
}

func TestBuildPrimaryBuySuccess(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{
		"transaction":"base64unsignedtx",
		"expected_wallet":"7xWallet",
		"network":"mainnet-beta",
		"build_reference":"br_1",
		"message_format":"v0"
	}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	build, err := c.BuildPrimaryBuy(context.Background(), "qr_1", "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build.TransactionData != "base64unsignedtx" {
		t.Errorf("expected tx data, got %s", build.TransactionData)
	}
	if build.ExpectedNetwork != "mainnet-beta" {
		t.Errorf("expected mainnet-beta, got %s", build.ExpectedNetwork)
	}
}

func TestBuildPrimaryBuyEmptyTransaction(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{"transaction":"","network":"mainnet-beta"}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.BuildPrimaryBuy(context.Background(), "qr_1", "7xWallet")
	if err == nil {
		t.Fatal("expected error for empty transaction")
	}
}

func TestReportTransactionSuccess(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{"order_reference":"order_1"}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	ref, err := c.ReportTransaction(context.Background(), "m1", "YES", "10.50", "sig123", "qr_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "order_1" {
		t.Errorf("expected order_1, got %s", ref)
	}
}

func TestVerifyTransactionVerified(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{"status":"verified","message":"ok"}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	v, err := c.VerifyTransaction(context.Background(), "sig123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != "verified" {
		t.Errorf("expected verified, got %s", v.Status)
	}
}

func TestGetPositionsSuccess(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{"positions":[
		{"market_id":"m1","side":"YES","quantity":"5","value_usdc":"4.75","status":"active"}
	]}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	positions, err := c.GetPositions(context.Background(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
	if positions[0].Side != "YES" {
		t.Errorf("expected YES, got %s", positions[0].Side)
	}
}

func TestGetAccountSuccess(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{
		"userId":"user-123","email":"test@example.com","name":"Test User",
		"status":"active","canCreateMarkets":true,"createdAt":"2026-01-01T00:00:00Z","apiKeyId":"key-1"
	}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	account, err := c.GetAccount(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account.UserID != "user-123" {
		t.Errorf("expected user-123, got %s", account.UserID)
	}
	if account.Status != "active" {
		t.Errorf("expected active, got %s", account.Status)
	}
}

func TestGetAccountMalformed(t *testing.T) {
	server := pantaTestServer(t, http.StatusOK, `{}`)
	defer server.Close()

	c := newPantaTestClient(t, server.URL)
	_, err := c.GetAccount(context.Background())
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed error, got %v", err)
	}
}

func TestNewPantaHTTPClientRequiresKey(t *testing.T) {
	if _, err := NewPantaHTTPClient("https://live-api.panta.market/api/v1/", "", 5*time.Second); err == nil {
		t.Fatal("expected error for empty API key")
	}
}
