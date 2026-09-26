package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"prophet/gateway/internal/intelligence"
	"prophet/types"
)

const maximumSignalLimit = 100

type intelligenceReader interface {
	Signals(context.Context, intelligence.SignalQuery) (intelligence.SignalPage, error)
	MarketIntelligence(context.Context, string) (intelligence.MarketIntelligence, error)
}

type intelligenceHandler struct {
	service intelligenceReader
}

func registerIntelligenceRoutes(mux *http.ServeMux, service intelligenceReader) {
	handler := intelligenceHandler{service: service}
	mux.HandleFunc("GET /api/v1/intelligence/signals", handler.signals)
	mux.HandleFunc("GET /api/v1/markets/{id}/intelligence", handler.market)
}

func (handler intelligenceHandler) signals(w http.ResponseWriter, r *http.Request) {
	if handler.service == nil {
		writeMarketError(w, http.StatusServiceUnavailable, "INTELLIGENCE_UNAVAILABLE", "intelligence service is unavailable")
		return
	}
	query := intelligence.SignalQuery{MarketID: r.URL.Query().Get("market_id"), SignalType: r.URL.Query().Get("signal_type"), Severity: r.URL.Query().Get("severity")}
	if query.MarketID != "" && !types.IsBase58Address(query.MarketID) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "market_id must be a base58 market address")
		return
	}
	if query.SignalType != "" && !validSignalType(query.SignalType) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "signal_type is invalid")
		return
	}
	if query.Severity != "" && !validSeverity(query.Severity) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "severity is invalid")
		return
	}
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from must be RFC3339")
			return
		}
		query.From = &parsed
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "to must be RFC3339")
			return
		}
		query.To = &parsed
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from must not be after to")
		return
	}
	query.Limit = 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maximumSignalLimit {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be an integer from 1 to 100")
			return
		}
		query.Limit = limit
	}
	query.Cursor = r.URL.Query().Get("cursor")
	page, err := handler.service.Signals(r.Context(), query)
	if err != nil {
		writeIntelligenceError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, intelligence.ToSignalPageDTO(page))
}

func (handler intelligenceHandler) market(w http.ResponseWriter, r *http.Request) {
	if handler.service == nil {
		writeMarketError(w, http.StatusServiceUnavailable, "INTELLIGENCE_UNAVAILABLE", "intelligence service is unavailable")
		return
	}
	marketID := r.PathValue("id")
	if !types.IsBase58Address(marketID) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "market id must be a base58 address")
		return
	}
	result, err := handler.service.MarketIntelligence(r.Context(), marketID)
	if err != nil {
		writeIntelligenceError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, intelligence.ToMarketIntelligenceDTO(result))
}

func writeIntelligenceError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrMarketNotFound) {
		writeMarketError(w, http.StatusNotFound, "MARKET_NOT_FOUND", "market was not found")
		return
	}
	writeMarketError(w, http.StatusInternalServerError, "INTELLIGENCE_QUERY_FAILED", "intelligence data is unavailable")
}

func validSignalType(value string) bool {
	switch value {
	case "NEW_MARKET", "PROBABILITY_SHIFT", "ACTIVITY_CHANGE", "LIQUIDITY_CHANGE", "MARKET_MOVEMENT":
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch value {
	case "INFO", "WATCH", "SIGNIFICANT", "CRITICAL":
		return true
	default:
		return false
	}
}

var ErrMarketNotFound = errors.New("market not found")
