package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prophet/gateway/internal/trading"
)

// fakeTradeService implements TradeService in memory for HTTP tests.
type fakeTradeService struct {
	quoteErr     error
	buildErr     error
	broadcastErr error
	confirmErr   error
	reportErr    error
	attempt      *trading.Attempt
}

func (f *fakeTradeService) Quote(ctx context.Context, marketID, side, amountUSDC, walletPubkey string) (trading.PantaQuote, *trading.Attempt, error) {
	if err := trading.ValidateQuoteInput(marketID, side, amountUSDC, walletPubkey); err != nil {
		return trading.PantaQuote{}, nil, err
	}
	if f.quoteErr != nil {
		return trading.PantaQuote{}, nil, f.quoteErr
	}
	return trading.PantaQuote{
		MarketID:       marketID,
		Side:           side,
		AmountUSDC:     amountUSDC,
		PricePerShare:  "0.95",
		TotalCost:      amountUSDC,
		SharesReceived: "10",
		QuoteReference: "quote_test_1",
	}, &trading.Attempt{ID: "attempt_test_1", Status: trading.StatusQuoteReady}, nil
}

func (f *fakeTradeService) Build(ctx context.Context, quoteReference, walletPubkey string) (trading.PantaBuild, error) {
	if err := trading.ValidateBuildInput(quoteReference, walletPubkey, walletPubkey); err != nil {
		return trading.PantaBuild{}, err
	}
	if f.buildErr != nil {
		return trading.PantaBuild{}, f.buildErr
	}
	return trading.PantaBuild{
		TransactionData: "base64tx",
		ExpectedWallet:  walletPubkey,
		ExpectedNetwork: "mainnet-beta",
		BuildReference:  "build_test_1",
		MessageFormat:   "v0",
	}, nil
}

func (f *fakeTradeService) Broadcast(ctx context.Context, tradeAttemptID, signedTx, walletPubkey string) (trading.BroadcastResult, error) {
	if err := trading.ValidateBroadcastInput(tradeAttemptID, signedTx, walletPubkey); err != nil {
		return trading.BroadcastResult{}, err
	}
	if f.broadcastErr != nil {
		return trading.BroadcastResult{}, f.broadcastErr
	}
	return trading.BroadcastResult{Signature: "sig_test_1", SubmitStatus: "submitted"}, nil
}

func (f *fakeTradeService) Confirm(ctx context.Context, tradeAttemptID, signature string) (trading.BroadcastResult, error) {
	if err := trading.ValidateReportInput(tradeAttemptID, signature); err != nil {
		return trading.BroadcastResult{}, err
	}
	if f.confirmErr != nil {
		return trading.BroadcastResult{}, f.confirmErr
	}
	return trading.BroadcastResult{Signature: signature, ConfirmStatus: "confirmed", Confirmed: true}, nil
}

func (f *fakeTradeService) ReportAndVerify(ctx context.Context, tradeAttemptID string) (string, error) {
	if f.reportErr != nil {
		return "", f.reportErr
	}
	return "order_ref_test_1", nil
}

func (f *fakeTradeService) RefreshPositions(ctx context.Context, walletPubkey string) ([]trading.PantaPosition, error) {
	return []trading.PantaPosition{{MarketID: "m1", Side: "YES", Quantity: "5", Status: "active"}}, nil
}

func (f *fakeTradeService) Complete(ctx context.Context, tradeAttemptID string) error { return nil }

func (f *fakeTradeService) GetAttempt(ctx context.Context, id string) (*trading.Attempt, error) {
	if f.attempt != nil {
		return f.attempt, nil
	}
	return nil, nil
}

func newTradingHandler(t *testing.T, svc TradeService) http.Handler {
	t.Helper()
	return NewHandlerWithServices(nil, nil, svc)
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(dest); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
}

func TestTradeQuoteSuccess(t *testing.T) {
	handler := newTradingHandler(t, &fakeTradeService{})

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
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeBody(t, rec, &resp)
	if resp["side"] != "YES" {
		t.Errorf("expected YES, got %v", resp["side"])
	}
	if resp["trade_attempt_id"] != "attempt_test_1" {
		t.Errorf("expected attempt_test_1, got %v", resp["trade_attempt_id"])
	}
}

func TestTradeQuoteValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"invalid side", `{"market_id":"m","side":"MAYBE","amount_usdc":"10","wallet_pubkey":"w"}`},
		{"missing market", `{"side":"YES","amount_usdc":"10","wallet_pubkey":"w"}`},
		{"missing wallet", `{"market_id":"m","side":"YES","amount_usdc":"10"}`},
		{"invalid amount", `{"market_id":"m","side":"YES","amount_usdc":"abc","wallet_pubkey":"w"}`},
		{"zero amount", `{"market_id":"m","side":"YES","amount_usdc":"0","wallet_pubkey":"w"}`},
		{"malformed json", `{not json`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newTradingHandler(t, &fakeTradeService{})
			req := httptest.NewRequest("POST", "/api/v1/trades/quote", bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				t.Errorf("expected non-200, got %d", rec.Code)
			}
		})
	}
}

func TestTradeBuildSuccess(t *testing.T) {
	handler := newTradingHandler(t, &fakeTradeService{})

	body := bytes.NewBufferString(`{
		"quote_reference": "quote_test_1",
		"wallet_pubkey": "7xTestWalletPubkey123"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/build", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	decodeBody(t, rec, &resp)
	if resp["expected_wallet"] != "7xTestWalletPubkey123" {
		t.Errorf("expected wallet match, got %v", resp["expected_wallet"])
	}
	if resp["expected_network"] != "mainnet-beta" {
		t.Errorf("expected mainnet-beta, got %v", resp["expected_network"])
	}
}

func TestTradeBroadcastSuccess(t *testing.T) {
	handler := newTradingHandler(t, &fakeTradeService{})

	body := bytes.NewBufferString(`{
		"trade_attempt_id": "attempt_test_1",
		"signed_tx": "base64signed",
		"wallet_pubkey": "7xTestWalletPubkey123"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/broadcast", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeBody(t, rec, &resp)
	if resp["signature"] != "sig_test_1" {
		t.Errorf("expected sig_test_1, got %v", resp["signature"])
	}
	if resp["status"] != "SUBMITTED" {
		t.Errorf("expected SUBMITTED, got %v", resp["status"])
	}
}

func TestTradeBroadcastMissingSignature(t *testing.T) {
	handler := newTradingHandler(t, &fakeTradeService{})

	body := bytes.NewBufferString(`{
		"trade_attempt_id": "attempt_test_1",
		"signed_tx": "",
		"wallet_pubkey": "w"
	}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/broadcast", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Errorf("expected non-200 for empty signed tx")
	}
}

func TestTradeReportSuccess(t *testing.T) {
	handler := newTradingHandler(t, &fakeTradeService{})

	body := bytes.NewBufferString(`{"trade_attempt_id": "attempt_test_1", "signature": "sig"}`)

	req := httptest.NewRequest("POST", "/api/v1/trades/report", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeBody(t, rec, &resp)
	if resp["status"] != "VERIFIED" {
		t.Errorf("expected VERIFIED, got %v", resp["status"])
	}
}

func TestTradeStatusNotFound(t *testing.T) {
	handler := newTradingHandler(t, &fakeTradeService{})

	req := httptest.NewRequest("GET", "/api/v1/trades/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
