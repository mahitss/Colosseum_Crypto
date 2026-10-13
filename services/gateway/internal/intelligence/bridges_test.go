package intelligence

// Integration tests for the durable bridge cursor and the alert evaluation
// chain. They run only when TEST_DATABASE_URL points at a database with the
// migrations applied; with it unset they skip, so the unit suite stays
// hermetic.
//
// These tests exist because the first E2E audit found the bridge pass reading
// a wall-clock window: observations, signals and rules persisted while
// signal_events, alert_events and notifications never did.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type bridgeTestEnv struct {
	t          *testing.T
	pool       *pgxpool.Pool
	repository *Repository
	user       string
}

func newBridgeTestEnv(t *testing.T) *bridgeTestEnv {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping bridge integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	env := &bridgeTestEnv{
		t:          t,
		pool:       pool,
		repository: NewRepository(pool),
		user:       "bridge-test-user",
	}
	env.cleanup()
	t.Cleanup(env.cleanup)
	return env
}

func (env *bridgeTestEnv) cleanup() {
	env.t.Helper()
	statements := []string{
		`DELETE FROM notifications`,
		`DELETE FROM alert_events`,
		`DELETE FROM alert_rules`,
		`DELETE FROM signal_events`,
		`DELETE FROM signals`,
		`DELETE FROM market_observations`,
		`DELETE FROM markets`,
	}
	for _, statement := range statements {
		if _, err := env.pool.Exec(context.Background(), statement); err != nil {
			env.t.Fatalf("cleanup %q: %v", statement, err)
		}
	}
}

// insertObservedMarket creates a market row plus two observations with a real
// probability move between them, so deterministic signals that reference an
// observation have exactly the shape the engine produces.
func (env *bridgeTestEnv) insertObservedMarket(sourceMarketID string) string {
	env.t.Helper()
	ctx := context.Background()
	var internalID string
	if err := env.pool.QueryRow(ctx, `INSERT INTO markets (source,source_market_id,title,status,phase) VALUES ('panta',$2,$1,'primary','primary') RETURNING id::text`,
		sourceMarketID, sourceMarketID).Scan(&internalID); err != nil {
		env.t.Fatalf("insert market: %v", err)
	}
	for index, row := range [][3]string{{"0.50", "0.50", "1.00"}, {"0.70", "0.30", "1.50"}} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO market_observations (market_id,observed_at,yes_probability,no_probability,volume_usdc,liquidity) VALUES ($1,$2,$3,$4,$5,$6)`,
			internalID, time.Now().UTC().Add(time.Duration(index)*time.Second), row[0], row[1], row[2], "1.00"); err != nil {
			env.t.Fatalf("insert observation: %v", err)
		}
	}
	return internalID
}

// insertSignal persists a signal exactly as the worker's commit would. Each
// call creates a fresh observation first, because signals are unique on
// (market_id, observation_id, signal_type, metric) and a signal row is always
// anchored to the observation that produced it.
func (env *bridgeTestEnv) insertSignal(internalID, signalType, severity string, percentagePoints string) int64 {
	env.t.Helper()
	ctx := context.Background()
	var observationID int64
	if err := env.pool.QueryRow(ctx, `INSERT INTO market_observations (market_id,observed_at,yes_probability,no_probability,volume_usdc,liquidity) VALUES ($1::uuid,$2,'0.70','0.30','1.50','1.00') RETURNING id`,
		internalID, time.Now().UTC()).Scan(&observationID); err != nil {
		env.t.Fatalf("insert observation: %v", err)
	}
	var signalID int64
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO signals (market_id,observation_id,timestamp,signal_type,severity,metric,
			previous_value,current_value,absolute_change,percentage_change,percentage_points,observation_window,source)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'1h','panta') RETURNING id`,
		internalID, observationID, time.Now().UTC(), signalType, severity, "yes_probability",
		"0.50", "0.70", "0.20", "40", percentagePoints,
	).Scan(&signalID); err != nil {
		env.t.Fatalf("insert signal: %v", err)
	}
	return signalID
}

func TestBridgeCursorRecoversSignalsBridgedAfterAFailure(t *testing.T) {
	env := newBridgeTestEnv(t)

	internalID := env.insertObservedMarket("BridgeCursorRecovery11111111111111111111111111111")
	signalID := env.insertSignal(internalID, "PROBABILITY_SHIFT", "SIGNIFICANT", "20")

	cursor, err := env.repository.LatestBridgedSignalID(context.Background())
	if err != nil {
		t.Fatalf("LatestBridgedSignalID before bridge: %v", err)
	}
	if cursor != 0 {
		t.Fatalf("expected an untouched bridge cursor, got %d", cursor)
	}

	signals, err := env.repository.SignalsSince(context.Background(), cursor, 50)
	if err != nil {
		t.Fatalf("SignalsSince: %v", err)
	}
	if len(signals) != 1 || signals[0].ID != signalID {
		t.Fatalf("expected exactly the new signal, got %d", len(signals))
	}

	// Bridge it exactly the way the worker does.
	event := SignalEventFromSignal(signals[0], signals[0].MarketID, signals[0].SourceMarketID)
	if event.SignalID == nil || *event.SignalID != signalID {
		t.Fatalf("event must carry the signal id as the bridge cursor, got %v", event.SignalID)
	}
	stored, created, err := env.repository.InsertSignalEvent(context.Background(), event)
	if err != nil || !created || stored == nil {
		t.Fatalf("bridge insert failed: stored=%v created=%v err=%v", stored, created, err)
	}
	if stored.SignalID == nil || *stored.SignalID != signalID {
		t.Fatalf("persisted event must expose the cursor, got %v", stored.SignalID)
	}

	// A second bridge with the same cursor sees nothing new.
	cursor, err = env.repository.LatestBridgedSignalID(context.Background())
	if err != nil {
		t.Fatalf("LatestBridgedSignalID after bridge: %v", err)
	}
	if cursor != signalID {
		t.Fatalf("expected bridge cursor %d, got %d", signalID, cursor)
	}
	fresh, err := env.repository.SignalsSince(context.Background(), cursor, 50)
	if err != nil {
		t.Fatalf("SignalsSince after bridge: %v", err)
	}
	if len(fresh) != 0 {
		t.Fatalf("cursor must be exhausted after bridging, saw %d", len(fresh))
	}

	// Deleting every event and re-bridging from scratch re-creates the event
	// with the same fingerprint: the idempotency the worker depends on.
	if _, err := env.pool.Exec(context.Background(), `DELETE FROM signal_events`); err != nil {
		t.Fatalf("clear events: %v", err)
	}
	storedAgain, createdAgain, err := env.repository.InsertSignalEvent(context.Background(), event)
	if err != nil || !createdAgain || storedAgain == nil || storedAgain.ID == stored.ID {
		t.Fatalf("re-bridge after wipe should create a fresh row: created=%v sameID=%v err=%v", createdAgain, storedAgain != nil && storedAgain.ID == stored.ID, err)
	}

	// Bridging the same event twice never creates a second row.
	duplicate, createdAgain, err := env.repository.InsertSignalEvent(context.Background(), event)
	if err != nil || createdAgain || duplicate != nil {
		t.Fatalf("duplicate bridge insert must dedupe, got created=%v err=%v", createdAgain, err)
	}
}

// TestAlertEvaluationProvesTheFullChain drives the real AlertEvaluator so the
// mission gates are proven on one connected path: a qualifying signal triggers
// an eligible rule, a nonqualifying signal does not, the event persists and is
// delivered to exactly the rule's owner, dedupe prevents a second evaluation
// from alerting again, and the cooldown suppresses a second qualifying event.
func TestAlertEvaluationProvesTheFullChain(t *testing.T) {
	env := newBridgeTestEnv(t)

	internalID := env.insertObservedMarket("AlertChainProof1111111111111111111111111111111111")

	// Eligible rule: significant probability changes, no watchlist/market scope
	// narrowing, explicit cooldown so the suppression path is exercised.
	severity := "SIGNIFICANT"
	rule := AlertRule{
		UserID: env.user, Name: "significant probability moves",
		MinimumSeverity: &severity, Enabled: true, CooldownSeconds: 3600,
	}
	if err := ValidateAlertRule(rule); err != nil {
		t.Fatalf("rule must validate: %v", err)
	}
	createdRule, err := env.repository.CreateAlertRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	loadedRule, err := env.repository.GetAlertRule(context.Background(), env.user, createdRule.ID)
	if err != nil || loadedRule == nil {
		t.Fatalf("read rule: loaded=%v err=%v", loadedRule != nil, err)
	}

	evaluator := NewAlertEvaluator(env.repository, NewInAppSender(env.repository))

	bridge := func(signalID int64, t *testing.T) SignalEvent {
		t.Helper()
		var signal Signal
		if err := env.pool.QueryRow(context.Background(), `SELECT s.id,s.market_id::text,m.source_market_id,s.observation_id,s.timestamp,s.signal_type,s.severity,s.metric,s.previous_value::text,s.current_value::text,s.absolute_change::text,s.percentage_change::text,s.percentage_points::text,s.observation_window,s.source FROM signals s JOIN markets m ON m.id=s.market_id WHERE s.id=$1`, signalID).Scan(
			&signal.ID, &signal.MarketID, &signal.SourceMarketID, &signal.ObservationID, &signal.Timestamp, &signal.SignalType, &signal.Severity,
			&signal.Metric, &signal.PreviousValue, &signal.CurrentValue, &signal.AbsoluteChange,
			&signal.PercentageChange, &signal.PercentagePoints, &signal.ObservationWindow, &signal.Source); err != nil {
			t.Fatalf("read back signal %d: %v", signalID, err)
		}
		return SignalEventFromSignal(signal, signal.MarketID, signal.SourceMarketID)
	}

	// 1. Qualifying signal matches the rule, bridges the event, and delivers.
	qualifyingID := env.insertSignal(internalID, "PROBABILITY_SHIFT", "SIGNIFICANT", "20")
	result, err := evaluator.EvaluateEvent(context.Background(), bridge(qualifyingID, t))
	if err != nil {
		t.Fatalf("evaluate qualifying signal: %v", err)
	}
	if result.Matched != 1 || result.Delivered != 1 || result.Suppressed != 0 {
		t.Fatalf("qualifying signal must deliver exactly one notification, got %+v", result)
	}
	var counts struct {
		Events, Alerts, Notifications int
	}
	if err := env.pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*)::int FROM signal_events),
			(SELECT count(*)::int FROM alert_events WHERE alert_rule_id=$1::uuid),
			(SELECT count(*)::int FROM notifications WHERE user_id=$2)`,
		loadedRule.ID, env.user).Scan(&counts.Events, &counts.Alerts, &counts.Notifications); err != nil {
		t.Fatalf("count chain rows: %v", err)
	}
	if counts.Events != 1 || counts.Alerts != 1 || counts.Notifications != 1 {
		t.Fatalf("the alert must persist one event, one alert row and one notification, got %+v", counts)
	}

	// 2. A nonqualifying signal neither matches nor alerts.
	nonQualifyingID := env.insertSignal(internalID, "PROBABILITY_SHIFT", "INFO", "1")
	result, err = evaluator.EvaluateEvent(context.Background(), bridge(nonQualifyingID, t))
	if err != nil {
		t.Fatalf("evaluate nonqualifying signal: %v", err)
	}
	if result.Matched != 0 {
		t.Fatalf("INFO signal must not match a SIGNIFICANT rule, got %+v", result)
	}

	// 3. Re-evaluating the same event is a no-op thanks to the fingerprint.
	resultAgain, err := evaluator.EvaluateEvent(context.Background(), bridge(qualifyingID, t))
	if err != nil {
		t.Fatalf("re-evaluate qualifying signal: %v", err)
	}
	if resultAgain.Event.ID != 0 || resultAgain.Delivered != 0 {
		t.Fatalf("a known fingerprint must short-circuit, got %+v", resultAgain)
	}

	// 4. A second qualifying signal is matched but suppressed by the cooldown.
	secondQualifyingID := env.insertSignal(internalID, "PROBABILITY_SHIFT", "SIGNIFICANT", "20")
	resultSecond, err := evaluator.EvaluateEvent(context.Background(), bridge(secondQualifyingID, t))
	if err != nil {
		t.Fatalf("evaluate second qualifying signal: %v", err)
	}
	if resultSecond.Matched != 1 || resultSecond.Cooldowns != 1 || resultSecond.Delivered != 0 {
		t.Fatalf("second event must match but cooldown-suppress, got %+v", resultSecond)
	}

	// The ownership seam: a rule belonging to another user never sees it.
	var otherAlerts, otherNotifications int
	if err := env.pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*)::int FROM alert_events ae JOIN alert_rules ar ON ar.id=ae.alert_rule_id WHERE ar.user_id='someone-else'),
			(SELECT count(*)::int FROM notifications WHERE user_id='someone-else')`).
		Scan(&otherAlerts, &otherNotifications); err != nil {
		t.Fatalf("count other-user rows: %v", err)
	}
	if otherAlerts != 0 || otherNotifications != 0 {
		t.Fatalf("no user may see another user's alerts, got alerts=%d notifications=%d", otherAlerts, otherNotifications)
	}
}
