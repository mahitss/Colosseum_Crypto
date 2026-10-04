// Command worker is the standalone background worker for the Prophet gateway.
//
// It owns the whole intelligence chain and runs it on a fixed interval:
//
//	Panta -> adapter -> markets + observations persisted (intelligence.Syncer)
//	      -> signals persisted                     (intelligence.Syncer)
//	      -> signals bridged into signal_events    (AlertEvaluator step 2)
//	      -> alert rules evaluated                 (AlertEvaluator step 3)
//	      -> in-app notifications delivered         (AlertEvaluator steps 5-8)
//
// DESIGN NOTES THAT OPERATORS NEED TO KNOW:
//
// IDEMPOTENT AND RETRY-SAFE. Re-running a tick never creates duplicate signals
// and never creates duplicate alerts, because all three dedupe points are
// enforced by the database rather than by anything in this file:
//
//	(a) the UNIQUE (market_id, observed_at) constraint on market_observations
//	    makes a repeated observation a no-op, so the engine is not re-fed a
//	    measurement it has already seen;
//	(b) the UNIQUE (fingerprint) constraint on signal_events makes a repeated
//	    signal event a no-op, so a crashed-and-retried tick converges on one
//	    event row and the evaluator's early return skips all rule work;
//	(c) the UNIQUE (alert_rule_id, dedupe_key) constraint on alert_events makes a
//	    repeated alert a no-op, so no user is notified twice about one event.
//
// All three are ON CONFLICT DO NOTHING in the intelligence package, so a crash
// part-way through a tick followed by a retry converges on exactly the state the
// uninterrupted tick would have produced. This is also why the bridge pass below
// deliberately re-reads a window that OVERLAPS the previous tick: the overlap is
// safe precisely because the fingerprint makes reprocessing a no-op, and it is
// what stops a crash between the sync commit and the alert pass from silently
// dropping alerts for those signals forever.
//
// BOUNDED, WITH NO UNBOUNDED QUEUE ANYWHERE. Each tick runs under a hard context
// deadline, so a hung adapter, a hung engine subprocess, or a slow database
// cannot make a tick run forever. The bridge pass reads at most
// maxSignalsPerTick signals. Alert evaluation is additionally bounded per event by
// the evaluator's own maxAlertsPerEvent. Everything the worker holds is a
// slice bounded by a constant, so memory per tick is bounded too. The worker does
// not buffer work between ticks: whatever does not fit in a tick's window is
// picked up by a later tick, and the overlap window guarantees progress.
//
// BOUNDED RETRY WITH BACKOFF. A failed tick is logged and retried with
// exponential backoff capped at maxRetryBackoff. After maxConsecutiveFailures the
// worker does NOT exit: it drops to degradedInterval and keeps monitoring. A
// monitoring worker that exits stops monitoring, and an outage that begins while
// the worker is down is exactly the outage nobody is watching for.
//
// SINGLE INSTANCE. A session-level Postgres advisory lock is held for the life of
// the process. Two workers polling Panta concurrently would race on observation
// timestamps and double-charge the adapter rate limit, so the second one exits
// immediately rather than silently duplicating work.
//
// OBSERVABILITY. Every log line carries request_id so a tick can be followed from
// its start to its end. NEVER LOGGED, by policy and not by accident: the Panta API
// key, any Authorization header, wallet private keys, and raw signed
// transactions. The Panta API key is read from inside the adapter, the wallet
// keys and signed transactions live in the trading package which this binary does
// not import, and no such value is ever placed into a log attribute. The error
// strings this worker can print come from the adapter client, which already
// collapses transport failures to fixed messages such as "market adapter is
// unavailable" rather than echoing response bodies.
package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"qevryn/gateway/internal/intelligence"
	"qevryn/gateway/internal/markets"
	"qevryn/gateway/internal/requestid"
)

const (
	// workerAdvisoryLockKey is the fixed int64 key for the single-instance
	// advisory lock.
	//
	// IT MUST NEVER CHANGE. pg_try_advisory_lock takes a plain bigint with no
	// namespace, so this value shares one lock space with every other advisory
	// lock in the same database. Rotating it would let an old deployment and a
	// new one both believe they hold "the" worker lock and poll Panta together,
	// which is precisely the race the lock exists to prevent. The value is the
	// ASCII bytes "PROPHETE" (0x50 'P', 0x52 'R', 0x4F 'O', 0x50 'P', 0x48 'H',
	// 0x45 'E', 0x54 'T', 0x45 'E') so the constant is greppable and its intent
	// is readable without a lookup table.
	//
	// In production, this should be overridden via WORKER_ADVISORY_LOCK_KEY
	// environment variable to prevent conflicts between deployments sharing a
	// database (e.g., staging and prod).
	defaultWorkerAdvisoryLockKey int64 = 0x50524F5048455445

	// defaultSyncIntervalSeconds is the poll interval used when
	// PROPHET_SYNC_INTERVAL_SECONDS is unset.
	defaultSyncIntervalSeconds = 300
	// minSyncIntervalSeconds and maxSyncIntervalSeconds bound the configured
	// interval. The lower bound stops an operator from hammering Panta and
	// getting the adapter rate-limited; the upper bound keeps the value inside
	// the int32 seconds range used by the rest of the config parsing.
	minSyncIntervalSeconds = 30
	maxSyncIntervalSeconds = 86400

	// maxSignalsPerTick bounds the bridge pass. Signals are read newest first,
	// so a window holding more than this leaves the oldest ones for a later
	// tick; the worker logs when the window is full so a permanently saturated
	// bound is visible instead of silently dropping signals.
	maxSignalsPerTick = 500

	// maxTickTimeout caps the per-tick deadline.
	maxTickTimeout = 10 * time.Minute
	// minTickTimeout floors it so a very short interval still allows one
	// adapter round trip and one engine subprocess.
	minTickTimeout = time.Minute

	// drainTimeout bounds how long shutdown waits for the in-flight tick.
	drainTimeout = 30 * time.Second

	// baseRetryBackoff and maxRetryBackoff bound the exponential backoff.
	baseRetryBackoff = 5 * time.Second
	maxRetryBackoff  = 5 * time.Minute

	// maxConsecutiveFailures is the cap on consecutive failures. Past it the
	// worker keeps running at degradedInterval rather than exiting.
	maxConsecutiveFailures = 5
	degradedInterval       = 15 * time.Minute
)

type config struct {
	databaseURL         string
	adapterURL          string
	enginePath          string
	syncInterval        time.Duration
	tickTimeout         time.Duration
	bridgeLookback      time.Duration
	advisoryLockKey     int64
}

type tickStats struct {
	sync              intelligence.SyncStats
	eventsBridged     int
	eventsDedupe      int
	alertsMatched     int
	alertsCooldown    int
	alertsDedupe      int
	notificationsSent int
	notificationFails int
	evaluationErrors  int
}

type worker struct {
	logger     *slog.Logger
	config     config
	pool       *pgxpool.Pool
	repository *intelligence.Repository
	syncer     *intelligence.Syncer
	evaluator  *intelligence.AlertEvaluator
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := loadConfig(logger)
	if err != nil {
		logger.Error("worker configuration is invalid", slog.String("error", err.Error()))
		os.Exit(1)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()
	pool, err := pgxpool.New(startupCtx, cfg.databaseURL)
	if err != nil {
		logger.Error("could not configure PostgreSQL connection", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	if err := intelligence.Migrate(startupCtx, pool); err != nil {
		logger.Error("database migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	lockConn, err := pool.Acquire(startupCtx)
	if err != nil {
		logger.Error("could not acquire a database connection for the worker lock", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := acquireWorkerLock(startupCtx, lockConn, logger, cfg.advisoryLockKey); err != nil {
		lockConn.Release()
		logger.Error("another worker already holds the advisory lock", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		// The lock is session scoped, so it is released explicitly on shutdown
		// rather than left to the connection close. pg_advisory_unlock is given
		// its own bounded context because the parent context is already
		// cancelled by this point in a signal-driven shutdown.
		releaseCtx, cancelRelease := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelRelease()
		if _, err := lockConn.Exec(releaseCtx, `SELECT pg_advisory_unlock($1)`, cfg.advisoryLockKey); err != nil {
			logger.Error("could not release the worker advisory lock", slog.String("error", err.Error()))
		}
		lockConn.Release()
	}()

	adapterClient, err := markets.NewAdapterClient(cfg.adapterURL, 15*time.Second)
	if err != nil {
		logger.Error("PANTA_ADAPTER_URL is invalid", slog.String("error", err.Error()))
		os.Exit(1)
	}
	signalConfig, err := intelligence.LoadSignalConfig()
	if err != nil {
		logger.Error("signal configuration is invalid", slog.String("error", err.Error()))
		os.Exit(1)
	}
	repository := intelligence.NewRepository(pool)
	run := &worker{
		logger:     logger,
		config:     cfg,
		pool:       pool,
		repository: repository,
		syncer: intelligence.NewSyncer(
			repository, adapterClient, intelligence.RustEngine{Path: cfg.enginePath}, signalConfig,
		),
		evaluator: intelligence.NewAlertEvaluator(repository, intelligence.NewInAppSender(repository)),
	}

	logger.Info("worker started",
		slog.String("adapter_url", cfg.adapterURL),
		slog.Duration("sync_interval", cfg.syncInterval),
		slog.Duration("tick_timeout", cfg.tickTimeout),
		slog.Duration("bridge_lookback", cfg.bridgeLookback),
		slog.Int("max_signals_per_tick", maxSignalsPerTick),
		slog.Int64("advisory_lock_key", cfg.advisoryLockKey))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	run.loop(ctx)
	logger.Info("worker stopped")
}

// loop is the scheduling half of the worker. It owns the interval, the retry
// backoff, and shutdown. The tick itself lives in run.tick.
func (w *worker) loop(ctx context.Context) {
	consecutiveFailures := 0
	// The first tick runs immediately rather than after one interval, so a
	// freshly started worker is useful immediately instead of silent for five
	// minutes.
	wait := time.Duration(0)
	for {
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return
		}

		ok := w.runTickWithDrain(ctx)
		if ok {
			consecutiveFailures = 0
			wait = w.config.syncInterval
			continue
		}

		consecutiveFailures++
		wait = w.retryWait(consecutiveFailures)
		w.logger.Error("tick failed",
			slog.Int("consecutive_failures", consecutiveFailures),
			slog.Duration("retry_in", wait),
			slog.Int("max_consecutive_failures", maxConsecutiveFailures))
	}
}

// retryWait returns the delay before the next attempt after the given number of
// consecutive failures.
//
// The delay doubles from baseRetryBackoff up to maxRetryBackoff. Once
// consecutive failures passes maxConsecutiveFailures the worker does not exit and
// does not keep hammering either: it settles at degradedInterval, which is a
// slower cadence than the healthy one so a persistently broken downstream (Panta
// down, database down) is polled gently, and the worker is still alive to notice
// recovery.
func (w *worker) retryWait(consecutiveFailures int) time.Duration {
	if consecutiveFailures >= maxConsecutiveFailures {
		if degradedInterval > w.config.syncInterval {
			return degradedInterval
		}
		return w.config.syncInterval
	}
	delay := baseRetryBackoff
	for attempt := 1; attempt < consecutiveFailures; attempt++ {
		delay *= 2
		if delay >= maxRetryBackoff {
			return maxRetryBackoff
		}
	}
	if delay > maxRetryBackoff {
		return maxRetryBackoff
	}
	return delay
}

// runTickWithDrain executes one tick and returns whether it succeeded.
//
// On shutdown the tick is NOT abandoned. The tick runs under its own deadline
// derived from a background context rather than from the signal context, so a
// SIGTERM during a tick lets the in-flight transaction commit or roll back
// normally instead of being torn out from under itself. The wait for it is
// bounded by drainTimeout: after that the process continues shutting down and
// the tick is left to finish or expire on its own deadline, because a shutdown
// that hangs forever is worse than a shutdown that gives up on one tick. The
// tick is safe to abandon precisely because of the idempotency described at the
// top of this file.
func (w *worker) runTickWithDrain(ctx context.Context) bool {
	requestID, err := newRequestID()
	if err != nil {
		w.logger.Error("could not create a request id for this tick", slog.String("error", err.Error()))
		return false
	}
	// tickCtx is rooted in Background, not in ctx: ctx is cancelled by SIGINT or
	// SIGTERM, and cancelling it here is exactly the mid-transaction abort this
	// is designed to avoid. The deadline still bounds the tick.
	tickCtx, cancelTick := context.WithTimeout(context.Background(), w.config.tickTimeout)
	defer cancelTick()
	tickCtx = requestid.With(tickCtx, requestID)

	results := make(chan error, 1)
	go func() {
		_, tickErr := w.tick(tickCtx, requestID)
		results <- tickErr
	}()

	select {
	case tickErr := <-results:
		return tickErr == nil
	case <-ctx.Done():
		select {
		case tickErr := <-results:
			if tickErr != nil {
				w.logger.Warn("in-flight tick failed during shutdown drain",
					slog.String("request_id", requestID), slog.String("error", tickErr.Error()))
			}
		case <-time.After(drainTimeout):
			w.logger.Warn("shutdown drain timed out with a tick still in flight",
				slog.String("request_id", requestID),
				slog.Duration("drain_timeout", drainTimeout),
				slog.Duration("tick_timeout", w.config.tickTimeout))
		}
		return true
	}
}

// tick is one pass of the whole chain. It returns stats and an error, and the
// error is non-nil only for a condition that makes the tick as a whole
// incomplete.
func (w *worker) tick(ctx context.Context, requestID string) (tickStats, error) {
	started := time.Now()
	log := w.logger.With(slog.String("request_id", requestID))
	stats := tickStats{}

	log.Info("tick started",
		slog.Time("started_at", started.UTC()),
		slog.Duration("tick_timeout", w.config.tickTimeout))

	syncStats, err := w.syncer.Run(ctx, requestID)
	stats.sync = syncStats
	log.Info("market sync finished",
		slog.Time("started_at", syncStats.StartedAt),
		slog.Time("ended_at", syncStats.EndedAt),
		slog.Int("markets_fetched", syncStats.MarketsFetched),
		slog.Int("markets_normalized", syncStats.MarketsNormalized),
		slog.Int("observations_created", syncStats.ObservationsCreated),
		slog.Int("observations_skipped", syncStats.ObservationsSkipped),
		slog.Int("signals_generated", syncStats.SignalsGenerated),
		slog.Int("signals_persisted", syncStats.SignalsPersisted),
		slog.Int("errors", syncStats.Errors))
	// A failed sync is a failed tick. The Syncer's writes are transactional, so
	// a failure means it rolled back and there is nothing new to bridge; the
	// retry backoff is the right response rather than alerting on a stale or
	// empty signal set.
	if err != nil {
		return stats, err
	}

	bridgeErr := w.bridgeAndEvaluate(ctx, requestID, &stats)
	elapsed := time.Since(started)

	log.Info("tick finished",
		slog.Time("ended_at", started.Add(elapsed).UTC()),
		slog.Duration("duration", elapsed),
		slog.Int("signals_bridged", stats.eventsBridged),
		slog.Int("signal_events_deduplicated", stats.eventsDedupe),
		slog.Int("alerts_matched", stats.alertsMatched),
		slog.Int("alerts_suppressed_by_cooldown", stats.alertsCooldown),
		slog.Int("alerts_suppressed_by_dedupe", stats.alertsDedupe),
		slog.Int("notifications_delivered", stats.notificationsSent),
		slog.Int("notification_failures", stats.notificationFails),
		slog.Int("evaluation_errors", stats.evaluationErrors))
	return stats, bridgeErr
}

// bridgeAndEvaluate reads the signals the sync just persisted and runs each one
// past the alert pipeline.
//
// WHY A READ-BACK PASS INSTEAD OF InsertSignalEventsFromSignals:
// intelligence.Syncer.Run does not return the signals it generated. It returns
// only counts (SignalsGenerated, SignalsPersisted) and has already committed its
// transaction, so the generated signals are no longer in the caller's hands.
// intelligence.InsertSignalEventsFromSignals would take them, but it needs both
// the []Signal values and a pgx.Tx, and neither is reachable from here. Rather
// than change the Syncer's contract, this pass reads the persisted signals back
// through the supported intelligence.Repository.Signals query and hands them to
// the evaluator. The one small addition this required in the intelligence
// package is the exported SignalEventFromSignal helper, which is the same
// Signal -> SignalEvent mapping InsertSignalEventsFromSignals already performed
// inline; sharing it guarantees both paths produce the same fingerprint, which
// is what makes the overlap window below safe.
//
// The bridging itself happens inside AlertEvaluator.EvaluateEvent, at its step 2:
// it computes the fingerprint and inserts with ON CONFLICT (fingerprint) DO
// NOTHING, the identical statement InsertSignalEventsFromSignals uses. So the
// event is durably recorded exactly once no matter how many times this pass sees
// it, and evaluation is what makes it durable. The worker therefore calls the
// evaluator and not the repository bridge, which would be a redundant second
// write of the same rows.
//
// THE WINDOW OVERLAPS DELIBERATELY. Since is set to now()-bridgeLookback rather
// than to the exact sync start, so a tick that crashed after the sync committed
// but before it could alert leaves signals that a later tick still picks up.
// Re-reading them costs nothing: the fingerprint constraint makes the insert a
// no-op and the evaluator returns before any rule work.
func (w *worker) bridgeAndEvaluate(ctx context.Context, requestID string, stats *tickStats) error {
	log := w.logger.With(slog.String("request_id", requestID))
	since := time.Now().UTC().Add(-w.config.bridgeLookback)
	page, err := w.repository.Signals(ctx, intelligence.SignalQuery{
		From: &since, Limit: maxSignalsPerTick,
	})
	if err != nil {
		log.Error("could not read persisted signals for the bridge pass", slog.String("error", err.Error()))
		return err
	}
	if page.NextCursor != nil {
		log.Warn("bridge window is full, the oldest signals are deferred to a later tick",
			slog.Int("max_signals_per_tick", maxSignalsPerTick),
			slog.Int("signals_in_window", len(page.Items)),
			slog.Time("window_start", since))
	}

	var firstErr error
	for _, signal := range page.Items {
		// A signal read back from the signals table carries the internal
		// markets.id in MarketID, which is the identity signal_events needs, and
		// the Panta id in SourceMarketID, which is how the evaluator resolves
		// the market.
		event := intelligence.SignalEventFromSignal(signal, signal.MarketID, signal.SourceMarketID)
		result, evalErr := w.evaluator.EvaluateEvent(ctx, event)
		if evalErr != nil {
			stats.evaluationErrors++
			if firstErr == nil {
				firstErr = evalErr
			}
			if ctx.Err() != nil {
				// A cancelled or expired context will fail every remaining event
				// the same way, so stop instead of stamping the whole window.
				return firstErr
			}
			log.Error("alert evaluation failed for a signal event",
				slog.Int64("signal_id", signal.ID),
				slog.String("source_market_id", signal.SourceMarketID),
				slog.String("error", evalErr.Error()))
			continue
		}
		if result.Event.ID != 0 {
			stats.eventsBridged++
		} else {
			// The fingerprint was already present, so the event and therefore
			// its alerts already exist. Expected on every tick after the first
			// because the window overlaps.
			stats.eventsDedupe++
			continue
		}
		stats.alertsMatched += result.Matched
		stats.alertsCooldown += result.Cooldowns
		// Suppressed minus the cooldown subset is exactly the dedupe-key
		// suppressions: a rule that had already recorded this alert for this
		// signal event.
		stats.alertsDedupe += result.Suppressed - result.Cooldowns
		stats.notificationsSent += result.Delivered
		stats.notificationFails += result.Failed
	}

	if stats.eventsBridged > 0 {
		log.Info("signals bridged into the durable signal event stream",
			slog.Int("events_bridged", stats.eventsBridged))
	}
	if stats.eventsDedupe > 0 {
		log.Info("signals already present in the event stream were deduplicated",
			slog.Int("events_deduplicated", stats.eventsDedupe))
	}
	if stats.alertsMatched > 0 {
		log.Info("alert rules matched signal events", slog.Int("alerts_matched", stats.alertsMatched))
	}
	if stats.alertsCooldown > 0 {
		log.Info("alerts suppressed by an open cooldown window", slog.Int("alerts_suppressed", stats.alertsCooldown))
	}
	if stats.alertsDedupe > 0 {
		log.Info("alerts suppressed because this alert was already recorded for this event",
			slog.Int("alerts_suppressed", stats.alertsDedupe))
	}
	if stats.notificationsSent > 0 {
		log.Info("in-app notifications delivered", slog.Int("notifications_delivered", stats.notificationsSent))
	}
	if stats.notificationFails > 0 {
		log.Error("notification delivery failures",
			slog.Int("notification_failures", stats.notificationFails),
			slog.Int("alerts_matched", stats.alertsMatched))
	}
	return firstErr
}

// acquireWorkerLock takes the single-instance advisory lock. pg_try_advisory_lock
// returns false rather than blocking when someone else holds it, which is what
// makes this a safe test instead of a silent queue behind the incumbent.
func acquireWorkerLock(ctx context.Context, conn *pgxpool.Conn, logger *slog.Logger, lockKey int64) error {
	var acquired bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&acquired); err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("advisory lock %d is held by another worker", lockKey)
	}
	logger.Info("acquired the single-instance worker advisory lock", slog.Int64("advisory_lock_key", lockKey))
	return nil
}

func loadConfig(logger *slog.Logger) (config, error) {
	cfg := config{}
	cfg.databaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.databaseURL == "" {
		return config{}, fmt.Errorf("DATABASE_URL is required")
	}
	cfg.adapterURL = strings.TrimSpace(os.Getenv("PANTA_ADAPTER_URL"))
	if cfg.adapterURL == "" {
		cfg.adapterURL = "http://127.0.0.1:8081"
	}

	// The deterministic engine is required, and an unset value is a hard error
	// rather than a default. Without it Syncer.Run produces no signals at all,
	// which means no signal events, which means no alerts, which means the whole
	// monitoring chain silently does nothing while appearing healthy. A
	// monitoring worker that cannot monitor must not pretend to run.
	cfg.enginePath = strings.TrimSpace(os.Getenv("MARKET_ENGINE_BIN"))
	if cfg.enginePath == "" {
		logger.Error("the deterministic market engine is unavailable",
			slog.String("missing_env", "MARKET_ENGINE_BIN"),
			slog.String("consequence", "no signals would be produced, so no alerts would ever fire"))
		return config{}, fmt.Errorf("MARKET_ENGINE_BIN is required: without the deterministic engine the worker produces no signals and no alerts")
	}
	if info, err := os.Stat(cfg.enginePath); err != nil {
		logger.Error("the deterministic market engine is unavailable",
			slog.String("missing_env", "MARKET_ENGINE_BIN"),
			slog.String("consequence", "no signals would be produced, so no alerts would ever fire"),
			slog.String("stat_error", err.Error()))
		return config{}, fmt.Errorf("MARKET_ENGINE_BIN is set but %q cannot be used: %w", cfg.enginePath, err)
	} else if info.IsDir() || info.Mode()&0o111 == 0 {
		logger.Error("the deterministic market engine is unavailable",
			slog.String("missing_env", "MARKET_ENGINE_BIN"),
			slog.String("consequence", "no signals would be produced, so no alerts would ever fire"),
			slog.String("path", cfg.enginePath))
		return config{}, fmt.Errorf("MARKET_ENGINE_BIN %q is not an executable file", cfg.enginePath)
	}

	// Advisory lock key - configurable to prevent conflicts between deployments
	// sharing the same database (e.g., staging and prod).
	lockKeyRaw := strings.TrimSpace(os.Getenv("WORKER_ADVISORY_LOCK_KEY"))
	if lockKeyRaw == "" {
		cfg.advisoryLockKey = defaultWorkerAdvisoryLockKey
	} else {
		parsed, err := strconv.ParseInt(lockKeyRaw, 0, 64)
		if err != nil {
			return config{}, fmt.Errorf("WORKER_ADVISORY_LOCK_KEY must be a valid int64: %w", err)
		}
		cfg.advisoryLockKey = parsed
	}

	seconds, err := syncIntervalSeconds()
	if err != nil {
		return config{}, err
	}
	cfg.syncInterval = time.Duration(seconds) * time.Second

	// The tick deadline tracks the interval: a tick may legitimately run longer
	// than one period, but not without bound. Clamping to [1m, 10m] keeps a very
	// short interval from cancelling ticks that are still making progress and
	// a very long one from leaving a tick running for a day.
	cfg.tickTimeout = 2 * cfg.syncInterval
	if cfg.tickTimeout < minTickTimeout {
		cfg.tickTimeout = minTickTimeout
	}
	if cfg.tickTimeout > maxTickTimeout {
		cfg.tickTimeout = maxTickTimeout
	}
	// One full interval of overlap plus the tick timeout: long enough to cover a
	// tick that failed right after its sync committed, short enough that the
	// window stays a small, constant amount of work.
	cfg.bridgeLookback = cfg.syncInterval + cfg.tickTimeout
	return cfg, nil
}

func syncIntervalSeconds() (int, error) {
	raw := strings.TrimSpace(os.Getenv("PROPHET_SYNC_INTERVAL_SECONDS"))
	if raw == "" {
		return defaultSyncIntervalSeconds, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("PROPHET_SYNC_INTERVAL_SECONDS must be an integer number of seconds, got %q", raw)
	}
	if seconds < minSyncIntervalSeconds || seconds > maxSyncIntervalSeconds {
		return 0, fmt.Errorf("PROPHET_SYNC_INTERVAL_SECONDS must be from %d to %d, got %d", minSyncIntervalSeconds, maxSyncIntervalSeconds, seconds)
	}
	return seconds, nil
}

func newRequestID() (string, error) {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

