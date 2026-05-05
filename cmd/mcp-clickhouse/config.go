package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Mode controls whether non-SELECT statements are allowed.
type Mode string

const (
	ModeReadOnly  Mode = "readonly"
	ModeReadWrite Mode = "readwrite"
)

// String implements fmt.Stringer.
func (m Mode) String() string { return string(m) }

// IsValid reports whether m is a recognized mode.
func (m Mode) IsValid() bool {
	switch m {
	case ModeReadOnly, ModeReadWrite:
		return true
	}
	return false
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *Mode) UnmarshalText(text []byte) error {
	candidate := Mode(text)
	if !candidate.IsValid() {
		return fmt.Errorf("invalid mode %q (allowed: %s, %s)",
			text, ModeReadOnly, ModeReadWrite)
	}
	*m = candidate
	return nil
}

type config struct {
	ClickHouseURL      string
	ClickHouseUsername string
	ClickHousePassword string
	ClickHouseDatabase string

	QueryTimeout time.Duration
	MaxRows      int
	Mode         Mode

	LogLevel slog.Level
}

// loadConfig reads and validates configuration from environment variables.
func loadConfig() (*config, error) {
	c := &config{
		ClickHouseURL:      os.Getenv("CLICKHOUSE_URL"),
		ClickHouseUsername: getenvDefault("CLICKHOUSE_USERNAME", "default"),
		ClickHousePassword: os.Getenv("CLICKHOUSE_PASSWORD"),
		ClickHouseDatabase: os.Getenv("CLICKHOUSE_DATABASE"),
	}

	if c.ClickHouseURL == "" {
		return nil, fmt.Errorf("CLICKHOUSE_URL is required")
	}

	timeout, err := parseDurationEnv("MCP_QUERY_TIMEOUT", "30s")
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("MCP_QUERY_TIMEOUT must be positive, got %s", timeout)
	}
	c.QueryTimeout = timeout

	maxRows, err := parsePositiveIntEnv("MCP_MAX_ROWS", "1000")
	if err != nil {
		return nil, err
	}
	c.MaxRows = maxRows

	if err := c.Mode.UnmarshalText([]byte(getenvDefault("MCP_MODE", string(ModeReadOnly)))); err != nil {
		return nil, fmt.Errorf("MCP_MODE: %w", err)
	}

	if err := c.LogLevel.UnmarshalText([]byte(getenvDefault("MCP_LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("MCP_LOG_LEVEL: %w", err)
	}

	return c, nil
}

func parseDurationEnv(key, def string) (time.Duration, error) {
	v := getenvDefault(key, def)
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func parsePositiveIntEnv(key, def string) (int, error) {
	v := getenvDefault(key, def)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %d", key, n)
	}
	return n, nil
}

// getenvDefault returns os.Getenv(key) when non-empty, otherwise def.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
