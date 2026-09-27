package marketstudio

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists creation attempts.
type Store interface {
	Create(ctx context.Context, attempt Attempt) (string, error)
	GetByID(ctx context.Context, id string) (*Attempt, error)
	GetByCreateID(ctx context.Context, createID string) (*Attempt, error)
	SetStatus(ctx context.Context, id string, status Status) error
	RecordFailure(ctx context.Context, id string, code ErrorCode, message string) error
	AttachQuote(ctx context.Context, id string, quote CreateQuote) error
	AttachBuild(ctx context.Context, id, buildFingerprint string) error
	AttachSignature(ctx context.Context, id, signature string) error
	AttachRegistration(ctx context.Context, id, marketID, signature string) error
}

// Repository implements Store on PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Create(ctx context.Context, attempt Attempt) (string, error) {
	payload, err := attempt.MarshalDraft()
	if err != nil {
		return "", err
	}

	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO market_creation_attempts (
			wallet_address, create_id, expected_event_pda, draft_hash, draft_payload,
			status, market_type
		)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, NULLIF($7, ''))
		RETURNING id::text`,
		attempt.WalletAddress,
		attempt.CreateID,
		attempt.ExpectedEventPDA,
		attempt.DraftHash,
		payload,
		string(attempt.Status),
		attempt.MarketType,
	).Scan(&id)
	return id, err
}

const attemptColumns = `id::text, wallet_address, COALESCE(create_id, ''),
	COALESCE(expected_event_pda, ''), COALESCE(payment_usdc, ''),
	COALESCE(liquidity_injection_usdc, ''), COALESCE(platform_revenue_usdc, ''),
	COALESCE(market_type, ''), COALESCE(draft_hash, ''), draft_payload,
	status, COALESCE(error_code, ''), COALESCE(error_message, ''),
	COALESCE(solana_signature, ''), COALESCE(market_id, ''),
	created_at, updated_at, registered_at, indexed_at`

func (r *Repository) GetByID(ctx context.Context, id string) (*Attempt, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+attemptColumns+` FROM market_creation_attempts WHERE id = $1::uuid`, id)
	return scanAttempt(row)
}

func (r *Repository) GetByCreateID(ctx context.Context, createID string) (*Attempt, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+attemptColumns+` FROM market_creation_attempts WHERE create_id = $1 ORDER BY created_at DESC LIMIT 1`, createID)
	return scanAttempt(row)
}

func (r *Repository) SetStatus(ctx context.Context, id string, status Status) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE market_creation_attempts
		SET status = $2, updated_at = now()
		WHERE id = $1::uuid`, id, string(status))
	return err
}

// RecordFailure stores a failure code alongside its status.
//
// StatusForError keeps the two consistent, with one deliberate exception:
// INDEXING_PENDING is not a failure of creation, it means registration succeeded
// and only Prophet's own indexing lags. That must never render as FAILED.
func (r *Repository) RecordFailure(ctx context.Context, id string, code ErrorCode, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE market_creation_attempts
		SET status = $2, error_code = $3, error_message = $4, updated_at = now()
		WHERE id = $1::uuid`,
		id, string(StatusForError(code)), string(code), message)
	return err
}

func (r *Repository) AttachQuote(ctx context.Context, id string, quote CreateQuote) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE market_creation_attempts
		SET create_id = $2,
		    expected_event_pda = $3,
		    payment_usdc = $4,
		    liquidity_injection_usdc = $5,
		    platform_revenue_usdc = $6,
		    market_type = $7,
		    status = 'CREATED',
		    error_code = NULL,
		    error_message = NULL,
		    updated_at = now()
		WHERE id = $1::uuid`,
		id,
		quote.CreateID,
		quote.ExpectedEventPDA,
		quote.PaymentUSDC,
		quote.LiquidityInjectionUSDC,
		quote.PlatformRevenueUSDC,
		quote.MarketType,
	)
	return err
}

func (r *Repository) AttachBuild(ctx context.Context, id, buildFingerprint string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE market_creation_attempts
		SET build_fingerprint = $2, updated_at = now()
		WHERE id = $1::uuid`, id, buildFingerprint)
	return err
}

// AttachSignature records the public transaction signature.
//
// The signed transaction blob is deliberately never persisted. It is relayed to
// the RPC and discarded; only the signature, which is public, is kept.
func (r *Repository) AttachSignature(ctx context.Context, id, signature string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE market_creation_attempts
		SET solana_signature = $2, updated_at = now()
		WHERE id = $1::uuid`, id, signature)
	return err
}

// AttachRegistration stores the authoritative market id returned by Panta. This
// is the only place market_id is written, so a market can never appear locally
// before Panta actually registered it.
func (r *Repository) AttachRegistration(ctx context.Context, id, marketID, signature string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE market_creation_attempts
		SET market_id = $2,
		    solana_signature = COALESCE(NULLIF($3, ''), solana_signature),
		    status = 'REGISTERED',
		    error_code = NULL,
		    error_message = NULL,
		    registered_at = now(),
		    updated_at = now()
		WHERE id = $1::uuid`, id, marketID, signature)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAttempt(row rowScanner) (*Attempt, error) {
	var (
		attempt  Attempt
		status   string
		payload  []byte
		marketID *string
	)
	err := row.Scan(
		&attempt.ID,
		&attempt.WalletAddress,
		&attempt.CreateID,
		&attempt.ExpectedEventPDA,
		&attempt.PaymentUSDC,
		&attempt.LiquidityInjectionUSDC,
		&attempt.PlatformRevenueUSDC,
		&attempt.MarketType,
		&attempt.DraftHash,
		&payload,
		&status,
		&attempt.ErrorCode,
		&attempt.ErrorMessage,
		&attempt.SolanaSignature,
		&marketID,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
		&attempt.RegisteredAt,
		&attempt.IndexedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAttemptNotFound
		}
		return nil, err
	}

	attempt.Status = Status(status)
	if marketID != nil {
		attempt.MarketID = *marketID
	}
	if len(payload) > 0 {
		var draft Draft
		if err := json.Unmarshal(payload, &draft); err == nil {
			attempt.DraftPayload = &draft
		}
	}
	return &attempt, nil
}

// MemoryStore is an in-memory Store for tests and for local runs without a
// database. It mirrors the production semantics that matter: amounts stay
// strings and signatures stay idempotent.
type MemoryStore struct {
	mu       sync.Mutex
	attempts map[string]*Attempt
	order    []string
	// now is injectable so tests can assert on timestamps.
	now func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{attempts: make(map[string]*Attempt), now: time.Now}
}

func (s *MemoryStore) Create(_ context.Context, attempt Attempt) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if attempt.Status == "" {
		attempt.Status = StatusCreated
	}
	id := attempt.ID
	if id == "" {
		id = "attempt-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	attempt.ID = id
	now := s.now()
	attempt.CreatedAt = now
	attempt.UpdatedAt = now
	copied := attempt
	s.attempts[id] = &copied
	s.order = append(s.order, id)
	return id, nil
}

func (s *MemoryStore) GetByID(_ context.Context, id string) (*Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[id]
	if !ok {
		return nil, ErrAttemptNotFound
	}
	copied := *attempt
	return &copied, nil
}

func (s *MemoryStore) GetByCreateID(_ context.Context, createID string) (*Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.order) - 1; i >= 0; i-- {
		attempt, ok := s.attempts[s.order[i]]
		if ok && attempt.CreateID == createID {
			copied := *attempt
			return &copied, nil
		}
	}
	return nil, ErrAttemptNotFound
}

func (s *MemoryStore) SetStatus(_ context.Context, id string, status Status) error {
	return s.mutate(id, func(attempt *Attempt) { attempt.Status = status })
}

func (s *MemoryStore) RecordFailure(_ context.Context, id string, code ErrorCode, message string) error {
	return s.mutate(id, func(attempt *Attempt) {
		attempt.Status = StatusForError(code)
		attempt.ErrorCode = string(code)
		attempt.ErrorMessage = message
	})
}

func (s *MemoryStore) AttachQuote(_ context.Context, id string, quote CreateQuote) error {
	return s.mutate(id, func(attempt *Attempt) {
		attempt.CreateID = quote.CreateID
		attempt.ExpectedEventPDA = quote.ExpectedEventPDA
		attempt.PaymentUSDC = quote.PaymentUSDC
		attempt.LiquidityInjectionUSDC = quote.LiquidityInjectionUSDC
		attempt.PlatformRevenueUSDC = quote.PlatformRevenueUSDC
		attempt.MarketType = quote.MarketType
		attempt.Status = StatusCreated
		attempt.ErrorCode = ""
		attempt.ErrorMessage = ""
	})
}

func (s *MemoryStore) AttachBuild(_ context.Context, id, buildFingerprint string) error {
	return s.mutate(id, func(attempt *Attempt) { attempt.BuildFingerprint = buildFingerprint })
}

func (s *MemoryStore) AttachSignature(_ context.Context, id, signature string) error {
	return s.mutate(id, func(attempt *Attempt) { attempt.SolanaSignature = signature })
}

func (s *MemoryStore) AttachRegistration(_ context.Context, id, marketID, signature string) error {
	now := s.now()
	return s.mutate(id, func(attempt *Attempt) {
		attempt.MarketID = marketID
		if signature != "" {
			attempt.SolanaSignature = signature
		}
		attempt.Status = StatusRegistered
		attempt.ErrorCode = ""
		attempt.ErrorMessage = ""
		attempt.RegisteredAt = &now
	})
}

func (s *MemoryStore) mutate(id string, apply func(*Attempt)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[id]
	if !ok {
		return ErrAttemptNotFound
	}
	apply(attempt)
	attempt.UpdatedAt = s.now()
	return nil
}

func (s *MemoryStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.attempts)
}
