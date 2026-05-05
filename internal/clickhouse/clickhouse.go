// Package clickhouse wraps clickhouse-go/v2 with a narrow Querier interface.
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type Config struct {
	Addr         string
	Username     string
	Password     string
	Database     string
	QueryTimeout time.Duration
	MaxRows      int
}

func (c Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("addr is required")
	}
	return nil
}

type QueryResult struct {
	Columns   []string
	Types     []string
	Rows      [][]any
	Truncated bool
}

type Querier interface {
	Query(ctx context.Context, query string, args ...any) (*QueryResult, error)
	Ping(ctx context.Context) error
}

type Client struct {
	db      *sql.DB
	timeout time.Duration
	maxRows int
}

func Open(ctx context.Context, cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	timeout := cfg.QueryTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxRows := cfg.MaxRows
	if maxRows <= 0 {
		maxRows = 1000
	}

	opts := &clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": int(timeout.Seconds()),
		},
		DialTimeout: 10 * time.Second,
		ReadTimeout: timeout,
	}

	db := clickhouse.OpenDB(opts)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &Client{db: db, timeout: timeout, maxRows: maxRows}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func (c *Client) Query(ctx context.Context, query string, args ...any) (*QueryResult, error) {
	queryCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	rows, err := c.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("column types: %w", err)
	}
	types := make([]string, len(colTypes))
	for i, t := range colTypes {
		types[i] = t.DatabaseTypeName()
	}

	out := &QueryResult{
		Columns: cols,
		Types:   types,
		Rows:    make([][]any, 0),
	}

	for rows.Next() {
		if len(out.Rows) >= c.maxRows {
			out.Truncated = true
			break
		}
		scanArgs := make([]any, len(cols))
		for i := range scanArgs {
			var v any
			scanArgs[i] = &v
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("scan row %d: %w", len(out.Rows), err)
		}
		row := make([]any, len(cols))
		for i, p := range scanArgs {
			row[i] = *(p.(*any))
		}
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return out, nil
}
