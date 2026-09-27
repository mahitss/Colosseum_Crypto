package trading

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status represents the explicit trade lifecycle state machine.
type Status string

const (
	StatusIDLE               Status = "IDLE"
	StatusQuoting            Status = "QUOTING"
	StatusQuoteReady         Status = "QUOTE_READY"
	StatusBuilding           Status = "BUILDING"
	StatusReadyToSign        Status = "READY_TO_SIGN"
	StatusSigning            Status = "SIGNING"
	StatusSigned             Status = "SIGNED"
	StatusBroadcasting       Status = "BROADCASTING"
	StatusSubmitted          Status = "SUBMITTED"
	StatusConfirming         Status = "CONFIRMING"
	StatusConfirmed          Status = "CONFIRMED"
	StatusReporting          Status = "REPORTING"
	StatusVerified           Status = "VERIFIED"
	StatusPositionRefreshing Status = "POSITION_REFRESHING"
	StatusCompleted          Status = "COMPLETED"
	StatusCancelled          Status = "CANCELLED"
	StatusFailed             Status = "FAILED"
	StatusUnknown            Status = "UNKNOWN"
)

// ErrorCode classifies failures for accurate UI messaging.
type ErrorCode string

const (
	ErrorQuoteFailed             ErrorCode = "QUOTE_FAILED"
	ErrorBuildFailed             ErrorCode = "BUILD_FAILED"
	ErrorWalletNotConnected      ErrorCode = "WALLET_NOT_CONNECTED"
	ErrorUserRejected            ErrorCode = "USER_REJECTED"
	ErrorInvalidTransaction      ErrorCode = "INVALID_TRANSACTION"
	ErrorBroadcastFailed         ErrorCode = "BROADCAST_FAILED"
	ErrorConfirmationTimeout     ErrorCode = "CONFIRMATION_TIMEOUT"
	ErrorTransactionFailed       ErrorCode = "TRANSACTION_FAILED"
	ErrorPantaReportFailed       ErrorCode = "PANTA_REPORT_FAILED"
	ErrorPantaVerificationFailed ErrorCode = "PANTA_VERIFICATION_FAILED"
	ErrorPositionRefreshFailed   ErrorCode = "POSITION_REFRESH_FAILED"
)

// Attempt models one row of the trade_attempts table.
type Attempt struct {
	ID              string
	WalletAddress   string
	MarketID        string
	Side            string
	AmountUSDC      string
	QuoteReference  string
	SolanaSignature string
	PantaReference  string
	Status          Status
	ErrorCode       string
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ConfirmedAt     *time.Time
	VerifiedAt      *time.Time
}

// Store persists trade attempts. It is implemented by the SQL repository and
// can be faked in tests with an in-memory implementation.
type Store interface {
	CreateAttempt(ctx context.Context, tx pgx.Tx, attempt Attempt) (string, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, id string, status Status) error
	RecordFailure(ctx context.Context, tx pgx.Tx, id string, code ErrorCode, message string) error
	AttachSignature(ctx context.Context, tx pgx.Tx, id, signature string) error
	AttachPantaReference(ctx context.Context, tx pgx.Tx, id, reference string) error
	MarkConfirmed(ctx context.Context, tx pgx.Tx, id string) error
	MarkVerified(ctx context.Context, tx pgx.Tx, id string) error
	GetAttempt(ctx context.Context, q rowQueryer, id string) (*Attempt, error)
}

type rowQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Repository implements Store on PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) CreateAttempt(ctx context.Context, tx pgx.Tx, attempt Attempt) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO trade_attempts (wallet_address, market_id, side, amount_usdc, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`,
		attempt.WalletAddress, attempt.MarketID, attempt.Side, attempt.AmountUSDC, attempt.Status,
	).Scan(&id)
	return id, err
}

func (r *Repository) UpdateStatus(ctx context.Context, tx pgx.Tx, id string, status Status) error {
	_, err := tx.Exec(ctx, `
		UPDATE trade_attempts SET status = $2, updated_at = now() WHERE id = $1::uuid`,
		id, string(status),
	)
	return err
}

func (r *Repository) RecordFailure(ctx context.Context, tx pgx.Tx, id string, code ErrorCode, message string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trade_attempts
		SET status = 'FAILED', error_code = $2, error_message = $3, updated_at = now()
		WHERE id = $1::uuid`,
		id, string(code), message,
	)
	return err
}

func (r *Repository) AttachSignature(ctx context.Context, tx pgx.Tx, id, signature string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trade_attempts
		SET solana_signature = $2, status = 'SIGNED', updated_at = now()
		WHERE id = $1::uuid`,
		id, signature,
	)
	return err
}

func (r *Repository) AttachPantaReference(ctx context.Context, tx pgx.Tx, id, reference string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trade_attempts
		SET panta_reference = $2, updated_at = now()
		WHERE id = $1::uuid`,
		id, reference,
	)
	return err
}

func (r *Repository) MarkConfirmed(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trade_attempts
		SET status = 'CONFIRMED', confirmed_at = now(), updated_at = now()
		WHERE id = $1::uuid`,
		id,
	)
	return err
}

func (r *Repository) MarkVerified(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `
		UPDATE trade_attempts
		SET status = 'VERIFIED', verified_at = now(), updated_at = now()
		WHERE id = $1::uuid`,
		id,
	)
	return err
}

func (r *Repository) GetAttempt(ctx context.Context, q rowQueryer, id string) (*Attempt, error) {
	row := q.QueryRow(ctx, `
		SELECT id::text, wallet_address, market_id, side, amount_usdc::text,
		       COALESCE(quote_reference, ''), COALESCE(solana_signature, ''),
		       COALESCE(panta_reference, ''), status,
		       COALESCE(error_code, ''), COALESCE(error_message, ''),
		       created_at, updated_at, confirmed_at, verified_at
		FROM trade_attempts WHERE id = $1::uuid`, id)
	var a Attempt
	var confirmedAt, verifiedAt *time.Time
	err := row.Scan(&a.ID, &a.WalletAddress, &a.MarketID, &a.Side, &a.AmountUSDC,
		&a.QuoteReference, &a.SolanaSignature, &a.PantaReference,
		&a.Status, &a.ErrorCode, &a.ErrorMessage,
		&a.CreatedAt, &a.UpdatedAt, &confirmedAt, &verifiedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a.ConfirmedAt = confirmedAt
	a.VerifiedAt = verifiedAt
	return &a, nil
}
