// Deterministic alert evaluation pipeline.
//
// This file is the only place where the pure matching logic in alertlogic.go is
// joined to persistence and delivery. It is deliberately boring I/O glue: every
// decision about *whether* an alert fires was already made by MatchesRule, and
// every sentence shown to a user was already assembled by ExplainSignal or
// ExplainMatch. Nothing here consults a model, samples, or guesses.
//
// NO LLM ANYWHERE IN THIS FILE. Alert evaluation is a fan-out over a bounded
// candidate set followed by a bounded delivery loop, and both the membership
// test and the copy shown to the user are total functions of stored values. An
// LLM in this path would be actively harmful: a rule that says "SIGNIFICANT or
// worse, probability shift of 10pp" must fire on exactly the events its author
// predicted, every time, and a user must be able to re-derive the outcome from
// the rule and the event alone.
//
// THE PIPELINE, IN ORDER, AND WHY THE ORDER MATTERS:
//
//  1. Fingerprint + market resolution. An event with no market is not
//     evaluable: every rule scope, the dedupe key, and the notification all key
//     on the market, and a fabricated market would attribute an alert to a
//     market that never generated it. We fail instead of guessing.
//  2. Signal event insert. If the insert reports created=false, the fingerprint
//     was already recorded. That is duplicate-signal suppression, and it is
//     checked BEFORE any rule work: the alert rows that this event would produce
//     already exist too, so evaluating rules again would be duplicated work
//     whose every outcome is a suppression. Returning early also keeps the
//     cooldown probe from being charged for a signal that never arrived.
//  3. Candidate rules. The set is bounded by the store query and by
//     maxAlertsPerEvent.
//  4. Cooldown, then dedupe insert. See COOLDOWN BEFORE DEDUPE below.
//  5. Delivery. A failure is recorded and the loop continues; see DELIVERY
//     FAILURES DO NOT ABORT below.
//
// COOLDOWN BEFORE DEDUPE (why the order is cooldown first):
//
// Both gates suppress the same alert, and they answer different questions. The
// dedupe key asks "have we already recorded THIS alert for THIS signal event?"
// and is decided by a unique constraint. The cooldown asks "did this rule
// notify this user recently?" and is a question about the rule's history.
//
// Order matters because the cooldown is the gate that can suppress an alert
// whose dedupe key has never been seen. If the dedupe insert ran first, a rule
// in cooldown would get a brand new alert_events row in PENDING, and that row
// would then have to be either deleted or moved to SUPPRESSED to keep the state
// machine honest. Worse, a naive implementation would mark it DELIVERED, since
// a PENDING row left behind by a suppressed alert looks exactly like a delivery
// attempt that has not been resolved yet.
//
// Checking cooldown first means a suppressed alert leaves no row at all, which
// is both cheaper and the truthful record: the user was not notified, and the
// history says so. It also protects the durable cooldown evidence, since
// LastDeliveredAlertEvent only counts DELIVERED rows, so a suppressed alert
// cannot poison the next window.
//
// The converse cost of this order is accepted and is the smaller one: a rule
// suppressed by cooldown has no dedupe-key record, so if the same signal event
// is re-evaluated after the cooldown expires, it is evaluated again. That is the
// correct behaviour, not a bug. The signal event is itself deduplicated upstream
// in step 2, so a re-evaluation of the same fingerprint does not happen in
// practice, and when it legitimately does the user genuinely should be told.
//
// DELIVERY FAILURES DO NOT ABORT (why one failure must not stop the loop):
//
// A candidate rule set is a fan-out across independent users and watchlists. If
// rule A's send fails, nothing about rule B's send changes: B may be owned by a
// different user, write to a different row, and depend on no shared state. Let
// one transport error cancel the remaining deliveries would convert a single
// transient failure into a silent outage for every user after the first, and the
// most affected users would be exactly those later in the ordering, which is
// arbitrary from their point of view.
//
// We therefore record the failure on the rule's own alert event
// (MarkAlertEventFailed), count it, and continue. The failed row does not count
// toward the cooldown, so the alert remains eligible for a later retry. The
// caller still gets an error if the context was cancelled, so a genuine shutdown
// is not mistaken for a normal completion.
package intelligence

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// ErrAlertMarketUnresolved is returned when a signal event cannot be attributed
// to a stored market. Evaluation is refused rather than continued against an
// assumed market id.
var ErrAlertMarketUnresolved = errors.New("alert evaluation: signal event market could not be resolved")

// AlertStore is the persistence surface the evaluator needs. The *Repository
// implementation satisfies it; tests can supply a fake.
type AlertStore interface {
	InsertSignalEvent(context.Context, SignalEvent) (*SignalEvent, bool, error)
	EnabledRulesForMarket(context.Context, string) ([]AlertRule, error)
	WatchlistIDsForMarket(context.Context, string) ([]string, error)
	InsertAlertEvent(context.Context, AlertEvent) (*AlertEvent, bool, error)
	LastDeliveredAlertEvent(context.Context, string, time.Time) (*AlertEvent, error)
	MarkAlertEventDelivered(context.Context, int64) error
	MarkAlertEventFailed(context.Context, int64) error
	MarketBySourceID(context.Context, string) (Market, error)
}

// NotificationSender delivers an alert. Alert evaluation never depends on a
// concrete transport, so email/webhook providers can be added later without
// touching the matching logic.
type NotificationSender interface {
	Send(ctx context.Context, notification Notification) error
}

// notificationCreator is the narrow write surface an in-app sender needs. It is
// deliberately a single method rather than the full repository so a test can
// supply a one-field fake and so the sender cannot reach into unrelated tables.
type notificationCreator interface {
	CreateNotification(context.Context, Notification) (*Notification, error)
}

// InAppSender is the NotificationSender that writes the notification into the
// in-app inbox. It is the default transport precisely because it needs no
// external dependency: delivery success is a successful row write.
type InAppSender struct {
	store notificationCreator
}

// NewInAppSender returns a sender that persists notifications through store.
func NewInAppSender(store notificationCreator) *InAppSender {
	return &InAppSender{store: store}
}

// Send writes the notification and returns the store error unchanged, so the
// evaluator records the failure without needing to interpret it.
func (s *InAppSender) Send(ctx context.Context, notification Notification) error {
	if s == nil || s.store == nil {
		return errors.New("alert evaluation: in-app sender has no store")
	}
	_, err := s.store.CreateNotification(ctx, notification)
	return err
}

// defaultMaxAlertsPerEvent bounds the delivery fan-out for a single signal
// event. A market that moves hard can satisfy many rules at once, and without a
// cap one event could produce an unbounded number of notifications and writes.
// The bound is on MATCHED rules, which is the set that does work, so the cost of
// evaluation is bounded rather than merely the cost of delivery.
const defaultMaxAlertsPerEvent = 50

// AlertEvaluator runs a signal event past the rules that apply to its market and
// delivers notifications for the ones that match.
//
// The struct holds no mutable state beyond the clock, so a single evaluator is
// safe for concurrent use: every field is read-only after construction, and the
// only per-event data lives on the stack of EvaluateEvent.
type AlertEvaluator struct {
	store             AlertStore
	sender            NotificationSender
	now               func() time.Time
	maxAlertsPerEvent int
}

// NewAlertEvaluator wires an evaluator to its store and transport. The clock is
// time.Now and the fan-out bound is the package default; both are fields rather
// than literals so tests can pin them without reaching into a global.
func NewAlertEvaluator(store AlertStore, sender NotificationSender) *AlertEvaluator {
	return &AlertEvaluator{
		store:             store,
		sender:            sender,
		now:               time.Now,
		maxAlertsPerEvent: defaultMaxAlertsPerEvent,
	}
}

// EvaluationResult reports what a single evaluation did. The counters are the
// operational picture of one event and are meant to be logged as they are, so
// they distinguish "suppressed because the rule was in cooldown" from
// "suppressed because this exact alert was already recorded":
//
//	Matched     rules that passed MatchesRule, including ones later suppressed
//	Suppressed  alerts that were deliberately not delivered (cooldown + dedupe)
//	Cooldowns   the subset of Suppressed caused by an open cooldown window
//	Delivered   notifications successfully handed to the sender
//	Failed      deliveries attempted and rejected by the sender
type EvaluationResult struct {
	Event      SignalEvent
	Matched    int
	Suppressed int
	Delivered  int
	Failed     int
	Cooldowns  int
}

// EvaluateEvent persists a signal event and delivers an alert for every
// applicable rule that matches it.
//
// The returned error is reserved for conditions that make evaluation impossible:
// an unresolvable market, a store failure, or a cancelled context. A delivery
// failure against a single rule is not one of these; it is counted in Failed and
// recorded on that rule's alert event, and evaluation continues.
func (e *AlertEvaluator) EvaluateEvent(ctx context.Context, event SignalEvent) (EvaluationResult, error) {
	result := EvaluationResult{Event: event}
	if e == nil || e.store == nil {
		return result, errors.New("alert evaluation: no store configured")
	}
	if e.sender == nil {
		return result, errors.New("alert evaluation: no notification sender configured")
	}

	// Step 1: fingerprint and market resolution. The fingerprint is computed
	// here rather than left to the repository so the value this function later
	// feeds to AlertDedupeKey is byte-identical to the one the unique
	// constraint on signal_events will arbitrate on.
	if event.Fingerprint == "" {
		event.Fingerprint = SignalFingerprint(event)
	}
	marketUUID, err := e.resolveMarket(ctx, event)
	if err != nil {
		return result, err
	}
	event.MarketID = marketUUID
	result.Event = event

	// Step 2: persist the event. A false created flag means the fingerprint was
	// already present, which is duplicate-signal suppression: this exact
	// observation is already in the stream and its alerts already exist, so the
	// correct outcome is zero work, not an error and not a re-notification.
	stored, created, err := e.store.InsertSignalEvent(ctx, event)
	if err != nil {
		return result, err
	}
	if !created {
		return result, nil
	}
	// Prefer the stored row: it carries the database-assigned ID that
	// alert_events.signal_event_id must reference, and the RETURNING projection
	// is authoritative for the timestamps.
	if stored != nil {
		event = *stored
		if event.Fingerprint == "" {
			event.Fingerprint = SignalFingerprint(event)
		}
		if event.MarketID == "" {
			event.MarketID = marketUUID
		}
	}
	result.Event = event

	rules, err := e.store.EnabledRulesForMarket(ctx, marketUUID)
	if err != nil {
		return result, err
	}
	// The watchlist membership set is fetched once per event, not once per rule.
	// It is also the only correct way to answer MatchesRule's marketInWatchlist
	// question: a rule scoped to a watchlist fires only when the market is
	// actually on that watchlist.
	watchlistIDs, err := e.store.WatchlistIDsForMarket(ctx, marketUUID)
	if err != nil {
		return result, err
	}
	inWatchlist := make(map[string]bool, len(watchlistIDs))
	for _, id := range watchlistIDs {
		inWatchlist[id] = true
	}

	// A bound of zero or less would mean "deliver nothing", which is never the
	// intent of a zero-valued field, so fall back to the default rather than
	// silently muting alerts.
	bound := e.maxAlertsPerEvent
	if bound <= 0 {
		bound = defaultMaxAlertsPerEvent
	}

	// seen guards against a store that returns the same rule twice, which a
	// join in the underlying query could do. Delivering the same rule twice
	// would produce two identical notifications, and the dedupe key would not
	// catch it because both rows carry the same rule id and the same event.
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		if seen[rule.ID] {
			continue
		}
		seen[rule.ID] = true

		marketInWatchlist := rule.WatchlistID != nil && inWatchlist[*rule.WatchlistID]
		if !MatchesRule(rule, event, marketInWatchlist) {
			continue
		}
		result.Matched++

		// Step 9: bound the fan-out. The cap is applied at match time and the
		// match that would exceed it is not counted, so Matched always reports
		// the rules that were actually worked through.
		if result.Matched > bound {
			result.Matched--
			break
		}

		// Step 5: cooldown. The probe asks for DELIVERED rows at or after
		// now()-cooldown, and any row it returns means this rule already
		// notified the user inside the open window. The suppression is not
		// re-scheduled: the next signal event for this market re-runs the same
		// probe and fires as soon as the window closes.
		cutoff := e.now().Add(-time.Duration(rule.CooldownSeconds) * time.Second)
		last, err := e.store.LastDeliveredAlertEvent(ctx, rule.ID, cutoff)
		if err != nil {
			return result, err
		}
		if last != nil {
			result.Suppressed++
			result.Cooldowns++
			continue
		}

		// Steps 6 and 7: dedupe key and insert. created=false means the unique
		// constraint (alert_rule_id, dedupe_key) already holds this pair, so the
		// user has been told about this exact signal event under this exact rule.
		key := AlertDedupeKey(rule.ID, event)
		alert, created, err := e.store.InsertAlertEvent(ctx, AlertEvent{
			AlertRuleID:   rule.ID,
			SignalEventID: event.ID,
			MarketID:      event.MarketID,
			Status:        AlertStatusPending,
			TriggeredAt:   e.now(),
			DedupeKey:     key,
		})
		if err != nil {
			return result, err
		}
		if !created || alert == nil {
			result.Suppressed++
			continue
		}

		// Step 8: deliver. A failure is confined to this rule: it is stamped on
		// this alert event, counted, and the loop continues, because the
		// remaining rules belong to other users and depend on no shared state.
		// A FAILED row is not counted by the cooldown probe, so this alert stays
		// eligible for a later retry rather than being silently muted.
		notification := buildNotification(rule, event, alert.ID)
		if sendErr := e.sender.Send(ctx, notification); sendErr != nil {
			result.Failed++
			if markErr := e.store.MarkAlertEventFailed(ctx, alert.ID); markErr != nil {
				return result, fmt.Errorf("mark alert %d failed after send error: %w", alert.ID, markErr)
			}
			// A cancelled or expired context will fail every remaining send the
			// same way, so stop here rather than stamp the whole remaining rule
			// set FAILED on the way out. The context error is still surfaced.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			continue
		}
		if err := e.store.MarkAlertEventDelivered(ctx, alert.ID); err != nil {
			return result, err
		}
		result.Delivered++
	}

	return result, nil
}

// resolveMarket returns the internal market UUID for the event.
//
// Resolution is by source market id, which is the only identity the upstream
// providers guarantee to be stable, and it is deliberately allowed to fail: an
// event whose market is not stored would otherwise be attributed to whatever
// market id happened to be on the struct, and a rule scoped to another market
// could then fire on a signal that market never produced. When the event already
// carries an internal MarketID and no source id to resolve it by, that id is
// used as-is; it is still an error if both are absent.
func (e *AlertEvaluator) resolveMarket(ctx context.Context, event SignalEvent) (string, error) {
	if event.SourceMarketID == "" {
		if event.MarketID != "" {
			return event.MarketID, nil
		}
		return "", fmt.Errorf("%w: event carries neither source_market_id nor market_id", ErrAlertMarketUnresolved)
	}
	market, err := e.store.MarketBySourceID(ctx, event.SourceMarketID)
	if err != nil {
		// A missing row and a transport failure are both "we cannot attribute
		// this event", and both are wrapped so callers can still inspect them
		// with errors.Is and errors.As.
		return "", fmt.Errorf("%w: source_market_id=%q: %w", ErrAlertMarketUnresolved, event.SourceMarketID, err)
	}
	if market.ID == "" {
		return "", fmt.Errorf("%w: source_market_id=%q resolved to a row with no id", ErrAlertMarketUnresolved, event.SourceMarketID)
	}
	return market.ID, nil
}

// shortMarketID renders a market identifier for a notification title. Titles are
// read in a list, so a full UUID would dominate the line without helping the
// reader recognise anything; the first segment is enough to locate the market in
// a log. Identifiers shorter than that are returned whole, so the title stays
// bounded for any input.
func shortMarketID(marketID string) string {
	const shortLen = 8
	if len(marketID) <= shortLen {
		return marketID
	}
	return marketID[:shortLen]
}

// buildNotification renders the alert as the notification the user reads.
//
// The title carries the three facts a reader scans for - how urgent, what kind
// of signal, and which market - separated by a middle dot so a list of them
// stays visually parseable. The body is ExplainSignal, the same deterministic
// sentence the radar shows, so a user who has already read the event in the UI
// sees identical wording in their inbox and is never shown two different
// accounts of the same movement.
//
// The notification carries market_id and alert_event_id so the client can deep
// link to the market and so the notification stays traceable to the delivery
// row whose status this evaluation just advanced.
func buildNotification(rule AlertRule, event SignalEvent, alertEventID int64) Notification {
	marketID := event.MarketID
	severity := event.Severity
	if severity == "" {
		severity = SeverityInfo
	}
	title := strings.Join([]string{
		strings.ToUpper(severity),
		strings.ToUpper(event.SignalType),
		shortMarketID(event.MarketID),
	}, " \u00b7 ")

	alertID := alertEventID
	return Notification{
		UserID:       rule.UserID,
		AlertEventID: &alertID,
		Title:        title,
		Body:         ExplainSignal(event),
		Severity:     severity,
		MarketID:     &marketID,
	}
}

// ExplainMatch renders why a rule matched a signal event, in the same spirit as
// ExplainSignal: a mechanical sentence assembled only from values that are
// actually present, with no model and no inference.
//
// Every clause corresponds to one predicate inside MatchesRule, so the sentence
// enumerates the conditions the event satisfied. The order is fixed (scope, then
// signal type, then severity, then the applicable numeric threshold), so the
// same rule and event always produce byte-identical text and the UI can be
// diffed against expectations. Percentage figures are scaled and rounded with
// big.Rat so no value passes through float64 and no clause can disagree with the
// comparison that actually gated the alert.
func ExplainMatch(rule AlertRule, event SignalEvent) string {
	clauses := make([]string, 0, 4)

	// Scope clauses first: they are the coarsest filter, so naming them first
	// answers "why was this rule even considered?".
	if rule.WatchlistID != nil {
		clauses = append(clauses, "market is on the rule's watchlist")
	}
	if rule.MarketID != nil {
		clauses = append(clauses, "market is the rule's target")
	}
	if rule.SignalType != nil {
		clauses = append(clauses, fmt.Sprintf("signal type %s", *rule.SignalType))
	}
	if rule.MinimumSeverity != nil {
		clauses = append(clauses, fmt.Sprintf(
			"minimum severity %s met (event %s)", *rule.MinimumSeverity, event.Severity))
	}

	// Then the numeric thresholds. Only the threshold MatchesRule would actually
	// apply to this signal type is described: a probability threshold is ignored
	// for a liquidity event, so explaining it would tell the user about a
	// condition that played no part in the decision.
	switch event.SignalType {
	case SignalTypeProbabilityShift:
		if rule.ProbabilityChangeThreshold != nil {
			clauses = append(clauses, thresholdClause(
				"probability shift", event.PercentagePoints, *rule.ProbabilityChangeThreshold, "pp"))
		}
	case SignalTypeActivityChange:
		if rule.ActivityChangeThreshold != nil {
			clauses = append(clauses, thresholdClause(
				"activity change", event.PercentageChange, *rule.ActivityChangeThreshold, "%"))
		}
	case SignalTypeLiquidityChange:
		if rule.LiquidityChangeThreshold != nil {
			clauses = append(clauses, thresholdClause(
				"liquidity change", event.PercentageChange, *rule.LiquidityChangeThreshold, "%"))
		}
	}

	if len(clauses) == 0 {
		// An unconditional rule genuinely matches every signal for its market,
		// and saying so is more useful than emitting an empty sentence.
		return fmt.Sprintf("rule has no conditions beyond being enabled for %s", event.SignalType)
	}
	return strings.Join(clauses, "; ") + "."
}

// thresholdClause renders one numeric comparison, e.g. "probability shift
// 16.6pp >= threshold 10pp". The measured value is shown as an unsigned
// magnitude because MatchesRule compares magnitudes too; a negative movement is
// therefore reported as how far it moved rather than in which direction, which
// keeps the sentence identical for an up-move and a down-move of equal size.
//
// A measurement that is absent or unparseable is rendered as "unavailable"
// rather than omitted, so a reader can see the comparison was made on something
// rather than assuming the clause was skipped.
func thresholdClause(label string, measured *string, threshold, unit string) string {
	limit, ok := parseDecimalRat(&threshold)
	if !ok {
		return fmt.Sprintf("%s threshold %s could not be read", label, threshold)
	}
	limitText := formatRatio(limit, unit)
	value, ok := parseDecimalRat(measured)
	if !ok {
		return fmt.Sprintf("%s unavailable, threshold %s", label, limitText)
	}
	return fmt.Sprintf("%s %s >= threshold %s", label, formatRatio(value, unit), limitText)
}

// formatRatio scales a ratio by 100, reports its magnitude, and appends the
// unit. Scaling and rounding stay in exact rational arithmetic, so the text can
// never disagree with the big.Rat comparison that produced it.
func formatRatio(value *big.Rat, unit string) string {
	return new(big.Rat).Abs(new(big.Rat).Mul(value, hundred)).FloatString(1) + unit
}

// Compile-time proof that the production repository satisfies the surface the
// evaluator depends on. If a repository method is renamed or its signature
// drifts, this fails at build time in the package that owns the contract rather
// than at a call site in a wiring file nobody reads.
var _ AlertStore = (*Repository)(nil)

// Compile-time proof that the in-app sender satisfies the transport interface,
// so a second implementation can be added without touching the evaluator.
var _ NotificationSender = (*InAppSender)(nil)
