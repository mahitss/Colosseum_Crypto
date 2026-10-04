package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"qevryn/gateway/internal/intelligence"
	"qevryn/gateway/internal/markets"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatal("DATABASE_URL is required")
	}
	adapterURL := os.Getenv("PANTA_ADAPTER_URL")
	if adapterURL == "" {
		adapterURL = "http://127.0.0.1:8081"
	}
	signalConfig, err := intelligence.LoadSignalConfig()
	if err != nil {
		fatal("signal configuration is invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal("could not configure PostgreSQL connection")
	}
	defer pool.Close()
	if err := intelligence.Migrate(ctx, pool); err != nil {
		fatal("database migration failed")
	}
	adapterClient, err := markets.NewAdapterClient(adapterURL, 15*time.Second)
	if err != nil {
		fatal("PANTA_ADAPTER_URL is invalid")
	}
	enginePath := os.Getenv("MARKET_ENGINE_BIN")
	if enginePath == "" {
		enginePath = "../../services/market-engine/target/release/market-engine"
	}
	syncer := intelligence.NewSyncer(intelligence.NewRepository(pool), adapterClient, intelligence.RustEngine{Path: enginePath}, signalConfig)
	stats, err := syncer.Run(ctx, "")
	if err != nil {
		logger.Error("market synchronization failed", slog.String("request_id", stats.RequestID), slog.Int("errors", stats.Errors))
		os.Exit(1)
	}
	logger.Info("market synchronization completed",
		slog.String("request_id", stats.RequestID), slog.Time("started_at", stats.StartedAt),
		slog.Time("ended_at", stats.EndedAt), slog.Int("markets_fetched", stats.MarketsFetched),
		slog.Int("markets_normalized", stats.MarketsNormalized), slog.Int("observations_created", stats.ObservationsCreated),
		slog.Int("observations_skipped", stats.ObservationsSkipped), slog.Int("signals_generated", stats.SignalsGenerated),
		slog.Int("errors", stats.Errors))
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

