package intelligence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) UpsertMarket(ctx context.Context, transaction pgx.Tx, market Market) (id string, isNew bool, err error) {
	err = transaction.QueryRow(ctx, `
		INSERT INTO markets (
			source, source_market_id, title, description, category, status, phase,
			yes_probability, no_probability, liquidity, volume_usdc, created_at, closes_at, resolution_status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (source, source_market_id) DO UPDATE SET
			title=EXCLUDED.title, description=EXCLUDED.description, category=EXCLUDED.category,
			status=EXCLUDED.status, phase=EXCLUDED.phase, yes_probability=EXCLUDED.yes_probability,
			no_probability=EXCLUDED.no_probability, liquidity=EXCLUDED.liquidity, volume_usdc=EXCLUDED.volume_usdc,
			created_at=EXCLUDED.created_at, closes_at=EXCLUDED.closes_at,
			resolution_status=EXCLUDED.resolution_status, updated_at=now()
		RETURNING id::text, (xmax = 0)`, market.Source, market.SourceMarketID, market.Title,
		market.Description, market.Category, market.Status, market.Phase, market.YesProbability,
		market.NoProbability, market.Liquidity, market.VolumeUSDC, market.CreatedAt, market.ClosesAt,
		market.ResolutionStatus).Scan(&id, &isNew)
	return id, isNew, err
}

type rowQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *Repository) PreviousObservation(ctx context.Context, transaction rowQueryer, marketID string, before time.Time) (*Observation, error) {
	return readObservation(ctx, transaction, `SELECT id,market_id::text,observed_at,yes_probability::text,no_probability::text,volume_usdc::text,liquidity::text FROM market_observations WHERE market_id=$1 AND observed_at < $2 ORDER BY observed_at DESC,id DESC LIMIT 1`, marketID, before)
}

func (r *Repository) WindowObservation(ctx context.Context, transaction rowQueryer, marketID string, before time.Time) (*Observation, error) {
	return readObservation(ctx, transaction, `SELECT id,market_id::text,observed_at,yes_probability::text,no_probability::text,volume_usdc::text,liquidity::text FROM market_observations WHERE market_id=$1 AND observed_at <= $2 ORDER BY observed_at DESC,id DESC LIMIT 1`, marketID, before)
}

func readObservation(ctx context.Context, transaction rowQueryer, query, marketID string, before time.Time) (*Observation, error) {
	var observation Observation
	err := transaction.QueryRow(ctx, query, marketID, before).Scan(
		&observation.ID, &observation.MarketID, &observation.Timestamp,
		&observation.YesProbability, &observation.NoProbability, &observation.VolumeUSDC, &observation.Liquidity,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &observation, nil
}

func (r *Repository) InsertObservation(ctx context.Context, transaction pgx.Tx, observation Observation) (int64, bool, error) {
	var id int64
	err := transaction.QueryRow(ctx, `
		INSERT INTO market_observations (market_id,observed_at,yes_probability,no_probability,volume_usdc,liquidity)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (market_id,observed_at) DO NOTHING
		RETURNING id`, observation.MarketID, observation.Timestamp, observation.YesProbability,
		observation.NoProbability, observation.VolumeUSDC, observation.Liquidity).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (r *Repository) InsertSignals(ctx context.Context, transaction pgx.Tx, signals []Signal) (int, error) {
	created := 0
	for _, signal := range signals {
		command, err := transaction.Exec(ctx, `
			INSERT INTO signals (
				market_id,observation_id,timestamp,signal_type,severity,metric,previous_value,current_value,
				absolute_change,percentage_change,percentage_points,observation_window,source
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (market_id,observation_id,signal_type,metric) DO NOTHING`,
			signal.MarketID, signal.ObservationID, signal.Timestamp, signal.SignalType, signal.Severity,
			signal.Metric, signal.PreviousValue, signal.CurrentValue, signal.AbsoluteChange,
			signal.PercentageChange, signal.PercentagePoints, signal.ObservationWindow, signal.Source)
		if err != nil {
			return created, err
		}
		created += int(command.RowsAffected())
	}
	return created, nil
}

func (r *Repository) Signals(ctx context.Context, query SignalQuery) (SignalPage, error) {
	countWhere, countArgs := signalWhere(query, false, 0, time.Time{})
	var page SignalPage
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM signals s JOIN markets m ON m.id=s.market_id `+countWhere, countArgs...).Scan(&page.Total); err != nil {
		return SignalPage{}, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM markets`).Scan(&page.MarketCount); err != nil {
		return SignalPage{}, err
	}
	var cursorTimestamp time.Time
	var cursorID int64
	if query.Cursor != "" {
		parsedTimestamp, parsedID, err := decodeSignalCursor(query.Cursor)
		if err != nil {
			return SignalPage{}, err
		}
		cursorTimestamp, cursorID = parsedTimestamp, parsedID
	}
	where, args := signalWhere(query, true, cursorID, cursorTimestamp)
	limitArgs := append(append([]any{}, args...), query.Limit+1)
	rows, err := r.pool.Query(ctx, `SELECT s.id,s.market_id::text,m.source_market_id,s.timestamp,s.signal_type,s.severity,s.metric,s.previous_value::text,s.current_value::text,s.absolute_change::text,s.percentage_change::text,s.percentage_points::text,s.observation_window,s.source FROM signals s JOIN markets m ON m.id=s.market_id `+where+` ORDER BY s.timestamp DESC,s.id DESC LIMIT $`+fmt.Sprint(len(limitArgs)), limitArgs...)
	if err != nil {
		return SignalPage{}, err
	}
	defer rows.Close()
	page.Items = make([]Signal, 0, query.Limit)
	for rows.Next() {
		var signal Signal
		if err := rows.Scan(&signal.ID, &signal.MarketID, &signal.SourceMarketID, &signal.Timestamp, &signal.SignalType, &signal.Severity,
			&signal.Metric, &signal.PreviousValue, &signal.CurrentValue, &signal.AbsoluteChange,
			&signal.PercentageChange, &signal.PercentagePoints, &signal.ObservationWindow, &signal.Source); err != nil {
			return SignalPage{}, err
		}
		page.Items = append(page.Items, signal)
	}
	if err := rows.Err(); err != nil {
		return SignalPage{}, err
	}
	if len(page.Items) > query.Limit {
		page.Items = page.Items[:query.Limit]
		last := page.Items[len(page.Items)-1]
		cursor, err := encodeSignalCursor(last.Timestamp, last.ID)
		if err != nil {
			return SignalPage{}, err
		}
		page.NextCursor = &cursor
	}
	return page, nil
}

func signalWhere(query SignalQuery, includeCursor bool, cursorID int64, cursorTimestamp time.Time) (string, []any) {
	clauses := []string{"TRUE"}
	args := make([]any, 0, 7)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if query.MarketID != "" {
		add("m.source='panta' AND m.source_market_id=$%d", query.MarketID)
	}
	if query.SignalType != "" {
		add("s.signal_type=$%d", query.SignalType)
	}
	if query.Severity != "" {
		add("s.severity=$%d", query.Severity)
	}
	if query.From != nil {
		add("s.timestamp >= $%d", *query.From)
	}
	if query.To != nil {
		add("s.timestamp <= $%d", *query.To)
	}
	if includeCursor && cursorID > 0 {
		args = append(args, cursorTimestamp, cursorID)
		clauses = append(clauses, fmt.Sprintf("(s.timestamp,s.id) < ($%d,$%d)", len(args)-1, len(args)))
	}
	return "WHERE " + joinAnd(clauses), args
}

type signalCursor struct {
	Timestamp time.Time `json:"timestamp"`
	ID        int64     `json:"id"`
}

func encodeSignalCursor(timestamp time.Time, id int64) (string, error) {
	value, err := json.Marshal(signalCursor{Timestamp: timestamp, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func decodeSignalCursor(value string) (time.Time, int64, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor")
	}
	var cursor signalCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.ID <= 0 || cursor.Timestamp.IsZero() {
		return time.Time{}, 0, fmt.Errorf("invalid cursor")
	}
	return cursor.Timestamp, cursor.ID, nil
}

func (r *Repository) MarketIntelligence(ctx context.Context, sourceMarketID string) (MarketIntelligence, error) {
	var result MarketIntelligence
	err := r.pool.QueryRow(ctx, `SELECT id::text,source,source_market_id,title,description,category,status,phase,yes_probability::text,no_probability::text,liquidity::text,volume_usdc::text,created_at,closes_at,resolution_status FROM markets WHERE source_market_id=$1`, sourceMarketID).Scan(
		&result.Market.ID, &result.Market.Source, &result.Market.SourceMarketID, &result.Market.Title,
		&result.Market.Description, &result.Market.Category, &result.Market.Status, &result.Market.Phase,
		&result.Market.YesProbability, &result.Market.NoProbability, &result.Market.Liquidity,
		&result.Market.VolumeUSDC, &result.Market.CreatedAt, &result.Market.ClosesAt, &result.Market.ResolutionStatus,
	)
	if err != nil {
		return MarketIntelligence{}, err
	}
	latest, err := readObservation(ctx, r.pool, `SELECT id,market_id::text,observed_at,yes_probability::text,no_probability::text,volume_usdc::text,liquidity::text FROM market_observations WHERE market_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1`, result.Market.ID, time.Now().Add(time.Nanosecond))
	if err != nil {
		return MarketIntelligence{}, err
	}
	result.LatestObservation = latest
	if latest != nil {
		latest.MarketID = result.Market.SourceMarketID
	}
	var observationCount int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM market_observations WHERE market_id=$1`, result.Market.ID).Scan(&observationCount); err != nil {
		return MarketIntelligence{}, err
	}
	result.CurrentMetrics = MarketMetrics{InsufficientData: observationCount < 2}
	if latest != nil {
		result.CurrentMetrics.YesProbability = latest.YesProbability
		result.CurrentMetrics.NoProbability = latest.NoProbability
		result.CurrentMetrics.VolumeUSDC = latest.VolumeUSDC
		result.CurrentMetrics.Liquidity = latest.Liquidity
		result.CurrentMetrics.ObservedAt = &latest.Timestamp
	}
	rows, err := r.pool.Query(ctx, `SELECT s.id,s.market_id::text,m.source_market_id,s.timestamp,s.signal_type,s.severity,s.metric,s.previous_value::text,s.current_value::text,s.absolute_change::text,s.percentage_change::text,s.percentage_points::text,s.observation_window,s.source FROM signals s JOIN markets m ON m.id=s.market_id WHERE s.market_id=$1 ORDER BY s.timestamp DESC,s.id DESC LIMIT 20`, result.Market.ID)
	if err != nil {
		return MarketIntelligence{}, err
	}
	defer rows.Close()
	result.RecentSignals = make([]Signal, 0, 20)
	for rows.Next() {
		var signal Signal
		if err := rows.Scan(&signal.ID, &signal.MarketID, &signal.SourceMarketID, &signal.Timestamp, &signal.SignalType, &signal.Severity,
			&signal.Metric, &signal.PreviousValue, &signal.CurrentValue, &signal.AbsoluteChange,
			&signal.PercentageChange, &signal.PercentagePoints, &signal.ObservationWindow, &signal.Source); err != nil {
			return MarketIntelligence{}, err
		}
		signal.MarketID = signal.SourceMarketID
		result.RecentSignals = append(result.RecentSignals, signal)
	}
	return result, rows.Err()
}

func (r *Repository) MarketBySourceID(ctx context.Context, sourceMarketID string) (Market, error) {
	var market Market
	err := r.pool.QueryRow(ctx, `SELECT id::text,source,source_market_id,title,description,category,status,phase,yes_probability::text,no_probability::text,liquidity::text,volume_usdc::text,created_at,closes_at,resolution_status FROM markets WHERE source_market_id=$1`, sourceMarketID).Scan(
		&market.ID, &market.Source, &market.SourceMarketID, &market.Title, &market.Description, &market.Category,
		&market.Status, &market.Phase, &market.YesProbability, &market.NoProbability, &market.Liquidity,
		&market.VolumeUSDC, &market.CreatedAt, &market.ClosesAt, &market.ResolutionStatus)
	return market, err
}

func (r *Repository) MarketCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM markets WHERE source='panta'`).Scan(&count)
	return count, err
}

func (r *Repository) LatestObservation(ctx context.Context, transaction rowQueryer, marketID string) (*Observation, error) {
	return readObservation(ctx, transaction, `SELECT id,market_id::text,observed_at,yes_probability::text,no_probability::text,volume_usdc::text,liquidity::text FROM market_observations WHERE market_id=$1 ORDER BY observed_at DESC,id DESC LIMIT 1`, marketID, time.Now().Add(time.Nanosecond))
}

func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) { return r.pool.Begin(ctx) }
func (r *Repository) Close()                                    { r.pool.Close() }

func joinAnd(parts []string) string {
	result := ""
	for index, part := range parts {
		if index > 0 {
			result += " AND "
		}
		result += part
	}
	return result
}
