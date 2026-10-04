package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qevryn/gateway/internal/marketstudio"
)

// stubMarketStudio is a scriptable MarketStudioService. Every field is a
// function so each test can drive exactly one behaviour and assert the
// handler's translation of it.
type stubMarketStudio struct {
	interpret func(context.Context, marketstudio.Interpreter, string) (marketstudio.InterpretationResult, error)
	validate  func(*marketstudio.Draft) marketstudio.ValidationReport
	quote     func(context.Context, *marketstudio.Draft, string) (marketstudio.QuoteResult, error)
	build     func(context.Context, string, string) (marketstudio.BuildResult, error)
	broadcast func(context.Context, string, string, string, string) (marketstudio.BroadcastOutcome, error)
	register  func(context.Context, string) (marketstudio.RegisterResult, error)
	get       func(context.Context, string) (*marketstudio.Attempt, error)
}

func (s stubMarketStudio) Interpret(ctx context.Context, i marketstudio.Interpreter, d string) (marketstudio.InterpretationResult, error) {
	if s.interpret == nil {
		return marketstudio.InterpretationResult{}, nil
	}
	return s.interpret(ctx, i, d)
}

func (s stubMarketStudio) Validate(d *marketstudio.Draft) marketstudio.ValidationReport {
	if s.validate == nil {
		return marketstudio.ValidationReport{Valid: true}
	}
	return s.validate(d)
}

func (s stubMarketStudio) Quote(ctx context.Context, d *marketstudio.Draft, w string) (marketstudio.QuoteResult, error) {
	if s.quote == nil {
		return marketstudio.QuoteResult{}, nil
	}
	return s.quote(ctx, d, w)
}

func (s stubMarketStudio) Build(ctx context.Context, id, w string) (marketstudio.BuildResult, error) {
	if s.build == nil {
		return marketstudio.BuildResult{}, nil
	}
	return s.build(ctx, id, w)
}

func (s stubMarketStudio) Broadcast(ctx context.Context, id, tx, w, h string) (marketstudio.BroadcastOutcome, error) {
	if s.broadcast == nil {
		return marketstudio.BroadcastOutcome{}, nil
	}
	return s.broadcast(ctx, id, tx, w, h)
}

func (s stubMarketStudio) Register(ctx context.Context, id string) (marketstudio.RegisterResult, error) {
	if s.register == nil {
		return marketstudio.RegisterResult{}, nil
	}
	return s.register(ctx, id)
}

func (s stubMarketStudio) Get(ctx context.Context, id string) (*marketstudio.Attempt, error) {
	if s.get == nil {
		return nil, marketstudio.ErrAttemptNotFound
	}
	return s.get(ctx, id)
}

func marketStudioHandler(service MarketStudioService) http.Handler {
	return NewHandlerWithAllServices(nil, nil, nil, service, nil)
}

func postJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeMarketStudioBody(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("response body is not valid JSON: %v body=%s", err, response.Body.String())
	}
}

// Every route is reachable and answers with JSON. This is the routing
// contract: a 404 here means an endpoint was renamed and the UI broke.
func TestMarketStudioRoutesAreRegistered(t *testing.T) {
	service := stubMarketStudio{
		interpret: func(context.Context, marketstudio.Interpreter, string) (marketstudio.InterpretationResult, error) {
			return marketstudio.InterpretationResult{}, nil
		},
		validate: func(*marketstudio.Draft) marketstudio.ValidationReport {
			return marketstudio.ValidationReport{Valid: true}
		},
		quote: func(context.Context, *marketstudio.Draft, string) (marketstudio.QuoteResult, error) {
			return marketstudio.QuoteResult{}, nil
		},
		build: func(context.Context, string, string) (marketstudio.BuildResult, error) {
			return marketstudio.BuildResult{}, nil
		},
		broadcast: func(context.Context, string, string, string, string) (marketstudio.BroadcastOutcome, error) {
			return marketstudio.BroadcastOutcome{}, nil
		},
		register: func(context.Context, string) (marketstudio.RegisterResult, error) {
			return marketstudio.RegisterResult{}, nil
		},
		get: func(context.Context, string) (*marketstudio.Attempt, error) {
			return &marketstudio.Attempt{ID: "attempt-1"}, nil
		},
	}
	handler := marketStudioHandler(service)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/market-studio/interpret", `{"prompt":"Will BTC close above 100000 on 2026-12-31?"}`},
		{http.MethodPost, "/api/v1/market-studio/validate", `{"draft":{}}`},
		{http.MethodPost, "/api/v1/market-studio/quote", `{"draft":{},"wallet_address":"wallet-1"}`},
		{http.MethodPost, "/api/v1/market-studio/build", `{"creation_attempt_id":"attempt-1","wallet_address":"wallet-1"}`},
		{http.MethodPost, "/api/v1/market-studio/broadcast", `{"creation_attempt_id":"attempt-1","signed_transaction":"c2ln","wallet_address":"wallet-1","draft_hash":"hash-1"}`},
		{http.MethodPost, "/api/v1/market-studio/register", `{"creation_attempt_id":"attempt-1"}`},
		{http.MethodGet, "/api/v1/market-studio/attempts/attempt-1", ""},
	}
	for _, test := range cases {
		response := postJSON(t, handler, test.method, test.path, test.body)
		if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
			t.Fatalf("%s %s is not routed: status=%d", test.method, test.path, response.Code)
		}
	}
}

// A nil studio service must not register routes at all, rather than panicking
// or exposing half-wired endpoints.
func TestMarketStudioRoutesAreAbsentWhenServiceIsNil(t *testing.T) {
	handler := NewHandlerWithAllServices(nil, nil, nil, nil, nil)
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/validate", `{"draft":{}}`)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when the studio service is nil, got %d", response.Code)
	}
}

func TestMarketStudioInterpretRequiresPrompt(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/interpret", `{"prompt":"   "}`)
	if response.Code != http.StatusBadRequest || !contains(response.Body.String(), "INVALID_REQUEST") {
		t.Fatalf("unexpected response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMarketStudioInterpretSurfacesClarification(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		interpret: func(context.Context, marketstudio.Interpreter, string) (marketstudio.InterpretationResult, error) {
			return marketstudio.InterpretationResult{
				NeedsClarification: true,
				Clarification:      "Your question needs more detail before a market can be created.",
			}, nil
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/interpret", `{"prompt":"Will BTC go up?"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		NeedsClarification bool   `json:"needs_clarification"`
		Clarification      string `json:"clarification"`
	}
	decodeMarketStudioBody(t, response, &payload)
	if !payload.NeedsClarification || !strings.Contains(payload.Clarification, "needs more detail") {
		t.Fatalf("clarification was not surfaced: %+v", payload)
	}
}

func TestMarketStudioInterpretMapsFailureToBadGateway(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		interpret: func(context.Context, marketstudio.Interpreter, string) (marketstudio.InterpretationResult, error) {
			return marketstudio.InterpretationResult{}, &marketstudio.Failure{
				Code:    marketstudio.ErrorAIInterpretationFailed,
				Message: "assistant unavailable",
			}
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/interpret", `{"prompt":"something"}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for an upstream assistant failure, got %d", response.Code)
	}
	if !contains(response.Body.String(), string(marketstudio.ErrorAIInterpretationFailed)) {
		t.Fatalf("error code missing from body: %s", response.Body.String())
	}
}

func TestMarketStudioValidateReturnsReportVerbatim(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		validate: func(*marketstudio.Draft) marketstudio.ValidationReport {
			return marketstudio.ValidationReport{
				Valid: false,
				Issues: []marketstudio.ValidationIssue{{
					Field:    "sources_of_truth",
					Code:     "SOURCE_NOT_CONFIRMED",
					Message:  "The resolution source must be confirmed by the user.",
					Severity: "error",
				}},
			}
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/validate", `{"draft":{}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	var report marketstudio.ValidationReport
	decodeMarketStudioBody(t, response, &report)
	if report.Valid {
		t.Fatal("a blocking issue must not be reported as valid")
	}
	if len(report.Errors()) != 1 || report.Errors()[0].Code != "SOURCE_NOT_CONFIRMED" {
		t.Fatalf("validation issue was not returned: %+v", report)
	}
}

func TestMarketStudioQuoteRequiresWallet(t *testing.T) {
	called := false
	handler := marketStudioHandler(stubMarketStudio{
		quote: func(context.Context, *marketstudio.Draft, string) (marketstudio.QuoteResult, error) {
			called = true
			return marketstudio.QuoteResult{}, nil
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/quote", `{"draft":{}}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when the wallet is missing, got %d", response.Code)
	}
	if called {
		t.Fatal("quote must not reach Panta without a connected wallet")
	}
}

func TestMarketStudioQuoteReturnsPantaAmountsAsStrings(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		quote: func(_ context.Context, _ *marketstudio.Draft, wallet string) (marketstudio.QuoteResult, error) {
			return marketstudio.QuoteResult{
				Attempt: &marketstudio.Attempt{ID: "attempt-1", WalletAddress: wallet, Status: marketstudio.StatusCreated},
				Quote: marketstudio.CreateQuote{
					CreateID:    "create-1",
					PaymentUSDC: "50000000",
					MarketType:  "standard",
				},
				Report: marketstudio.ValidationReport{Valid: true},
			}, nil
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/quote", `{"draft":{},"wallet_address":"wallet-1"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
	}
	// The quote object mirrors Panta's own camelCase wire format verbatim, and
	// amounts stay exact integer strings. A JSON number would mean the server
	// had parsed them into a float, which is exactly what Panta forbids.
	if !contains(response.Body.String(), `"paymentUsdc":"50000000"`) {
		t.Fatalf("USDC amount was not preserved as a string: %s", response.Body.String())
	}

	// The attempt object is Prophet's own shape and uses snake_case, so the
	// client reads one naming convention for state and another for Panta
	// passthrough. The draft is quoted but nothing exists yet.
	var payload struct {
		Attempt struct {
			Status       string `json:"status"`
			MarketExists bool   `json:"market_exists"`
		} `json:"attempt"`
		Quote struct {
			PaymentUSDC string `json:"paymentUsdc"`
		} `json:"quote"`
	}
	decodeMarketStudioBody(t, response, &payload)
	if payload.Quote.PaymentUSDC != "50000000" {
		t.Fatalf("unexpected quote amount: %q", payload.Quote.PaymentUSDC)
	}
	if payload.Attempt.MarketExists {
		t.Fatal("a quote does not create a market")
	}
}

// The single most important HTTP rule in this feature: a confirmed Solana
// transaction whose Panta registration failed is an upstream failure (502),
// never a client error, and the response must still carry the confirmed
// attempt so the UI can say "confirmed on Solana, registration pending".
func TestPantaRegistrationFailureIsBadGatewayAndCarriesConfirmedAttempt(t *testing.T) {
	confirmed := &marketstudio.Attempt{
		ID:              "attempt-1",
		WalletAddress:   "wallet-1",
		Status:          marketstudio.StatusConfirmed,
		SolanaSignature: "sig-1",
		// Deliberately no MarketID: the market does not exist yet.
	}
	handler := marketStudioHandler(stubMarketStudio{
		register: func(context.Context, string) (marketstudio.RegisterResult, error) {
			return marketstudio.RegisterResult{Attempt: confirmed}, &marketstudio.Failure{
				Code:    marketstudio.ErrorPantaRegistrationFailed,
				Message: "panta rejected the registration",
			}
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/register", `{"creation_attempt_id":"attempt-1"}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("registration failure must be 502, got %d", response.Code)
	}
	if contains(response.Body.String(), "creation failed") {
		t.Fatalf("response implies the creation failed: %s", response.Body.String())
	}

	var payload marketStudioErrorResponse
	decodeMarketStudioBody(t, response, &payload)
	if payload.Attempt == nil {
		t.Fatal("the confirmed attempt must be attached to a registration failure")
	}
	if payload.Attempt.Status != string(marketstudio.StatusConfirmed) {
		t.Fatalf("expected CONFIRMED status, got %s", payload.Attempt.Status)
	}
	if payload.Attempt.MarketExists {
		t.Fatal("market_exists must be false before Panta registration succeeds")
	}
	if payload.Attempt.Message != "Transaction confirmed on Solana. Panta registration is pending." {
		t.Fatalf("unexpected message: %q", payload.Attempt.Message)
	}
}

func TestDraftChangedAfterQuoteIsConflict(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		broadcast: func(context.Context, string, string, string, string) (marketstudio.BroadcastOutcome, error) {
			return marketstudio.BroadcastOutcome{}, &marketstudio.Failure{
				Code:    marketstudio.ErrorDraftChangedAfterQuote,
				Message: "the draft changed after it was quoted",
			}
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/broadcast",
		`{"creation_attempt_id":"attempt-1","signed_transaction":"c2ln","wallet_address":"wallet-1","draft_hash":"stale"}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("a stale quote must be 409, got %d", response.Code)
	}
}

func TestConfirmationTimeoutIsGatewayTimeout(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		broadcast: func(context.Context, string, string, string, string) (marketstudio.BroadcastOutcome, error) {
			// The attempt is present and NOT failed: we do not know the outcome.
			return marketstudio.BroadcastOutcome{Attempt: &marketstudio.Attempt{
				ID:        "attempt-1",
				Status:    marketstudio.StatusConfirming,
				DraftHash: "hash-1",
			}}, &marketstudio.Failure{
				Code:    marketstudio.ErrorConfirmationTimeout,
				Message: "confirmation timed out",
			}
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/broadcast",
		`{"creation_attempt_id":"attempt-1","signed_transaction":"c2ln","wallet_address":"wallet-1","draft_hash":"hash-1"}`)
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("a confirmation timeout must be 504, got %d", response.Code)
	}
	if contains(response.Body.String(), "creation failed") {
		t.Fatalf("a timeout must never be rendered as a creation failure: %s", response.Body.String())
	}
}

func TestMarketStudioBroadcastRequiresWallet(t *testing.T) {
	called := false
	handler := marketStudioHandler(stubMarketStudio{
		broadcast: func(context.Context, string, string, string, string) (marketstudio.BroadcastOutcome, error) {
			called = true
			return marketstudio.BroadcastOutcome{}, nil
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/broadcast",
		`{"creation_attempt_id":"attempt-1","signed_transaction":"c2ln","draft_hash":"hash-1"}`)
	if response.Code != http.StatusBadRequest || called {
		t.Fatalf("expected 400 without reaching the service, got %d called=%v", response.Code, called)
	}
}

func TestMarketStudioRegisterSuccessReportsMarketID(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		register: func(context.Context, string) (marketstudio.RegisterResult, error) {
			return marketstudio.RegisterResult{
				Registered: true,
				MarketID:   "market-authoritative-1",
				Message:    marketstudio.RegisteredMessage(),
				Attempt: &marketstudio.Attempt{
					ID:        "attempt-1",
					Status:    marketstudio.StatusRegistered,
					MarketID:  "market-authoritative-1",
					DraftHash: "hash-1",
				},
			}, nil
		},
	})
	response := postJSON(t, handler, http.MethodPost, "/api/v1/market-studio/register", `{"creation_attempt_id":"attempt-1"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Registered bool   `json:"registered"`
		MarketID   string `json:"market_id"`
		Attempt    struct {
			MarketExists bool `json:"market_exists"`
		} `json:"attempt"`
	}
	decodeMarketStudioBody(t, response, &payload)
	if !payload.Registered || payload.MarketID != "market-authoritative-1" {
		t.Fatalf("registration result was not returned: %+v", payload)
	}
	if !payload.Attempt.MarketExists {
		t.Fatal("market_exists must be true only after Panta returns a market id")
	}
}

func TestMarketStudioAttemptLookup(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{
		get: func(_ context.Context, id string) (*marketstudio.Attempt, error) {
			return &marketstudio.Attempt{
				ID:              id,
				Status:          marketstudio.StatusRegistered,
				MarketID:        "market-1",
				SolanaSignature: "sig-1",
				PaymentUSDC:     "50000000",
				DraftHash:       "hash-1",
			}, nil
		},
	})
	response := postJSON(t, handler, http.MethodGet, "/api/v1/market-studio/attempts/attempt-1", "")
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	var attempt marketStudioAttemptDTO
	decodeMarketStudioBody(t, response, &attempt)
	if attempt.Status != string(marketstudio.StatusRegistered) || !attempt.MarketExists {
		t.Fatalf("unexpected attempt payload: %+v", attempt)
	}
	if attempt.PaymentUSDC != "50000000" {
		t.Fatalf("amount was not preserved: %q", attempt.PaymentUSDC)
	}
}

func TestMarketStudioAttemptNotFoundIs404(t *testing.T) {
	handler := marketStudioHandler(stubMarketStudio{})
	response := postJSON(t, handler, http.MethodGet, "/api/v1/market-studio/attempts/missing", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}

func TestMarketStudioStatusMapping(t *testing.T) {
	cases := []struct {
		code   marketstudio.ErrorCode
		status int
	}{
		{marketstudio.ErrorInvalidDraft, http.StatusBadRequest},
		{marketstudio.ErrorAmbiguousMarket, http.StatusBadRequest},
		{marketstudio.ErrorWalletNotConnected, http.StatusBadRequest},
		{marketstudio.ErrorUserRejected, http.StatusBadRequest},
		{marketstudio.ErrorInvalidTransaction, http.StatusBadRequest},
		{marketstudio.ErrorDraftChangedAfterQuote, http.StatusConflict},
		{marketstudio.ErrorAIInterpretationFailed, http.StatusBadGateway},
		{marketstudio.ErrorQuoteFailed, http.StatusBadGateway},
		{marketstudio.ErrorBuildFailed, http.StatusBadGateway},
		{marketstudio.ErrorBroadcastFailed, http.StatusBadGateway},
		{marketstudio.ErrorPantaRegistrationFailed, http.StatusBadGateway},
		{marketstudio.ErrorConfirmationTimeout, http.StatusGatewayTimeout},
	}
	for _, test := range cases {
		if got := marketStudioStatus(test.code); got != test.status {
			t.Fatalf("%s: expected %d, got %d", test.code, test.status, got)
		}
	}
}

