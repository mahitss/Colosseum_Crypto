package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"prophet/gateway/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("gateway starting")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	logger.Info("gateway listening", slog.String("address", addr))

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
