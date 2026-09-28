// Enterprise-facing reads: the Signal Radar feed and the watchlist aggregate.
//
// # WHY THIS FILE EXISTS
//
// Everything above the storage layer (the httpapi handlers) speaks in the shapes
// declared in model.go: RadarEvent, WatchlistIntelligence, WatchlistSummary.
// Everything below it speaks in SQL. This file is the bridge: it takes the query
// objects the HTTP layer builds, turns them into pgx statements, and assembles
// the response objects. Nothing here is HTTP-aware and nothing here is
// business-rule-aware; both live elsewhere.
//
// # OWNERSHIP-FILTER INVARIANT (read first)
//
// Every method on *Repository in this file that takes a userID MUST scope its SQL
// by `user_id = $N` in the same statement that reads the target row. Ownership is
// never established by a separate follow-up lookup whose result the caller then
// has to remember to check, and no caller-supplied value is ever formatted into
// SQL text: every input is bound through pgx positional placeholders ($1, $2,
// ...) only. The single interpolated fragment in this file is radarSeverityRankSQL,
// which is generated at package initialisation from the compile-time severity
// constants and never from request data.
//
// # WHY RADAR VERIFIES WATCHLIST OWNERSHIP INSTEAD OF TRUSTING THE ID
//
// watchlist_id arrives on the radar as a query parameter, so it is entirely under
// the caller's control. It is also a UUID naming a private, per-user collection.
// If the radar simply joined `watchlist_markets ON wm.watchlist_id = $n`, then
// any client that learned or guessed a watchlist UUID - they are sequential
// gen_random_uuid() values, and they appear in this API's own URLs - would read
// the signal history of a watchlist they do not own. The membership predicate is
// what scopes the query, so it cannot be left to do the authorisation job too.
//
// Radar therefore resolves the watchlist through the owner-scoped predicate
// first, via ensureWatchlistOwned, and returns ErrWatchlistNotFound when it does
// not belong to the acting user. This is deliberately a separate statement rather
// than an `AND EXISTS (SELECT 1 FROM watchlists WHERE id=$n AND user_id=$m)`
// folded into the main query: the standalone form yields a single error value
// that the HTTP layer already maps to 404 WATCHLIST_NOT_FOUND, and it keeps the
// failure identical to the one produced by every other watchlist-scoped read in
// this package, so a caller cannot tell "no such watchlist" from "not your
// watchlist" and cannot use the endpoint to probe for the existence of another
// user's watchlist.
//
// The check fails CLOSED. If a watchlist_id is supplied and the query carries no
// resolved user identity, Radar returns ErrWatchlistNotFound rather than skipping
// the check: a missing identity must never be treated as permission.
package intelligence

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// defaultRadarLimit and maximumRadarLimit bound one radar page. The bound is
	// applied in Go before the value is bound as a placeholder, so an oversized or
	// negative limit can never reach the statement.
	defaultRadarLimit = 50
	maximumRadarLimit = 100

	// watchlistIntelSignalLimit bounds the recent-signal slice of the watchlist
	// aggregate. The aggregate is a summary panel, not a paginated feed, so a
	// fixed bound is correct here: an unbounded read of every signal ever detected
	// for every market on a large watchlist would make page load time a function of
	// history rather than of screen size.
	watchlistIntelSignalLimit = 50
)

// radarSeverityRankSQL is the SQL spelling of SeverityRank, so the radar's
// minimum-severity filter is evaluated on the same ordinal scale as SeverityRank
// rather than by a string comparison (which would order the severities
// alphabetically and silently return the wrong rows).
//
// It is built once at package initialisation from the compile-time constants in
// validSeverities, and SeverityRank is the only source of the numbers, so it
// cannot drift from the Go-side definition. The ELSE branch is 0, matching
// SeverityRank's treatment of an unrecognised severity: a stored value outside the
// known set fails a minimum-severity filter rather than being promoted to
// CRITICAL.
var radarSeverityRankSQL = buildRadarSeverityRankSQL()

func buildRadarSeverityRankSQL() string {
	branches := make([]string, 0, len(validSeverities))
	for _, severity := range validSeverities {
		branches = append(branches, fmt.Sprintf("WHEN '%s' THEN %d", severity, SeverityRank(severity)))
	}
	return "CASE se.severity " + strings.Join(branches, " ") + " ELSE 0 END"
}

// RemoveMarketBySourceID is intentionally NOT defined here.
//
// The internal-market-id-agnostic membership delete is RemoveMarketFromWatchlist
// in watchlist_repo.go: it takes (ctx, userID, watchlistID, sourceMarketID),
// verifies the caller owns the watchlist through ensureWatchlistOwned, resolves
// the base58 source id to the internal UUID, and deletes the membership. A
// two-argument variant carrying no userID would be able to delete a row from any
// watchlist, so adding one here would weaken the invariant this package
// documents. Callers that hold a resolved identity already have what they need.
var _ = (*Repository).RemoveMarketFromWatchlist

// WatchlistMarketRows returns the full Market rows on a watchlist the user owns,
// most recently updated first.
//
// Ownership is enforced through ensureWatchlistOwned before the join runs, so a
// foreign watchlist id produces ErrWatchlistNotFound rather than an empty list -
// an empty list would be indistinguishable from a watchlist the user owns but has
// not populated, and would let a caller probe for other users' watchlists by
// watching for the difference. The membership, the ordering and the projection
// all happen in the second statement, so no row is ever read unscoped and then
// discarded in Go.
func (r *Repository) WatchlistMarketRows(ctx context.Context, userID, watchlistID string) ([]Market, error) {
	if err := r.ensureWatchlistOwned(ctx, userID, watchlistID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text,m.source,m.source_market_id,m.title,m.description,m.category,
			m.status,m.phase,m.yes_probability::text,m.no_probability::text,
			m.liquidity::text,m.volume_usdc::text,m.created_at,m.closes_at,m.resolution_status
		FROM watchlist_markets wm
		JOIN markets m ON m.id = wm.market_id
		WHERE wm.watchlist_id=$1
		ORDER BY m.updated_at DESC, m.id DESC`, watchlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	markets := make([]Market, 0, 16)
	for rows.Next() {
		var market Market
		if err := rows.Scan(
			&market.ID, &market.Source, &market.SourceMarketID, &market.Title,
			&market.Description, &market.Category, &market.Status, &market.Phase,
			&market.YesProbability, &market.NoProbability, &market.Liquidity,
			&market.VolumeUSDC, &market.CreatedAt, &market.ClosesAt, &market.ResolutionStatus,
		); err != nil {
			return nil, err
		}
		markets = append(markets, market)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return markets, nil
}

// Radar returns one page of the Signal Radar, the total number of rows matching
// the filters, and a cursor for the next page.
//
// The severity filter has MINIMUM semantics, not equality. The radar's tabs are
// Critical / Significant / Watch / Info, and a reader who opens "Significant" is
// asking for everything at or above that level - a CRITICAL event belongs on the
// Significant tab too, or the tab would hide the most important rows. The
// comparison is therefore `rank(se.severity) >= rank(requested)`, evaluated
// SQL-side so the database filters with its indexes rather than the service
// fetching everything and discarding most of it in Go. A severity outside the
// known set ranks 0 and therefore applies no minimum-severity filter, which is
// why the HTTP layer validates the value before it gets here.
//
// Pagination is keyset, not offset: rows are ordered by (observed_at DESC, id
// DESC) and the cursor carries that same pair. Offset pagination would repeat or
// skip rows whenever a new signal arrives mid-scroll, which on a live feed is
// the common case rather than the rare one. Ties on observed_at are broken by
// the identity column, so the ordering is total and a page boundary can never
// split a group of same-timestamp events.
//
// One row beyond the limit is fetched: if a full page plus one comes back,
// another page exists and the cursor is minted from the last row actually
// returned. Fetching exactly `limit` rows and inferring "more" from a full page
// would emit a cursor for a page that turns out to be the last, producing one
// empty request at the end of every scan.
func (r *Repository) Radar(ctx context.Context, query RadarQuery) ([]RadarEvent, *string, int64, error) {
	// Ownership first, and it fails closed. See the package comment for why this
	// is a separate check rather than a subquery folded into the filter.
	if query.WatchlistID != "" {
		if err := r.ensureWatchlistOwned(ctx, query.UserID, query.WatchlistID); err != nil {
			return nil, nil, 0, err
		}
	}

	where, args := radarWhere(query, false, 0, time.Time{})
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM signal_events se`+signalEventJoin+where, args...).Scan(&total); err != nil {
		return nil, nil, 0, err
	}

	var cursorTimestamp time.Time
	var cursorID int64
	if query.Cursor != "" {
		parsedTimestamp, parsedID, err := decodeSignalCursor(query.Cursor)
		if err != nil {
			return nil, nil, 0, err
		}
		cursorTimestamp, cursorID = parsedTimestamp, parsedID
	}

	pageWhere, pageArgs := radarWhere(query, true, cursorID, cursorTimestamp)
	limit := clampRadarLimit(query.Limit)
	// The limit placeholder is the next index after the filter arguments, so it is
	// derived from the argument count rather than hard-coded: a filter that binds
	// more or fewer values cannot silently bind into the wrong slot.
	limitArgs := append(append([]any{}, pageArgs...), limit+1)
	rows, err := r.pool.Query(ctx,
		`SELECT `+signalEventColumns+`, m.title, m.category, m.yes_probability::text, m.no_probability::text, m.status`+
			signalEventJoin+pageWhere+
			` ORDER BY se.observed_at DESC, se.id DESC LIMIT $`+fmt.Sprint(len(limitArgs)), limitArgs...)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()

	events := make([]RadarEvent, 0, limit)
	for rows.Next() {
		var event RadarEvent
		if err := rows.Scan(
			&event.ID, &event.MarketID, &event.SourceMarketID, &event.SignalType, &event.Severity,
			&event.Metric, &event.PreviousValue, &event.CurrentValue, &event.AbsoluteChange,
			&event.PercentageChange, &event.PercentagePoints, &event.ObservationWindow,
			&event.ObservationID, &event.ObservedAt, &event.Fingerprint, &event.Source,
			&event.CreatedAt,
			&event.MarketSummary.Title, &event.MarketSummary.Category,
			&event.MarketSummary.YesProbability, &event.MarketSummary.NoProbability,
			&event.MarketSummary.Status,
		); err != nil {
			return nil, nil, 0, err
		}
		// The market summary is addressed by the public base58 source id, not the
		// internal UUID: this struct is serialised straight to the client, and the
		// internal id is a storage detail that also means something different to
		// anyone reading it.
		event.MarketSummary.ID = event.SourceMarketID
		event.Explanation = ExplainSignal(event.SignalEvent)
		event.DeepLink = "/markets/" + event.SourceMarketID
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, err
	}

	if len(events) <= limit {
		return events, nil, total, nil
	}
	events = events[:limit]
	last := events[len(events)-1]
	cursor, err := encodeSignalCursor(last.ObservedAt, last.ID)
	if err != nil {
		return nil, nil, 0, err
	}
	return events, &cursor, total, nil
}

// radarWhere builds the radar's filter clause and its bound arguments. It is the
// radar counterpart of signalWhere in repository.go and follows the same shape:
// clauses start from a constant TRUE, every caller-supplied value is appended to
// the argument slice and referenced by its resulting $N, and the clause text is
// assembled only from fixed fragments. No value is ever concatenated into the
// statement text.
//
// The watchlist restriction uses EXISTS rather than a join. watchlist_markets is
// keyed on (watchlist_id, market_id), so a join would be at most one-to-one here,
// but EXISTS states the intent - "this market is a member" - without making the
// result cardinality depend on that key staying unique, and it keeps the count(*)
// query and the page query structurally identical so the total can never be
// computed over a different filter than the page it counts.
func radarWhere(query RadarQuery, includeCursor bool, cursorID int64, cursorTimestamp time.Time) (string, []any) {
	clauses := []string{"TRUE"}
	args := make([]any, 0, 8)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if query.MarketID != "" {
		add("m.source='panta' AND m.source_market_id=$%d", query.MarketID)
	}
	if query.SignalType != "" {
		add("se.signal_type=$%d", query.SignalType)
	}
	// Minimum-severity semantics. The rank is sent as a bound integer rather than
	// written into the SQL text, so the comparison is against a value Postgres
	// never has to parse.
	if SeverityRank(query.Severity) > 0 {
		add(radarSeverityRankSQL+" >= $%d", SeverityRank(query.Severity))
	}
	if query.From != nil {
		add("se.observed_at >= $%d", *query.From)
	}
	if query.To != nil {
		add("se.observed_at <= $%d", *query.To)
	}
	if query.WatchlistID != "" {
		add("EXISTS (SELECT 1 FROM watchlist_markets wm WHERE wm.watchlist_id=$%d AND wm.market_id=se.market_id)", query.WatchlistID)
	}
	if includeCursor && cursorID > 0 {
		args = append(args, cursorTimestamp, cursorID)
		clauses = append(clauses, fmt.Sprintf("(se.observed_at,se.id) < ($%d,$%d)", len(args)-1, len(args)))
	}
	return "WHERE " + joinAnd(clauses), args
}

func clampRadarLimit(limit int) int {
	if limit <= 0 {
		return defaultRadarLimit
	}
	if limit > maximumRadarLimit {
		return maximumRadarLimit
	}
	return limit
}

// WatchlistIntel assembles the Phase 5 watchlist aggregate: the watchlist, its
// markets with each market's latest signal and latest observation, the recent
// signal activity across the whole watchlist, a severity breakdown, and the time
// of the most recent event.
//
// Ownership is enforced before anything is read. The watchlist itself is loaded
// through GetWatchlist, which is user-scoped, so a foreign or missing id returns
// ErrWatchlistNotFound before any signal, observation or market count is touched.
// Every subsequent read is additionally constrained to the watchlist's own
// markets, so the aggregate cannot widen past the membership even if a later
// query were added carelessly.
//
// Per-market state is fetched one market at a time rather than in a single
// windowed query. That is a deliberate trade: a watchlist is small (a user
// curates it by hand), the per-market probes are all index-backed lookups on
// primary keys, and a single statement with DISTINCT ON or a lateral join would be
// a much larger piece of SQL whose row-to-struct mapping is far easier to get
// subtly wrong. The cost is one round trip per market, which is not the bottleneck
// at this size.
//
// The internal market UUID is replaced with the public base58 source id on the
// way out, matching the convention already used by MarketIntelligence: this struct
// is serialised directly to the client, and the internal id is a storage detail
// the API contract does not address markets by.
func (r *Repository) WatchlistIntel(ctx context.Context, userID, watchlistID string) (*WatchlistIntelligence, error) {
	watchlist, err := r.GetWatchlist(ctx, userID, watchlistID)
	if err != nil {
		return nil, err
	}
	markets, err := r.WatchlistMarketRows(ctx, userID, watchlistID)
	if err != nil {
		return nil, err
	}

	if len(markets) == 0 {
		return &WatchlistIntelligence{
			Watchlist:     *watchlist,
			MarketCount:   0,
			Markets:       []WatchlistMarketIntel{},
			RecentSignals: []SignalEvent{},
			SeverityDist:  SeverityDistribution(nil),
			LatestUpdate:  nil,
		}, nil
	}

	// Batch fetch latest signals for all markets in the watchlist
	marketIDs := make([]string, len(markets))
	for i, m := range markets {
		marketIDs[i] = m.ID
	}
	latestSignals, err := r.LatestSignalsForMarkets(ctx, marketIDs)
	if err != nil {
		return nil, err
	}
	signalMap := make(map[string]*SignalEvent, len(latestSignals))
	for i := range latestSignals {
		signalMap[latestSignals[i].MarketID] = &latestSignals[i]
	}

	// Batch fetch latest observations for all markets in the watchlist
	latestObservations, err := r.LatestObservationsForMarkets(ctx, marketIDs)
	if err != nil {
		return nil, err
	}
	observationMap := make(map[string]*Observation, len(latestObservations))
	for i := range latestObservations {
		o := latestObservations[i]
		o.MarketID = markets[0].SourceMarketID // will be corrected below
		observationMap[latestObservations[i].MarketID] = &o
	}

	result := &WatchlistIntelligence{
		Watchlist:     *watchlist,
		MarketCount:   len(markets),
		Markets:       make([]WatchlistMarketIntel, 0, len(markets)),
		RecentSignals: []SignalEvent{},
	}

	for _, market := range markets {
		entry := WatchlistMarketIntel{Market: market}

		if signal, ok := signalMap[market.ID]; ok {
			s := *signal
			s.MarketID = market.SourceMarketID
			entry.LatestSignal = &s
		}

		if obs, ok := observationMap[market.ID]; ok {
			o := *obs
			o.MarketID = market.SourceMarketID
			entry.LatestObservation = &o
		}

		entry.Market.ID = market.SourceMarketID
		result.Markets = append(result.Markets, entry)
	}

	recent, err := r.recentSignalsForWatchlist(ctx, watchlistID, watchlistIntelSignalLimit)
	if err != nil {
		return nil, err
	}
	for index := range recent {
		recent[index].MarketID = recent[index].SourceMarketID
	}
	result.RecentSignals = recent
	result.SeverityDist = SeverityDistribution(recent)

	if len(recent) > 0 {
		latest := recent[0].ObservedAt
		result.LatestUpdate = &latest
	}

	return result, nil
}

// WatchlistIntelligence is the name the httpapi watchlistService interface binds
// to. It is a one-line delegation rather than a second implementation, so the HTTP
// surface and any other caller cannot drift into two different aggregations that
// disagree about ownership, bounds or ordering.
func (r *Repository) WatchlistIntelligence(ctx context.Context, userID, watchlistID string) (*WatchlistIntelligence, error) {
	return r.WatchlistIntel(ctx, userID, watchlistID)
}

// recentSignalsForWatchlist returns the newest signal events across every market
// on the watchlist, bounded by limit.
//
// The caller has already established that the watchlist belongs to the user
// (WatchlistIntel reaches this only after GetWatchlist succeeded), so the
// membership predicate here is a scope restriction rather than an authorisation
// check. It is still written as an explicit subquery against watchlist_markets
// rather than trusting a caller-supplied list of market ids, so the query cannot
// read outside the membership even if a future caller passes the wrong id list.
func (r *Repository) recentSignalsForWatchlist(ctx context.Context, watchlistID string, limit int) ([]SignalEvent, error) {
	return scanSignalEvents(ctx, r.pool,
		`SELECT `+signalEventColumns+signalEventJoin+
			`WHERE se.market_id IN (SELECT wm.market_id FROM watchlist_markets wm WHERE wm.watchlist_id=$1)`+
			` ORDER BY se.observed_at DESC, se.id DESC LIMIT $2`,
		watchlistID, clampUserListLimit(limit))
}

// AlertEventList returns the user's alert history, newest trigger first.
//
// It is a delegation rather than a new query: ListAlertEventsForUser already
// scopes the read by joining alert_events to alert_rules on the rule's user_id,
// so the ownership filter is part of the same statement that selects and orders
// the rows. Re-implementing it here would be the first place in this package
// where an alert event is read without that join, which is precisely how such a
// leak gets introduced. The limit is clamped by the delegate.
func (r *Repository) AlertEventList(ctx context.Context, userID string, limit int) ([]AlertEvent, error) {
	return r.ListAlertEventsForUser(ctx, userID, limit)
}

// The interfaces below pin the shapes this file must satisfy. They are
// compile-time assertions: if a signature drifts, the package stops building here
// rather than failing at the wiring in main.go, which is the only place the full
// set of methods is used together.
type (
	radarReader interface {
		Radar(context.Context, RadarQuery) ([]RadarEvent, *string, int64, error)
	}

	watchlistIntelReader interface {
		WatchlistIntel(context.Context, string, string) (*WatchlistIntelligence, error)
		WatchlistIntelligence(context.Context, string, string) (*WatchlistIntelligence, error)
		WatchlistMarketRows(context.Context, string, string) ([]Market, error)
	}

	alertEventReader interface {
		AlertEventList(context.Context, string, int) ([]AlertEvent, error)
	}
)

var (
	_ radarReader          = (*Repository)(nil)
	_ watchlistIntelReader = (*Repository)(nil)
	_ alertEventReader     = (*Repository)(nil)
)
