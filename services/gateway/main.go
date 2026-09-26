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
	logger.Info("gateway listening", slog.String("address", addr))

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandlerWithServices(adapterClient, repository),
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
