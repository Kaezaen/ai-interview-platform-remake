// Package database opens the PostgreSQL connection pool.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connectTimeout bounds the startup ping so a wrong host fails in seconds
// instead of hanging until the OS gives up.
const connectTimeout = 5 * time.Second

// Connect creates a pgx connection pool and verifies the database answers.
//
// It fails fast on purpose: a server that cannot reach its database at
// startup should exit and let its supervisor (Docker, systemd, the hosting
// platform) restart it, rather than accept traffic it cannot serve.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: parse DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping %s:%d: %w", cfg.ConnConfig.Host, cfg.ConnConfig.Port, err)
	}

	return pool, nil
}
