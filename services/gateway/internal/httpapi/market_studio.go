package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"qevryn/gateway/internal/marketstudio"
	"qevryn/gateway/internal/ratelimit"
)

// MarketStudioService is the interface the HTTP layer depends on.
type MarketStudioService interface {
	Interpret(ctx context.Context, interpreter marketstudio.Interpreter, description string) (marketstudio.InterpretationResult, error)
	Validate(draft *marketstudio.Draft) marketstudio.ValidationReport
	Quote(ctx context.Context, draft *marketstudio.Draft, wallet string) (marketstudio.QuoteResult, error)
	Build(ctx context.Context, attemptID, wallet string) (marketstudio.BuildResult, error)
	Broadcast(ctx context.Context, attemptID, signedTx, wallet, draftHash string) (marketstudio.BroadcastOutcome, error)
	Register(ctx context.Context, attemptID string) (marketstudio.RegisterResult, error)
	Get(ctx context.Context, id string) (*marketstudio.Attempt, error)
}

// Request DTOs. These are Prophet-domain shapes; the Panta payload is built
// inside the service and never assembled by the client.
type marketStudioDraftRequest struct {
	Draft *marketstudio.Draft `json:"draft"`
}

type marketStudioQuoteRequest struct {
	Draft  *marketstudio.Draft `json:"draft"`
	Wallet string              `json:"wallet_address"`
}

type marketStudioBuildRequest struct {
	AttemptID string `json:"creation_attempt_id"`
	Wallet    string `json:"wallet_address"`
}

type marketStudioBroadcastRequest struct {
	AttemptID string `json:"creation_attempt_id"`
	SignedTx  string `json:"signed_transaction"`
	Wallet    string `json:"wallet_address"`
	// DraftHash pins the content the user signed. If a material field changed
	// after the quote, this mismatch invalidates the built transaction.
	DraftHash string `json:"draft_hash"`
}

type marketStudioRegisterRequest struct {
	AttemptID string `json:"creation_attempt_id"`
}

// Response DTOs
type marketStudioAttemptDTO struct {
	ID                     string `json:"id"`
	WalletAddress          string `json:"wallet_address"`
	Status                 string `json:"status"`
	CreateID               string `json:"create_id,omitempty"`
	ExpectedEventPDA       string `json:"expected_event_pda,omitempty"`
	PaymentUSDC            string `json:"payment_usdc,omitempty"`
	LiquidityInjectionUSDC string `json:"liquidity_injection_usdc,omitempty"`
	PlatformRevenueUSDC    string `json:"platform_revenue_usdc,omitempty"`
	MarketType             string `json:"market_type,omitempty"`
	DraftHash              string `json:"draft_hash"`
	SolanaSignature        string `json:"solana_signature,omitempty"`
	MarketID               string `json:"market_id,omitempty"`
	ErrorCode              string `json:"error_code,omitempty"`
	ErrorMessage           string `json:"error_message,omitempty"`
	// Message is the server-rendered user-facing state text. Clients display
	// this rather than composing their own, so the confirmed-but-unregistered
	// wording can never be lost.
	Message      string `json:"message"`
	MarketExists bool   `json:"market_exists"`
}

func toMarketStudioAttemptDTO(attempt *marketstudio.Attempt) marketStudioAttemptDTO {
	if attempt == nil {
		return marketStudioAttemptDTO{Message: ""}
	}
	return marketStudioAttemptDTO{
		ID:                     attempt.ID,
		WalletAddress:          attempt.WalletAddress,
		Status:                 string(attempt.Status),
		CreateID:               attempt.CreateID,
		ExpectedEventPDA:       attempt.ExpectedEventPDA,
		PaymentUSDC:            attempt.PaymentUSDC,
		LiquidityInjectionUSDC: attempt.LiquidityInjectionUSDC,
		PlatformRevenueUSDC:    attempt.PlatformRevenueUSDC,
		MarketType:             attempt.MarketType,
		DraftHash:              attempt.DraftHash,
		SolanaSignature:        attempt.SolanaSignature,
		MarketID:               attempt.MarketID,
		ErrorCode:              attempt.ErrorCode,
		ErrorMessage:           attempt.ErrorMessage,
		Message:                marketstudio.StatusMessage(attempt),
		MarketExists:           attempt.MarketExists(),
	}
}

type marketStudioErrorResponse struct {
	Error   string                  `json:"error"`
	Detail  string                  `json:"detail,omitempty"`
	Attempt *marketStudioAttemptDTO `json:"attempt,omitempty"`
}

// marketStudioStatus maps a service error code to an HTTP status.
//
// The PANTA_REGISTRATION_FAILED case is the important one: it is a 502, not a
// 4xx. The request was not rejected; the upstream failed after the chain
// already settled. Reporting it as a client error would push the UI toward
// "creation failed", which would be false.
func marketStudioStatus(code marketstudio.ErrorCode) int {
	switch code {
	case marketstudio.ErrorInvalidDraft,
		marketstudio.ErrorAmbiguousMarket,
		marketstudio.ErrorWalletNotConnected,
		marketstudio.ErrorUserRejected,
		marketstudio.ErrorInvalidTransaction:
		return http.StatusBadRequest
	case marketstudio.ErrorDraftChangedAfterQuote:
		// The reviewed content no longer matches the quote. A conflict is the
		// honest answer: the client must re-quote, not retry blindly.
		return http.StatusConflict
	case marketstudio.ErrorAIInterpretationFailed,
		marketstudio.ErrorIntelligenceServiceOffline,
		marketstudio.ErrorQuoteFailed,
		marketstudio.ErrorBuildFailed,
		marketstudio.ErrorBroadcastFailed,
		marketstudio.ErrorPantaRegistrationFailed:
		return http.StatusBadGateway
	case marketstudio.ErrorConfirmationTimeout:
		return http.StatusGatewayTimeout
	case marketstudio.ErrorUnsupported:
		return http.StatusNotImplemented
	default:
		return http.StatusServiceUnavailable
	}
}

// writeMarketStudioError renders a failure, attaching the attempt when one is
// known so the client can always show the true current state.
func writeMarketStudioError(w http.ResponseWriter, err error, attempt *marketstudio.Attempt) {
	code := marketstudio.Code(err)
	payload := marketStudioErrorResponse{Error: string(code)}
	if err != nil {
		payload.Detail = err.Error()
	}
	if attempt != nil {
		dto := toMarketStudioAttemptDTO(attempt)
		payload.Attempt = &dto
	}
	writeJSONResponse(w, marketStudioStatus(code), payload)
}

// Rate limit configs for market studio endpoints (stricter for expensive operations)
var (
	marketStudioInterpretLimiter = ratelimit.NewSlidingWindowLog(10, time.Minute)     // 10 req/min
	marketStudioQuoteLimiter     = ratelimit.NewSlidingWindowLog(20, time.Minute)     // 20 req/min
	marketStudioBuildLimiter     = ratelimit.NewSlidingWindowLog(20, time.Minute)     // 20 req/min
	marketStudioBroadcastLimiter = ratelimit.NewSlidingWindowLog(10, time.Minute)     // 10 req/min
	marketStudioRegisterLimiter  = ratelimit.NewSlidingWindowLog(10, time.Minute)     // 10 req/min
	marketStudioValidateLimiter  = ratelimit.NewSlidingWindowLog(50, time.Minute)     // 50 req/min
	marketStudioAttemptsLimiter  = ratelimit.NewSlidingWindowLog(60, time.Minute)     // 60 req/min
)

func registerMarketStudioRoutes(mux *http.ServeMux, service MarketStudioService, interpreter marketstudio.Interpreter) {
	if service == nil {
		return
	}

	// Interpret turns a description into a draft, or asks for clarification.
	// It never creates anything.
	mux.HandleFunc("POST /api/v1/market-studio/interpret", ratelimitMiddleware(marketStudioInterpretLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		if strings.TrimSpace(req.Prompt) == "" {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{
				Error:  "INVALID_REQUEST",
				Detail: "prompt is required",
			})
			return
		}
		result, err := service.Interpret(r.Context(), interpreter, req.Prompt)
		if err != nil {
			writeMarketStudioError(w, err, nil)
			return
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"draft":               result.Draft,
			"needs_clarification": result.NeedsClarification,
			"missing_fields":      result.MissingFields,
			"clarification":       result.Clarification,
			"validation":          result.Validation,
			"used_fallback":       result.UsedFallback,
		})
	}))

	// Validate is the deterministic gate. The client uses this to decide whether
	// to show a Create Market action; it is never skipped.
	mux.HandleFunc("POST /api/v1/market-studio/validate", ratelimitMiddleware(marketStudioValidateLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req marketStudioDraftRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		report := service.Validate(req.Draft)
		writeJSONResponse(w, http.StatusOK, report)
	}))

	mux.HandleFunc("POST /api/v1/market-studio/quote", ratelimitMiddleware(marketStudioQuoteLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req marketStudioQuoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		if strings.TrimSpace(req.Wallet) == "" {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{
				Error:  string(marketstudio.ErrorWalletNotConnected),
				Detail: "wallet_address is required",
			})
			return
		}
		result, err := service.Quote(r.Context(), req.Draft, req.Wallet)
		if err != nil {
			writeMarketStudioError(w, err, nil)
			return
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"attempt":    toMarketStudioAttemptDTO(result.Attempt),
			"quote":      result.Quote,
			"validation": result.Report,
		})
	}))

	// Build returns an unsigned transaction. This server never signs it.
	mux.HandleFunc("POST /api/v1/market-studio/build", ratelimitMiddleware(marketStudioBuildLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req marketStudioBuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		result, err := service.Build(r.Context(), req.AttemptID, req.Wallet)
		if err != nil {
			writeMarketStudioError(w, err, nil)
			return
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"attempt": toMarketStudioAttemptDTO(result.Attempt),
			"build":   result.Build,
		})
	}))

	// Broadcast relays the wallet-signed transaction and waits for confirmation.
	mux.HandleFunc("POST /api/v1/market-studio/broadcast", ratelimitMiddleware(marketStudioBroadcastLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req marketStudioBroadcastRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		if strings.TrimSpace(req.Wallet) == "" {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{
				Error:  string(marketstudio.ErrorWalletNotConnected),
				Detail: "wallet_address is required",
			})
			return
		}
		result, err := service.Broadcast(r.Context(), req.AttemptID, req.SignedTx, req.Wallet, req.DraftHash)
		if err != nil {
			// A broadcast failure after submission can leave the transaction in
			// an unknown state, so the attempt is still returned when we have it.
			writeMarketStudioError(w, err, result.Attempt)
			return
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"attempt": toMarketStudioAttemptDTO(result.Attempt),
			"result":  result.Result,
		})
	}))

	// Register is the only step that makes a market exist.
	mux.HandleFunc("POST /api/v1/market-studio/register", ratelimitMiddleware(marketStudioRegisterLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req marketStudioRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		result, err := service.Register(r.Context(), req.AttemptID)
		if err != nil {
			// Registration failure after a confirmed transaction must still show
			// the confirmed attempt so the client can say "confirmed on Solana,
			// registration pending" rather than "creation failed".
			writeMarketStudioError(w, err, result.Attempt)
			return
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"registered": result.Registered,
			"market_id":  result.MarketID,
			"message":    result.Message,
			"attempt":    toMarketStudioAttemptDTO(result.Attempt),
		})
	}))

	// Attempts status endpoint, for polling after a broadcast or registration.
	mux.HandleFunc("GET /api/v1/market-studio/attempts/{id}", ratelimitMiddleware(marketStudioAttemptsLimiter, func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeJSONResponse(w, http.StatusBadRequest, marketStudioErrorResponse{Error: "INVALID_REQUEST"})
			return
		}
		attempt, err := service.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, marketstudio.ErrAttemptNotFound) {
				writeJSONResponse(w, http.StatusNotFound, marketStudioErrorResponse{Error: "ATTEMPT_NOT_FOUND"})
				return
			}
			writeMarketStudioError(w, err, nil)
			return
		}
		writeJSONResponse(w, http.StatusOK, toMarketStudioAttemptDTO(attempt))
	}))
}
