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
	"qevryn/gateway/internal/auth"
	"qevryn/gateway/internal/httpapi"
	"qevryn/gateway/internal/intelligence"
	"qevryn/gateway/internal/markets"
	"qevryn/gateway/internal/marketstudio"
	"qevryn/gateway/internal/trading"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("gateway starting")

	// Load JWT configuration
	jwtConfig, err := auth.LoadConfig()
	if err != nil {
		logger.Error("failed to load JWT config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create JWT user resolver and register it
	jwtResolver := auth.NewJWTUserResolver(jwtConfig, logger)
	httpapi.RegisterUserResolver(jwtResolver)

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

	// TASK 008 enterprise surface: watchlists, the signal radar, alert rules and
	// the notification inbox. All four read and write rows that already live in
	// this database, so they are always available here and need no credentials
	// beyond DATABASE_URL. Trading and market-studio remain optional below -
	// they are gated on PANTA/SOLANA configuration and register no routes when it
	// is absent - whereas these enterprise routes are registered unconditionally
	// whenever a database is present, because a deployment that reached this
	// point without a database already exited.
	//
	// The alert evaluator that actually fires these rules is started by the
	// ingestion path, not here; this wiring only exposes the user-facing surface.
	enterprise := enterpriseAPI{repository: repository}

	// Trading service: quote/build/broadcast/report against Panta and Solana.
	// Trading is optional - enabled only when Panta API key and Solana RPC are configured.
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
	logger.Info("enterprise api enabled",
		slog.Bool("trading", tradeHandler != nil),
	)

	logger.Info("gateway listening", slog.String("address", addr))

	// Build the handler chain: AuthMiddleware -> RequestID -> Routes
	baseHandler := httpapi.NewHandlerWithEnterprise(
		adapterClient,
		repository,
		tradeHandler,
		studioHandler,
		studioInterpreter,
		enterprise,
		enterprise,
		enterprise,
		enterprise,
	)
	// Wrap with auth middleware (validates JWT, sets user context)
	authenticatedHandler := auth.AuthMiddleware(jwtConfig, logger, baseHandler)

	server := &http.Server{
		Addr:              addr,
		Handler:           authenticatedHandler,
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

// enterpriseAPI adapts *intelligence.Repository to the four service interfaces
// the httpapi package declares for the TASK 008 routes.
//
// The adapter lives here rather than in the httpapi package on purpose. The
// repository method names are the storage layer's vocabulary
// (AddMarketToWatchlist, ListAlertRules, MarkNotificationRead, ...) while the
// HTTP surface deliberately uses short names, and the two should not be made to
// agree by giving the HTTP package a persistence dependency. Each method below
// is a one-line delegation: no ownership decision, no re-scoping and no
// translation happens in the adapter, so the ownership filters the repository
// documents stay the only place a user id is turned into SQL.
//
// CreateRule and UpdateRule are the two methods that take an explicit userID
// separately from the rule. The repository takes the owner from the rule struct
// itself on create and from the WHERE clause on update, so the adapter writes
// the resolved identity into the rule before delegating. That identity comes
// from ResolveUser, never from the request body, so this cannot widen a write
// beyond the acting user.
type enterpriseAPI struct {
	repository *intelligence.Repository
}

func (api enterpriseAPI) ListWatchlists(ctx context.Context, userID string) ([]intelligence.WatchlistSummary, error) {
	return api.repository.ListWatchlists(ctx, userID)
}

func (api enterpriseAPI) GetWatchlist(ctx context.Context, userID, watchlistID string) (*intelligence.Watchlist, error) {
	return api.repository.GetWatchlist(ctx, userID, watchlistID)
}

func (api enterpriseAPI) CreateWatchlist(ctx context.Context, userID, name string, description *string) (*intelligence.Watchlist, error) {
	return api.repository.CreateWatchlist(ctx, userID, name, description)
}

func (api enterpriseAPI) UpdateWatchlist(ctx context.Context, userID, watchlistID, name string, description *string) (*intelligence.Watchlist, error) {
	return api.repository.UpdateWatchlist(ctx, userID, watchlistID, name, description)
}

func (api enterpriseAPI) DeleteWatchlist(ctx context.Context, userID, watchlistID string) error {
	return api.repository.DeleteWatchlist(ctx, userID, watchlistID)
}

func (api enterpriseAPI) AddMarket(ctx context.Context, userID, watchlistID, marketID string) error {
	return api.repository.AddMarketToWatchlist(ctx, userID, watchlistID, marketID)
}

func (api enterpriseAPI) RemoveMarket(ctx context.Context, userID, watchlistID, marketID string) error {
	return api.repository.RemoveMarketFromWatchlist(ctx, userID, watchlistID, marketID)
}

func (api enterpriseAPI) WatchlistIntelligence(ctx context.Context, userID, watchlistID string) (*intelligence.WatchlistIntelligence, error) {
	return api.repository.WatchlistIntelligence(ctx, userID, watchlistID)
}

func (api enterpriseAPI) Radar(ctx context.Context, query intelligence.RadarQuery) ([]intelligence.RadarEvent, *string, int64, error) {
	return api.repository.Radar(ctx, query)
}

func (api enterpriseAPI) ListRules(ctx context.Context, userID string) ([]intelligence.AlertRule, error) {
	return api.repository.ListAlertRules(ctx, userID)
}

func (api enterpriseAPI) GetRule(ctx context.Context, userID, ruleID string) (*intelligence.AlertRule, error) {
	return api.repository.GetAlertRule(ctx, userID, ruleID)
}

func (api enterpriseAPI) CreateRule(ctx context.Context, userID string, rule intelligence.AlertRule) (*intelligence.AlertRule, error) {
	rule.UserID = userID
	return api.repository.CreateAlertRule(ctx, rule)
}

func (api enterpriseAPI) UpdateRule(ctx context.Context, userID, ruleID string, rule intelligence.AlertRule) (*intelligence.AlertRule, error) {
	rule.UserID = userID
	return api.repository.UpdateAlertRule(ctx, userID, ruleID, rule)
}

func (api enterpriseAPI) DeleteRule(ctx context.Context, userID, ruleID string) error {
	return api.repository.DeleteAlertRule(ctx, userID, ruleID)
}

func (api enterpriseAPI) ListEvents(ctx context.Context, userID string, limit int) ([]intelligence.AlertEvent, error) {
	return api.repository.AlertEventList(ctx, userID, limit)
}

func (api enterpriseAPI) ListNotifications(ctx context.Context, userID string, limit int, unreadOnly bool) ([]intelligence.Notification, error) {
	return api.repository.ListNotifications(ctx, userID, limit, unreadOnly)
}

func (api enterpriseAPI) UnreadCount(ctx context.Context, userID string) (int, error) {
	return api.repository.UnreadNotificationCount(ctx, userID)
}

func (api enterpriseAPI) MarkRead(ctx context.Context, userID string, id int64) error {
	return api.repository.MarkNotificationRead(ctx, userID, id)
}

func (api enterpriseAPI) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	return api.repository.MarkAllNotificationsRead(ctx, userID)
}

