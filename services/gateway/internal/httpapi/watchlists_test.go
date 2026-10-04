package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qevryn/gateway/internal/intelligence"
)

// --- identifiers ------------------------------------------------------------

// The handlers validate path identifiers before touching the service: a
// watchlist id must be a UUID and a market id must be a base58 address. These
// are the real formats, so a test that uses "wl-1" would be asserting against a
// request that is rejected for the wrong reason.
const (
	testWatchlistID  = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	testWatchlistID2 = "9c858901-8a57-4791-81fe-4c455b099bc9"
	// A syntactically valid 32-byte base58 address. This is a test fixture, not
	// a real market: no test here ever contacts Panta or the chain.
	testMarketID = "11111111111111111111111111111112"
	// Alert rules are addressed by UUID too.
	testRuleID = "b8e6a1f2-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
)

// recordingWatchlists captures the user id every call was scoped to, so a test
// can assert ownership was actually applied rather than assumed.
type recordingWatchlists struct {
	calls []watchlistCall
	err   error
}

type watchlistCall struct {
	method    string
	userID    string
	watchlist string
	market    string
	name      string
	desc      *string
}

func (r *recordingWatchlists) record(method, userID, watchlist, market string) {
	r.calls = append(r.calls, watchlistCall{method: method, userID: userID, watchlist: watchlist, market: market})
}

func (r *recordingWatchlists) ListWatchlists(_ context.Context, userID string) ([]intelligence.WatchlistSummary, error) {
	r.record("list", userID, "", "")
	return nil, r.err
}

func (r *recordingWatchlists) GetWatchlist(_ context.Context, userID, id string) (*intelligence.Watchlist, error) {
	r.record("get", userID, id, "")
	return &intelligence.Watchlist{ID: id, UserID: userID, Name: "list"}, r.err
}

func (r *recordingWatchlists) CreateWatchlist(_ context.Context, userID, name string, description *string) (*intelligence.Watchlist, error) {
	r.record("create", userID, "", "")
	r.calls[len(r.calls)-1].name = name
	r.calls[len(r.calls)-1].desc = description
	return &intelligence.Watchlist{ID: "wl-1", UserID: userID, Name: name, Description: description}, r.err
}

func (r *recordingWatchlists) UpdateWatchlist(_ context.Context, userID, id, name string, description *string) (*intelligence.Watchlist, error) {
	r.record("update", userID, id, "")
	r.calls[len(r.calls)-1].name = name
	return &intelligence.Watchlist{ID: id, UserID: userID, Name: name, Description: description}, r.err
}

func (r *recordingWatchlists) DeleteWatchlist(_ context.Context, userID, id string) error {
	r.record("delete", userID, id, "")
	return r.err
}

func (r *recordingWatchlists) AddMarket(_ context.Context, userID, id, marketID string) error {
	r.record("add-market", userID, id, marketID)
	return r.err
}

func (r *recordingWatchlists) RemoveMarket(_ context.Context, userID, id, marketID string) error {
	r.record("remove-market", userID, id, marketID)
	return r.err
}

func (r *recordingWatchlists) WatchlistIntelligence(_ context.Context, userID, id string) (*intelligence.WatchlistIntelligence, error) {
	r.record("intelligence", userID, id, "")
	return &intelligence.WatchlistIntelligence{Watchlist: intelligence.Watchlist{ID: id, Name: "list"}}, r.err
}

// fixedResolver returns a constant identity, standing in for a real auth provider.
type fixedResolver struct {
	userID string
	err    error
}

func (f fixedResolver) ResolveUser(*http.Request) (string, error) {
	return f.userID, f.err
}

func newWatchlistHandler(t *testing.T, service watchlistService) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterWatchlistRoutes(mux, service, nil)
	RegisterRadarRoutes(mux, nil)
	return withRequestID(mux)
}

func do(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// --- ownership --------------------------------------------------------------

// A deployment that installs a real resolver must have every user-scoped
// call scoped to that identity, never to a caller-supplied id.
func TestWatchlistRoutesScopeEveryCallToTheResolvedUser(t *testing.T) {
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })
	SetUserResolver(fixedResolver{userID: "user-42"})

	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	cases := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodGet, "/api/v1/watchlists", ""},
		{http.MethodPost, "/api/v1/watchlists", `{"name":"Elections"}`},
		{http.MethodGet, "/api/v1/watchlists/" + testWatchlistID, ""},
		{http.MethodPatch, "/api/v1/watchlists/" + testWatchlistID, `{"name":"Renamed"}`},
		{http.MethodDelete, "/api/v1/watchlists/" + testWatchlistID, ""},
		{http.MethodPost, "/api/v1/watchlists/" + testWatchlistID + "/markets/" + testMarketID, ""},
		{http.MethodDelete, "/api/v1/watchlists/" + testWatchlistID + "/markets/" + testMarketID, ""},
		{http.MethodGet, "/api/v1/watchlists/" + testWatchlistID + "/intelligence", ""},
	}
	for _, testCase := range cases {
		response := do(t, handler, testCase.method, testCase.target, testCase.body)
		if response.Code == http.StatusUnauthorized {
			t.Fatalf("%s %s unexpectedly rejected", testCase.method, testCase.target)
		}
	}

	if len(service.calls) != len(cases) {
		t.Fatalf("expected %d service calls, got %d", len(cases), len(service.calls))
	}
	for i, call := range service.calls {
		if call.userID != "user-42" {
			t.Fatalf("call %d (%s) was scoped to %q, want user-42", i, call.method, call.userID)
		}
	}
}

// The most important security property: a user_id in the body must be ignored,
// never trusted. If it were honoured, the repository's ownership filter would be
// decorative and any client could read or mutate another user's watchlists.
func TestWatchlistRoutesIgnoreCallerSuppliedUserID(t *testing.T) {
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })
	SetUserResolver(fixedResolver{userID: "real-user"})

	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	// user_id is deliberately sent at the top level, and nested in a plausible
	// spot, to prove neither can widen the write beyond the acting user.
	response := do(t, handler, http.MethodPost, "/api/v1/watchlists",
		`{"name":"Mine","user_id":"victim","owner_id":"victim","userId":"victim"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	if len(service.calls) != 1 {
		t.Fatalf("expected exactly one call, got %d", len(service.calls))
	}
	if got := service.calls[0].userID; got != "real-user" {
		t.Fatalf("write was scoped to %q, want the resolved identity real-user", got)
	}
	if contains(response.Body.String(), "victim") {
		t.Fatalf("response echoed a caller-supplied identity: %s", response.Body.String())
	}
}

// An empty resolved id is a failure, not an anonymous caller. Otherwise a
// misbehaving resolver degrades into an empty-owner query that could match or
// expose rows belonging to nobody.
func TestWatchlistRoutesRejectAnEmptyResolvedIdentity(t *testing.T) {
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })

	for name, resolver := range map[string]UserResolver{
		"empty id":     fixedResolver{userID: ""},
		"whitespace":   fixedResolver{userID: "   "},
		"resolver err": fixedResolver{err: errors.New("token invalid")},
	} {
		t.Run(name, func(t *testing.T) {
			SetUserResolver(resolver)
			service := &recordingWatchlists{}
			handler := newWatchlistHandler(t, service)

			response := do(t, handler, http.MethodGet, "/api/v1/watchlists", "")
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", response.Code, response.Body.String())
			}
			if len(service.calls) != 0 {
				t.Fatalf("service was reached %d times without an identity", len(service.calls))
			}
		})
	}
}

// --- validation -------------------------------------------------------------

func TestWatchlistCreateValidatesTheName(t *testing.T) {
	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	for name, body := range map[string]string{
		"missing name": `{}`,
		"empty name":   `{"name":""}`,
		"whitespace":   `{"name":"   "}`,
		"too long":     `{"name":"` + strings.Repeat("a", 121) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			before := len(service.calls)
			response := do(t, handler, http.MethodPost, "/api/v1/watchlists", body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}
			if len(service.calls) != before {
				t.Fatal("an invalid name reached the service")
			}
		})
	}
}

func TestWatchlistNameBoundaryIsInclusive(t *testing.T) {
	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	// 120 runes is the documented maximum and must be accepted; 121 must not.
	accepted := do(t, handler, http.MethodPost, "/api/v1/watchlists",
		`{"name":"`+strings.Repeat("a", 120)+`"}`)
	if accepted.Code != http.StatusCreated {
		t.Fatalf("120 runes should be accepted, got %d: %s", accepted.Code, accepted.Body.String())
	}

	rejected := do(t, handler, http.MethodPost, "/api/v1/watchlists",
		`{"name":"`+strings.Repeat("a", 121)+`"}`)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("121 runes should be rejected, got %d", rejected.Code)
	}
}

// A multi-byte name must be measured in runes, not bytes. A 40-character name
// in a 3-byte script is well within the limit and must not be rejected as if it
// were 120 bytes.
func TestWatchlistNameIsMeasuredInRunes(t *testing.T) {
	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	// Each of these is well under 120 runes but over 120 bytes.
	name := strings.Repeat("\u4e2d", 60) // 60 runes, 180 bytes
	if len(name) <= 120 {
		t.Fatalf("test precondition failed: name is only %d bytes", len(name))
	}
	response := do(t, handler, http.MethodPost, "/api/v1/watchlists",
		`{"name":"`+name+`"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("a 60-rune name was rejected (%d bytes): %s", len(name), response.Body.String())
	}
}

func TestWatchlistDescriptionIsOptionalAndNullable(t *testing.T) {
	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	omitted := do(t, handler, http.MethodPost, "/api/v1/watchlists", `{"name":"No description"}`)
	if omitted.Code != http.StatusCreated {
		t.Fatalf("a missing description should be accepted, got %d", omitted.Code)
	}
	if service.calls[0].desc != nil {
		t.Fatal("an omitted description should arrive as nil, not an empty string")
	}

	explicitNull := do(t, handler, http.MethodPost, "/api/v1/watchlists", `{"name":"Nulled","description":null}`)
	if explicitNull.Code != http.StatusCreated {
		t.Fatalf("an explicit null description should be accepted, got %d", explicitNull.Code)
	}
	if service.calls[1].desc != nil {
		t.Fatal("an explicit null description should arrive as nil")
	}
}

// --- CRUD contract ----------------------------------------------------------

func TestWatchlistCRUDReturnsThePersistedShape(t *testing.T) {
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })
	SetUserResolver(fixedResolver{userID: "user-1"})

	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	created := do(t, handler, http.MethodPost, "/api/v1/watchlists", `{"name":"Crypto","description":"majors"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", created.Code, created.Body.String())
	}
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatalf("create response was not JSON: %v", err)
	}
	if body.ID != "wl-1" || body.Name != "Crypto" {
		t.Fatalf("unexpected create response: %s", created.Body.String())
	}

	deleted := do(t, handler, http.MethodDelete, "/api/v1/watchlists/"+testWatchlistID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", deleted.Code, deleted.Body.String())
	}
}

// A syntactically invalid id must be rejected before the service is reached.
// This is a validation failure, not a not-found, and it must not be reported as
// one: the distinction tells a client whether its request was malformed or
// whether the resource genuinely does not exist for them.
func TestWatchlistRejectsMalformedIdentifiersBeforeTheService(t *testing.T) {
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })
	SetUserResolver(fixedResolver{userID: "user-1"})

	service := &recordingWatchlists{}
	handler := newWatchlistHandler(t, service)

	badWatchlists := []string{
		"not-a-uuid",
		"3f2504e0-4f89-41d3-9a0c",              // too short
		"3f2504e04f8941d39a0c0305e82c3301000",  // no hyphens
		"zzzzzzzz-4f89-41d3-9a0c-0305e82c3301", // non-hex
	}
	for _, id := range badWatchlists {
		response := do(t, handler, http.MethodGet, "/api/v1/watchlists/"+id, "")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("watchlist id %q: expected 400, got %d", id, response.Code)
		}
	}
	if len(service.calls) != 0 {
		t.Fatalf("%d malformed requests reached the service", len(service.calls))
	}

	// A valid watchlist id with a market id that is not base58 must also fail.
	response := do(t, handler, http.MethodPost,
		"/api/v1/watchlists/"+testWatchlistID+"/markets/not-base58-0OIl", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-base58 market id, got %d: %s", response.Code, response.Body.String())
	}
	if len(service.calls) != 0 {
		t.Fatal("a malformed market id reached the service")
	}
}

func TestWatchlistServiceErrorsAreSanitized(t *testing.T) {
	original := currentResolver
	t.Cleanup(func() { SetUserResolver(original) })
	SetUserResolver(fixedResolver{userID: "user-1"})

	service := &recordingWatchlists{err: errors.New("pq: relation watchlists does not exist")}
	handler := newWatchlistHandler(t, service)

	response := do(t, handler, http.MethodGet, "/api/v1/watchlists", "")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.Code)
	}
	// The driver and schema text must not reach the client.
	for _, leak := range []string{"pq:", "relation watchlists", "does not exist"} {
		if contains(response.Body.String(), leak) {
			t.Fatalf("response leaked %q: %s", leak, response.Body.String())
		}
	}
}

func TestNilWatchlistServiceRegistersNoRoutes(t *testing.T) {
	handler := NewHandlerWithEnterprise(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Not 404: the route genuinely does not exist, so the mux reports its own
	// default rather than a handler returning "not found".
	response := do(t, handler, http.MethodGet, "/api/v1/watchlists", "")
	if response.Code == http.StatusOK {
		t.Fatal("a nil service must not mount working watchlist routes")
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected the mux 404, got %d", response.Code)
	}
}

func TestNewHandlerWithAllServicesStaysAWatchlistFreeSubset(t *testing.T) {
	handler := NewHandlerWithAllServices(nil, nil, nil, nil, nil)

	response := do(t, handler, http.MethodGet, "/api/v1/watchlists", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("the pre-TASK-008 constructor registered enterprise routes: %d", response.Code)
	}
	// The routes it does register must still work.
	health := do(t, handler, http.MethodGet, "/health", "")
	if health.Code != http.StatusOK {
		t.Fatalf("health regressed: %d", health.Code)
	}
}

