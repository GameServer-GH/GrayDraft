package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Client wraps a PostgreSQL connection pool.
type Client struct {
	pool *pgxpool.Pool
}

// New creates a pool from a DSN.
func New(ctx context.Context, dsn string) (*Client, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool new: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &Client{pool: pool}, nil
}

// Pool exposes the underlying pool for health checks.
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

// Close shuts down the pool.
func (c *Client) Close() {
	c.pool.Close()
}
