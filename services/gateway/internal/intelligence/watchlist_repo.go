// Package-level watchlist persistence for the intelligence repository.
//
// OWNERSHIP-FILTER INVARIANT (read first):
//
// Every method in this file that accepts a userID MUST scope its SQL by
// `user_id = $N` in the same statement that resolves the target row. Ownership
// is never checked only as a separate follow-up step, and no statement may
// interpolate caller-supplied values into SQL: all inputs are bound through
// pgx positional placeholders ($1, $2, ...) only.
//
// Consequences callers can rely on:
//   - A user passing another user's watchlist id receives ErrWatchlistNotFound
//     (or a zero-row no-op for the idempotent mutations), never their data.
//   - A missing and a foreign-owned watchlist are indistinguishable to the
//     caller, so watchlist ids cannot be probed for existence.
//   - Membership mutations resolve the watchlist through the same user-scoped
//     predicate before touching watchlist_markets, so a row can never be linked
//     to a watchlist the caller does not own.
//
// The single deliberate exception is WatchlistIDsForMarket, a system-facing
// lookup for alert fan-out. It answers "which watchlists contain this market",
// not "what may this user see", and its result must not be handed to a user
// directly.
package intelligence

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrWatchlistNotFound  = errors.New("watchlist not found")
	ErrWatchlistNameTaken = errors.New("a watchlist with that name already exists")
	ErrMarketNotFound     = errors.New("market not found")
)

const uniqueViolationCode = "23505"

const watchlistColumns = `id::text,user_id,name,description,created_at,updated_at`

// WatchlistSummary is a watchlist enriched with aggregate market and signal
// metadata for list views.
type WatchlistSummary struct {
	Watchlist
	MarketCount          int        `json:"market_count"`
	LatestSignalSeverity *string    `json:"latest_signal_severity"`
	LatestSignalAt       *time.Time `json:"latest_signal_at"`
}

func (r *Repository) CreateWatchlist(ctx context.Context, userID, name string, description *string) (*Watchlist, error) {
	watchlist, err := scanWatchlist(r.pool.QueryRow(ctx, `
		INSERT INTO watchlists (user_id,name,description)
		VALUES ($1,$2,$3)
		RETURNING `+watchlistColumns, userID, name, description))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWatchlistNameTaken
		}
		return nil, err
	}
	return watchlist, nil
}

func (r *Repository) GetWatchlist(ctx context.Context, userID, watchlistID string) (*Watchlist, error) {
	watchlist, err := scanWatchlist(r.pool.QueryRow(ctx, `
		SELECT `+watchlistColumns+` FROM watchlists
		WHERE id=$1 AND user_id=$2`, watchlistID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWatchlistNotFound
	}
	if err != nil {
		return nil, err
	}
	return watchlist, nil
}

func (r *Repository) UpdateWatchlist(ctx context.Context, userID, watchlistID, name string, description *string) (*Watchlist, error) {
	watchlist, err := scanWatchlist(r.pool.QueryRow(ctx, `
		UPDATE watchlists
		SET name=$3, description=$4, updated_at=now()
		WHERE id=$1 AND user_id=$2
		RETURNING `+watchlistColumns, watchlistID, userID, name, description))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWatchlistNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrWatchlistNameTaken
		}
		return nil, err
	}
	return watchlist, nil
}

// DeleteWatchlist removes a watchlist owned by the user. It is idempotent:
// deleting an already-deleted or a foreign watchlist is a successful no-op, so
// the caller cannot distinguish the two cases.
func (r *Repository) DeleteWatchlist(ctx context.Context, userID, watchlistID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM watchlists WHERE id=$1 AND user_id=$2`, watchlistID, userID)
	return err
}

func (r *Repository) AddMarketToWatchlist(ctx context.Context, userID, watchlistID, sourceMarketID string) error {
	if err := r.ensureWatchlistOwned(ctx, userID, watchlistID); err != nil {
		return err
	}
	marketID, err := r.marketUUIDBySourceID(ctx, sourceMarketID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO watchlist_markets (watchlist_id,market_id)
		VALUES ($1,$2)
		ON CONFLICT DO NOTHING`, watchlistID, marketID)
	return err
}

// RemoveMarketFromWatchlist is idempotent once the watchlist exists: removing
// a market that is not on it is a successful no-op.
func (r *Repository) RemoveMarketFromWatchlist(ctx context.Context, userID, watchlistID, sourceMarketID string) error {
	if err := r.ensureWatchlistOwned(ctx, userID, watchlistID); err != nil {
		return err
	}
	marketID, err := r.marketUUIDBySourceID(ctx, sourceMarketID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		DELETE FROM watchlist_markets
		WHERE watchlist_id=$1 AND market_id=$2`, watchlistID, marketID)
	return err
}

// WatchlistMarketIDs returns the internal market UUIDs on the watchlist.
func (r *Repository) WatchlistMarketIDs(ctx context.Context, userID, watchlistID string) ([]string, error) {
	if err := r.ensureWatchlistOwned(ctx, userID, watchlistID); err != nil {
		return nil, err
	}
	return r.collectMarketIDs(ctx, `
		SELECT market_id::text FROM watchlist_markets
		WHERE watchlist_id=$1
		ORDER BY created_at,market_id`, watchlistID)
}

// WatchlistIDsForMarket returns every watchlist id containing the market. This
// is a system lookup for alert fan-out and is intentionally not user-scoped.
func (r *Repository) WatchlistIDsForMarket(ctx context.Context, marketUUID string) ([]string, error) {
	return r.collectMarketIDs(ctx, `
		SELECT watchlist_id::text FROM watchlist_markets
		WHERE market_id=$1
		ORDER BY created_at,watchlist_id`, marketUUID)
}

func (r *Repository) collectMarketIDs(ctx context.Context, query, id string) ([]string, error) {
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, 16)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		ids = append(ids, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// ListWatchlists returns the user's watchlists, most recently updated first,
// each with a correlated market count and the severity and timestamp of the
// latest signal event across its markets.
func (r *Repository) ListWatchlists(ctx context.Context, userID string) ([]WatchlistSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT w.id::text,w.user_id,w.name,w.description,w.created_at,w.updated_at,
			(SELECT count(*) FROM watchlist_markets wm WHERE wm.watchlist_id=w.id),
			(SELECT se.severity FROM signal_events se
				JOIN watchlist_markets wm2 ON wm2.market_id=se.market_id
				WHERE wm2.watchlist_id=w.id
				ORDER BY se.observed_at DESC,se.id DESC LIMIT 1),
			(SELECT se.observed_at FROM signal_events se
				JOIN watchlist_markets wm3 ON wm3.market_id=se.market_id
				WHERE wm3.watchlist_id=w.id
				ORDER BY se.observed_at DESC,se.id DESC LIMIT 1)
		FROM watchlists w
		WHERE w.user_id=$1
		ORDER BY w.updated_at DESC,w.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summaries := make([]WatchlistSummary, 0, 16)
	for rows.Next() {
		var summary WatchlistSummary
		if err := rows.Scan(
			&summary.ID, &summary.UserID, &summary.Name, &summary.Description,
			&summary.CreatedAt, &summary.UpdatedAt, &summary.MarketCount,
			&summary.LatestSignalSeverity, &summary.LatestSignalAt,
		); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summaries, nil
}

func (r *Repository) CountWatchlistMarkets(ctx context.Context, userID, watchlistID string) (int, error) {
	if err := r.ensureWatchlistOwned(ctx, userID, watchlistID); err != nil {
		return 0, err
	}
	var count int64
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM watchlist_markets WHERE watchlist_id=$1`, watchlistID).Scan(&count); err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *Repository) ensureWatchlistOwned(ctx context.Context, userID, watchlistID string) error {
	var owned string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text FROM watchlists WHERE id=$1 AND user_id=$2`, watchlistID, userID).Scan(&owned)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWatchlistNotFound
	}
	return err
}

func (r *Repository) marketUUIDBySourceID(ctx context.Context, sourceMarketID string) (string, error) {
	var marketID string
	err := r.pool.QueryRow(ctx, `SELECT id::text FROM markets WHERE source_market_id=$1`, sourceMarketID).Scan(&marketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrMarketNotFound
	}
	if err != nil {
		return "", err
	}
	return marketID, nil
}

func scanWatchlist(row pgx.Row) (*Watchlist, error) {
	var watchlist Watchlist
	if err := row.Scan(&watchlist.ID, &watchlist.UserID, &watchlist.Name, &watchlist.Description,
		&watchlist.CreatedAt, &watchlist.UpdatedAt); err != nil {
		return nil, err
	}
	return &watchlist, nil
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		return pgError.Code == uniqueViolationCode
	}
	return false
}
