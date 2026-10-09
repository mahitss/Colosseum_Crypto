package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    // Test different connection string formats
    urls := []string{
        "postgresql://prophet:prophet@127.0.0.1:5432/prophet?sslmode=disable",
        "postgresql://prophet:prophet@localhost:5432/prophet?sslmode=disable",
        "postgresql://prophet:prophet@[::1]:5432/prophet?sslmode=disable",
    }

    for _, url := range urls {
        fmt.Printf("Testing: %s\n", url)
        pool, err := pgxpool.New(context.Background(), url)
        if err != nil {
            fmt.Printf("  ERROR creating pool: %v\n", err)
            continue
        }
        defer pool.Close()

        var result int
        err = pool.QueryRow(context.Background(), "SELECT 1").Scan(&result)
        if err != nil {
            fmt.Printf("  ERROR querying: %v\n", err)
        } else {
            fmt.Printf("  SUCCESS: %d\n", result)
        }
    }
}