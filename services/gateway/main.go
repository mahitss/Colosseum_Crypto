package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"prophet/gateway/internal/httpapi"
	"prophet/gateway/internal/intelligence"
	"prophet/gateway/internal/markets"
	"prophet/gateway/internal/marketstudio"
	"prophet/gateway/internal/trading"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("gateway starting")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	adapterURL := os.Getenv("PANTA_ADAPTER_URL")
	if adapterURL == "" {
		adapterURL = "http://127.0.0.1:8081"
	}
	adapterClient, err := markets.NewAdapterClient(adapterURL, 12*time.Second)
	if err != nil {
		logger.Error("invalid PANTA_ADAPTER_URL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required for the intelligence API")
		os.Exit(1)
	}
	database, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		logger.Error("could not configure PostgreSQL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close()
	startupContext, startupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := intelligence.Migrate(startupContext, database); err != nil {
		startupCancel()
		logger.Error("intelligence database migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	startupCancel()
	repository := intelligence.NewRepository(database)

	// Trading service: quote/build/broadcast/report against Panta and Solana.
	// Trading is optional — enabled only when Panta API key and Solana RPC are configured.
	var tradeHandler httpapi.TradeService
	var (
		pantaCreation     marketstudio.PantaClient
		solanaBroadcaster *marketstudio.Broadcaster
	)
	pantaAPIURL := os.Getenv("PANTA_API_URL")
	pantaAPIKey := os.Getenv("PANTA_API_KEY")
	solanaRPC := os.Getenv("SOLANA_RPC_URL")
	if pantaAPIURL != "" && pantaAPIKey != "" && solanaRPC != "" {
		pantaTrading, err := trading.NewPantaHTTPClient(pantaAPIURL, pantaAPIKey, 30*time.Second)
		if err != nil {
			logger.Error("invalid PANTA_API_URL for trading", slog.String("error", err.Error()))
			os.Exit(1)
		}
		broadcaster, err := trading.NewBroadcaster(solanaRPC, trading.BroadcasterOptions{})
		if err != nil {
			logger.Error("invalid SOLANA_RPC_URL for trading", slog.String("error", err.Error()))
			os.Exit(1)
		}
		tradeHandler = trading.NewService(
			trading.NewRepository(database),
			database,
			pantaTrading,
			broadcaster,
		)
		logger.Info("trading enabled")

		// Market creation reuses the same Panta credentials and Solana RPC, but
		// hits a different set of endpoints, so it gets its own client.
		pantaCreation, err = marketstudio.NewPantaHTTPClient(pantaAPIURL, pantaAPIKey, 30*time.Second)
		if err != nil {
			logger.Error("invalid PANTA_API_URL for market creation", slog.String("error", err.Error()))
			os.Exit(1)
		}
		solanaBroadcaster, err = marketstudio.NewBroadcaster(solanaRPC, marketstudio.BroadcasterOptions{})
		if err != nil {
			logger.Error("invalid SOLANA_RPC_URL for market creation", slog.String("error", err.Error()))
			os.Exit(1)
		}
	} else {
		logger.Warn("trading disabled: PANTA_API_URL, PANTA_API_KEY, and SOLANA_RPC_URL are all required")
	}

	// Market Studio: AI-assisted market creation.
	//
	// The assistant proposes; the deterministic validator decides; the wallet
	// signs; Panta registers. This server never holds a key and never reports a
	// market as created before Panta returns a real market id.
	var studioHandler httpapi.MarketStudioService
	var studioInterpreter marketstudio.Interpreter
	intelligenceURL := os.Getenv("INTELLIGENCE_SERVICE_URL")
	if intelligenceURL == "" {
		intelligenceURL = "http://localhost:8001"
	}
	studioClient, err := marketstudio.NewIntelligenceClient(intelligenceURL, 45*time.Second)
	if err != nil {
		logger.Error("invalid INTELLIGENCE_SERVICE_URL for market studio", slog.String("error", err.Error()))
		os.Exit(1)
	}
	studioInterpreter = studioClient
	studioService := marketstudio.NewService(
		marketstudio.NewRepository(database),
		pantaCreation,
		solanaBroadcaster,
	)
	studioHandler = studioService
	logger.Info("market studio enabled",
		slog.String("intelligence_url", intelligenceURL),
		slog.Bool("panta_creation", pantaCreation != nil),
		slog.Bool("solana_broadcast", solanaBroadcaster != nil),
	)

	logger.Info("gateway listening", slog.String("address", addr))

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandlerWithAllServices(adapterClient, repository, tradeHandler, studioHandler, studioInterpreter),
		ReadHeaderTimeout: 5 * time.Second,
	}
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdown.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("gateway shutdown failed", slog.Any("error", err))
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
