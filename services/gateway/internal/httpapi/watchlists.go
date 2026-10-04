// Package-level HTTP API for watchlists (user-owned) and the signal radar.
//
// IDENTITY SEAM (read this before adding a field to a watchlist request)
//
// Watchlists are user-owned rows, but this repository has no authentication
// layer yet. Two ways to supply the owner are possible and one of them is a
// vulnerability:
//
//  1. Accept a "user_id" in the JSON body. Rejected. A caller-supplied owner
//     means any client can read, mutate, or probe another user's watchlists,
//     and the ownership filter in the repository becomes decorative. No request
//     type in this file carries a user id, by design.
//
//  2. Resolve the acting user from the request through a single interface. This
//     is what the file does. Every handler calls ResolveUser(r); none of them
//     read an identity from the payload or the query string.
//
// DefaultUserResolver returns a fixed development identity so the gateway runs
// before an auth provider is chosen. It is intentionally loud rather than
// subtle: a deployment that forgets to install a real resolver still compiles,
// still serves, and still scopes correctly, it just scopes everyone to one
// user. Wiring an authenticated implementation is a one-line change in main.go
// via SetUserResolver, and no handler is touched.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"qevryn/gateway/internal/intelligence"
	"qevryn/types"
)

const (
	minimumWatchlistNameRunes = 1
	maximumWatchlistNameRunes = 120
	defaultRadarLimit         = 50
)

type watchlistService interface {
	ListWatchlists(context.Context, string) ([]intelligence.WatchlistSummary, error)
	GetWatchlist(context.Context, string, string) (*intelligence.Watchlist, error)
	CreateWatchlist(context.Context, string, string, *string) (*intelligence.Watchlist, error)
	UpdateWatchlist(context.Context, string, string, string, *string) (*intelligence.Watchlist, error)
	DeleteWatchlist(context.Context, string, string) error
	AddMarket(context.Context, string, string, string) error
	RemoveMarket(context.Context, string, string, string) error
	WatchlistIntelligence(context.Context, string, string) (*intelligence.WatchlistIntelligence, error)
}

type radarService interface {
	Radar(context.Context, intelligence.RadarQuery) ([]intelligence.RadarEvent, *string, int64, error)
}

// userResolver maps a request to the acting user id. TASK 008 introduces
// watchlists and alerts, which are user-owned, but the repository has no
// authentication layer yet. Rather than trusting a user_id from the request
// body, which would let any caller impersonate any user, identity is resolved
// through this one seam. Deployments replace DefaultUserResolver with an
// authenticated implementation and every handler inherits it unchanged.
type userResolver interface {
	ResolveUser(r *http.Request) (string, error)
}

// UserResolver is the exported name for the identity seam, so main.go can
// install an implementation without reaching into unexported identifiers.
type UserResolver = userResolver

// ErrUnauthenticated is returned when no user identity can be established.
var ErrUnauthenticated = errors.New("unauthenticated")

// DefaultUserResolver is the single-user development identity. It exists so
// local development and tests work before an auth provider is chosen, and it
// is deliberately obvious: replace it in main.go via RegisterUserResolver.
type DefaultUserResolver struct{}

// ResolveUser returns the well-known development user id.
func (DefaultUserResolver) ResolveUser(r *http.Request) (string, error) {
	return "local-user", nil
}

var (
	resolverMutex   sync.RWMutex
	currentResolver UserResolver = DefaultUserResolver{}
)

// SetUserResolver installs the resolver every user-scoped handler uses. It is
// the single wiring point for a real authentication provider: call it once
// during startup, before serving traffic. Passing nil restores the default
// development identity rather than leaving the package unusable.
func SetUserResolver(resolver UserResolver) {
	resolverMutex.Lock()
	defer resolverMutex.Unlock()
	if resolver == nil {
		currentResolver = DefaultUserResolver{}
		return
	}
	currentResolver = resolver
}

// RegisterUserResolver is the startup-time spelling of SetUserResolver, so
// main.go reads as an explicit wiring decision rather than a setter call lost
// among configuration.
func RegisterUserResolver(resolver UserResolver) {
	SetUserResolver(resolver)
}

// ResolveUser returns the acting user id for a request. Any failure to
// establish an identity, including an empty id, is reported as
// ErrUnauthenticated so handlers have a single error to map and a misbehaving
// resolver cannot degrade into an empty-owner query.
func ResolveUser(r *http.Request) (string, error) {
	resolverMutex.RLock()
	resolver := currentResolver
	resolverMutex.RUnlock()

	userID, err := resolver.ResolveUser(r)
	if err != nil {
		return "", ErrUnauthenticated
	}
	if strings.TrimSpace(userID) == "" {
		return "", ErrUnauthenticated
	}
	return userID, nil
}

// Request DTOs. These are the only shapes a client can send; the database
// models stay behind the service interface. Note the absence of any user field.
type createWatchlistRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type updateWatchlistRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type watchlistHandler struct {
	service  watchlistService
	resolver UserResolver
}

// RegisterWatchlistRoutes mounts the user-owned watchlist routes. A nil service
// registers nothing; a nil resolver falls back to the package default.
func RegisterWatchlistRoutes(mux *http.ServeMux, service watchlistService, resolver UserResolver) {
	if service == nil {
		return
	}
	if resolver == nil {
		resolverMutex.RLock()
		resolver = currentResolver
		resolverMutex.RUnlock()
		if resolver == nil {
			resolver = DefaultUserResolver{}
		}
	}
	handler := watchlistHandler{service: service, resolver: resolver}

	mux.HandleFunc("GET /api/v1/watchlists", handler.list)
	mux.HandleFunc("POST /api/v1/watchlists", handler.create)
	mux.HandleFunc("GET /api/v1/watchlists/{id}", handler.get)
	mux.HandleFunc("PATCH /api/v1/watchlists/{id}", handler.update)
	mux.HandleFunc("DELETE /api/v1/watchlists/{id}", handler.remove)
	mux.HandleFunc("POST /api/v1/watchlists/{id}/markets/{marketId}", handler.addMarket)
	mux.HandleFunc("DELETE /api/v1/watchlists/{id}/markets/{marketId}", handler.removeMarket)
	mux.HandleFunc("GET /api/v1/watchlists/{id}/intelligence", handler.aggregate)
}

func (handler watchlistHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlists, err := handler.service.ListWatchlists(r.Context(), userID)
	if err != nil {
		writeWatchlistError(w, err)
		return
	}
	if watchlists == nil {
		watchlists = []intelligence.WatchlistSummary{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"watchlists": watchlists})
}

func (handler watchlistHandler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlistID, ok := handler.pathWatchlistID(w, r)
	if !ok {
		return
	}
	watchlist, err := handler.service.GetWatchlist(r.Context(), userID, watchlistID)
	if err != nil {
		writeWatchlistError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, watchlist)
}

func (handler watchlistHandler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	var req createWatchlistRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request body must be valid JSON")
		return
	}
	name, ok := validatedWatchlistName(w, req.Name)
	if !ok {
		return
	}
	watchlist, err := handler.service.CreateWatchlist(r.Context(), userID, name, req.Description)
	if err != nil {
		writeWatchlistError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, watchlist)
}

func (handler watchlistHandler) update(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlistID, ok := handler.pathWatchlistID(w, r)
	if !ok {
		return
	}
	var req updateWatchlistRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request body must be valid JSON")
		return
	}
	name, ok := validatedWatchlistName(w, req.Name)
	if !ok {
		return
	}
	watchlist, err := handler.service.UpdateWatchlist(r.Context(), userID, watchlistID, name, req.Description)
	if err != nil {
		writeWatchlistError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, watchlist)
}

func (handler watchlistHandler) remove(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlistID, ok := handler.pathWatchlistID(w, r)
	if !ok {
		return
	}
	if err := handler.service.DeleteWatchlist(r.Context(), userID, watchlistID); err != nil {
		writeWatchlistError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler watchlistHandler) addMarket(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlistID, marketID, ok := handler.pathMarket(w, r)
	if !ok {
		return
	}
	if err := handler.service.AddMarket(r.Context(), userID, watchlistID, marketID); err != nil {
		writeWatchlistError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler watchlistHandler) removeMarket(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlistID, marketID, ok := handler.pathMarket(w, r)
	if !ok {
		return
	}
	if err := handler.service.RemoveMarket(r.Context(), userID, watchlistID, marketID); err != nil {
		writeWatchlistError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler watchlistHandler) aggregate(w http.ResponseWriter, r *http.Request) {
	userID, ok := handler.identify(w, r)
	if !ok {
		return
	}
	watchlistID, ok := handler.pathWatchlistID(w, r)
	if !ok {
		return
	}
	result, err := handler.service.WatchlistIntelligence(r.Context(), userID, watchlistID)
	if err != nil {
		writeWatchlistError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, result)
}

// identify resolves the acting user and writes the 401 itself when it cannot.
func (handler watchlistHandler) identify(w http.ResponseWriter, r *http.Request) (string, bool) {
	resolver := handler.resolver
	if resolver == nil {
		resolver = DefaultUserResolver{}
	}
	userID, err := resolver.ResolveUser(r)
	if err != nil || strings.TrimSpace(userID) == "" {
		writeWatchlistError(w, ErrUnauthenticated)
		return "", false
	}
	return userID, true
}

func (handler watchlistHandler) pathWatchlistID(w http.ResponseWriter, r *http.Request) (string, bool) {
	watchlistID := strings.TrimSpace(r.PathValue("id"))
	if !validUUID(watchlistID) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "watchlist id must be a UUID")
		return "", false
	}
	return watchlistID, true
}

func (handler watchlistHandler) pathMarket(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	watchlistID, ok := handler.pathWatchlistID(w, r)
	if !ok {
		return "", "", false
	}
	marketID := strings.TrimSpace(r.PathValue("marketId"))
	if !types.IsBase58Address(marketID) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "market id must be a base58 address")
		return "", "", false
	}
	return watchlistID, marketID, true
}

// writeWatchlistError maps a service failure to a response. The mapping is
// deliberately narrow: anything not explicitly recognised is a 500, so a new
// error surfaced by the repository cannot accidentally reach clients labelled
// as a validation or not-found condition.
func writeWatchlistError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		writeMarketError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "a user identity is required")
	case errors.Is(err, intelligence.ErrWatchlistNotFound):
		writeMarketError(w, http.StatusNotFound, "WATCHLIST_NOT_FOUND", "watchlist was not found")
	case errors.Is(err, intelligence.ErrWatchlistNameTaken):
		writeMarketError(w, http.StatusConflict, "WATCHLIST_NAME_TAKEN", "a watchlist with that name already exists")
	case errors.Is(err, intelligence.ErrMarketNotFound):
		writeMarketError(w, http.StatusNotFound, "MARKET_NOT_FOUND", "market was not found")
	default:
		writeMarketError(w, http.StatusInternalServerError, "WATCHLIST_OPERATION_FAILED", "watchlist operation failed")
	}
}

type radarHandler struct {
	service radarService
}

// RegisterRadarRoutes mounts the signal radar feed. A nil service registers
// nothing, matching the other optional route groups.
func RegisterRadarRoutes(mux *http.ServeMux, service radarService) {
	if service == nil {
		return
	}
	handler := radarHandler{service: service}
	mux.HandleFunc("GET /api/v1/intelligence/radar", handler.radar)
}

func (handler radarHandler) radar(w http.ResponseWriter, r *http.Request) {
	if handler.service == nil {
		writeMarketError(w, http.StatusServiceUnavailable, "INTELLIGENCE_UNAVAILABLE", "intelligence service is unavailable")
		return
	}
	values := r.URL.Query()
	query := intelligence.RadarQuery{
		Severity: values.Get("severity"),
		MarketID: values.Get("market_id"),
	}
	if query.Severity != "" && !validSeverity(query.Severity) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "severity is invalid")
		return
	}
	if signalType := values.Get("signal_type"); signalType != "" {
		if !validSignalType(signalType) {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "signal_type is invalid")
			return
		}
		query.SignalType = signalType
	}
	if query.MarketID != "" && !types.IsBase58Address(query.MarketID) {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "market_id must be a base58 market address")
		return
	}
	if watchlistID := strings.TrimSpace(values.Get("watchlist_id")); watchlistID != "" {
		if !validUUID(watchlistID) {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "watchlist_id must be a UUID")
			return
		}
		query.WatchlistID = watchlistID
	}
	// A watchlist-scoped radar reads user-owned data, so the acting identity is
	// resolved here and carried on the query. The repository verifies the
	// watchlist belongs to this user and returns ErrWatchlistNotFound
	// otherwise; with no identity it fails closed. The unscoped radar reads no
	// user-owned data, so it does not require one.
	if query.WatchlistID != "" {
		userID, err := ResolveUser(r)
		if err != nil {
			writeWatchlistError(w, err)
			return
		}
		query.UserID = userID
	}
	if raw := values.Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from must be RFC3339")
			return
		}
		query.From = &parsed
	}
	if raw := values.Get("to"); raw != "" {
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
	query.Limit = defaultRadarLimit
	if raw := values.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maximumSignalLimit {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be an integer from 1 to 100")
			return
		}
		query.Limit = limit
	}
	// The cursor is an opaque pagination token minted by the service. It is
	// passed through untouched: this layer has no notion of its encoding, and
	// parsing it here would couple the HTTP surface to a storage detail.
	query.Cursor = values.Get("cursor")

	// One structured line per request. Filter values are client-supplied, so
	// only presence and length are recorded; the values themselves would put
	// attacker-controlled text into the log stream.
	slog.Info("radar query",
		slog.String("path", r.URL.Path),
		slog.Int("limit", query.Limit),
		slog.Int("cursor_len", len(query.Cursor)),
		slog.Int("severity_len", len(query.Severity)),
		slog.Int("signal_type_len", len(query.SignalType)),
		slog.Int("market_id_len", len(query.MarketID)),
		slog.Int("watchlist_id_len", len(query.WatchlistID)),
		slog.Bool("has_from", query.From != nil),
		slog.Bool("has_to", query.To != nil),
	)

	events, nextCursor, total, err := handler.service.Radar(r.Context(), query)
	if err != nil {
		writeWatchlistError(w, err)
		return
	}
	if events == nil {
		events = []intelligence.RadarEvent{}
	}
	writeJSONResponse(w, http.StatusOK, radarResponse{
		Events:     events,
		NextCursor: nextCursor,
		Total:      total,
	})
}

type radarResponse struct {
	Events     []intelligence.RadarEvent `json:"events"`
	NextCursor *string                   `json:"next_cursor"`
	Total      int64                     `json:"total"`
}

func decodeJSONBody(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	return json.NewDecoder(r.Body).Decode(target)
}

// validatedWatchlistName trims and length-checks a watchlist name. Length is
// measured in runes, not bytes, so a visually short name written in a
// multi-byte script is not rejected as too long.
func validatedWatchlistName(w http.ResponseWriter, raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(name)
	if length < minimumWatchlistNameRunes || length > maximumWatchlistNameRunes {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name must be between 1 and 120 characters")
		return "", false
	}
	return name, true
}

// validUUID accepts the canonical 8-4-4-12 hexadecimal form. It is a structural
// check on a path or query segment, not a semantic one: the service layer
// still scopes every lookup by the resolved user, so a syntactically valid id
// belonging to someone else resolves to not-found rather than to data.
func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index := 0; index < 36; index++ {
		character := value[index]
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !isHexDigit(character) {
			return false
		}
	}
	return true
}

func isHexDigit(character byte) bool {
	return (character >= '0' && character <= '9') ||
		(character >= 'a' && character <= 'f') ||
		(character >= 'A' && character <= 'F')
}

