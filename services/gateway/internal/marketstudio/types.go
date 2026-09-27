// Package marketstudio implements the Market Studio creation pipeline.
//
// Responsibilities are deliberately narrow:
//
//   - It persists and tracks a creation attempt's lifecycle.
//   - It talks to Panta for quote, build and registration only.
//   - It never signs, never holds a key, and never decides that a market is
//     live. A market becomes live only when Panta registration succeeds, and
//     only when the signature actually exists on chain.
//
// The AI assistant lives in the Python intelligence service. This package
// consumes its draft as untrusted input and re-validates deterministically.
package marketstudio

import (
	"encoding/json"
	"time"
)

// Status is the market creation lifecycle.
//
// The ordering matters: CONFIRMED means the Solana transaction is final on
// chain. REGISTERED means Panta accepted and indexed the market. INDEXED means
// Prophet's own market index has picked it up. Only REGISTERED and INDEXED mean
// a market actually exists.
type Status string

const (
	// StatusCreated means a draft was accepted and an attempt row exists.
	StatusCreated Status = "CREATED"
	// StatusSigned means the wallet returned a signed transaction.
	StatusSigned Status = "SIGNED"
	// StatusBroadcasting means the signed transaction is being submitted.
	StatusBroadcasting Status = "BROADCASTING"
	// StatusSubmitted means the RPC accepted the transaction.
	StatusSubmitted Status = "SUBMITTED"
	// StatusConfirming means we are polling for chain finality.
	StatusConfirming Status = "CONFIRMING"
	// StatusConfirmed means Solana confirmed. Registration is still pending.
	StatusConfirmed Status = "CONFIRMED"
	// StatusRegistering means we are calling Panta's register endpoint.
	StatusRegistering Status = "REGISTERING"
	// StatusRegistered means Panta registration succeeded. The market exists.
	StatusRegistered Status = "REGISTERED"
	// StatusIndexed means Prophet's market index has the market.
	StatusIndexed Status = "INDEXED"
	// StatusFailed means the attempt failed before registration.
	StatusFailed Status = "FAILED"
	// StatusUnknown means the outcome could not be determined.
	StatusUnknown Status = "UNKNOWN"
)

// ErrorCode classifies every failure the UI must distinguish.
//
// The distinction that matters most: ErrorPantaRegistrationFailed after a
// confirmed transaction does NOT mean the creation failed. The money moved and
// the chain is final; only Prophet's registration step failed.
type ErrorCode string

const (
	ErrorAIInterpretationFailed     ErrorCode = "AI_INTERPRETATION_FAILED"
	ErrorInvalidDraft               ErrorCode = "INVALID_DRAFT"
	ErrorAmbiguousMarket            ErrorCode = "AMBIGUOUS_MARKET"
	ErrorQuoteFailed                ErrorCode = "QUOTE_FAILED"
	ErrorBuildFailed                ErrorCode = "BUILD_FAILED"
	ErrorWalletNotConnected         ErrorCode = "WALLET_NOT_CONNECTED"
	ErrorUserRejected               ErrorCode = "USER_REJECTED"
	ErrorInvalidTransaction         ErrorCode = "INVALID_TRANSACTION"
	ErrorBroadcastFailed            ErrorCode = "BROADCAST_FAILED"
	ErrorConfirmationTimeout        ErrorCode = "CONFIRMATION_TIMEOUT"
	ErrorPantaRegistrationFailed    ErrorCode = "PANTA_REGISTRATION_FAILED"
	ErrorIndexingPending            ErrorCode = "INDEXING_PENDING"
	ErrorDraftChangedAfterQuote     ErrorCode = "DRAFT_CHANGED_AFTER_QUOTE"
	ErrorUnsupported                ErrorCode = "UNSUPPORTED_OPERATION"
	ErrorIntelligenceServiceOffline ErrorCode = "INTELLIGENCE_SERVICE_UNAVAILABLE"
)

// StatusForError maps an error code to the status it should leave behind.
func StatusForError(code ErrorCode) Status {
	if code == ErrorIndexingPending {
		return StatusRegistered
	}
	return StatusFailed
}

// Draft is the Prophet-side market draft. Only fields that the verified Panta
// create API accepts are ever forwarded upstream.
type Draft struct {
	Question            string   `json:"question"`
	ResolutionCriteria  string   `json:"resolution_criteria"`
	SourcesOfTruth      []string `json:"sources_of_truth"`
	Category            string   `json:"category"`
	ResolutionDate      string   `json:"resolution_date"`
	EndDate             string   `json:"end_date,omitempty"`
	StartDate           string   `json:"start_date,omitempty"`
	ImageURL            string   `json:"image_url"`
	Title               string   `json:"title,omitempty"`
	Description         string   `json:"description,omitempty"`
	Region              string   `json:"region,omitempty"`
	MarketType          string   `json:"market_type,omitempty"`
	OutcomeYes          string   `json:"outcome_yes,omitempty"`
	OutcomeNo           string   `json:"outcome_no,omitempty"`
	Notes               string   `json:"notes,omitempty"`
	ResolutionConfirmed bool     `json:"resolution_source_confirmed"`
}

// PantaCreateParams is the exact body sent to Panta's create-quote endpoint.
//
// Every field here is a documented Panta field. If Panta's API does not accept
// a key, it does not appear on this struct. No amount field exists because
// Panta computes fees from the market definition and we report them verbatim.
type PantaCreateParams struct {
	Wallet         string   `json:"wallet"`
	Question       string   `json:"question"`
	ResolutionRule string   `json:"resolutionRule"`
	SourcesOfTruth []string `json:"sourcesOfTruth"`
	Category       string   `json:"category"`
	StartTime      int64    `json:"startTime"`
	EndTime        int64    `json:"endTime"`
	ResolutionTime int64    `json:"resolutionTime"`
	ImageURL       string   `json:"imageUrl"`
	MarketType     string   `json:"marketType"`
	Title          string   `json:"title,omitempty"`
	Description    string   `json:"description,omitempty"`
	Region         string   `json:"region,omitempty"`
}

// ValidationIssue is one deterministic finding against a draft.
type ValidationIssue struct {
	Field    string `json:"field"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// ValidationReport is the deterministic gate result. The LLM is never the final
// validator; this report is the only thing that can enable a create action.
type ValidationReport struct {
	Valid              bool              `json:"valid"`
	Issues             []ValidationIssue `json:"issues"`
	MissingFields      []string          `json:"missing_fields"`
	NeedsClarification bool              `json:"needs_clarification"`
	DraftFingerprint   string            `json:"draft_fingerprint,omitempty"`
}

// Errors returns only the blocking issues.
func (r ValidationReport) Errors() []ValidationIssue {
	var out []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == "error" {
			out = append(out, issue)
		}
	}
	return out
}

// Warnings returns non-blocking issues. These are always surfaced to the user;
// they are never silently dropped.
func (r ValidationReport) Warnings() []ValidationIssue {
	var out []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == "warning" {
			out = append(out, issue)
		}
	}
	return out
}

// CreateQuote is Panta's create-quote response.
//
// Every money value is an integer string of USDC base units (6 decimals)
// exactly as Panta returned it. These are never parsed into float64 and never
// reformatted, so no rounding error can enter the system.
type CreateQuote struct {
	CreateID               string `json:"createId"`
	ExpectedEventPDA       string `json:"expectedEventPda"`
	PaymentUSDC            string `json:"paymentUsdc"`
	LiquidityInjectionUSDC string `json:"liquidityInjectionUsdc"`
	PlatformRevenueUSDC    string `json:"platformRevenueUsdc"`
	MarketType             string `json:"marketType"`
	ExpiresAt              string `json:"expiresAt"`
	BlockhashExpiryHintSec *int   `json:"blockhashExpiryHintSec"`
}

// CreateBuild is Panta's create-build response. Transaction is a base64
// VersionedTransaction that the user must sign. We never sign it ourselves.
type CreateBuild struct {
	Transaction            string         `json:"transaction"`
	RecentBlockhash        string         `json:"recentBlockhash"`
	LastValidBlockHeight   *int64         `json:"lastValidBlockHeight"`
	BlockhashExpiryHintSec *int           `json:"blockhashExpiryHintSec"`
	BuildFingerprint       string         `json:"buildFingerprint"`
	PaymentUSDC            string         `json:"paymentUsdc"`
	LiquidityInjectionUSDC string         `json:"liquidityInjectionUsdc"`
	PlatformRevenueUSDC    string         `json:"platformRevenueUsdc"`
	MarketType             string         `json:"marketType"`
	Derived                map[string]any `json:"derived,omitempty"`
	ExpiresAt              string         `json:"expiresAt"`
}

// RegisterResponse is Panta's register response. MarketID is the authoritative
// market identifier. Status is "registered" on success.
type RegisterResponse struct {
	CreateID  string   `json:"createId"`
	MarketID  string   `json:"marketId"`
	Status    string   `json:"status"`
	Signature string   `json:"signature"`
	Category  string   `json:"category,omitempty"`
	Title     string   `json:"title,omitempty"`
	Images    []string `json:"images,omitempty"`
}

// Attempt models one row of market_creation_attempts.
//
// Note what is absent: no private key, no seed phrase, and no signed
// transaction blob. The signed payload is relayed to the Solana RPC and
// discarded; only the public signature is retained.
type Attempt struct {
	ID                     string
	WalletAddress          string
	CreateID               string
	BuildFingerprint       string
	ExpectedEventPDA       string
	PaymentUSDC            string
	LiquidityInjectionUSDC string
	PlatformRevenueUSDC    string
	MarketType             string
	DraftHash              string
	DraftPayload           *Draft
	Status                 Status
	ErrorCode              string
	ErrorMessage           string
	SolanaSignature        string
	MarketID               string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	RegisteredAt           *time.Time
	IndexedAt              *time.Time
}

// MarshalDraft renders the draft for storage, or nil when unset.
func (a *Attempt) MarshalDraft() ([]byte, error) {
	if a.DraftPayload == nil {
		return nil, nil
	}
	return json.Marshal(a.DraftPayload)
}

// IsTerminal reports whether the attempt has reached a state it will not leave
// on its own. Terminal does not mean success.
func (a *Attempt) IsTerminal() bool {
	switch a.Status {
	case StatusRegistered, StatusIndexed, StatusFailed, StatusUnknown:
		return true
	default:
		return false
	}
}

// MarketExists reports whether an authoritative market id has been obtained.
// A signature alone does not make this true.
func (a *Attempt) MarketExists() bool {
	return a.Status == StatusRegistered || a.Status == StatusIndexed
}
