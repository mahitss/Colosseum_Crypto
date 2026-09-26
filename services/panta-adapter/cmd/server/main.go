package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prophet/panta-adapter/internal/client"
	"prophet/panta-adapter/internal/config"
	"prophet/panta-adapter/internal/transport"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid Panta adapter configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}
	apiClient, err := client.New(cfg.BaseURL, cfg.APIKey, cfg.Timeout, client.Options{Logger: logger})
	if err != nil {
		logger.Error("invalid Panta adapter configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	server := &http.Server{
		Addr: ":" + port, Handler: transport.NewHandler(cfg, apiClient, transport.NewMarketService(apiClient)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("Panta adapter listening", slog.String("address", server.Addr))

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdown.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Panta adapter shutdown failed", slog.String("error", err.Error()))
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Panta adapter server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
