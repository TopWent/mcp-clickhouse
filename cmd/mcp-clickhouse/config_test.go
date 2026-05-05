package main

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestMode_IsValid(t *testing.T) {
	cases := []struct {
		mode  Mode
		valid bool
	}{
		{ModeReadOnly, true},
		{ModeReadWrite, true},
		{Mode(""), false},
		{Mode("readonly "), false},
		{Mode("READONLY"), false},
		{Mode("rw"), false},
	}
	for _, tc := range cases {
		if got := tc.mode.IsValid(); got != tc.valid {
			t.Errorf("Mode(%q).IsValid() = %v, want %v", tc.mode, got, tc.valid)
		}
	}
}

func TestMode_String(t *testing.T) {
	if got := ModeReadOnly.String(); got != "readonly" {
		t.Errorf("ModeReadOnly.String() = %q, want %q", got, "readonly")
	}
	if got := ModeReadWrite.String(); got != "readwrite" {
		t.Errorf("ModeReadWrite.String() = %q, want %q", got, "readwrite")
	}
}

func TestMode_UnmarshalText(t *testing.T) {
	cases := []struct {
		input   string
		want    Mode
		wantErr bool
	}{
		{"readonly", ModeReadOnly, false},
		{"readwrite", ModeReadWrite, false},
		{"", Mode(""), true},
		{"unknown", Mode(""), true},
		{"READONLY", Mode(""), true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var m Mode
			err := m.UnmarshalText([]byte(tc.input))
			if (err != nil) != tc.wantErr {
				t.Fatalf("UnmarshalText(%q) err = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
			if m != tc.want {
				t.Errorf("UnmarshalText(%q) m = %q, want %q", tc.input, m, tc.want)
			}
		})
	}
}

// TestLoadConfig exercises every observable behavior of loadConfig: defaults,
// each individual validation error, and successful overrides. Each subtest
// resets the relevant env vars via t.Setenv so cases stay isolated.
func TestLoadConfig(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string // substring match; empty means success expected
		check   func(t *testing.T, c *config)
	}{
		{
			name: "defaults applied with only required URL",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
			},
			check: func(t *testing.T, c *config) {
				if c.ClickHouseURL != "http://localhost:8123" {
					t.Errorf("URL = %q", c.ClickHouseURL)
				}
				if c.ClickHouseUsername != "default" {
					t.Errorf("username default = %q", c.ClickHouseUsername)
				}
				if c.QueryTimeout != 30*time.Second {
					t.Errorf("timeout default = %s", c.QueryTimeout)
				}
				if c.MaxRows != 1000 {
					t.Errorf("max rows default = %d", c.MaxRows)
				}
				if c.Mode != ModeReadOnly {
					t.Errorf("mode default = %s", c.Mode)
				}
				if c.LogLevel != slog.LevelInfo {
					t.Errorf("log level default = %s", c.LogLevel)
				}
			},
		},
		{
			name:    "missing URL returns clear error",
			env:     map[string]string{},
			wantErr: "CLICKHOUSE_URL is required",
		},
		{
			name: "unparseable timeout",
			env: map[string]string{
				"CLICKHOUSE_URL":    "http://localhost:8123",
				"MCP_QUERY_TIMEOUT": "not-a-duration",
			},
			wantErr: "MCP_QUERY_TIMEOUT",
		},
		{
			name: "negative timeout rejected",
			env: map[string]string{
				"CLICKHOUSE_URL":    "http://localhost:8123",
				"MCP_QUERY_TIMEOUT": "-1s",
			},
			wantErr: "must be positive",
		},
		{
			name: "zero timeout rejected",
			env: map[string]string{
				"CLICKHOUSE_URL":    "http://localhost:8123",
				"MCP_QUERY_TIMEOUT": "0s",
			},
			wantErr: "must be positive",
		},
		{
			name: "non-numeric max rows",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_MAX_ROWS":   "abc",
			},
			wantErr: "MCP_MAX_ROWS",
		},
		{
			name: "zero max rows rejected",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_MAX_ROWS":   "0",
			},
			wantErr: "must be positive",
		},
		{
			name: "negative max rows rejected",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_MAX_ROWS":   "-5",
			},
			wantErr: "must be positive",
		},
		{
			name: "unknown mode",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_MODE":       "yolo",
			},
			wantErr: "MCP_MODE",
		},
		{
			name: "readwrite mode is honored",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_MODE":       "readwrite",
			},
			check: func(t *testing.T, c *config) {
				if c.Mode != ModeReadWrite {
					t.Errorf("Mode = %s, want readwrite", c.Mode)
				}
			},
		},
		{
			name: "unknown log level",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_LOG_LEVEL":  "verbose",
			},
			wantErr: "MCP_LOG_LEVEL",
		},
		{
			name: "debug log level is honored",
			env: map[string]string{
				"CLICKHOUSE_URL": "http://localhost:8123",
				"MCP_LOG_LEVEL":  "debug",
			},
			check: func(t *testing.T, c *config) {
				if c.LogLevel != slog.LevelDebug {
					t.Errorf("LogLevel = %s, want debug", c.LogLevel)
				}
			},
		},
		{
			name: "credentials propagated",
			env: map[string]string{
				"CLICKHOUSE_URL":      "http://chnode:8123",
				"CLICKHOUSE_USERNAME": "analytics_ro",
				"CLICKHOUSE_PASSWORD": "s3cret",
			},
			check: func(t *testing.T, c *config) {
				if c.ClickHouseUsername != "analytics_ro" {
					t.Errorf("Username = %q", c.ClickHouseUsername)
				}
				if c.ClickHousePassword != "s3cret" {
					t.Errorf("Password not propagated")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := loadConfig()

			switch {
			case tc.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.check != nil {
				if cfg == nil {
					t.Fatal("config is nil but no error was returned")
				}
				tc.check(t, cfg)
			}
		})
	}
}

// clearConfigEnv blanks every env var loadConfig touches so each subtest
// starts from a deterministic baseline. t.Setenv restores prior values
// when the test exits.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CLICKHOUSE_URL",
		"CLICKHOUSE_USERNAME",
		"CLICKHOUSE_PASSWORD",
		"MCP_QUERY_TIMEOUT",
		"MCP_MAX_ROWS",
		"MCP_MODE",
		"MCP_LOG_LEVEL",
	} {
		t.Setenv(key, "")
	}
}
