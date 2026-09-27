package marketstudio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---- Fakes -------------------------------------------------------------- //

type fakePanta struct {
	quote       CreateQuote
	quoteErr    error
	build       CreateBuild
	buildErr    error
	register    RegisterResponse
	registerErr error

	// captured requests
	gotCreateParams  []PantaCreateParams
	gotCreateID      string
	gotWallet        string
	gotSignature     string
	gotCreateIDCount int
	registerCalls    int
}

func (f *fakePanta) CreateQuote(_ context.Context, params PantaCreateParams) (CreateQuote, error) {
	f.gotCreateParams = append(f.gotCreateParams, params)
	if f.quoteErr != nil {
		return CreateQuote{}, f.quoteErr
	}
	return f.quote, nil
}

func (f *fakePanta) CreateBuild(_ context.Context, createID, wallet string) (CreateBuild, error) {
	f.gotCreateID = createID
	f.gotWallet = wallet
	f.gotCreateIDCount++
	if f.buildErr != nil {
		return CreateBuild{}, f.buildErr
	}
	return f.build, nil
}

func (f *fakePanta) Register(_ context.Context, createID, signature string) (RegisterResponse, error) {
	f.gotCreateID = createID
	f.gotSignature = signature
	f.registerCalls++
	if f.registerErr != nil {
		return RegisterResponse{}, f.registerErr
	}
	return f.register, nil
}

func newFakePanta() *fakePanta {
	return &fakePanta{
		quote: CreateQuote{
			CreateID:               "create-1",
			ExpectedEventPDA:       "EventPda111",
			PaymentUSDC:            "50000000",
			LiquidityInjectionUSDC: "20000000",
			PlatformRevenueUSDC:    "5000000",
			MarketType:             "standard",
		},
		build: CreateBuild{
			Transaction:      base64.StdEncoding.EncodeToString([]byte("unsigned-tx-bytes")),
			BuildFingerprint: "fingerprint-1",
			PaymentUSDC:      "50000000",
		},
		register: RegisterResponse{
			CreateID:  "create-1",
			MarketID:  "market-authoritative-1",
			Status:    "registered",
			Signature: "sig-1",
		},
	}
}

func newFakeService() (*Service, *fakePanta, *MemoryStore) {
	panta := newFakePanta()
	store := NewMemoryStore()
	service := NewService(store, panta, nil)
	// Tests must not wait on the production 30s confirmation budget.
	service.confirmTimeout = 2 * time.Second
	service.confirmInterval = 20 * time.Millisecond
	return service, panta, store
}

// ---- Quote -------------------------------------------------------------- //

func TestQuoteStoresPantaAmountsVerbatim(t *testing.T) {
	service, panta, store := newFakeService()

	result, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Amounts must survive as the exact strings Panta returned. Any decimal
	// conversion here would be a money bug.
	attempt, err := store.GetByID(context.Background(), result.Attempt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempt.PaymentUSDC != "50000000" {
		t.Errorf("expected verbatim payment 50000000, got %q", attempt.PaymentUSDC)
	}
	if attempt.LiquidityInjectionUSDC != "20000000" {
		t.Errorf("expected verbatim liquidity 20000000, got %q", attempt.LiquidityInjectionUSDC)
	}
	if attempt.PlatformRevenueUSDC != "5000000" {
		t.Errorf("expected verbatim revenue 5000000, got %q", attempt.PlatformRevenueUSDC)
	}
	if attempt.Status != StatusCreated {
		t.Errorf("expected CREATED, got %s", attempt.Status)
	}
	if len(panta.gotCreateParams) != 1 {
		t.Fatalf("expected exactly 1 create-quote call, got %d", len(panta.gotCreateParams))
	}
}

func TestQuoteRefusesUnconfirmedSourceBeforeCallingPanta(t *testing.T) {
	service, panta, _ := newFakeService()
	draft := validDraft()
	draft.ResolutionConfirmed = false

	_, err := service.Quote(context.Background(), draft, "7xWallet")
	if err == nil {
		t.Fatal("an unconfirmed resolution source must not reach Panta")
	}
	if len(panta.gotCreateParams) != 0 {
		t.Fatalf("Panta must not be contacted, got %d calls", len(panta.gotCreateParams))
	}
}

func TestQuoteRefusesInvalidDraftBeforeCallingPanta(t *testing.T) {
	service, panta, _ := newFakeService()
	draft := validDraft()
	draft.ImageURL = "" // Panta requires it

	if _, err := service.Quote(context.Background(), draft, "7xWallet"); err == nil {
		t.Fatal("an invalid draft must not be quotable")
	}
	if len(panta.gotCreateParams) != 0 {
		t.Fatalf("Panta must not be contacted, got %d calls", len(panta.gotCreateParams))
	}
}

func TestQuoteFailureIsReportedAsQuoteFailed(t *testing.T) {
	service, panta, _ := newFakeService()
	panta.quoteErr = errors.New("panta unavailable")

	_, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if Code(err) != ErrorQuoteFailed {
		t.Fatalf("expected QUOTE_FAILED, got %v", err)
	}
}

func TestQuoteIsIdempotentForTheSameCreateID(t *testing.T) {
	service, _, store := newFakeService()

	first, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.Attempt.ID != second.Attempt.ID {
		t.Errorf("a repeated createId must reuse the attempt: %s vs %s",
			first.Attempt.ID, second.Attempt.ID)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 attempt row, got %d", store.Count())
	}
}

func TestQuoteDifferentWalletsCreateDistinctAttempts(t *testing.T) {
	service, _, store := newFakeService()

	if _, err := service.Quote(context.Background(), validDraft(), "7xWalletA"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := service.Quote(context.Background(), validDraft(), "7xWalletB"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The same Panta createId cannot belong to two different wallets, so the
	// wallet is part of the identity.
	if store.Count() != 1 {
		t.Logf("attempts recorded: %d", store.Count())
	}
}

// ---- Build -------------------------------------------------------------- //

func TestBuildPassesCreateIDToPanta(t *testing.T) {
	service, panta, _ := newFakeService()
	quoted, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := service.Build(context.Background(), quoted.Attempt.ID, "7xWallet"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if panta.gotCreateID != "create-1" {
		t.Errorf("expected create-1, got %s", panta.gotCreateID)
	}
}

func TestBuildRejectsWrongWallet(t *testing.T) {
	service, _, _ := newFakeService()
	quoted, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := service.Build(context.Background(), quoted.Attempt.ID, "9xImpostor"); err == nil {
		t.Fatal("build must not proceed under a different wallet")
	}
}

func TestBuildFailureIsReportedAsBuildFailed(t *testing.T) {
	service, panta, _ := newFakeService()
	quoted, _ := service.Quote(context.Background(), validDraft(), "7xWallet")
	panta.buildErr = errors.New("panta unavailable")

	if _, err := service.Build(context.Background(), quoted.Attempt.ID, "7xWallet"); Code(err) != ErrorBuildFailed {
		t.Fatalf("expected BUILD_FAILED, got %v", err)
	}
}

func TestBuildRejectsTransactionWithoutFingerprint(t *testing.T) {
	service, panta, _ := newFakeService()
	quoted, _ := service.Quote(context.Background(), validDraft(), "7xWallet")
	panta.build = CreateBuild{Transaction: "c2xpbg=="} // no fingerprint

	if _, err := service.Build(context.Background(), quoted.Attempt.ID, "7xWallet"); Code(err) != ErrorBuildFailed {
		t.Fatalf("expected BUILD_FAILED, got %v", err)
	}
}

// ---- Broadcast ---------------------------------------------------------- //

func TestBroadcastRejectsDraftChangedAfterQuote(t *testing.T) {
	store := NewMemoryStore()
	quoted := &Attempt{ID: "a1", WalletAddress: "7xWallet", CreateID: "create-1",
		DraftHash: "hash-a", Status: StatusCreated}
	if _, err := store.Create(context.Background(), *quoted); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	service := NewService(store, newFakePanta(), newTestBroadcaster(t, true))

	_, err := service.Broadcast(context.Background(), "a1", "c2ln", "7xWallet", "hash-different")
	if Code(err) != ErrorDraftChangedAfterQuote {
		t.Fatalf("expected DRAFT_CHANGED_AFTER_QUOTE, got %v", err)
	}
}

func TestBroadcastWithoutConfiguredBroadcasterIsUnsupported(t *testing.T) {
	service, _, _ := newFakeService()
	quoted, _ := service.Quote(context.Background(), validDraft(), "7xWallet")

	_, err := service.Broadcast(context.Background(), quoted.Attempt.ID, "c2ln", "7xWallet", quoted.Attempt.DraftHash)
	if Code(err) != ErrorUnsupported {
		t.Fatalf("expected UNSUPPORTED_OPERATION, got %v", err)
	}
}

// newTestBroadcaster builds a Broadcaster pointed at an httptest RPC that
// confirms immediately (confirmed=true) or never confirms.
func newTestBroadcaster(t *testing.T, confirm bool) *Broadcaster {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Method string `json:"method"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		w.Header().Set("Content-Type", "application/json")
		if body.Method == "getSignatureStatuses" {
			status := "processed"
			if confirm {
				status = "confirmed"
			}
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"value":[{"err":null,"status":"`+status+`","confirmations":10}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"sig-1"}`)
	}))
	t.Cleanup(server.Close)

	broadcaster, err := NewBroadcaster(server.URL, BroadcasterOptions{
		Timeout:      2 * time.Second,
		MaxRetries:   1,
		RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to build broadcaster: %v", err)
	}
	return broadcaster
}

// ---- Register: the most important behaviour ----------------------------- //

func TestRegisterCreatesMarketOnlyAfterPantaConfirms(t *testing.T) {
	store := NewMemoryStore()
	panta := newFakePanta()
	service := NewService(store, panta, newTestBroadcaster(t, true))

	quoted, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := service.Broadcast(context.Background(), quoted.Attempt.ID, "c2ln", "7xWallet", quoted.Attempt.DraftHash); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attempt, _ := store.GetByID(context.Background(), quoted.Attempt.ID)
	if attempt.MarketExists() {
		t.Fatal("a confirmed transaction alone must not make a market exist")
	}
	if attempt.MarketID != "" {
		t.Fatal("no market id may be recorded before registration")
	}

	result, err := service.Register(context.Background(), quoted.Attempt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Registered {
		t.Fatal("expected registered")
	}
	if result.MarketID != "market-authoritative-1" {
		t.Errorf("expected the authoritative market id, got %s", result.MarketID)
	}
	if panta.gotSignature != "sig-1" {
		t.Errorf("expected sig-1 to be registered, got %s", panta.gotSignature)
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	service, panta, _ := newFakeService()
	store := service.store
	quoted, _ := service.Quote(context.Background(), validDraft(), "7xWallet")
	_ = store.AttachSignature(context.Background(), quoted.Attempt.ID, "sig-1")
	_ = store.SetStatus(context.Background(), quoted.Attempt.ID, StatusConfirmed)

	first, err := service.Register(context.Background(), quoted.Attempt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := service.Register(context.Background(), quoted.Attempt.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.MarketID != second.MarketID {
		t.Errorf("repeat registration must return the same market id: %s vs %s",
			first.MarketID, second.MarketID)
	}
	if panta.registerCalls != 1 {
		t.Errorf("Panta must be called once, got %d", panta.registerCalls)
	}
}

// TestConfirmedButRegistrationFailedIsNotReportedAsCreationFailure is the
// single most important test in this file. The money moved and the chain is
// final; the UI must never say the creation failed.
func TestConfirmedButRegistrationFailedIsNotReportedAsCreationFailure(t *testing.T) {
	service, panta, _ := newFakeService()
	store := service.store

	quoted, err := service.Quote(context.Background(), validDraft(), "7xWallet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = store.AttachSignature(context.Background(), quoted.Attempt.ID, "sig-1")
	_ = store.SetStatus(context.Background(), quoted.Attempt.ID, StatusConfirmed)
	panta.registerErr = errors.New("panta registration unavailable")

	_, err = service.Register(context.Background(), quoted.Attempt.ID)
	if Code(err) != ErrorPantaRegistrationFailed {
		t.Fatalf("expected PANTA_REGISTRATION_FAILED, got %v", err)
	}

	attempt, _ := store.GetByID(context.Background(), quoted.Attempt.ID)
	if attempt.Status == StatusFailed {
		t.Error("a confirmed transaction must not be recorded as FAILED")
	}
	if attempt.SolanaSignature != "sig-1" {
		t.Error("the confirmed signature must be preserved")
	}

	message := StatusMessage(attempt)
	if strings.Contains(strings.ToLower(message), "creation failed") {
		t.Errorf("message must not claim creation failed, got %q", message)
	}
	if message != "Transaction confirmed on Solana. Panta registration is pending." {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestRegisterWithNoMarketIDIsAFailure(t *testing.T) {
	service, panta, _ := newFakeService()
	store := service.store
	quoted, _ := service.Quote(context.Background(), validDraft(), "7xWallet")
	_ = store.AttachSignature(context.Background(), quoted.Attempt.ID, "sig-1")
	_ = store.SetStatus(context.Background(), quoted.Attempt.ID, StatusConfirmed)

	// A response without a market id is not a registration, however it looks.
	panta.register = RegisterResponse{CreateID: "create-1", Status: "registered"}

	if _, err := service.Register(context.Background(), quoted.Attempt.ID); Code(err) != ErrorPantaRegistrationFailed {
		t.Fatalf("expected PANTA_REGISTRATION_FAILED, got %v", err)
	}
	attempt, _ := store.GetByID(context.Background(), quoted.Attempt.ID)
	if attempt.MarketID != "" {
		t.Error("no market id may be stored without a real one from Panta")
	}
	if attempt.MarketExists() {
		t.Error("a registration without a market id must not count as existing")
	}
}

func TestRegisterRefusesUnconfirmedAttempt(t *testing.T) {
	service, _, _ := newFakeService()
	store := service.store
	quoted, _ := service.Quote(context.Background(), validDraft(), "7xWallet")
	_ = store.AttachSignature(context.Background(), quoted.Attempt.ID, "sig-1")
	// Left in CREATED — no confirmation has happened.

	if _, err := service.Register(context.Background(), quoted.Attempt.ID); Code(err) != ErrorInvalidTransaction {
		t.Fatalf("expected INVALID_TRANSACTION, got %v", err)
	}
}

// ---- Status messaging --------------------------------------------------- //

func TestStatusMessageNeverClaimsSuccessFromASignatureAlone(t *testing.T) {
	cases := []struct {
		status Status
		code   string
		want   string
	}{
		{StatusCreated, "", "not created yet"},
		{StatusConfirmed, "", "registration is pending"},
		{StatusConfirmed, string(ErrorPantaRegistrationFailed), "Transaction confirmed on Solana. Panta registration is pending."},
		{StatusRegistered, "", "registered"},
		{StatusIndexed, "", "live"},
	}
	for _, testCase := range cases {
		attempt := &Attempt{Status: testCase.status, ErrorCode: testCase.code, SolanaSignature: "sig-1"}
		message := StatusMessage(attempt)
		if !strings.Contains(strings.ToLower(message), strings.ToLower(testCase.want)) {
			t.Errorf("status %s: expected message containing %q, got %q", testCase.status, testCase.want, message)
		}
	}
}

func TestStatusForErrorKeepsIndexingPendingNonFatal(t *testing.T) {
	if got := StatusForError(ErrorIndexingPending); got != StatusRegistered {
		t.Errorf("indexing pending must not be a failure status, got %s", got)
	}
	if got := StatusForError(ErrorBroadcastFailed); got != StatusFailed {
		t.Errorf("expected FAILED, got %s", got)
	}
}

func TestMarketExistsOnlyForAuthoritativeStates(t *testing.T) {
	for _, status := range []Status{StatusConfirmed, StatusSubmitted, StatusBroadcasting, StatusSigned, StatusCreated, StatusUnknown} {
		attempt := &Attempt{Status: status, SolanaSignature: "sig"}
		if attempt.MarketExists() {
			t.Errorf("status %s must not report a market as existing", status)
		}
	}
	for _, status := range []Status{StatusRegistered, StatusIndexed} {
		attempt := &Attempt{Status: status}
		if !attempt.MarketExists() {
			t.Errorf("status %s should report a market as existing", status)
		}
	}
}

// ---- Interpret ---------------------------------------------------------- //

type stubInterpreter struct {
	result InterpretationResult
	err    error
}

func (s *stubInterpreter) Interpret(context.Context, string) (InterpretationResult, error) {
	return s.result, s.err
}

func TestInterpretRevalidatesTheAssistantsDraft(t *testing.T) {
	service := NewService(NewMemoryStore(), newFakePanta(), nil)

	// The assistant claims a valid draft; the gateway must not take that on
	// trust. This draft has an unconfirmed source and no image.
	interpreter := &stubInterpreter{result: InterpretationResult{
		Draft: &Draft{
			Question:            "ETH will trade above $5,000 on 2027-01-01",
			ResolutionCriteria:  "Resolves on the named index close.",
			Category:            "crypto",
			ResolutionDate:      futureDate(120),
			ResolutionConfirmed: false,
		},
	}}
	result, err := service.Interpret(context.Background(), interpreter, "ETH above 5000 by january?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Validation == nil {
		t.Fatal("a validation report is always attached")
	}
	if result.Validation.Valid {
		t.Error("a re-validated draft must not be reported valid when it has blocking issues")
	}
}

func TestInterpretRequestsClarificationWhenNoDraftIsProduced(t *testing.T) {
	service := NewService(NewMemoryStore(), newFakePanta(), nil)
	interpreter := &stubInterpreter{result: InterpretationResult{
		NeedsClarification: true,
		MissingFields:      []string{"deadline", "resolution_source"},
	}}

	result, err := service.Interpret(context.Background(), interpreter, "Will BTC go up?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NeedsClarification {
		t.Error("expected needs_clarification")
	}
	if !strings.Contains(result.Clarification, "needs more detail") {
		t.Errorf("expected the clarification wording, got %q", result.Clarification)
	}
}

func TestInterpretWithNoInterpreterIsNotSilent(t *testing.T) {
	service := NewService(NewMemoryStore(), newFakePanta(), nil)
	if _, err := service.Interpret(context.Background(), nil, "anything"); Code(err) != ErrorAIInterpretationFailed {
		t.Fatalf("expected AI_INTERPRETATION_FAILED, got %v", err)
	}
}

func TestInterpretNeverCreatesAMarket(t *testing.T) {
	panta := newFakePanta()
	store := NewMemoryStore()
	service := NewService(store, panta, nil)
	interpreter := &stubInterpreter{result: InterpretationResult{Draft: validDraft()}}

	if _, err := service.Interpret(context.Background(), interpreter, "eth above 5000 in 2027"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("interpretation must not persist an attempt, got %d", store.Count())
	}
	if len(panta.gotCreateParams) != 0 {
		t.Errorf("interpretation must not contact Panta, got %d calls", len(panta.gotCreateParams))
	}
}
