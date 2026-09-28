// Package-level persistence for the durable signal event stream.
//
// DEDUPLICATION IS ENFORCED BY THE DATABASE, NOT BY APPLICATION LOGIC (read first):
//
// The only thing that makes a signal event a duplicate is the UNIQUE constraint
// `signal_events_fingerprint_unique` on `signal_events.fingerprint`. Every insert
// path in this file is written as `ON CONFLICT (fingerprint) DO NOTHING`, and the
// uniqueness of that index is what resolves the race. This matters because signal
// detection can run concurrently: several workers, replicas, or a retried sync may
// all observe the same market movement and try to record the same event at the
// same moment.
//
// A read-then-write guard ("SELECT to see if the fingerprint exists, then INSERT")
// would be incorrect here. Between the SELECT and the INSERT another worker can
// commit the same fingerprint, and both workers would proceed to insert, so the
// duplicate would be written anyway. The window is small but it is real, and it is
// exactly the window a retry loop widens. Postgres evaluates the unique index
// during the INSERT itself, inside the statement, so exactly one of N concurrent
// workers sees a row come back from RETURNING and the rest see no row at all.
//
// Application code therefore has exactly one job here: compute a deterministic
// fingerprint and let the database arbitrate. It must never pre-check existence and
// must never treat "conflict" as an error. The single enforcement point in the
// write path is InsertSignalEvent, which fills in an empty Fingerprint with
// SignalFingerprint before the statement runs; every other writer
// (InsertSignalEventsFromSignals) reuses the same computation so its fingerprints
// are comparable with the ones produced by the direct path.
package intelligence

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrSignalEventNotFound = errors.New("signal event not found")

// signalEventColumns is the canonical projection for a signal event, aliased
// `se` for the event table and `m` for the joined market. Numeric columns are
// cast to text so they scan into *string without passing through float64 and
// without losing the stored NUMERIC(38,12) precision.
const signalEventColumns = `se.id, se.market_id::text, m.source_market_id, se.signal_type, se.severity, se.metric, se.previous_value::text, se.current_value::text, se.absolute_change::text, se.percentage_change::text, se.percentage_points::text, se.observation_window, se.observation_id, se.observed_at, se.fingerprint, se.source, se.created_at`

// signalEventJoin joins the event to its market so SourceMarketID is available
// without a second query. The join is an inner join because market_id is a
// NOT NULL foreign key into markets.
const signalEventJoin = ` FROM signal_events se JOIN markets m ON m.id=se.market_id `

func scanSignalEvent(row rowScanner) (SignalEvent, error) {
	var event SignalEvent
	if err := row.Scan(
		&event.ID, &event.MarketID, &event.SourceMarketID, &event.SignalType, &event.Severity,
		&event.Metric, &event.PreviousValue, &event.CurrentValue, &event.AbsoluteChange,
		&event.PercentageChange, &event.PercentagePoints, &event.ObservationWindow,
		&event.ObservationID, &event.ObservedAt, &event.Fingerprint, &event.Source,
		&event.CreatedAt,
	); err != nil {
		return SignalEvent{}, err
	}
	return event, nil
}

// InsertSignalEvent records an event and reports whether it was actually stored.
//
// The second return value is false, with a nil error, when the fingerprint was
// already present: the event was deduplicated by the unique constraint, and the
// caller may treat that as success. A deduplicated event is not an error
// condition, so it is never reported as one.
//
// An empty Fingerprint is filled in from SignalFingerprint here, which makes this
// the single dedupe enforcement point of the write path. The database, not this
// check, is what resolves concurrent inserts.
func (r *Repository) InsertSignalEvent(ctx context.Context, event SignalEvent) (*SignalEvent, bool, error) {
	return insertSignalEvent(ctx, r.pool, event)
}

// insertSignalEvent is the shared statement behind both the pool-backed and the
// transaction-backed insert paths, so the two cannot drift apart.
func insertSignalEvent(ctx context.Context, executor rowQueryer, event SignalEvent) (*SignalEvent, bool, error) {
	if event.Fingerprint == "" {
		event.Fingerprint = SignalFingerprint(event)
	}
	var id int64
	createdAt := event.CreatedAt
	err := executor.QueryRow(ctx, `
		INSERT INTO signal_events (
			market_id,signal_type,severity,metric,previous_value,current_value,
			absolute_change,percentage_change,percentage_points,observation_window,
			observation_id,observed_at,fingerprint,source
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
		)
		ON CONFLICT (fingerprint) DO NOTHING
		RETURNING id, created_at`,
		event.MarketID, event.SignalType, event.Severity, event.Metric,
		event.PreviousValue, event.CurrentValue, event.AbsoluteChange,
		event.PercentageChange, event.PercentagePoints, event.ObservationWindow,
		event.ObservationID, event.ObservedAt, event.Fingerprint, event.Source,
	).Scan(&id, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// The fingerprint already exists. Another worker, an earlier sync, or a
		// retry of this same call got there first.
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	event.ID = id
	event.CreatedAt = createdAt
	return &event, true, nil
}

// SignalEventByFingerprint resolves a single event by its dedupe key, returning
// ErrSignalEventNotFound when no such event exists.
func (r *Repository) SignalEventByFingerprint(ctx context.Context, fingerprint string) (*SignalEvent, error) {
	event, err := scanSignalEvent(r.pool.QueryRow(ctx,
		`SELECT `+signalEventColumns+signalEventJoin+`WHERE se.fingerprint=$1`, fingerprint))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSignalEventNotFound
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) CountSignalEvents(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM signal_events`).Scan(&count)
	return count, err
}

// RecentSignalEvents returns the most recent events across all markets, newest
// first. Ties on observed_at are broken by the identity column so the ordering is
// total and stable across pages.
func (r *Repository) RecentSignalEvents(ctx context.Context, limit int) ([]SignalEvent, error) {
	return scanSignalEvents(ctx, r.pool,
		`SELECT `+signalEventColumns+signalEventJoin+`ORDER BY se.observed_at DESC, se.id DESC LIMIT $1`,
		limit)
}

// SignalEventsForMarket returns the most recent events for a single market,
// addressed by the market's own UUID.
func (r *Repository) SignalEventsForMarket(ctx context.Context, marketUUID string, limit int) ([]SignalEvent, error) {
	return scanSignalEvents(ctx, r.pool,
		`SELECT `+signalEventColumns+signalEventJoin+`WHERE se.market_id=$1 ORDER BY se.observed_at DESC, se.id DESC LIMIT $2`,
		marketUUID, limit)
}

type eventQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func scanSignalEvents(ctx context.Context, executor eventQueryer, query string, args ...any) ([]SignalEvent, error) {
	rows, err := executor.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]SignalEvent, 0, 32)
	for rows.Next() {
		event, err := scanSignalEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// InsertSignalEventsFromSignals bridges the existing sync path into the durable
// event stream. Each Signal is resolved to its market by source_market_id, turned
// into a SignalEvent, fingerprinted, and inserted with the same
// ON CONFLICT (fingerprint) DO NOTHING statement the direct path uses, so a signal
// already recorded is deduplicated rather than duplicated.
//
// A signal whose market cannot be resolved is skipped rather than failing the
// batch: the event stream is a best-effort durable mirror of detection, and one
// unresolvable market must not discard the events for every other market in the
// same sync. Only a genuine database error aborts the batch.
//
// The returned slice holds the events that were actually stored, so its length may
// be smaller than len(signals) for both skipped and deduplicated signals.
// SignalEventFromSignal projects a stored signal onto the durable event shape.
//
// The mapping is factored out of InsertSignalEventsFromSignals rather than
// duplicated, because a second caller now needs it: a background worker that
// reads back the signals a tick persisted (the Syncer reports only counts, not
// the generated signals themselves) and feeds them to AlertEvaluator. Both
// callers must produce byte-identical events, or the same observation would
// fingerprint differently depending on which path recorded it and the unique
// constraint on signal_events.fingerprint would stop deduplicating.
//
// marketUUID and sourceMarketID are passed explicitly rather than read off the
// signal because the two identities are not interchangeable. A signal returned by
// the Rust engine carries the Panta source id in its MarketID field, because that
// is the identity the engine was given, while a signal read back from the signals
// table carries the internal markets.id there. Taking both as arguments makes each
// caller state which identity it holds instead of relying on a field whose meaning
// depends on where the value came from.
func SignalEventFromSignal(signal Signal, marketUUID, sourceMarketID string) SignalEvent {
	return SignalEvent{
		MarketID:          marketUUID,
		SourceMarketID:    sourceMarketID,
		SignalType:        signal.SignalType,
		Severity:          signal.Severity,
		Metric:            signal.Metric,
		PreviousValue:     signal.PreviousValue,
		CurrentValue:      signal.CurrentValue,
		AbsoluteChange:    signal.AbsoluteChange,
		PercentageChange:  signal.PercentageChange,
		PercentagePoints:  signal.PercentagePoints,
		ObservationWindow: signal.ObservationWindow,
		ObservationID:     signal.ObservationID,
		ObservedAt:        signal.Timestamp,
		Source:            signal.Source,
	}
}
func (r *Repository) InsertSignalEventsFromSignals(ctx context.Context, tx pgx.Tx, signals []Signal) ([]SignalEvent, error) {
	inserted := make([]SignalEvent, 0, len(signals))
	for _, signal := range signals {
		sourceMarketID := signal.SourceMarketID
		if sourceMarketID == "" {
			continue
		}
		var marketUUID string
		err := tx.QueryRow(ctx, `SELECT id::text FROM markets WHERE source_market_id=$1`, sourceMarketID).Scan(&marketUUID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		event := SignalEventFromSignal(signal, marketUUID, sourceMarketID)
		stored, ok, err := insertSignalEvent(ctx, tx, event)
		if err != nil {
			return nil, err
		}
		if ok {
			inserted = append(inserted, *stored)
		}
	}
	return inserted, nil
}

// LatestSignalsForMarkets returns the latest signal event for each of the given market UUIDs.
func (r *Repository) LatestSignalsForMarkets(ctx context.Context, marketUUIDs []string) ([]SignalEvent, error) {
	if len(marketUUIDs) == 0 {
		return []SignalEvent{}, nil
	}

	// Use a CTE with ROW_NUMBER to get the latest signal per market
	placeholders := make([]string, len(marketUUIDs))
	args := make([]any, len(marketUUIDs))
	for i, id := range marketUUIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM (
			SELECT %s,
			       ROW_NUMBER() OVER (PARTITION BY se.market_id ORDER BY se.observed_at DESC, se.id DESC) as rn
			FROM signal_events se
			WHERE se.market_id IN (%s)
		) sub
		WHERE sub.rn = 1
		ORDER BY sub.observed_at DESC, sub.id DESC
	`, signalEventColumns, signalEventColumns, strings.Join(placeholders, ", "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]SignalEvent, 0, len(marketUUIDs))
	for rows.Next() {
		event, err := scanSignalEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// LatestObservationsForMarkets returns the latest observation for each of the given market UUIDs.
func (r *Repository) LatestObservationsForMarkets(ctx context.Context, marketUUIDs []string) ([]Observation, error) {
	if len(marketUUIDs) == 0 {
		return []Observation{}, nil
	}

	placeholders := make([]string, len(marketUUIDs))
	args := make([]any, len(marketUUIDs))
	for i, id := range marketUUIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT mo.id, mo.market_id::text, mo.observed_at,
		       mo.yes_probability::text, mo.no_probability::text,
		       mo.volume_usdc::text, mo.liquidity::text
		FROM (
			SELECT *,
			       ROW_NUMBER() OVER (PARTITION BY market_id ORDER BY observed_at DESC, id DESC) as rn
			FROM market_observations
			WHERE market_id IN (%s)
		) sub
		WHERE sub.rn = 1
		ORDER BY sub.observed_at DESC
	`, strings.Join(placeholders, ", "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	observations := make([]Observation, 0, len(marketUUIDs))
	for rows.Next() {
		var obs Observation
		if err := rows.Scan(
			&obs.ID, &obs.MarketID, &obs.Timestamp,
			&obs.YesProbability, &obs.NoProbability, &obs.VolumeUSDC, &obs.Liquidity,
		); err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return observations, nil
}
