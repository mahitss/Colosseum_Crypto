package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTradeQuote(t *testing.T) {
	handler := NewHandler()

	body := bytes.NewBufferString(`{
		"market_id": "test-market-1",
		"side": "YES",
		"amount_usdc": "10.50",
		"wallet_pubkey": "7xTestWalletPubkey123"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/quote", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp QuoteResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Side != "YES" {
		t.Errorf("expected YES side, got %s", resp.Side)
	}
	if resp.MarketID != "test-market-1" {
		t.Errorf("expected test-market-1, got %s", resp.MarketID)
	}
	if resp.QuoteReference == "" {
		t.Error("expected quote reference")
	}
}

func TestTradeQuoteInvalidSide(t *testing.T) {
	handler := NewHandler()

	body := bytes.NewBufferString(`{
		"market_id": "test-market-1",
		"side": "MAYBE",
		"amount_usdc": "10.50",
		"wallet_pubkey": "7xTestWalletPubkey123"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/quote", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid side, got %d", rec.Code)
	}
}

func TestTradeQuoteMissingFields(t *testing.T) {
	handler := NewHandler()

	body := bytes.NewBufferString(`{"side": "YES"}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/quote", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d", rec.Code)
	}
}

func TestTradeBuild(t *testing.T) {
	handler := NewHandler()

	body := bytes.NewBufferString(`{
		"quote_reference": "quote_123",
		"wallet_pubkey": "7xTestWalletPubkey123"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/build", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp BuildResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ExpectedWallet != "7xTestWalletPubkey123" {
		t.Errorf("expected wallet pubkey to match, got %s", resp.ExpectedWallet)
	}
	if resp.ExpectedNetwork != "mainnet-beta" {
		t.Errorf("expected mainnet-beta network, got %s", resp.ExpectedNetwork)
	}
}

func TestTradeStatusRequiresID(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest("GET", "/api/v1/trades/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("expected non-OK for missing trade ID")
	}
}
