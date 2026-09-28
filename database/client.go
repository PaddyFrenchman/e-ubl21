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

// Connection is the bounded operation surface available during a WithConn
// callback. It does not expose the raw pgx connection.
type Connection interface {
	ScanRow(sql string, arguments []any, destinations ...any) error
	Exec(sql string, arguments ...any) (pgconn.CommandTag, error)
}

type leasedConnection struct {
	mu         sync.RWMutex
	connection *pgxpool.Conn
	ctx        context.Context
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
	poolConfig.MaxConns = config.maxConnections
	poolConfig.MinConns = config.minConnections
	poolConfig.ConnConfig.ConnectTimeout = config.connectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, operationError(ErrorPool, "pool creation", err)
	}
	return &Client{
		pool:           pool,
		connectTimeout: config.connectTimeout,
		queryTimeout:   config.queryTimeout,
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

// ForEachRow runs a bounded query and invokes callback once for every row.
// It keeps pgx rows internal to this package while allowing small operational
// inventories to be read without exposing the pool.
func (c *Client) ForEachRow(ctx context.Context, sql string, arguments []any, callback func(func(...any) error) error) error {
	if callback == nil {
		return configError("row callback", "is required")
	}
	operationCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	rows, err := c.pool.Query(operationCtx, sql, arguments...)
	if err != nil {
		return operationError(ErrorQuery, "query", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := callback(rows.Scan); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return operationError(ErrorQuery, "query rows", err)
	}
	return nil
}

// WithConn leases one pooled connection for a callback. Acquisition and every
// operation on Connection share the configured query deadline; the lease is
// always released before WithConn returns.
func (c *Client) WithConn(ctx context.Context, callback func(Connection) error) error {
	if callback == nil {
		return configError("database connection callback", "is required")
	}
	operationCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	connection, err := c.pool.Acquire(operationCtx)
	if err != nil {
		return operationError(ErrorPool, "connection acquisition", err)
	}
	lease := &leasedConnection{connection: connection, ctx: operationCtx}
	defer func() {
		cancel()
		lease.invalidate()
		connection.Release()
	}()
	if err := callback(lease); err != nil {
		return err
	}
	if err := operationCtx.Err(); err != nil {
		return operationError(ErrorQuery, "connection callback", err)
	}
	return nil
}

// ScanRow runs a single-row query on the leased connection.
func (c *leasedConnection) ScanRow(sql string, arguments []any, destinations ...any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.connection == nil {
		return operationError(ErrorPool, "expired connection lease", context.Canceled)
	}
	if err := c.connection.QueryRow(c.ctx, sql, arguments...).Scan(destinations...); err != nil {
		return operationError(ErrorQuery, "connection query", err)
	}
	return nil
}

// Exec runs a statement on the leased connection.
func (c *leasedConnection) Exec(sql string, arguments ...any) (pgconn.CommandTag, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.connection == nil {
		return pgconn.CommandTag{}, operationError(ErrorPool, "expired connection lease", context.Canceled)
	}
	tag, err := c.connection.Exec(c.ctx, sql, arguments...)
	if err != nil {
		return pgconn.CommandTag{}, operationError(ErrorQuery, "connection exec", err)
	}
	return tag, nil
}

func (c *leasedConnection) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connection = nil
}

// Close gracefully drains and closes the pool. It is safe to call repeatedly.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(c.pool.Close)
}
