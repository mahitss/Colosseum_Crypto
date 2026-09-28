package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"prophet/gateway/internal/marketstudio"
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
	return NewHandlerWithAllServices(service, intelligence, trades, nil, nil)
}

// NewHandlerWithAllServices is the pre-TASK-008 constructor and keeps its
// signature so existing callers and tests are unaffected. It delegates to
// NewHandlerWithEnterprise with the four enterprise services nil, which
// registers no watchlist, radar, alert or notification route.
func NewHandlerWithAllServices(
	service marketService,
	intelligence intelligenceReader,
	trades TradeService,
	studio MarketStudioService,
	interpreter marketstudio.Interpreter,
) http.Handler {
	return NewHandlerWithEnterprise(service, intelligence, trades, studio, interpreter, nil, nil, nil, nil)
}

// NewHandlerWithEnterprise wires every route group the gateway serves, including
// the user-owned enterprise surface introduced by TASK 008: watchlists, the
// signal radar, alert rules, and the notification inbox.
//
// Every service is optional in the same way and for the same reason: a nil
// service registers no routes rather than mounting handlers that would fail at
// request time. That is what lets the gateway start against a database with no
// trading credentials, and what keeps NewHandlerWithAllServices a strict subset
// of this constructor.
//
// The watchlist routes are mounted with a nil identity resolver, which makes
// them fall back to the package-level resolver installed by
// RegisterUserResolver. The alert, notification and watchlist-scoped radar
// handlers resolve identity through that same seam rather than through a
// resolver passed at mount time, so a deployment installs its provider once.
func NewHandlerWithEnterprise(
	service marketService,
	intelligence intelligenceReader,
	trades TradeService,
	studio MarketStudioService,
	interpreter marketstudio.Interpreter,
	watchlists watchlistService,
	radar radarService,
	alerts alertService,
	notifications notificationService,
) http.Handler {
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
	registerMarketStudioRoutes(mux, studio, interpreter)
	// The enterprise route groups. Each registration is a no-op for a nil
	// service, so passing nils here is how NewHandlerWithAllServices opts out.
	RegisterWatchlistRoutes(mux, watchlists, nil)
	RegisterRadarRoutes(mux, radar)
	RegisterAlertRoutes(mux, alerts)
	RegisterNotificationRoutes(mux, notifications)
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
