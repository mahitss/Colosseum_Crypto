package marketstudio

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Production confirmation budget. Solana settles creation transactions in a
// couple of blocks, so 30s is generous; exceeding it yields
// CONFIRMATION_TIMEOUT, which is explicitly *not* a failure claim.
const (
	defaultConfirmTimeout  = 30 * time.Second
	defaultConfirmInterval = 2 * time.Second
)

// Failure carries a stable error code so the UI can render the right message
// without parsing prose. This is what keeps "Solana confirmed but Panta
// registration failed" from being rendered as "Market creation failed".
type Failure struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (f *Failure) Error() string {
	if f.Err != nil {
		return fmt.Sprintf("%s: %s", f.Code, f.Err.Error())
	}
	return fmt.Sprintf("%s: %s", f.Code, f.Message)
}

func (f *Failure) Unwrap() error { return f.Err }

func failf(code ErrorCode, err error) *Failure {
	return &Failure{Code: code, Err: err}
}

func fail(code ErrorCode, message string) *Failure {
	return &Failure{Code: code, Message: message}
}

// Code extracts the error code from any error produced by this package.
func Code(err error) ErrorCode {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Code
	}
	return ""
}

// Service orchestrates draft → quote → build → sign → broadcast → register.
//
// It never signs and never claims success. Every success path below requires an
// authoritative response from Panta; a local signature is not sufficient.
type Service struct {
	store       Store
	panta       PantaClient
	broadcaster *Broadcaster

	// confirmTimeout bounds the wait for Solana confirmation and
	// confirmInterval is the poll cadence. They are fields rather than
	// constants so tests can shrink them; production uses the defaults.
	confirmTimeout  time.Duration
	confirmInterval time.Duration
}

// NewService wires the pipeline. Any component may be nil, in which case the
// corresponding step returns an explicit error rather than silently succeeding.
func NewService(store Store, panta PantaClient, broadcaster *Broadcaster) *Service {
	return &Service{
		store:           store,
		panta:           panta,
		broadcaster:     broadcaster,
		confirmTimeout:  defaultConfirmTimeout,
		confirmInterval: defaultConfirmInterval,
	}
}

// Interpret asks the assistant to structure a description.
//
// The returned draft is a proposal. It is validated here, and the caller must
// present the result to the user before any creation step can run.
func (s *Service) Interpret(ctx context.Context, interpreter Interpreter, description string) (InterpretationResult, error) {
	if interpreter == nil {
		return InterpretationResult{}, fail(ErrorAIInterpretationFailed, "market architect is not configured")
	}
	result, err := interpreter.Interpret(ctx, description)
	if err != nil {
		return InterpretationResult{}, failf(ErrorAIInterpretationFailed, err)
	}

	// Re-validate locally regardless of what the assistant already reported. The
	// gateway does not trust a remote "valid" flag.
	if result.Draft != nil {
		report := ValidateDraft(result.Draft)
		result.Validation = &report
		if len(result.MissingFields) == 0 {
			result.MissingFields = report.MissingFields
		}
	}
	if result.Draft == nil || result.NeedsClarification {
		result.NeedsClarification = true
		message, _ := SummarizeMissing(result.MissingFields)
		result.Clarification = message
	}
	return result, nil
}

// Validate reports on a draft. This is the only gate that can enable the
// Create Market action, and it never consults the model.
func (s *Service) Validate(draft *Draft) ValidationReport {
	return ValidateDraft(draft)
}

// QuoteRequest is a validated draft plus the wallet that will pay for it.
type QuoteResult struct {
	Attempt *Attempt         `json:"attempt"`
	Quote   CreateQuote      `json:"quote"`
	Report  ValidationReport `json:"validation"`
}

// Quote creates a Panta creation attempt and records it.
//
// Order matters: validation happens before Panta is contacted, so an invalid or
// unconfirmed draft never produces a Panta createId. The draft hash recorded
// here is what later invalidates the build if a material field changes.
func (s *Service) Quote(ctx context.Context, draft *Draft, wallet string) (QuoteResult, error) {
	if s.panta == nil {
		return QuoteResult{}, fail(ErrorUnsupported, "market creation is not configured")
	}
	if err := ValidateQuoteRequest(draft, wallet); err != nil {
		return QuoteResult{}, failf(ErrorInvalidDraft, err)
	}
	if !draft.ResolutionConfirmed {
		// Defence in depth: ValidateQuoteRequest already covers this, but the
		// spec is explicit that an unconfirmed source must never reach Panta.
		return QuoteResult{}, fail(ErrorAmbiguousMarket, ErrResolutionSourceNeeded.Error())
	}

	params, err := BuildPantaCreateParams(draft, wallet)
	if err != nil {
		return QuoteResult{}, failf(ErrorInvalidDraft, err)
	}

	quote, err := s.panta.CreateQuote(ctx, params)
	if err != nil {
		return QuoteResult{}, failf(ErrorQuoteFailed, err)
	}

	// Idempotency: the same createId already has an attempt, so this is a retry
	// of the same logical creation, not a new one.
	if existing, lookupErr := s.store.GetByCreateID(ctx, quote.CreateID); lookupErr == nil && existing != nil {
		return QuoteResult{Attempt: existing, Quote: quote, Report: ValidateDraft(draft)}, nil
	}

	attempt := Attempt{
		WalletAddress: strings.TrimSpace(wallet),
		CreateID:      quote.CreateID,
		DraftHash:     DraftFingerprint(draft),
		DraftPayload:  draft,
		Status:        StatusCreated,
		MarketType:    quote.MarketType,
	}
	id, err := s.store.Create(ctx, attempt)
	if err != nil {
		return QuoteResult{}, failf(ErrorQuoteFailed, err)
	}
	if err := s.store.AttachQuote(ctx, id, quote); err != nil {
		return QuoteResult{}, failf(ErrorQuoteFailed, err)
	}
	created, err := s.store.GetByID(ctx, id)
	if err != nil {
		return QuoteResult{}, failf(ErrorQuoteFailed, err)
	}
	return QuoteResult{Attempt: created, Quote: quote, Report: ValidateDraft(draft)}, nil
}

// BuildResult carries the transaction the user must sign, plus Panta's exact
// fee amounts. There is no locally estimated fee anywhere in this struct.
type BuildResult struct {
	Attempt *Attempt    `json:"attempt"`
	Build   CreateBuild `json:"build"`
}

// Build asks Panta for the unsigned transaction for a recorded attempt.
func (s *Service) Build(ctx context.Context, attemptID, wallet string) (BuildResult, error) {
	if s.panta == nil {
		return BuildResult{}, fail(ErrorUnsupported, "market creation is not configured")
	}
	attempt, err := s.store.GetByID(ctx, attemptID)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			return BuildResult{}, failf(ErrorInvalidDraft, ErrAttemptNotFound)
		}
		return BuildResult{}, failf(ErrorBuildFailed, err)
	}
	if err := ValidateBuildRequest(attempt, wallet); err != nil {
		return BuildResult{}, failf(ErrorBuildFailed, err)
	}

	build, err := s.panta.CreateBuild(ctx, attempt.CreateID, wallet)
	if err != nil {
		return BuildResult{}, failf(ErrorBuildFailed, err)
	}
	if build.BuildFingerprint == "" {
		return BuildResult{}, fail(ErrorBuildFailed, "panta build returned no fingerprint")
	}

	// A rebuilt transaction supersedes any prior fingerprint. Storing it lets a
	// later request prove it is acting on the transaction the user signed.
	if attempt.BuildFingerprint != build.BuildFingerprint {
		if err := s.store.AttachBuild(ctx, attempt.ID, build.BuildFingerprint); err != nil {
			return BuildResult{}, failf(ErrorBuildFailed, err)
		}
	}
	refreshed, err := s.store.GetByID(ctx, attempt.ID)
	if err == nil {
		attempt = refreshed
	}
	return BuildResult{Attempt: attempt, Build: build}, nil
}

// BroadcastResult pairs the persisted attempt with the RPC outcome.
type BroadcastOutcome struct {
	Attempt *Attempt        `json:"attempt"`
	Result  BroadcastResult `json:"result"`
}

// Broadcast relays the wallet-signed transaction and waits for confirmation.
//
// The signed payload is used here and then dropped. It is never stored, and no
// key material ever reaches this server. A confirmed signature moves the
// attempt to CONFIRMED, not to success: registration is a separate step that
// must still succeed before any market exists.
func (s *Service) Broadcast(ctx context.Context, attemptID, signedTx, wallet, draftHash string) (BroadcastOutcome, error) {
	if s.broadcaster == nil {
		return BroadcastOutcome{}, fail(ErrorUnsupported, "solana broadcasting is not configured")
	}
	attempt, err := s.store.GetByID(ctx, attemptID)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			return BroadcastOutcome{}, failf(ErrorInvalidDraft, ErrAttemptNotFound)
		}
		return BroadcastOutcome{}, failf(ErrorBroadcastFailed, err)
	}
	if err := ValidateBroadcastRequest(attempt, wallet, signedTx, draftHash); err != nil {
		if errors.Is(err, ErrDraftChanged) {
			return BroadcastOutcome{}, failf(ErrorDraftChangedAfterQuote, err)
		}
		if errors.Is(err, ErrSignedTxRequired) {
			return BroadcastOutcome{}, failf(ErrorInvalidTransaction, err)
		}
		return BroadcastOutcome{}, failf(ErrorInvalidTransaction, err)
	}

	if err := s.store.SetStatus(ctx, attempt.ID, StatusBroadcasting); err != nil {
		return BroadcastOutcome{}, failf(ErrorBroadcastFailed, err)
	}

	sendResult, err := s.broadcaster.SendRawTransaction(ctx, signedTx)
	if err != nil {
		_ = s.store.RecordFailure(ctx, attempt.ID, ErrorBroadcastFailed, err.Error())
		return BroadcastOutcome{}, failf(ErrorBroadcastFailed, err)
	}

	if err := s.store.AttachSignature(ctx, attempt.ID, sendResult.Signature); err != nil {
		return BroadcastOutcome{}, failf(ErrorBroadcastFailed, err)
	}
	if err := s.store.SetStatus(ctx, attempt.ID, StatusSubmitted); err != nil {
		return BroadcastOutcome{}, failf(ErrorBroadcastFailed, err)
	}

	confirmation, err := s.confirm(ctx, sendResult.Signature)
	if err != nil {
		// The transaction may still land. Reporting UNKNOWN rather than FAILED is
		// the honest outcome: claiming failure could strand real funds.
		_ = s.store.SetStatus(ctx, attempt.ID, StatusUnknown)
		return BroadcastOutcome{}, failf(ErrorConfirmationTimeout, err)
	}

	if !confirmation.Confirmed {
		_ = s.store.SetStatus(ctx, attempt.ID, StatusUnknown)
		return BroadcastOutcome{}, fail(ErrorConfirmationTimeout,
			"Transaction was submitted but has not been confirmed yet. Check the attempt again shortly.")
	}

	if err := s.store.SetStatus(ctx, attempt.ID, StatusConfirmed); err != nil {
		return BroadcastOutcome{}, failf(ErrorConfirmationTimeout, err)
	}
	refreshed, _ := s.store.GetByID(ctx, attempt.ID)
	return BroadcastOutcome{Attempt: refreshed, Result: confirmation}, nil
}

// confirm polls the RPC for a bounded period.
func (s *Service) confirm(ctx context.Context, signature string) (BroadcastResult, error) {
	deadline := time.Now().Add(s.confirmTimeout)
	interval := s.confirmInterval
	var last BroadcastResult
	for {
		status, err := s.broadcaster.GetSignatureStatuses(ctx, signature)
		if err != nil {
			return last, err
		}
		last = status
		if status.Confirmed {
			return status, nil
		}
		if status.ConfirmStatus == "failed" {
			return status, errors.New("transaction was dropped by the network")
		}
		if time.Now().After(deadline) {
			return last, errors.New("confirmation timed out")
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// RegisterResult reports the authoritative outcome of registration.
type RegisterResult struct {
	Attempt    *Attempt `json:"attempt"`
	MarketID   string   `json:"market_id,omitempty"`
	Registered bool     `json:"registered"`
	// Message is the exact wording the UI should show. It is generated here so
	// the same rule holds for every client.
	Message string `json:"message"`
}

// Register finalises market creation with Panta.
//
// The critical behaviour: if Solana already confirmed but registration fails,
// the message is "Transaction confirmed on Solana. Panta registration is
// pending." — never "Market creation failed." The money moved and the chain is
// final; only Prophet's registration step needs a retry.
func (s *Service) Register(ctx context.Context, attemptID string) (RegisterResult, error) {
	if s.panta == nil {
		return RegisterResult{}, fail(ErrorUnsupported, "market creation is not configured")
	}
	attempt, err := s.store.GetByID(ctx, attemptID)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			return RegisterResult{}, failf(ErrorInvalidDraft, ErrAttemptNotFound)
		}
		return RegisterResult{}, failf(ErrorPantaRegistrationFailed, err)
	}
	if err := ValidateRegisterRequest(attempt, attempt.SolanaSignature); err != nil {
		return RegisterResult{}, failf(ErrorInvalidTransaction, err)
	}

	// Idempotent: registering an already-registered attempt returns the same
	// market id rather than creating a second one.
	if attempt.MarketExists() && attempt.MarketID != "" {
		return RegisterResult{
			Attempt:    attempt,
			MarketID:   attempt.MarketID,
			Registered: true,
			Message:    registeredMessage,
		}, nil
	}

	if attempt.Status != StatusConfirmed && attempt.Status != StatusUnknown && attempt.Status != StatusFailed {
		return RegisterResult{}, fail(ErrorInvalidTransaction,
			"market creation can only be registered after the transaction is confirmed")
	}

	if err := s.store.SetStatus(ctx, attempt.ID, StatusRegistering); err != nil {
		return RegisterResult{}, failf(ErrorPantaRegistrationFailed, err)
	}

	result, err := s.panta.Register(ctx, attempt.CreateID, attempt.SolanaSignature)
	if err != nil {
		// Preserve the fact that the transaction confirmed. The status stays
		// CONFIRMED so no layer can report this as a failed creation.
		_ = s.store.RecordFailure(ctx, attempt.ID, ErrorPantaRegistrationFailed, err.Error())
		_ = s.store.SetStatus(ctx, attempt.ID, StatusConfirmed)
		return RegisterResult{}, failf(ErrorPantaRegistrationFailed, err)
	}

	if strings.TrimSpace(result.MarketID) == "" {
		_ = s.store.RecordFailure(ctx, attempt.ID, ErrorPantaRegistrationFailed,
			"panta returned no market id")
		_ = s.store.SetStatus(ctx, attempt.ID, StatusConfirmed)
		return RegisterResult{}, fail(ErrorPantaRegistrationFailed,
			"panta registration returned no market id")
	}

	if err := s.store.AttachRegistration(ctx, attempt.ID, result.MarketID, result.Signature); err != nil {
		return RegisterResult{}, failf(ErrorPantaRegistrationFailed, err)
	}
	refreshed, _ := s.store.GetByID(ctx, attempt.ID)
	return RegisterResult{
		Attempt:    refreshed,
		MarketID:   result.MarketID,
		Registered: true,
		Message:    registeredMessage,
	}, nil
}

// confirmedPendingMessage is the exact wording required when the chain settled
// but registration did not. It is exported through StatusMessage so the HTTP
// layer and the UI cannot drift apart.
const confirmedPendingMessage = "Transaction confirmed on Solana. Panta registration is pending."

// registeredMessage is shown only after Panta returned a market id.
const registeredMessage = "Market registered with Panta. It will appear in Prophet once indexing completes."

// RegisteredMessage returns the post-registration wording.
func RegisteredMessage() string { return registeredMessage }

// StatusMessage renders the user-facing state for an attempt.
//
// This is the single place that decides what a status means to a human, so the
// UI cannot accidentally render a confirmed-but-unregistered attempt as a
// failure or as a success.
func StatusMessage(attempt *Attempt) string {
	if attempt == nil {
		return ""
	}
	switch attempt.Status {
	case StatusCreated:
		return "Draft accepted and quoted. The market is not created yet."
	case StatusSigned, StatusBroadcasting, StatusSubmitted, StatusConfirming:
		return "Transaction submitted. Waiting for confirmation on Solana."
	case StatusConfirmed:
		if attempt.ErrorCode == string(ErrorPantaRegistrationFailed) {
			return confirmedPendingMessage
		}
		return "Transaction confirmed on Solana. Panta registration is pending."
	case StatusRegistering:
		return "Transaction confirmed. Registering the market with Panta."
	case StatusRegistered:
		return registeredMessage
	case StatusIndexed:
		return "Market is live."
	case StatusUnknown:
		return "The transaction outcome could not be determined. Check the attempt before retrying."
	case StatusFailed:
		switch attempt.ErrorCode {
		case string(ErrorUserRejected):
			return "You rejected the transaction in your wallet. No funds were moved."
		case string(ErrorWalletNotConnected):
			return "No wallet was connected."
		case string(ErrorBroadcastFailed):
			return "The transaction could not be submitted to Solana."
		default:
			return "Market creation failed before the transaction was confirmed. No funds were moved."
		}
	default:
		return ""
	}
}

// Get returns an attempt for the status endpoint.
func (s *Service) Get(ctx context.Context, id string) (*Attempt, error) {
	attempt, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAttemptNotFound) {
			return nil, failf(ErrorInvalidDraft, ErrAttemptNotFound)
		}
		return nil, err
	}
	return attempt, nil
}
