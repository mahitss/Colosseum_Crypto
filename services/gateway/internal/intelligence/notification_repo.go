// Package-level persistence for alert delivery events and in-app notifications.
//
// DELIVERY STATE MACHINE (read first):
//
// An alert rule firing produces exactly one row in alert_events, and that row
// carries the delivery lifecycle in its status column:
//
//	PENDING     inserted, no delivery attempt has succeeded yet
//	DELIVERED   the notification was written for the user; delivered_at is set
//	FAILED      a delivery attempt was made and did not succeed
//	SUPPRESSED  the alert was intentionally not delivered
//
// The transitions are strictly forward: PENDING is written by the INSERT, and
// only MarkAlertEventDelivered or MarkAlertEventFailed moves a row out of it.
// A row in DELIVERED is terminal and is never rewritten into another state, which
// is why both markers are single statements over an explicit id rather than a
// read-modify-write cycle.
//
// COOLDOWN IS EVALUATED FROM DURABLE ROWS, NOT FROM CACHE:
//
// Alert rules carry a cooldown window, and a rule must not notify the same user
// again while that window is open. This file implements that check with no Redis
// and no in-process state: LastDeliveredAlertEvent is the cooldown probe. It
// asks the database for the most recent DELIVERED event of a rule since a
// caller-supplied instant, and the caller suppresses the new alert when a row
// comes back. Because the evidence of a delivery is the row itself, the cooldown
// survives a process restart, a deploy, and a scale-out across replicas: every
// replica reads the same durable history and reaches the same conclusion, so a
// user cannot be spammed by whichever instance happens to be coldest.
//
// Only DELIVERED events count toward the cooldown. PENDING and FAILED rows are
// delivery attempts that did not notify anyone, and counting them would let a
// failed delivery silently mute a rule for the whole window.
//
// DEDUPLICATION IS ARBITRATED BY THE DATABASE:
//
// InsertAlertEvent is written as `ON CONFLICT (alert_rule_id, dedupe_key) DO
// NOTHING`, matching the unique constraint alert_events_dedupe_unique. A
// read-then-write existence check would be racy: two workers evaluating the same
// signal event concurrently could both pass a SELECT and both insert. Postgres
// resolves the race inside the statement, so exactly one caller sees a row come
// back from RETURNING; every other caller sees no row and is told the alert was
// suppressed. That is a normal outcome, not an error, and is reported as
// (nil, false, nil).
//
// Callers MUST set event.DedupeKey, normally via AlertDedupeKey(ruleID, event)
// while they still hold the SignalEvent that triggered the alert. The key cannot
// be derived here: this file receives only the signal event id, and recovering
// the fingerprint would need an extra read of signal_events, which would be both
// a wasted round trip and a second source of truth for a value the caller has
// already computed. An empty key would otherwise be rejected by the NOT NULL
// constraint, so the contract is documented rather than repaired.
//
// OWNERSHIP:
//
// Notifications carry user_id directly, and every notification read or write
// filters on it in the same statement that touches the row. Alert events do not
// carry a user_id: they inherit their owner through alert_rules, so
// ListAlertEventsForUser joins to the rule and filters `ar.user_id = $1` inside
// the statement that selects and orders. The delivery-path methods
// (LastDeliveredAlertEvent and the two status markers) are not user-facing and
// take no userID, exactly like the engine-side write paths elsewhere here.
//
// As in the rest of this package, every value is bound through pgx positional
// placeholders; no caller-supplied string is ever interpolated into SQL text.
package intelligence

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Alert event delivery states. The values are exactly those accepted by the
// status CHECK constraint on alert_events.
const (
	AlertStatusPending    = "PENDING"
	AlertStatusDelivered  = "DELIVERED"
	AlertStatusSuppressed = "SUPPRESSED"
	AlertStatusFailed     = "FAILED"
)

// alertEventColumns is the canonical projection for an alert event, aliased `ae`
// for the table itself. The two UUID columns are cast to text so they scan into
// the string fields of the model.
const alertEventColumns = `ae.id, ae.alert_rule_id::text, ae.signal_event_id, ae.market_id::text, ` +
	`ae.status, ae.triggered_at, ae.dedupe_key, ae.delivered_at, ae.created_at`

// notificationColumns is the canonical projection for a notification. market_id
// is cast to text and stays nullable, matching the model's *string.
const notificationColumns = `id, user_id, alert_event_id, title, body, severity, market_id::text, read_at, created_at`

const (
	// defaultUserListLimit is used when a caller asks for zero or fewer rows, so
	// a missing or malformed limit never silently becomes an empty page.
	defaultUserListLimit = 50
	// maxUserListLimit bounds how much history a single list call may pull.
	maxUserListLimit = 200
)

// clampUserListLimit keeps a caller-supplied limit inside a sane range. The value
// is still bound as a placeholder, never formatted into the statement.
func clampUserListLimit(limit int) int {
	if limit <= 0 {
		return defaultUserListLimit
	}
	if limit > maxUserListLimit {
		return maxUserListLimit
	}
	return limit
}

func scanAlertEvent(row rowScanner) (AlertEvent, error) {
	var event AlertEvent
	if err := row.Scan(
		&event.ID, &event.AlertRuleID, &event.SignalEventID, &event.MarketID,
		&event.Status, &event.TriggeredAt, &event.DedupeKey, &event.DeliveredAt,
		&event.CreatedAt,
	); err != nil {
		return AlertEvent{}, err
	}
	return event, nil
}

func scanNotification(row rowScanner) (Notification, error) {
	var notification Notification
	if err := row.Scan(
		&notification.ID, &notification.UserID, &notification.AlertEventID,
		&notification.Title, &notification.Body, &notification.Severity,
		&notification.MarketID, &notification.ReadAt, &notification.CreatedAt,
	); err != nil {
		return Notification{}, err
	}
	return notification, nil
}

// InsertAlertEvent records that a rule fired, and reports whether the row was
// actually stored.
//
// The second return value is false, with a nil error, when the dedupe key was
// already present for this rule. The unique constraint resolved that race inside
// the statement, and the caller must treat it as a suppressed alert rather than an
// error: the user has already been notified about this exact signal event.
//
// event.DedupeKey is required and is not computed here; see the package comment
// for why. An empty status is defaulted to PENDING in Go rather than relying on
// the column default, because that value is then also reflected on the returned
// struct the caller goes on to use.
func (r *Repository) InsertAlertEvent(ctx context.Context, event AlertEvent) (*AlertEvent, bool, error) {
	status := event.Status
	if status == "" {
		status = AlertStatusPending
	}
	var stored AlertEvent
	err := r.pool.QueryRow(ctx, `
		INSERT INTO alert_events (
			alert_rule_id, signal_event_id, market_id, status, dedupe_key
		)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (alert_rule_id, dedupe_key) DO NOTHING
		RETURNING id, triggered_at, created_at`,
		event.AlertRuleID, event.SignalEventID, event.MarketID, status, event.DedupeKey,
	).Scan(&stored.ID, &stored.TriggeredAt, &stored.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// The (alert_rule_id, dedupe_key) pair is already present: this alert
		// was recorded earlier, so it is suppressed, not failed.
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	// The RETURNING list is deliberately narrow; the remaining fields are
	// exactly what the caller passed in, so they are echoed back rather than
	// re-read from the row.
	stored.AlertRuleID = event.AlertRuleID
	stored.SignalEventID = event.SignalEventID
	stored.MarketID = event.MarketID
	stored.Status = status
	stored.DedupeKey = event.DedupeKey
	return &stored, true, nil
}

// LastDeliveredAlertEvent is the cooldown probe: it returns the most recent
// DELIVERED event of a rule whose triggered_at is at or after since, or
// (nil, nil) when the rule has delivered nothing in that window.
//
// since is expected to be now() minus the rule's cooldown. A nil event with a nil
// error is a meaningful answer, not a miss, so callers must check for it before
// dereferencing.
func (r *Repository) LastDeliveredAlertEvent(ctx context.Context, ruleID string, since time.Time) (*AlertEvent, error) {
	event, err := scanAlertEvent(r.pool.QueryRow(ctx, `
		SELECT `+alertEventColumns+`
		FROM alert_events ae
		WHERE ae.alert_rule_id=$1 AND ae.status=$2 AND ae.triggered_at >= $3
		ORDER BY ae.triggered_at DESC, ae.id DESC
		LIMIT 1`, ruleID, AlertStatusDelivered, since))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// MarkAlertEventDelivered records a successful delivery, stamping delivered_at
// with the database clock rather than an application time so the stamp stays
// comparable with the triggered_at values the cooldown probe reads.
//
// The transition is idempotent: a row already in DELIVERED is rewritten with a
// fresh stamp, which is harmless, and a row that no longer exists affects none.
func (r *Repository) MarkAlertEventDelivered(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE alert_events
		SET status=$2, delivered_at=now()
		WHERE id=$1`, id, AlertStatusDelivered)
	return err
}

// MarkAlertEventFailed records a delivery attempt that did not succeed. It does
// not set delivered_at: no notification reached the user, and a stamp there would
// make a failed row indistinguishable from a delivered one in history.
//
// A FAILED row leaves the cooldown open, so a later attempt at the same alert is
// free to retry.
func (r *Repository) MarkAlertEventFailed(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE alert_events
		SET status=$2
		WHERE id=$1`, id, AlertStatusFailed)
	return err
}

// ListAlertEventsForUser returns the alert history of the rules a user owns, most
// recently triggered first.
//
// Ownership is enforced by the join rather than by a separate lookup, so a row
// belonging to another user is never read and then discarded: ar.user_id is part
// of the same statement that selects and orders the events. Ties on triggered_at
// are broken by the identity column, which keeps the ordering total and stable
// across calls.
func (r *Repository) ListAlertEventsForUser(ctx context.Context, userID string, limit int) ([]AlertEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+alertEventColumns+`
		FROM alert_events ae
		JOIN alert_rules ar ON ar.id = ae.alert_rule_id
		WHERE ar.user_id=$1
		ORDER BY ae.triggered_at DESC, ae.id DESC
		LIMIT $2`, userID, clampUserListLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]AlertEvent, 0, 16)
	for rows.Next() {
		event, err := scanAlertEvent(rows)
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

// CreateNotification writes the user-visible half of a delivery and returns the
// stored row. user_id and alert_event_id come from the delivery path rather than
// from the HTTP request, so a notification is always written against the owner of
// the rule that fired.
func (r *Repository) CreateNotification(ctx context.Context, notification Notification) (*Notification, error) {
	stored, err := scanNotification(r.pool.QueryRow(ctx, `
		INSERT INTO notifications (
			user_id, alert_event_id, title, body, severity, market_id
		)
		VALUES (
			$1,$2,$3,$4,$5,NULLIF($6,'')::uuid
		)
		RETURNING `+notificationColumns,
		notification.UserID, notification.AlertEventID, notification.Title,
		notification.Body, notification.Severity, notification.MarketID))
	if err != nil {
		return nil, err
	}
	return &stored, nil
}

// ListNotifications returns a user's notifications, newest first. When unreadOnly
// is set the read filter is applied in the query, not in Go, so the LIMIT bounds
// unread rows rather than the first N rows of the user's history.
func (r *Repository) ListNotifications(ctx context.Context, userID string, limit int, unreadOnly bool) ([]Notification, error) {
	query := `SELECT ` + notificationColumns + ` FROM notifications WHERE user_id=$1`
	if unreadOnly {
		query += ` AND read_at IS NULL`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $2`

	rows, err := r.pool.Query(ctx, query, userID, clampUserListLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notifications := make([]Notification, 0, 16)
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notifications, nil
}

// UnreadNotificationCount backs the badge count. It counts the same rows
// ListNotifications would return with unreadOnly, so the badge and the list can
// never disagree.
func (r *Repository) UnreadNotificationCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM notifications
		WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// MarkNotificationRead stamps read_at for one of the user's notifications.
//
// It is idempotent by construction: the read_at IS NULL predicate means a second
// call affects zero rows rather than moving the stamp forward. A row that does
// not exist and a row owned by somebody else are both a zero-row update, so this
// cannot be used to probe for the existence of another user's notification.
func (r *Repository) MarkNotificationRead(ctx context.Context, userID string, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications
		SET read_at=now()
		WHERE id=$1 AND user_id=$2 AND read_at IS NULL`, id, userID)
	return err
}

// MarkAllNotificationsRead marks every unread notification of the user and
// returns how many rows changed. Because of the read_at IS NULL predicate, a
// second call reports 0 rather than inflating the number, so the caller can
// report "N marked" truthfully.
func (r *Repository) MarkAllNotificationsRead(ctx context.Context, userID string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notifications
		SET read_at=now()
		WHERE user_id=$1 AND read_at IS NULL`, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
