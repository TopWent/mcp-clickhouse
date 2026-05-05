// Command mcp-clickhouse is a Model Context Protocol server that exposes
// a ClickHouse instance to MCP clients over stdio.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
	"github.com/TopWent/mcp-clickhouse/internal/mcp"
	"github.com/TopWent/mcp-clickhouse/internal/tools"
)

const (
	version         = "0.1.0"
	protocolVersion = "2024-11-05"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config, logger *slog.Logger) error {
	logger.Info("connecting to clickhouse",
		"addr", cfg.ClickHouseURL,
		"database", cfg.ClickHouseDatabase,
		"mode", cfg.Mode,
	)

	client, err := chpkg.Open(ctx, chpkg.Config{
		Addr:         cfg.ClickHouseURL,
		Username:     cfg.ClickHouseUsername,
		Password:     cfg.ClickHousePassword,
		Database:     cfg.ClickHouseDatabase,
		QueryTimeout: cfg.QueryTimeout,
		MaxRows:      cfg.MaxRows,
	})
	if err != nil {
		return fmt.Errorf("clickhouse: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			logger.Warn("close clickhouse", "err", err)
		}
	}()

	registry := tools.NewRegistry()
	registry.Register(&tools.ListDatabases{DB: client})
	registry.Register(&tools.ListTables{DB: client})
	registry.Register(&tools.DescribeTable{DB: client})
	registry.Register(&tools.GetTableStats{DB: client})
	registry.Register(&tools.RunQuery{DB: client, ReadOnly: cfg.Mode == ModeReadOnly})

	server := mcp.NewServer(logger)
	registerHandlers(server, registry, logger)

	logger.Info("mcp-clickhouse ready",
		"version", version,
		"tools", len(registry.Definitions()),
		"query_timeout", cfg.QueryTimeout,
		"max_rows", cfg.MaxRows,
	)

	return server.Serve(ctx, os.Stdin, os.Stdout)
}

func registerHandlers(s *mcp.Server, r *tools.Registry, logger *slog.Logger) {
	s.Handle("initialize", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]any{
			"protocolVersion": protocolVersion,
			"serverInfo": map[string]string{
				"name":    "mcp-clickhouse",
				"version": version,
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}, nil
	})

	s.Handle("notifications/initialized", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, nil
	})

	s.Handle("tools/list", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]any{"tools": r.Definitions()}, nil
	})

	s.Handle("tools/call", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, mcp.NewErrorf(mcp.CodeInvalidParams, "parse params: %s", err)
		}
		if p.Name == "" {
			return nil, mcp.NewError(mcp.CodeInvalidParams, "missing tool name")
		}
		tool, ok := r.Get(p.Name)
		if !ok {
			return nil, mcp.NewErrorf(mcp.CodeMethodNotFound, "tool not found: %s", p.Name)
		}
		result, err := tool.Call(ctx, p.Arguments)
		if err != nil {
			logger.Warn("tool failed", "tool", p.Name, "err", err)
			return nil, err
		}
		return result, nil
	})
}
