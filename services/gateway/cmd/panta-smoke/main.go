package main

import (
	"context"
	"log"
	"net/http"
	"os"
)

func main() {
	ctx := context.Background()

	pantaAPIURL := os.Getenv("PANTA_API_URL")
	pantaAPIKey := os.Getenv("PANTA_API_KEY")

	endpoints := []string{"account/", "markets/", "positions/"}

	for _, ep := range endpoints {
		log.Printf("\n=== Testing %s ===", ep)
		
		// With API key
		req, _ := http.NewRequestWithContext(ctx, "GET", pantaAPIURL+ep, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Api-Key", pantaAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("  With key - ERROR: %v", err)
		} else {
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			resp.Body.Close()
			log.Printf("  With key: status=%d, body=%.100s", resp.StatusCode, string(body[:n]))
		}

		// Without API key
		req2, _ := http.NewRequestWithContext(ctx, "GET", pantaAPIURL+ep, nil)
		req2.Header.Set("Accept", "application/json")

		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			log.Printf("  Without key - ERROR: %v", err)
		} else {
			body := make([]byte, 1024)
			n, _ := resp2.Body.Read(body)
			resp2.Body.Close()
			log.Printf("  Without key: status=%d, body=%.100s", resp2.StatusCode, string(body[:n]))
		}
	}
}