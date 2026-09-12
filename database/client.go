package database

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Client owns a bounded pgx connection pool.
type Client struct {
	pool           *pgxpool.Pool
	queryTimeout   time.Duration
	connectTimeout time.Duration
	closeOnce      sync.Once
}

// Open creates a connection pool without requiring an immediate database
// connection. Use Connect when startup must prove connectivity.
func Open(ctx context.Context, config Config) (*Client, error) {
	if config.connectionString == "" {
		return nil, configError("database configuration", "must be created with ParseConfig")
	}
	poolConfig, err := pgxpool.ParseConfig(config.connectionString)
	if err != nil {
		return nil, operationError(ErrorConfiguration, "pool configuration", err)
	}
	poolConfig.MaxConns = config.MaxConnections
	poolConfig.MinConns = config.MinConnections
	poolConfig.ConnConfig.ConnectTimeout = config.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, operationError(ErrorPool, "pool creation", err)
	}
	return &Client{
		pool:           pool,
		connectTimeout: config.ConnectTimeout,
		queryTimeout:   config.QueryTimeout,
	}, nil
}

// Connect creates a pool and runs the startup connectivity probe.
func Connect(ctx context.Context, config Config) (*Client, error) {
	client, err := Open(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := client.Probe(ctx); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

// Probe verifies that the pool can reach the configured database.
func (c *Client) Probe(ctx context.Context) error {
	operationCtx, cancel := context.WithTimeout(ctx, c.connectTimeout)
	defer cancel()
	if err := c.pool.Ping(operationCtx); err != nil {
		return operationError(ErrorConnectivity, "connectivity probe", err)
	}
	return nil
}

// ScanRow runs a single-row query within the configured query deadline.
func (c *Client) ScanRow(ctx context.Context, sql string, arguments []any, destinations ...any) error {
	operationCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	if err := c.pool.QueryRow(operationCtx, sql, arguments...).Scan(destinations...); err != nil {
		return operationError(ErrorQuery, "query", err)
	}
	return nil
}

// Exec runs a statement within the configured query deadline.
func (c *Client) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	operationCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	tag, err := c.pool.Exec(operationCtx, sql, arguments...)
	if err != nil {
		return pgconn.CommandTag{}, operationError(ErrorQuery, "exec", err)
	}
	return tag, nil
}

// Acquire obtains a pooled connection within the configured query deadline.
// The caller must release a successful acquisition.
func (c *Client) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	operationCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	connection, err := c.pool.Acquire(operationCtx)
	if err != nil {
		return nil, operationError(ErrorPool, "connection acquisition", err)
	}
	return connection, nil
}

// Close gracefully drains and closes the pool. It is safe to call repeatedly.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(c.pool.Close)
}
