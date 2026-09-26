package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"prophet/panta-adapter/internal/client"
	"prophet/panta-adapter/internal/config"
	adaptererrors "prophet/panta-adapter/internal/errors"
	"prophet/panta-adapter/internal/markets"
	"prophet/types"
)

type pantaClient interface {
	ListMarkets(context.Context, client.PantaMarketQuery) (client.PantaMarketPage, error)
	GetMarket(context.Context, string) (client.PantaMarket, error)
	GetAccount(context.Context) (client.Account, error)
}

type marketService interface {
	ListMarkets(context.Context, types.MarketQuery) (types.MarketPage, error)
	GetMarket(context.Context, string) (types.Market, error)
}

type Handler struct {
	config  config.Config
	client  pantaClient
	markets marketService
}

func NewHandler(cfg config.Config, apiClient pantaClient, marketService marketService) http.Handler {
	handler := &Handler{config: cfg, client: apiClient, markets: marketService}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /health/panta", handler.pantaHealth)
	mux.HandleFunc("GET /markets", handler.listMarkets)
	mux.HandleFunc("GET /markets/", handler.listMarkets)
	mux.HandleFunc("GET /markets/{id}", handler.getMarket)
	mux.HandleFunc("GET /markets/{id}/", handler.getMarket)
	return requestID(mux)
}

func NewMarketService(apiClient pantaClient) marketService {
	return markets.NewService(apiClient)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	if err := h.config.Validate(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, types.ErrorResponse{Code: "CONFIGURATION_INVALID", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}

func (h *Handler) pantaHealth(w http.ResponseWriter, r *http.Request) {
	if err := h.config.Validate(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, types.ErrorResponse{Code: "CONFIGURATION_INVALID", Message: err.Error()})
		return
	}
	if _, err := h.client.GetAccount(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}

func (h *Handler) listMarkets(w http.ResponseWriter, r *http.Request) {
	query := types.MarketQuery{
		Category: r.URL.Query().Get("category"), Status: r.URL.Query().Get("status"),
		CreatedBy: r.URL.Query().Get("createdBy"), Cursor: r.URL.Query().Get("cursor"),
	}
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > 50 {
			writeJSON(w, http.StatusBadRequest, types.ErrorResponse{Code: "VALIDATION_ERROR", Message: "limit must be an integer from 1 to 50"})
			return
		}
		query.Limit = limit
	}
	page, err := h.markets.ListMarkets(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) getMarket(w http.ResponseWriter, r *http.Request) {
	market, err := h.markets.GetMarket(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, market)
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if !validRequestID(requestID) {
			var bytes [16]byte
			if _, err := rand.Read(bytes[:]); err != nil {
				requestID = "unavailable"
			} else {
				requestID = hex.EncodeToString(bytes[:])
			}
		}
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(client.WithRequestID(r.Context(), requestID)))
	})
}

func validRequestID(value string) bool {
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

func writeError(w http.ResponseWriter, err error) {
	var pantaError *adaptererrors.Error
	if errors.As(err, &pantaError) {
		status, code := publicError(pantaError)
		writeJSON(w, status, types.ErrorResponse{Code: code, Message: pantaError.Error(), RequestID: pantaError.RequestID})
		return
	}
	writeJSON(w, http.StatusBadGateway, types.ErrorResponse{Code: "UPSTREAM_ERROR", Message: "upstream request failed"})
}

func publicError(err *adaptererrors.Error) (int, string) {
	switch err.Kind {
	case adaptererrors.KindConfiguration:
		return http.StatusServiceUnavailable, "CONFIGURATION_INVALID"
	case adaptererrors.KindAuthentication:
		return http.StatusBadGateway, "UPSTREAM_AUTHENTICATION_FAILED"
	case adaptererrors.KindAuthorization:
		return http.StatusBadGateway, "UPSTREAM_AUTHORIZATION_FAILED"
	case adaptererrors.KindRateLimit:
		return http.StatusTooManyRequests, "UPSTREAM_RATE_LIMITED"
	case adaptererrors.KindValidation:
		return http.StatusBadRequest, "VALIDATION_ERROR"
	case adaptererrors.KindNotFound:
		return http.StatusNotFound, "MARKET_NOT_FOUND"
	case adaptererrors.KindTimeout:
		return http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT"
	case adaptererrors.KindNetwork:
		return http.StatusBadGateway, "UPSTREAM_UNAVAILABLE"
	case adaptererrors.KindMalformed:
		return http.StatusBadGateway, "UPSTREAM_MALFORMED_RESPONSE"
	default:
		return http.StatusBadGateway, "UPSTREAM_ERROR"
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
