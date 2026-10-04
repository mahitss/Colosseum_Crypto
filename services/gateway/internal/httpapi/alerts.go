// Alert rules and in-app notifications.
//
// # WHY A RULE IS A STRUCTURED OBJECT AND NOT A PIECE OF CODE
//
// An alert rule is user-authored configuration that runs on every signal
// event, for every market the user can see. The tempting design is a single
// "condition" or "expression" string that the evaluator parses or evaluates at
// runtime. This file refuses that design, and the refusal is the security
// property rather than an incidental style choice:
//
//   - No field on alertRuleRequest accepts free-form code, SQL, a regular
//     expression, a template, or an expression string. There is no field to
//     put one in. The complete set of conditions a rule can express is the
//     fixed struct in this file, and each field is checked by a pure function
//     before the service is called.
//   - The rule is validated by intelligence.ValidateAlertRule, a deterministic
//     pure function, before any repository call. A rule that survives it is
//     total: MatchesRule can evaluate it without a parse step, without a
//     database round trip, and without a failure mode.
//   - Scope is structural, not textual. A rule targets exactly one of
//     watchlist_id, market_id, or the whole account (both null), and both ids
//     are checked structurally - UUID and base58 address - before use.
//
// The alternative is an injection surface evaluated on a background goroutine
// with the repository's own credentials in scope. Rejecting the request here
// means it never reaches that code.
//
// # STRICT DECODING
//
// The market studio endpoints decode with a plain json.Decoder, which silently
// ignores unrecognised fields. That is the wrong default here. Alert rules are
// user-authored configuration, so a field a client believes it set but that
// this struct does not declare would be dropped on the floor while the client
// is shown a successful response: the user believes a condition is active when
// it is not. A typo, or a client talking to a newer API version, would produce
// a rule that looks configured and silently is not. decodeAlertRuleBody uses
// DisallowUnknownFields so an unrecognised field is a loud 400 with code
// INVALID_RULE rather than a quiet, wrong rule.
//
// # IDENTITY
//
// Identity comes from ResolveUser, the seam defined in watchlists.go. No
// handler here reads a user id from the payload, the query string, or the path.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"qevryn/gateway/internal/intelligence"
	"qevryn/types"
)

const (
	defaultNotificationLimit = 50
	maximumNotificationLimit = 200
	maximumAlertEventLimit   = 200
)

type alertService interface {
	ListRules(context.Context, string) ([]intelligence.AlertRule, error)
	GetRule(context.Context, string, string) (*intelligence.AlertRule, error)
	CreateRule(context.Context, string, intelligence.AlertRule) (*intelligence.AlertRule, error)
	UpdateRule(context.Context, string, string, intelligence.AlertRule) (*intelligence.AlertRule, error)
	DeleteRule(context.Context, string, string) error
	ListEvents(context.Context, string, int) ([]intelligence.AlertEvent, error)
}

type notificationService interface {
	ListNotifications(context.Context, string, int, bool) ([]intelligence.Notification, error)
	UnreadCount(context.Context, string) (int, error)
	MarkRead(context.Context, string, int64) error
	MarkAllRead(context.Context, string) (int64, error)
}

// alertRuleRequest is the only shape a client can send. Every optional field is
// a pointer so an absent field is distinguishable from a zero value: a rule
// that omits minimum_severity means "any severity", which is not the same
// intent as "INFO only", and a non-pointer string cannot express it.
//
// The struct is the whole configuration surface. There is intentionally no
// expression, condition, filter, query, script, or pattern field.
type alertRuleRequest struct {
	Name                       string  `json:"name"`
	Enabled                    *bool   `json:"enabled"`
	WatchlistID                *string `json:"watchlist_id"`
	MarketID                   *string `json:"market_id"`
	SignalType                 *string `json:"signal_type"`
	MinimumSeverity            *string `json:"minimum_severity"`
	ProbabilityChangeThreshold *string `json:"probability_change_threshold"`
	ActivityChangeThreshold    *string `json:"activity_change_threshold"`
	LiquidityChangeThreshold   *string `json:"liquidity_change_threshold"`
	CooldownSeconds            *int    `json:"cooldown_seconds"`
}

// decodeAlertRuleBody decodes an alert rule with unknown fields rejected. A
// returned error covers both malformed JSON and an unrecognised field; both are
// client mistakes reported as INVALID_RULE, and neither is distinguishable to
// an attacker in any way that matters since the body is not echoed back.
func decodeAlertRuleBody(r *http.Request, target *alertRuleRequest) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	// A second value in the stream means the body was crafted to carry a
	// second document; decoding only the first would apply a rule the client
	// did not intend as the operative one.
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

// alertRuleFromRequest converts the DTO into the domain rule and validates it
// with the same pure function the evaluator relies on. The returned error is a
// client error, never a service error.
//
// Scope handling: an empty string in an optional id field means "unset", not
// "set to empty", so a client can send watchlist_id:"" to mean the global scope
// without being told it is malformed. A non-empty value is checked
// structurally before it is carried into the rule.
func alertRuleFromRequest(userID string, req alertRuleRequest) (intelligence.AlertRule, error) {
	rule := intelligence.AlertRule{
		UserID:                     userID,
		Name:                       strings.TrimSpace(req.Name),
		SignalType:                 trimmedOptional(req.SignalType),
		MinimumSeverity:            trimmedOptional(req.MinimumSeverity),
		ProbabilityChangeThreshold: trimmedOptional(req.ProbabilityChangeThreshold),
		ActivityChangeThreshold:    trimmedOptional(req.ActivityChangeThreshold),
		LiquidityChangeThreshold:   trimmedOptional(req.LiquidityChangeThreshold),
	}
	// A rule is created enabled unless the client says otherwise. Disabling a
	// rule is an explicit act; leaving one enabled is the ordinary case.
	rule.Enabled = true
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.CooldownSeconds != nil {
		rule.CooldownSeconds = *req.CooldownSeconds
	}

	watchlistID := trimmedOptional(req.WatchlistID)
	marketID := trimmedOptional(req.MarketID)

	// Exactly one scope, or none. Both set is rejected rather than silently
	// preferring one: a user who scopes a rule to a watchlist and also names a
	// market has not said which they meant, and guessing for them produces a
	// rule that fires on a different set of markets than they read off screen.
	if watchlistID != nil && marketID != nil {
		return intelligence.AlertRule{}, errors.New("watchlist_id and market_id are mutually exclusive")
	}
	if watchlistID != nil && !validUUID(*watchlistID) {
		return intelligence.AlertRule{}, errors.New("watchlist_id must be a UUID")
	}
	if marketID != nil && !types.IsBase58Address(*marketID) {
		return intelligence.AlertRule{}, errors.New("market_id must be a base58 market address")
	}
	rule.WatchlistID = watchlistID
	rule.MarketID = marketID

	if err := intelligence.ValidateAlertRule(rule); err != nil {
		return intelligence.AlertRule{}, err
	}
	return rule, nil
}

// trimmedOptional normalises a nullable string field: absent and blank both
// collapse to nil, so a rule never carries an empty optional value that the
// matching logic would have to interpret.
func trimmedOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

type alertHandler struct {
	service alertService
}

// RegisterAlertRoutes mounts the alert rule routes. A nil service registers
// nothing, matching the other optional route groups in this package.
func RegisterAlertRoutes(mux *http.ServeMux, service alertService) {
	if service == nil {
		return
	}
	handler := alertHandler{service: service}

	mux.HandleFunc("GET /api/v1/alert-rules", handler.list)
	mux.HandleFunc("POST /api/v1/alert-rules", handler.create)
	mux.HandleFunc("GET /api/v1/alert-rules/events", handler.events)
	mux.HandleFunc("GET /api/v1/alert-rules/{id}", handler.get)
	mux.HandleFunc("PATCH /api/v1/alert-rules/{id}", handler.update)
	mux.HandleFunc("DELETE /api/v1/alert-rules/{id}", handler.remove)
}

func (handler alertHandler) list(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	rules, err := handler.service.ListRules(r.Context(), userID)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	if rules == nil {
		rules = []intelligence.AlertRule{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"alert_rules": rules})
}

func (handler alertHandler) get(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	ruleID, ok := pathAlertRuleID(w, r)
	if !ok {
		return
	}
	rule, err := handler.service.GetRule(r.Context(), userID, ruleID)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, rule)
}

func (handler alertHandler) create(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	var req alertRuleRequest
	if err := decodeAlertRuleBody(r, &req); err != nil {
		writeAlertError(w, errInvalidAlertRule(err))
		return
	}
	rule, err := alertRuleFromRequest(userID, req)
	if err != nil {
		writeAlertError(w, errInvalidAlertRule(err))
		return
	}
	created, err := handler.service.CreateRule(r.Context(), userID, rule)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	logAlertRuleName("/api/v1/alert-rules", "alert rule created", created)
	writeJSONResponse(w, http.StatusCreated, created)
}

// update applies a partial edit. PATCH replaces the mutable fields rather than
// merging, because the repository rewrites them together and a merge would
// leave a client unable to clear a condition it previously set. The scope ids
// are not editable here: the service treats the scope columns as immutable, so
// a client that sends a different one has it ignored rather than silently
// retargeted. Name is therefore required, since a rule cannot be nameless and
// this layer has no "current name" to fall back to.
func (handler alertHandler) update(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	ruleID, ok := pathAlertRuleID(w, r)
	if !ok {
		return
	}
	var req alertRuleRequest
	if err := decodeAlertRuleBody(r, &req); err != nil {
		writeAlertError(w, errInvalidAlertRule(err))
		return
	}
	rule, err := alertRuleFromRequest(userID, req)
	if err != nil {
		writeAlertError(w, errInvalidAlertRule(err))
		return
	}
	updated, err := handler.service.UpdateRule(r.Context(), userID, ruleID, rule)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	logAlertRuleName("/api/v1/alert-rules", "alert rule updated", updated)
	writeJSONResponse(w, http.StatusOK, updated)
}

func (handler alertHandler) remove(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	ruleID, ok := pathAlertRuleID(w, r)
	if !ok {
		return
	}
	if err := handler.service.DeleteRule(r.Context(), userID, ruleID); err != nil {
		writeAlertError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler alertHandler) events(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	limit := maximumAlertEventLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maximumAlertEventLimit {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be an integer from 1 to 200")
			return
		}
		limit = parsed
	}
	events, err := handler.service.ListEvents(r.Context(), userID, limit)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	if events == nil {
		events = []intelligence.AlertEvent{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"alert_events": events})
}

// invalidAlertRuleError tags a client-side rule error so writeAlertError maps
// it to 400 INVALID_RULE. A distinct type rather than a bare sentinel keeps the
// message returned by the pure validator attached to the failure, and lets the
// mapping distinguish "the rule was bad" from "the store was unreachable".
type invalidAlertRuleError struct{ err error }

func (e invalidAlertRuleError) Error() string { return e.err.Error() }
func (e invalidAlertRuleError) Unwrap() error { return e.err }

func errInvalidAlertRule(err error) error { return invalidAlertRuleError{err: err} }

// writeAlertError maps a service failure to a response. The mapping is
// deliberately narrow, matching the writeWatchlistError style: anything not
// explicitly recognised is a 500, so a new error surfaced by the repository
// cannot accidentally reach clients labelled as a validation or not-found
// condition.
func writeAlertError(w http.ResponseWriter, err error) {
	var invalid invalidAlertRuleError
	switch {
	case errors.Is(err, ErrUnauthenticated):
		writeMarketError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "a user identity is required")
	case errors.As(err, &invalid):
		writeMarketError(w, http.StatusBadRequest, "INVALID_RULE", invalid.Error())
	case errors.Is(err, intelligence.ErrAlertRuleNotFound):
		writeMarketError(w, http.StatusNotFound, "ALERT_RULE_NOT_FOUND", "alert rule was not found")
	case errors.Is(err, intelligence.ErrNotificationNotFound):
		writeMarketError(w, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "notification was not found")
	case errors.Is(err, intelligence.ErrWatchlistNotFound):
		writeMarketError(w, http.StatusNotFound, "WATCHLIST_NOT_FOUND", "watchlist was not found")
	case errors.Is(err, intelligence.ErrMarketNotFound):
		writeMarketError(w, http.StatusNotFound, "MARKET_NOT_FOUND", "market was not found")
	default:
		writeMarketError(w, http.StatusInternalServerError, "ALERT_OPERATION_FAILED", "alert operation failed")
	}
}

// identifyAlertUser resolves the acting user through the shared seam in
// watchlists.go and writes the 401 itself when it cannot. A handler that
// continued past a failure would query with an empty owner, which in SQL means
// no filter at all.
func identifyAlertUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, err := ResolveUser(r)
	if err != nil {
		writeAlertError(w, ErrUnauthenticated)
		return "", false
	}
	return userID, true
}

func pathAlertRuleID(w http.ResponseWriter, r *http.Request) (string, bool) {
	ruleID := strings.TrimSpace(r.PathValue("id"))
	if !validUUID(ruleID) {
		writeAlertError(w, errInvalidAlertRule(errors.New("alert rule id must be a UUID")))
		return "", false
	}
	return ruleID, true
}

type notificationHandler struct {
	service notificationService
}

// RegisterNotificationRoutes mounts the in-app notification inbox routes.
func RegisterNotificationRoutes(mux *http.ServeMux, service notificationService) {
	if service == nil {
		return
	}
	handler := notificationHandler{service: service}

	mux.HandleFunc("GET /api/v1/notifications", handler.list)
	mux.HandleFunc("GET /api/v1/notifications/unread-count", handler.unreadCount)
	mux.HandleFunc("POST /api/v1/notifications/read-all", handler.readAll)
	mux.HandleFunc("POST /api/v1/notifications/{id}/read", handler.markRead)
}

func (handler notificationHandler) list(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	limit := defaultNotificationLimit
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maximumNotificationLimit {
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be an integer from 1 to 200")
			return
		}
		limit = parsed
	}
	// Only "true" and "1" are read as a request to filter. An unrecognised
	// value is a 400 rather than a silent false, because a client that meant to
	// filter and did not would otherwise render a full inbox and show a stale
	// unread badge.
	unreadOnly := false
	if raw := strings.TrimSpace(query.Get("unread_only")); raw != "" {
		switch raw {
		case "true", "1":
			unreadOnly = true
		case "false", "0":
			unreadOnly = false
		default:
			writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "unread_only must be true or false")
			return
		}
	}
	notifications, err := handler.service.ListNotifications(r.Context(), userID, limit, unreadOnly)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	if notifications == nil {
		notifications = []intelligence.Notification{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"notifications": notifications})
}

// unreadCount asks the store. The count is a real query result, never derived
// from the length of the last page the client happened to receive: a page is
// limited to 50 rows by default, so a client-side guess from it is wrong for
// every user with more notifications than fit on a page, and wrong in the
// direction of showing an alert as read when it is not.
func (handler notificationHandler) unreadCount(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	count, err := handler.service.UnreadCount(r.Context(), userID)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]int{"unread_count": count})
}

func (handler notificationHandler) markRead(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	notificationID, ok := pathNotificationID(w, r)
	if !ok {
		return
	}
	if err := handler.service.MarkRead(r.Context(), userID, notificationID); err != nil {
		writeAlertError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// readAll returns the number of rows the store actually changed rather than a
// constant. A client that read "0" knows the inbox was already clear; a client
// told "everything is marked read" in both cases cannot tell a failed
// transition from a no-op one.
func (handler notificationHandler) readAll(w http.ResponseWriter, r *http.Request) {
	logged := alertLog(w, r)
	defer logged()

	userID, ok := identifyAlertUser(w, r)
	if !ok {
		return
	}
	updated, err := handler.service.MarkAllRead(r.Context(), userID)
	if err != nil {
		writeAlertError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]int64{"updated": updated})
}

func pathNotificationID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.PathValue("id"))
	notificationID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || notificationID < 1 {
		writeMarketError(w, http.StatusBadRequest, "VALIDATION_ERROR", "notification id must be a positive integer")
		return 0, false
	}
	return notificationID, true
}

// alertLog wraps the ResponseWriter so the status is known when the request
// finishes, then emits the single structured line this file logs per request:
// method, path, status. Nothing else about the request is recorded. Rule
// bodies are user-authored and may carry market and watchlist identifiers; only
// the length of the rule name is ever logged, which is enough to correlate a
// write with a rule without putting user-supplied text into the log stream.
func alertLog(w http.ResponseWriter, r *http.Request) func() {
	recorder := &alertStatusRecorder{ResponseWriter: w, status: http.StatusOK}
	return func() {
		slog.Info("alert request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.status),
		)
	}
}

type alertStatusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (recorder *alertStatusRecorder) WriteHeader(status int) {
	if !recorder.written {
		recorder.status = status
		recorder.written = true
	}
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *alertStatusRecorder) Write(payload []byte) (int, error) {
	recorder.written = true
	return recorder.ResponseWriter.Write(payload)
}

// logAlertRuleName records a write against a rule without recording the rule.
// The name is user text, so only its length is kept.
func logAlertRuleName(path, message string, rule *intelligence.AlertRule) {
	if rule == nil {
		return
	}
	slog.Info(message,
		slog.String("path", path),
		slog.Int("rule_id_len", len(rule.ID)),
		slog.Int("name_len", len(rule.Name)),
	)
}

