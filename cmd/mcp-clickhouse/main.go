// Command mcp-clickhouse is a Model Context Protocol server that exposes
// a ClickHouse instance to MCP clients over stdio.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

const version = "0.1.0"

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

	logger.Info("mcp-clickhouse starting",
		"version", version,
		"mode", cfg.Mode,
		"query_timeout", cfg.QueryTimeout,
		"max_rows", cfg.MaxRows,
	)

	<-ctx.Done()
	logger.Info("shutdown signal received")
}
