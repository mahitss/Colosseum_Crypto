package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"prophet/panta-adapter/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("panta adapter starting")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	addr := fmt.Sprintf(":%s", port)
	logger.Info("panta adapter listening", slog.String("address", addr))

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
