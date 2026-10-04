package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"qevryn/panta-adapter/internal/client"
	"qevryn/panta-adapter/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	if err := cfg.Validate(); err != nil {
		fail(fmt.Errorf("configure PANTA_API_KEY before running the Panta smoke test"))
	}
	apiClient, err := client.New(cfg.BaseURL, cfg.APIKey, cfg.Timeout, client.Options{Logger: slog.New(slog.NewJSONHandler(os.Stderr, nil))})
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout*4)
	defer cancel()
	page, err := apiClient.ListMarkets(ctx, client.PantaMarketQuery{Limit: 1})
	if err != nil {
		fail(err)
	}
	fmt.Printf("Panta market read succeeded: returned %d market(s)\n", len(page.Items))
	for _, market := range page.Items {
		fmt.Printf("marketId=%s title=%q phase=%s\n", market.MarketID, market.Title, market.Phase)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

