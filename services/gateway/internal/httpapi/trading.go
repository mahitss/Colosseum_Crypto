package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Trading types
type QuoteRequest struct {
	MarketID     string `json:"market_id"`
	Side         string `json:"side"` // YES or NO
	AmountUSDC   string `json:"amount_usdc"`
	WalletPubkey string `json:"wallet_pubkey"`
}

type QuoteResponse struct {
	MarketID       string `json:"market_id"`
	Side           string `json:"side"`
	AmountUSDC     string `json:"amount_usdc"`
	PricePerShare  string `json:"price_per_share"`
	TotalCost      string `json:"total_cost"`
	SharesReceived string `json:"shares_received"`
	QuoteReference string `json:"quote_reference"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

type BuildRequest struct {
	QuoteReference string `json:"quote_reference"`
	WalletPubkey   string `json:"wallet_pubkey"`
}

type BuildResponse struct {
	TransactionData string `json:"transaction_data"`
	ExpectedWallet  string `json:"expected_wallet"`
	ExpectedNetwork string `json:"expected_network"`
	BuildReference  string `json:"build_reference"`
	MessageFormat   string `json:"message_format"`
}

type BroadcastRequest struct {
	TradeAttemptID string `json:"trade_attempt_id"`
	SignedTx       string `json:"signed_tx"`
	WalletPubkey   string `json:"wallet_pubkey"`
}

type BroadcastResponse struct {
	Signature string `json:"signature"`
	Status    string `json:"status"`
}

type ReportRequest struct {
	TradeAttemptID string `json:"trade_attempt_id"`
	Signature      string `json:"signature"`
}

type TradeStatus struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Signature    string `json:"signature,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func registerTradingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/trades/quote", handleTradeQuote)
	mux.HandleFunc("POST /api/v1/trades/build", handleTradeBuild)
	mux.HandleFunc("POST /api/v1/trades/broadcast", handleTradeBroadcast)
	mux.HandleFunc("POST /api/v1/trades/report", handleTradeReport)
	mux.HandleFunc("GET /api/v1/trades/{id}", handleTradeStatus)
}

func handleTradeQuote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Validate inputs
	if req.MarketID == "" || req.WalletPubkey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields"})
		return
	}

	if req.Side != "YES" && req.Side != "NO" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid side (must be YES or NO)"})
		return
	}

	// In production, forward to Panta adapter
	// For now, return placeholder indicating connection is ready
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(QuoteResponse{
		MarketID:       req.MarketID,
		Side:           req.Side,
		AmountUSDC:     req.AmountUSDC,
		QuoteReference: "quote_" + time.Now().Format("20060102150405"),
	})
}

func handleTradeBuild(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req BuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.QuoteReference == "" || req.WalletPubkey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields"})
		return
	}

	// In production, forward to Panta adapter to build unsigned transaction
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(BuildResponse{
		ExpectedWallet:  req.WalletPubkey,
		ExpectedNetwork: "mainnet-beta",
		BuildReference:  "build_" + time.Now().Format("20060102150405"),
		MessageFormat:   "v0",
	})
}

func handleTradeBroadcast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req BroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.SignedTx == "" || req.WalletPubkey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields"})
		return
	}

	// In production:
	// 1. Validate transaction context
	// 2. Broadcast to Solana via configured RPC
	// 3. Store trade attempt in database
	// 4. Return signature

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(BroadcastResponse{
		Signature: "sig_placeholder",
		Status:    "SUBMITTED",
	})
}

func handleTradeReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// In production: Report to Panta, verify, update trade attempt status

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "VERIFIED",
	})
}

func handleTradeStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/trades/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing trade ID"})
		return
	}

	// In production: Query database for trade attempt

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TradeStatus{
		ID:        id,
		Status:    "IDLE",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	})
}
