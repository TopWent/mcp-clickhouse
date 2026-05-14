package clickhouse

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid minimum",
			cfg:  Config{Addr: "localhost:9000"},
		},
		{
			name: "valid full",
			cfg: Config{
				Addr:         "localhost:9000",
				Username:     "default",
				Password:     "secret",
				Database:     "metrics",
				QueryTimeout: 5 * time.Second,
				MaxRows:      500,
			},
		},
		{
			name:    "missing addr",
			cfg:     Config{Username: "default"},
			wantErr: "addr is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestOpen_RejectsInvalidConfig(t *testing.T) {
	_, err := Open(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error for empty config, got nil")
	}
	if !strings.Contains(err.Error(), "addr is required") {
		t.Errorf("err = %q, want addr is required", err.Error())
	}
}

type fakeQuerier struct {
	result *QueryResult
	err    error
	pings  int
}

func (f *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (*QueryResult, error) {
	return f.result, f.err
}

func (f *fakeQuerier) Ping(_ context.Context) error {
	f.pings++
	return nil
}

func TestQuerierInterface_FakeImplements(t *testing.T) {
	var _ Querier = (*fakeQuerier)(nil)

	f := &fakeQuerier{
		result: &QueryResult{
			Columns: []string{"name"},
			Types:   []string{"String"},
			Rows:    [][]any{{"foo"}, {"bar"}},
		},
	}

	got, err := f.Query(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("fake Query returned error: %v", err)
	}
	if len(got.Rows) != 2 {
		t.Errorf("rows = %d, want 2", len(got.Rows))
	}

	if err := f.Ping(context.Background()); err != nil {
		t.Fatalf("fake Ping returned error: %v", err)
	}
	if f.pings != 1 {
		t.Errorf("pings = %d, want 1", f.pings)
	}
}

func TestQueryResult_TruncatedFlag(t *testing.T) {
	r := &QueryResult{Truncated: true}
	if !r.Truncated {
		t.Error("Truncated field should be assignable and readable")
	}
}
