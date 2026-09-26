package intelligence

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	var applied bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=1)`).Scan(&applied); err != nil {
		return fmt.Errorf("check migration version: %w", err)
	}
	if applied {
		return nil
	}
	sql, err := schema.ReadFile("migrations/001_market_intelligence.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, statement := range strings.Split(string(sql), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("apply migration statement: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES(1) ON CONFLICT DO NOTHING`); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit(ctx)
}
