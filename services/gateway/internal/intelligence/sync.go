package intelligence

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"prophet/types"
)

const (
	pantaPageLimit  = 50
	defaultMaxPages = 2
	maximumMaxPages = 100
)

type marketReader interface {
	ListMarkets(context.Context, types.MarketQuery) (types.MarketPage, error)
	GetMarket(context.Context, string) (types.Market, error)
}

type syncStore interface {
	Begin(context.Context) (pgx.Tx, error)
	UpsertMarket(context.Context, pgx.Tx, Market) (string, bool, error)
	PreviousObservation(context.Context, rowQueryer, string, time.Time) (*Observation, error)
	WindowObservation(context.Context, rowQueryer, string, time.Time) (*Observation, error)
	InsertObservation(context.Context, pgx.Tx, Observation) (int64, bool, error)
	InsertSignals(context.Context, pgx.Tx, []Signal) (int, error)
}

type Syncer struct {
	store        syncStore
	panta        marketReader
	engine       Engine
	config       SignalConfig
	maxPages     int
	newRequestID func() (string, error)
	now          func() time.Time
}

func NewSyncer(store syncStore, panta marketReader, engine Engine, config SignalConfig) *Syncer {
	return &Syncer{
		store: store, panta: panta, engine: engine, config: config,
		maxPages: defaultMaxPages, newRequestID: requestID, now: time.Now,
	}
}

func (syncer *Syncer) Run(ctx context.Context, suppliedRequestID string) (stats SyncStats, resultErr error) {
	started := syncer.now().UTC().Truncate(time.Microsecond)
	requestID := suppliedRequestID
	if requestID == "" {
		var err error
		requestID, err = syncer.newRequestID()
		if err != nil {
			return SyncStats{}, errors.New("could not create sync request id")
		}
	}
	stats = SyncStats{RequestID: requestID, StartedAt: started}
	defer func() { stats.EndedAt = syncer.now().UTC() }()
	if syncer.store == nil || syncer.panta == nil || syncer.engine == nil {
		stats.Errors++
		return stats, errors.New("market sync dependencies are incomplete")
	}
	if err := validateSignalConfig(syncer.config); err != nil {
		stats.Errors++
		return stats, err
	}

	catalog, err := syncer.fetchCatalog(ctx, &stats)
	if err != nil {
		stats.Errors++
		return stats, err
	}
	markets := make([]Market, 0, len(catalog))
	for _, catalogMarket := range catalog {
		detail, err := syncer.panta.GetMarket(ctx, catalogMarket.ID)
		if err != nil {
			stats.Errors++
			continue
		}
		detail.Source = SourcePanta
		detail.SourceMarketID = detail.ID
		normalized, err := NormalizeMarket(detail)
		if err != nil {
			stats.Errors++
			continue
		}
		stats.MarketsNormalized++
		markets = append(markets, normalized)
	}

	transaction, err := syncer.store.Begin(ctx)
	if err != nil {
		stats.Errors++
		return stats, fmt.Errorf("begin sync transaction: %w", err)
	}
	defer transaction.Rollback(ctx)
	observedAt := started
	engineInputs := make([]EngineObservation, 0, len(markets))
	for _, market := range markets {
		if err := syncer.persistObservation(ctx, transaction, observedAt, market, &stats, &engineInputs); err != nil {
			stats.Errors++
			return stats, err
		}
	}
	if len(engineInputs) > 0 {
		signals, err := syncer.engine.Generate(ctx, engineInputs, syncer.config)
		if err != nil {
			stats.Errors++
			return stats, fmt.Errorf("deterministic signal calculation failed: %w", err)
		}
		stats.SignalsGenerated = len(signals)
		created, err := syncer.store.InsertSignals(ctx, transaction, signals)
		if err != nil {
			stats.Errors++
			return stats, fmt.Errorf("persist signals: %w", err)
		}
		stats.SignalsPersisted = created
	}
	if err := transaction.Commit(ctx); err != nil {
		stats.Errors++
		return stats, fmt.Errorf("commit market sync: %w", err)
	}
	if stats.Errors > 0 {
		return stats, fmt.Errorf("market synchronization completed with %d item errors", stats.Errors)
	}
	return stats, nil
}

func (syncer *Syncer) fetchCatalog(ctx context.Context, stats *SyncStats) ([]types.Market, error) {
	if syncer.maxPages < 1 || syncer.maxPages > maximumMaxPages {
		return nil, errors.New("sync page limit is invalid")
	}
	markets := make([]types.Market, 0)
	seenCursors := make(map[string]struct{})
	cursor := ""
	for pageNumber := 0; pageNumber < syncer.maxPages; pageNumber++ {
		page, err := syncer.panta.ListMarkets(ctx, types.MarketQuery{Cursor: cursor, Limit: pantaPageLimit})
		if err != nil {
			return markets, fmt.Errorf("fetch Panta market page: %w", err)
		}
		stats.MarketsFetched += len(page.Items)
		markets = append(markets, page.Items...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			return markets, nil
		}
		if _, duplicate := seenCursors[*page.NextCursor]; duplicate {
			return markets, errors.New("Panta pagination cursor repeated")
		}
		seenCursors[*page.NextCursor] = struct{}{}
		cursor = *page.NextCursor
	}
	return markets, fmt.Errorf("Panta catalog exceeded the configured %d-page limit", syncer.maxPages)
}

func (syncer *Syncer) persistObservation(ctx context.Context, transaction pgx.Tx, observedAt time.Time, market Market, stats *SyncStats, engineInputs *[]EngineObservation) error {
	internalID, isNew, err := syncer.store.UpsertMarket(ctx, transaction, market)
	if err != nil {
		return fmt.Errorf("upsert market %s: %w", market.SourceMarketID, err)
	}
	previous, err := syncer.store.PreviousObservation(ctx, transaction, internalID, observedAt)
	if err != nil {
		return fmt.Errorf("read previous observation: %w", err)
	}
	windowPrevious, err := syncer.store.WindowObservation(ctx, transaction, internalID, observedAt.Add(-syncer.config.ObservationWindow))
	if err != nil {
		return fmt.Errorf("read window observation: %w", err)
	}
	observation := Observation{
		MarketID: internalID, Timestamp: observedAt,
		YesProbability: market.YesProbability, NoProbability: market.NoProbability,
		VolumeUSDC: market.VolumeUSDC, Liquidity: market.Liquidity,
	}
	if previous != nil {
		unchanged, err := observationsEqual(*previous, observation)
		if err != nil {
			return fmt.Errorf("compare market observations: %w", err)
		}
		if unchanged {
			stats.ObservationsSkipped++
			return nil
		}
	}
	observationID, inserted, err := syncer.store.InsertObservation(ctx, transaction, observation)
	if err != nil {
		return fmt.Errorf("insert market observation: %w", err)
	}
	if !inserted {
		stats.ObservationsSkipped++
		return nil
	}
	stats.ObservationsCreated++
	*engineInputs = append(*engineInputs, EngineObservation{
		MarketID: market.SourceMarketID, ObservationID: observationID, ObservedAt: observedAt.Format(time.RFC3339Nano),
		ObservationWindow: syncer.config.ObservationWindow.String(), IsNewMarket: isNew,
		Previous: previous, WindowPrevious: windowPrevious, Current: observation,
	})
	return nil
}

func observationsEqual(previous, current Observation) (bool, error) {
	for _, pair := range [][2]*string{
		{previous.YesProbability, current.YesProbability},
		{previous.NoProbability, current.NoProbability},
		{previous.VolumeUSDC, current.VolumeUSDC},
		{previous.Liquidity, current.Liquidity},
	} {
		equal, err := compareDecimal(pair[0], pair[1])
		if err != nil || !equal {
			return equal, err
		}
	}
	return true, nil
}

func requestID() (string, error) {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
