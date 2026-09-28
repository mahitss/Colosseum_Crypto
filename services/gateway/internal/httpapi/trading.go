package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"prophet/gateway/internal/ratelimit"
	"prophet/gateway/internal/trading"
)

// Rate limiters for trading endpoints (stricter for expensive operations)
var (
	tradesQuoteLimiter     = ratelimit.NewSlidingWindowLog(30, time.Minute)     // 30 req/min
	tradesBuildLimiter     = ratelimit.NewSlidingWindowLog(30, time.Minute)     // 30 req/min
	tradesBroadcastLimiter = ratelimit.NewSlidingWindowLog(20, time.Minute)     // 20 req/min
	tradesConfirmLimiter   = ratelimit.NewSlidingWindowLog(60, time.Minute)     // 60 req/min
	tradesReportLimiter    = ratelimit.NewSlidingWindowLog(20, time.Minute)     // 20 req/min
	tradesCompleteLimiter  = ratelimit.NewSlidingWindowLog(20, time.Minute)     // 20 req/min
	tradesPositionsLimiter = ratelimit.NewSlidingWindowLog(30, time.Minute)     // 30 req/min
	tradesAttemptLimiter   = ratelimit.NewSlidingWindowLog(60, time.Minute)     // 60 req/min
	tradesValidateLimiter  = ratelimit.NewSlidingWindowLog(60, time.Minute)     // 60 req/min
)

// TradeService is the interface the HTTP layer depends on.
type TradeService interface {
	Quote(ctx context.Context, marketID, side, amountUSDC, walletPubkey string) (trading.PantaQuote, *trading.Attempt, error)
	Build(ctx context.Context, quoteReference, walletPubkey string) (trading.PantaBuild, error)
	Broadcast(ctx context.Context, tradeAttemptID, signedTx, walletPubkey string) (trading.BroadcastResult, error)
	Confirm(ctx context.Context, tradeAttemptID, signature string) (trading.BroadcastResult, error)
	ReportAndVerify(ctx context.Context, tradeAttemptID string) (string, error)
	RefreshPositions(ctx context.Context, walletPubkey string) ([]trading.PantaPosition, error)
	Complete(ctx context.Context, tradeAttemptID string) error
	GetAttempt(ctx context.Context, id string) (*trading.Attempt, error)
}

// Request DTOs (Prophet domain, not Panta schemas)
type quoteRequest struct {
	MarketID     string `json:"market_id"`
	Side         string `json:"side"`
	AmountUSDC   string `json:"amount_usdc"`
	WalletPubkey string `json:"wallet_pubkey"`
}

type buildRequest struct {
	QuoteReference string `json:"quote_reference"`
	WalletPubkey   string `json:"wallet_pubkey"`
}

type broadcastRequest struct {
	TradeAttemptID string `json:"trade_attempt_id"`
	SignedTx       string `json:"signed_tx"`
	WalletPubkey   string `json:"wallet_pubkey"`
}

type validateRequest struct {
	TradeAttemptID  string `json:"trade_attempt_id"`
	WalletPubkey    string `json:"wallet_pubkey"`
	MarketID        string `json:"market_id"`
	Side            string `json:"side"`
	AmountUSDC      string `json:"amount_usdc"`
	ExpectedWallet  string `json:"expected_wallet"`
	ExpectedNetwork string `json:"expected_network"`
	ActualNetwork   string `json:"actual_network"`
}

type reportRequest struct {
	TradeAttemptID string `json:"trade_attempt_id"`
	Signature      string `json:"signature"`
}

// Response DTOs
type attemptDTO struct {
	ID              string `json:"id"`
	WalletAddress   string `json:"wallet_address"`
	MarketID        string `json:"market_id"`
	Side            string `json:"side"`
	AmountUSDC      string `json:"amount_usdc"`
	QuoteReference  string `json:"quote_reference,omitempty"`
	SolanaSignature string `json:"solana_signature,omitempty"`
	PantaReference  string `json:"panta_reference,omitempty"`
	Status          string `json:"status"`
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
}

func toAttemptDTO(a *trading.Attempt) attemptDTO {
	return attemptDTO{
		ID:              a.ID,
		WalletAddress:   a.WalletAddress,
		MarketID:        a.MarketID,
		Side:            a.Side,
		AmountUSDC:      a.AmountUSDC,
		QuoteReference:  a.QuoteReference,
		SolanaSignature: a.SolanaSignature,
		PantaReference:  a.PantaReference,
		Status:          string(a.Status),
		ErrorCode:       a.ErrorCode,
		ErrorMessage:    a.ErrorMessage,
	}
}

func writeTradeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var status int
	code := "TRADE_FAILED"
	switch {
	case errors.Is(err, trading.ErrInvalidAmount),
		errors.Is(err, trading.ErrInvalidSide),
		errors.Is(err, trading.ErrWalletRequired),
		errors.Is(err, trading.ErrMarketRequired),
		errors.Is(err, trading.ErrTradeAttemptRequired),
		errors.Is(err, trading.ErrSignatureRequired),
		errors.Is(err, trading.ErrSignedTransactionRequired):
		status = http.StatusBadRequest
		code = "INVALID_REQUEST"
	case errors.Is(err, trading.ErrWalletMismatch),
		errors.Is(err, trading.ErrNetworkMismatch),
		errors.Is(err, trading.ErrMarketMismatch),
		errors.Is(err, trading.ErrSideMismatch),
		errors.Is(err, trading.ErrAmountMismatch):
		status = http.StatusConflict
		code = "INVALID_TRANSACTION_CONTEXT"
	default:
		status = http.StatusServiceUnavailable
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "detail": err.Error()})
}

func registerTradingRoutes(mux *http.ServeMux, trades TradeService) {
	if trades == nil {
		return
	}

	mux.HandleFunc("POST /api/v1/trades/quote", ratelimitMiddleware(tradesQuoteLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req quoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		quote, attempt, err := trades.Quote(r.Context(), req.MarketID, req.Side, req.AmountUSDC, req.WalletPubkey)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"market_id":        quote.MarketID,
			"side":             quote.Side,
			"amount_usdc":      quote.AmountUSDC,
			"price_per_share":  quote.PricePerShare,
			"total_cost":       quote.TotalCost,
			"shares_received":  quote.SharesReceived,
			"quote_reference":  quote.QuoteReference,
			"expires_at":       quote.ExpiresAt,
			"trade_attempt_id": attempt.ID,
		})
	}))

	mux.HandleFunc("POST /api/v1/trades/build", ratelimitMiddleware(tradesBuildLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req buildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		build, err := trades.Build(r.Context(), req.QuoteReference, req.WalletPubkey)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transaction_data": build.TransactionData,
			"expected_wallet":  build.ExpectedWallet,
			"expected_network": build.ExpectedNetwork,
			"build_reference":  build.BuildReference,
			"message_format":   build.MessageFormat,
		})
	}))

	mux.HandleFunc("POST /api/v1/trades/broadcast", ratelimitMiddleware(tradesBroadcastLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req broadcastRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		result, err := trades.Broadcast(r.Context(), req.TradeAttemptID, req.SignedTx, req.WalletPubkey)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"signature": result.Signature,
			"status":    "SUBMITTED",
		})
	}))

	mux.HandleFunc("POST /api/v1/trades/validate", ratelimitMiddleware(tradesValidateLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req validateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		// The connected wallet must be the one that will sign, and the
		// wallet's actual network must match the transaction's expected network.
		if err := trading.ValidateTransactionContext(
			req.ExpectedWallet, req.WalletPubkey,
			req.ExpectedNetwork, req.ActualNetwork,
			req.MarketID, req.MarketID,
			req.Side, req.Side,
			req.AmountUSDC, req.AmountUSDC,
		); err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"valid": true})
	}))

	mux.HandleFunc("POST /api/v1/trades/confirm", ratelimitMiddleware(tradesConfirmLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req reportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		result, err := trades.Confirm(r.Context(), req.TradeAttemptID, req.Signature)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"signature":        result.Signature,
			"confirmation":     result.ConfirmStatus,
			"confirmed":        result.Confirmed,
			"status":           "CONFIRMED",
			"trade_attempt_id": req.TradeAttemptID,
		})
	}))

	mux.HandleFunc("POST /api/v1/trades/report", ratelimitMiddleware(tradesReportLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req reportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		reference, err := trades.ReportAndVerify(r.Context(), req.TradeAttemptID)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":           "VERIFIED",
			"panta_reference":  reference,
			"trade_attempt_id": req.TradeAttemptID,
		})
	}))

	mux.HandleFunc("POST /api/v1/trades/complete", ratelimitMiddleware(tradesCompleteLimiter, func(w http.ResponseWriter, r *http.Request) {
		var req reportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST"})
			return
		}
		if err := trades.Complete(r.Context(), req.TradeAttemptID); err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "COMPLETED", "trade_attempt_id": req.TradeAttemptID})
	}))

	mux.HandleFunc("GET /api/v1/trades/{id}", ratelimitMiddleware(tradesAttemptLimiter, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		attempt, err := trades.GetAttempt(r.Context(), id)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		if attempt == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "NOT_FOUND"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(toAttemptDTO(attempt))
	}))

	mux.HandleFunc("GET /api/v1/trades/positions/{wallet}", ratelimitMiddleware(tradesPositionsLimiter, func(w http.ResponseWriter, r *http.Request) {
		wallet := r.PathValue("wallet")
		positions, err := trades.RefreshPositions(r.Context(), wallet)
		if err != nil {
			writeTradeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"positions": positions})
	}))
}