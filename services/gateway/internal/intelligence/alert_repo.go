// Package-level alert rule persistence for the intelligence repository.
//
// OWNERSHIP-FILTER INVARIANT (read first):
//
// Every method here that takes a userID MUST scope its SQL by `user_id = $N` in
// the same statement that reads or writes the target row. Ownership is never
// verified by a separate follow-up lookup, so there is no window in which a row
// could be resolved before its owner is known, and no caller-supplied value is
// ever interpolated into SQL text: every input is bound through pgx positional
// placeholders ($1, $2, ...) only.
//
// Consequences callers can rely on:
//   - Reading or mutating another user's rule yields ErrAlertRuleNotFound (or a
//     successful no-op for the idempotent delete), never their data.
//   - A missing rule and a foreign-owned rule are indistinguishable, so rule ids
//     cannot be probed for existence.
//   - ListAlertRules filters inside the same query that orders, so no row is read
//     unscoped and then filtered in Go.
//
// Optional rule attributes (watchlist_id, market_id, signal_type,
// minimum_severity and the three NUMERIC thresholds) are written through a
// NULLIF against the empty string, so an empty value from a JSON request body is
// stored as NULL instead of failing a CHECK constraint or a UUID cast. The
// numeric thresholds are read back with ::text casts into *string, preserving
// the exact stored precision of NUMERIC(38, 12).
package intelligence

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var (
	ErrAlertRuleNotFound    = errors.New("alert rule not found")
	ErrNotificationNotFound = errors.New("notification not found")
)

const alertRuleColumns = `id::text,user_id,watchlist_id::text,market_id::text,name,enabled,signal_type,minimum_severity,` +
	`probability_change_threshold::text,activity_change_threshold::text,liquidity_change_threshold::text,` +
	`cooldown_seconds,created_at,updated_at`

// rowScanner is satisfied by both pgx.Row and pgx.Rows so every read path shares
// one scan implementation.
type rowScanner interface{ Scan(dest ...any) error }

func scanAlertRule(row rowScanner) (*AlertRule, error) {
	var rule AlertRule
	if err := row.Scan(
		&rule.ID, &rule.UserID, &rule.WatchlistID, &rule.MarketID, &rule.Name,
		&rule.Enabled, &rule.SignalType, &rule.MinimumSeverity,
		&rule.ProbabilityChangeThreshold, &rule.ActivityChangeThreshold, &rule.LiquidityChangeThreshold,
		&rule.CooldownSeconds, &rule.CreatedAt, &rule.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateAlertRule inserts a rule owned by rule.UserID. The scope comes from the
// rule itself: exactly one of WatchlistID, MarketID, or neither.
func (r *Repository) CreateAlertRule(ctx context.Context, rule AlertRule) (*AlertRule, error) {
	return scanAlertRule(r.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (
			user_id,watchlist_id,market_id,name,enabled,signal_type,minimum_severity,
			probability_change_threshold,activity_change_threshold,liquidity_change_threshold,cooldown_seconds
		)
		VALUES (
			$1,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,$4,$5,NULLIF($6,''),NULLIF($7,''),
			NULLIF($8,'')::numeric,NULLIF($9,'')::numeric,NULLIF($10,'')::numeric,$11
		)
		RETURNING `+alertRuleColumns,
		rule.UserID, rule.WatchlistID, rule.MarketID, rule.Name, rule.Enabled,
		rule.SignalType, rule.MinimumSeverity, rule.ProbabilityChangeThreshold,
		rule.ActivityChangeThreshold, rule.LiquidityChangeThreshold, rule.CooldownSeconds))
}

func (r *Repository) GetAlertRule(ctx context.Context, userID, ruleID string) (*AlertRule, error) {
	rule, err := scanAlertRule(r.pool.QueryRow(ctx, `
		SELECT `+alertRuleColumns+` FROM alert_rules
		WHERE id=$1 AND user_id=$2`, ruleID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAlertRuleNotFound
	}
	if err != nil {
		return nil, err
	}
	return rule, nil
}

// ListAlertRules returns the user's rules, newest first.
func (r *Repository) ListAlertRules(ctx context.Context, userID string) ([]AlertRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+alertRuleColumns+` FROM alert_rules
		WHERE user_id=$1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]AlertRule, 0, 16)
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

// UpdateAlertRule rewrites the mutable fields of a rule the user owns. The
// scope columns (watchlist_id, market_id) are immutable here; empty optional
// values are again coerced to NULL rather than stored as empty strings.
func (r *Repository) UpdateAlertRule(ctx context.Context, userID, ruleID string, rule AlertRule) (*AlertRule, error) {
	updated, err := scanAlertRule(r.pool.QueryRow(ctx, `
		UPDATE alert_rules
		SET name=$3, enabled=$4, signal_type=NULLIF($5,''), minimum_severity=NULLIF($6,''),
			probability_change_threshold=NULLIF($7,'')::numeric,
			activity_change_threshold=NULLIF($8,'')::numeric,
			liquidity_change_threshold=NULLIF($9,'')::numeric,
			cooldown_seconds=$10, updated_at=now()
		WHERE id=$1 AND user_id=$2
		RETURNING `+alertRuleColumns,
		ruleID, userID, rule.Name, rule.Enabled, rule.SignalType, rule.MinimumSeverity,
		rule.ProbabilityChangeThreshold, rule.ActivityChangeThreshold, rule.LiquidityChangeThreshold,
		rule.CooldownSeconds))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAlertRuleNotFound
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteAlertRule removes a rule owned by the user. It is idempotent: deleting
// an already-deleted or a foreign rule is a successful no-op, so the caller
// cannot distinguish the two cases.
func (r *Repository) DeleteAlertRule(ctx context.Context, userID, ruleID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1 AND user_id=$2`, ruleID, userID)
	return err
}

// alertRuleColumnsPrefixed is the same projection as alertRuleColumns with the
// alert_rules alias applied to every column. It is required for any query that
// joins another table also carrying an `id`, `created_at` or `updated_at` column,
// because the unprefixed projection would be ambiguous and Postgres would reject
// it rather than guess.
const alertRuleColumnsPrefixed = `ar.id::text,ar.user_id,ar.watchlist_id::text,ar.market_id::text,ar.name,ar.enabled,` +
	`ar.signal_type,ar.minimum_severity,ar.probability_change_threshold::text,` +
	`ar.activity_change_threshold::text,ar.liquidity_change_threshold::text,` +
	`ar.cooldown_seconds,ar.created_at,ar.updated_at`

// maxEnabledRulesForMarket bounds the candidate rule set a single signal event
// can be evaluated against. The join below can return a large set for a market
// on many watchlists, and the evaluator then does at least one indexed
// LastDeliveredAlertEvent probe per matched rule, so the set has to be bounded at
// the query rather than trusted to stay small. The ordering is stable
// (created_at, id) so the truncation always keeps the same rules rather than an
// arbitrary subset that could change between calls.
const maxEnabledRulesForMarket = 500

// EnabledRulesForMarket returns every enabled rule that could apply to a signal
// event on the given market, addressed by the market's own internal UUID.
//
// A rule qualifies when it is global (neither market nor watchlist scoped),
// market scoped to this market, or watchlist scoped to a watchlist that contains
// this market. The membership test lives in the SQL rather than in Go so no row
// is read and then discarded, and the watchlist join is a LEFT JOIN constrained
// to this market, which yields at most one joined row per rule because
// watchlist_markets is keyed on (watchlist_id, market_id). The evaluator still
// deduplicates by rule id before delivering, so a future change to the join
// cannot produce two notifications for one rule.
//
// This is a system lookup, not a user-facing one: alert evaluation is not acting
// on behalf of a single user, it is fanning an event out to every rule that
// covers the market, so there is no user scope to apply. Ownership is not
// discarded - each returned rule still carries its own user_id, which is what the
// evaluator uses to address the notification.
func (r *Repository) EnabledRulesForMarket(ctx context.Context, marketUUID string) ([]AlertRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+alertRuleColumnsPrefixed+`
		FROM alert_rules ar
		LEFT JOIN watchlist_markets wm
			ON wm.watchlist_id = ar.watchlist_id AND wm.market_id = ar.market_id
		WHERE ar.enabled = true
			AND (ar.market_id IS NULL OR ar.market_id = $1::uuid)
			AND (ar.watchlist_id IS NULL OR wm.market_id IS NOT NULL)
		ORDER BY ar.created_at, ar.id
		LIMIT $2`, marketUUID, maxEnabledRulesForMarket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]AlertRule, 0, 16)
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}
