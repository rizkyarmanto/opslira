// Package db opens the Postgres connection pool used by every feature slice.
package db

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"opslira/backend/internal/config"
)

// Open connects a pgx pool and verifies the connection with a ping.
func Open(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgresql://%s@%s/%s?sslmode=%s",
		url.UserPassword(cfg.User, cfg.Password).String(),
		net.JoinHostPort(cfg.Host, cfg.Port),
		url.PathEscape(cfg.Name),
		cfg.SSLMode,
	)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("db pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return pool, nil
}
