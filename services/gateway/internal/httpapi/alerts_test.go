package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"prophet/gateway/internal/intelligence"
)

type recordingAlerts struct {
	rules  []intelligence.AlertRule
	events []intelligence.AlertEvent
	notes  []intelligence.Notification
	calls  []string
	err    error
}

func (r *recordingAlerts) ListRules(_ context.Context, userID string) ([]intelligence.AlertRule, error) {
	r.calls = append(r.calls, "list:"+userID)
	return r.rules, r.err
}

func (r *recordingAlerts) GetRule(_ context.Context, userID, id string) (*intelligence.AlertRule, error) {
	r.calls = append(r.calls, "get:"+userID+":"+id)
	if r.err != nil {
		return nil, r.err
	}
	return &intelligence.AlertRule{ID: id, UserID: userID, Name: "rule", Enabled: true}, nil
}

func (r *recordingAlerts) CreateRule(_ context.Context, userID string, rule intelligence.AlertRule) (*intelligence.AlertRule, error) {
	r.calls = append(r.calls, "create:"+userID)
	if r.err != nil {
		return nil, r.err
	}
	rule.ID = "aaaaaaaa-1111-2222-3333-444444444444"
	return &rule, nil
}

func (r *recordingAlerts) UpdateRule(_ context.Context, userID, id string, rule intelligence.AlertRule) (*intelligence.AlertRule, error) {
	r.calls = append(r.calls, "update:"+userID+":"+id)
	if r.err != nil {
		return nil, r.err
	}
	rule.ID = id
	return &rule, nil
}

func (r *recordingAlerts) DeleteRule(_ context.Context, userID, id string) error {
	r.calls = append(r.calls, "delete:"+userID+":"+id)
	return r.err
}

func (r *recordingAlerts) ListEvents(_ context.Context, userID string, limit int) ([]intelligence.AlertEvent, error) {
	r.calls = append(r.calls, "events:"+userID)
	return r.events, r.err
}

func (r *recordingAlerts) ListNotifications(_ context.Context, userID string, _ int, unreadOnly bool) ([]intelligence.Notification, error) {
	r.calls = append(r.calls, "notifications:"+userID)
	if unreadOnly {
		return nil, r.err
	}
	return r.notes, r.err
}

func (r *recordingAlerts) UnreadCount(_ context.Context, userID string) (int, error) {
	r.calls = append(r.calls, "unread:"+userID)
	return len(r.notes), r.err
}

func (r *recordingAlerts) MarkRead(_ context.Context, userID string, id int64) error {
	r.calls = append(r.calls, "read:"+userID)
	return r.err
}

func (r *recordingAlerts) MarkAllRead(_ context.Context, userID string) (int64, error) {
	r.calls = append(r.calls, "read-all:"+userID)
	return int64(len(r.notes)), r.err
}

func newAlertHandler(t *testing.T, service alertService) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterAlertRoutes(mux, service)
	// The double implements both service interfaces. The assertion is a
	// deliberate compile-time check that the two stay in step: a route
	// registered against an interface the double does not satisfy is a test
	// that would silently never reach the double.
	notifications, ok := service.(notificationService)
	if !ok {
		t.Fatal("alert double must also satisfy notificationService")
	}
	RegisterNotificationRoutes(mux, notifications)
	return withRequestID(mux)
}

func withTestUser(t *testing.T, userID string) {
	t.Helper()
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })
	SetUserResolver(fixedResolver{userID: userID})
}

const validRuleBody = `{"name":"Significant moves","minimum_severity":"SIGNIFICANT"}`

// --- injection resistance (Phase 19) ---------------------------------------

// An alert rule runs on a background goroutine with the repository's own
// credentials in scope, so there must be no field that can carry executable
// content. The request struct has no such field, and strict decoding turns any
// attempt to supply one into a loud 400 instead of a silently-ignored key.
func TestAlertRulesRefuseExecutableContent(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	injections := map[string]string{
		"sql in a name":        `{"name":"x'; DROP TABLE markets; --"}`,
		"expression field":     `{"name":"ok","expression":"severity >= 3"}`,
		"condition field":      `{"name":"ok","condition":"always"}`,
		"filter field":         `{"name":"ok","filter":{"severity":"CRITICAL"}}`,
		"query field":          `{"name":"ok","query":"SELECT * FROM markets"}`,
		"script field":         `{"name":"ok","script":"import os; os.system('x')"}`,
		"pattern field":        `{"name":"ok","pattern":".*"}`,
		"regex field":          `{"name":"ok","regex":"^CRITICAL$"}`,
		"template field":       `{"name":"ok","template":"{{.Secrets}}"}`,
		"javascript scheme":    `{"name":"ok","javascript":"fetch('x')"}`,
		"nested sql":           `{"name":"ok","minimum_severity":"INFO","where":"1=1"}`,
		"unknown severity":     `{"name":"ok","minimum_severity":"EXTREME"}`,
		"unknown signal type":  `{"name":"ok","signal_type":"RUMOUR"}`,
		"negative threshold":   `{"name":"ok","minimum_severity":"WATCH","probability_change_threshold":"-5"}`,
		"zero threshold":       `{"name":"ok","minimum_severity":"WATCH","probability_change_threshold":"0"}`,
		"nonnumeric threshold": `{"name":"ok","minimum_severity":"WATCH","probability_change_threshold":"lots"}`,
		"negative cooldown":    `{"name":"ok","cooldown_seconds":-1}`,
		"absurd cooldown":      `{"name":"ok","cooldown_seconds":999999999}`,
		"missing name":         `{"minimum_severity":"WATCH"}`,
		"empty name":           `{"name":"   "}`,
		"malformed json":       `{"name":`,
		"two documents":        validRuleBody + validRuleBody,
	}

	for label, body := range injections {
		t.Run(label, func(t *testing.T) {
			before := len(service.calls)
			response := do(t, handler, http.MethodPost, "/api/v1/alert-rules", body)
			if response.Code == http.StatusCreated {
				t.Fatalf("an executable/unknown payload was accepted: %s", response.Body.String())
			}
			if len(service.calls) != before {
				t.Fatalf("a rejected payload still reached the service: %v", service.calls[before:])
			}
		})
	}
}

// Strict decoding is the point: a client that sends a field the API does not
// implement must be told, not shown a success for a rule that silently lacks
// the condition it thought it set.
func TestAlertRuleUnknownFieldIsALoudError(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	response := do(t, handler, http.MethodPost, "/api/v1/alert-rules",
		`{"name":"ok","minimum_severity":"WATCH","severity":"CRITICAL"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown field, got %d: %s", response.Code, response.Body.String())
	}
	if !contains(response.Body.String(), "INVALID_RULE") {
		t.Fatalf("expected an INVALID_RULE code, got %s", response.Body.String())
	}
}

// --- scope exclusivity -----------------------------------------------------

// Scoping to both a watchlist and a market is ambiguous, not a merge. Guessing
// would produce a rule firing on a different set of markets than the user read
// off screen, so it is rejected.
func TestAlertRuleRejectsAmbiguousScope(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	response := do(t, handler, http.MethodPost, "/api/v1/alert-rules",
		`{"name":"both","watchlist_id":"`+testWatchlistID+`","market_id":"`+testMarketID+`"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
	if len(service.calls) != 0 {
		t.Fatal("an ambiguous rule reached the service")
	}
}

func TestAlertRuleValidatesScopeIdentifiersStructurally(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	// A non-UUID watchlist id and a non-base58 market id are both rejected
	// before any repository call, so no id shaped like an injection payload
	// can reach SQL.
	for label, body := range map[string]string{
		"bad watchlist": `{"name":"r","watchlist_id":"' OR 1=1 --"}`,
		"bad market":    `{"name":"r","market_id":"'; DELETE FROM markets; --"}`,
	} {
		t.Run(label, func(t *testing.T) {
			response := do(t, handler, http.MethodPost, "/api/v1/alert-rules", body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
	if len(service.calls) != 0 {
		t.Fatalf("a malformed scope identifier reached the service: %v", service.calls)
	}
}

// --- ownership --------------------------------------------------------------

func TestAlertRoutesScopeEveryCallToTheResolvedUser(t *testing.T) {
	withTestUser(t, "user-7")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	cases := []struct{ method, target, body string }{
		{http.MethodGet, "/api/v1/alert-rules", ""},
		{http.MethodPost, "/api/v1/alert-rules", validRuleBody},
		{http.MethodGet, "/api/v1/alert-rules/events", ""},
		{http.MethodGet, "/api/v1/alert-rules/" + testRuleID, ""},
		{http.MethodPatch, "/api/v1/alert-rules/" + testRuleID, validRuleBody},
		{http.MethodDelete, "/api/v1/alert-rules/" + testRuleID, ""},
		{http.MethodGet, "/api/v1/notifications", ""},
		{http.MethodGet, "/api/v1/notifications/unread-count", ""},
		{http.MethodPost, "/api/v1/notifications/1/read", ""},
		{http.MethodPost, "/api/v1/notifications/read-all", ""},
	}
	for _, testCase := range cases {
		response := do(t, handler, testCase.method, testCase.target, testCase.body)
		if response.Code == http.StatusUnauthorized {
			t.Fatalf("%s %s unexpectedly rejected", testCase.method, testCase.target)
		}
	}
	if len(service.calls) != len(cases) {
		t.Fatalf("expected %d calls, got %d: %v", len(cases), len(service.calls), service.calls)
	}
	for _, call := range service.calls {
		if !contains(call, ":user-7") {
			t.Fatalf("call %q was not scoped to user-7", call)
		}
	}
}

func TestAlertRoutesRequireAnIdentity(t *testing.T) {
	withTestUser(t, "")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	for _, target := range []string{"/api/v1/alert-rules", "/api/v1/notifications", "/api/v1/notifications/unread-count"} {
		response := do(t, handler, http.MethodGet, target, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", target, response.Code)
		}
	}
	if len(service.calls) != 0 {
		t.Fatal("a request without an identity reached the service")
	}
}

// --- behaviour --------------------------------------------------------------

func TestAlertRuleDefaultsToEnabled(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	// A rule is created enabled unless the client says otherwise: disabling is
	// an explicit act, not the default.
	response := do(t, handler, http.MethodPost, "/api/v1/alert-rules", validRuleBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	if !contains(response.Body.String(), `"enabled":true`) {
		t.Fatalf("a rule was not enabled by default: %s", response.Body.String())
	}
}

func TestAlertRuleCanBeCreatedDisabled(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	response := do(t, handler, http.MethodPost, "/api/v1/alert-rules",
		`{"name":"Later","minimum_severity":"WATCH","enabled":false}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	if !contains(response.Body.String(), `"enabled":false`) {
		t.Fatalf("enabled:false was not honoured: %s", response.Body.String())
	}
}

func TestAlertRuleCooldownIsValidated(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	accepted := do(t, handler, http.MethodPost, "/api/v1/alert-rules",
		`{"name":"ok","minimum_severity":"WATCH","cooldown_seconds":1800}`)
	if accepted.Code != http.StatusCreated {
		t.Fatalf("a 30 minute cooldown should be accepted, got %d: %s", accepted.Code, accepted.Body.String())
	}
	if !contains(accepted.Body.String(), `"cooldown_seconds":1800`) {
		t.Fatalf("cooldown was not persisted: %s", accepted.Body.String())
	}
}

func TestAlertEventLimitIsBounded(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	// An unbounded history request must not be honoured verbatim.
	for _, raw := range []string{"0", "-1", "100000", "abc"} {
		response := do(t, handler, http.MethodGet, "/api/v1/alert-rules/events?limit="+raw, "")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("limit=%s: expected 400, got %d", raw, response.Code)
		}
	}
	if len(service.calls) != 0 {
		t.Fatal("an out-of-range limit reached the service")
	}

	accepted := do(t, handler, http.MethodGet, "/api/v1/alert-rules/events?limit=25", "")
	if accepted.Code != http.StatusOK {
		t.Fatalf("limit=25 should be accepted, got %d: %s", accepted.Code, accepted.Body.String())
	}
}

func TestAlertServiceErrorsAreSanitized(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{err: errors.New("pq: duplicate key value violates unique constraint alert_events_dedupe_unique")}
	handler := newAlertHandler(t, service)

	for _, target := range []string{"/api/v1/alert-rules", "/api/v1/notifications"} {
		response := do(t, handler, http.MethodGet, target, "")
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s: expected 500, got %d", target, response.Code)
		}
		for _, leak := range []string{"pq:", "alert_events", "unique constraint"} {
			if contains(response.Body.String(), leak) {
				t.Fatalf("%s leaked %q: %s", target, leak, response.Body.String())
			}
		}
	}
}

func TestNilAlertServiceRegistersNoRoutes(t *testing.T) {
	handler := NewHandlerWithEnterprise(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	for _, target := range []string{"/api/v1/alert-rules", "/api/v1/alert-rules/events", "/api/v1/notifications"} {
		response := do(t, handler, http.MethodGet, target, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s: a nil service mounted a working route (%d)", target, response.Code)
		}
	}
}

func TestNotificationUnreadCountComesFromTheStore(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{notes: []intelligence.Notification{
		{ID: 1, Severity: "WATCH", Title: "a", Body: "a"},
		{ID: 2, Severity: "INFO", Title: "b", Body: "b"},
		{ID: 3, Severity: "CRITICAL", Title: "c", Body: "c"},
	}}
	handler := newAlertHandler(t, service)

	response := do(t, handler, http.MethodGet, "/api/v1/notifications/unread-count", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	// The badge must be the real stored count, never a client-side guess.
	if !contains(response.Body.String(), `"unread_count":3`) {
		t.Fatalf("unread count was not the stored value: %s", response.Body.String())
	}
}

func TestNotificationListReturnsAnArrayNotNull(t *testing.T) {
	withTestUser(t, "user-1")
	service := &recordingAlerts{}
	handler := newAlertHandler(t, service)

	response := do(t, handler, http.MethodGet, "/api/v1/notifications", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if contains(response.Body.String(), `"notifications":null`) {
		t.Fatalf("an empty inbox serialised as null, which the client must special-case: %s", response.Body.String())
	}
}
