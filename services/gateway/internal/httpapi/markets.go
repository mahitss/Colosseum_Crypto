package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	adapter "prophet/gateway/internal/markets"
	"prophet/types"
)

type marketService interface {
	ListMarkets(context.Context, types.MarketQuery) (types.MarketPage, error)
	GetMarket(context.Context, string) (types.Market, error)
}

func registerMarketRoutes(mux *http.ServeMux, service marketService) {
	if service == nil {
		return
	}
	handler := marketHandler{service: service}
	mux.HandleFunc("GET /api/v1/markets", handler.list)
	mux.HandleFunc("GET /api/v1/markets/", handler.list)
	mux.HandleFunc("GET /api/v1/markets/{id}", handler.get)
	mux.HandleFunc("GET /api/v1/markets/{id}/", handler.get)
}

type marketHandler struct{ service marketService }

func (h marketHandler) list(w http.ResponseWriter, r *http.Request) {
	query := types.MarketQuery{
		Category: r.URL.Query().Get("category"), Status: r.URL.Query().Get("status"),
		CreatedBy: r.URL.Query().Get("createdBy"), Cursor: r.URL.Query().Get("cursor"),
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 50 {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be an integer from 1 to 50")
			return
		}
		query.Limit = limit
	}
	page, err := h.service.ListMarkets(r.Context(), query)
	if err != nil {
		writeMarketServiceError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, page)
}

func (h marketHandler) get(w http.ResponseWriter, r *http.Request) {
	market, err := h.service.GetMarket(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMarketServiceError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, market)
}

func writeMarketServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, adapter.ErrNotFound):
		writeMarketError(w, http.StatusNotFound, "MARKET_NOT_FOUND", "market was not found")
	case errors.Is(err, adapter.ErrInvalidQuery):
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "market query is invalid")
	case errors.Is(err, adapter.ErrRateLimited):
		writeMarketError(w, http.StatusTooManyRequests, "MARKET_RATE_LIMITED", "market service is rate limited")
	case errors.Is(err, adapter.ErrTimeout):
		writeMarketError(w, http.StatusGatewayTimeout, "MARKET_SERVICE_TIMEOUT", "market service timed out")
	default:
		writeMarketError(w, http.StatusBadGateway, "MARKET_SERVICE_UNAVAILABLE", "market service is unavailable")
	}
}
