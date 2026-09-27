package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"prophet/gateway/internal/requestid"
	"prophet/types"
)

type healthResponse struct {
	Status string `json:"status"`
}

func NewHandler() http.Handler {
	return NewHandlerWithMarkets(nil)
}

func NewHandlerWithMarkets(service marketService) http.Handler {
	return NewHandlerWithServices(service, nil, nil)
}

func NewHandlerWithServices(service marketService, intelligence intelligenceReader, trades TradeService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
	})
	registerMarketRoutes(mux, service)
	registerIntelligenceRoutes(mux, intelligence)
	registerCopilotRoutes(mux)
	registerTradingRoutes(mux, trades)
	return withRequestID(mux)
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if !validGatewayRequestID(requestID) {
			var value [16]byte
			if _, err := rand.Read(value[:]); err != nil {
				requestID = "unavailable"
			} else {
				requestID = hex.EncodeToString(value[:])
			}
		}
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), requestID)))
	})
}

func validGatewayRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func writeJSONResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeMarketError(w http.ResponseWriter, status int, code, message string) {
	writeJSONResponse(w, status, types.ErrorResponse{Code: code, Message: message})
}
