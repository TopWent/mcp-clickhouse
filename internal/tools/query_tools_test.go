package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

func TestRunQuery_Contract(t *testing.T) {
	tool := &RunQuery{}
	if tool.Name() != "run_query" {
		t.Errorf("Name = %q", tool.Name())
	}

	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("InputSchema invalid: %v", err)
	}
	required, _ := schema["required"].([]any)
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v, want [query]", required)
	}
}

func TestRunQuery_MissingQuery(t *testing.T) {
	tool := &RunQuery{DB: &fakeQuerier{}, ReadOnly: true}

	res, err := tool.Call(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError for missing query")
	}
}

func TestRunQuery_ReadOnlyAcceptsSelect(t *testing.T) {
	fake := &fakeQuerier{
		result: &chpkg.QueryResult{
			Columns: []string{"n"},
			Rows:    [][]any{{1}},
		},
	}
	tool := &RunQuery{DB: fake, ReadOnly: true}

	res, err := tool.Call(context.Background(),
		json.RawMessage(`{"query":"SELECT 1"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.IsError {
		t.Errorf("SELECT should be accepted: %s", res.Content[0].Text)
	}
	if !strings.Contains(fake.gotQuery, "SELECT 1") {
		t.Errorf("query was not forwarded: %s", fake.gotQuery)
	}
}

func TestRunQuery_ReadOnlyAcceptsExplain(t *testing.T) {
	fake := &fakeQuerier{result: &chpkg.QueryResult{Columns: []string{"plan"}, Rows: [][]any{{"x"}}}}
	tool := &RunQuery{DB: fake, ReadOnly: true}

	res, _ := tool.Call(context.Background(),
		json.RawMessage(`{"query":"EXPLAIN SELECT * FROM t"}`))
	if res.IsError {
		t.Errorf("EXPLAIN should be accepted: %s", res.Content[0].Text)
	}
}

func TestRunQuery_ReadOnlyRejectsDrop(t *testing.T) {
	fake := &fakeQuerier{}
	tool := &RunQuery{DB: fake, ReadOnly: true}

	res, err := tool.Call(context.Background(),
		json.RawMessage(`{"query":"DROP TABLE x"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !res.IsError {
		t.Fatal("DROP should be rejected in readonly")
	}
	if !strings.Contains(res.Content[0].Text, "rejected") {
		t.Errorf("rejection message missing: %q", res.Content[0].Text)
	}
	if fake.gotQuery != "" {
		t.Errorf("rejected query reached DB: %s", fake.gotQuery)
	}
}

func TestRunQuery_ReadOnlyRejectsMultiStatement(t *testing.T) {
	fake := &fakeQuerier{}
	tool := &RunQuery{DB: fake, ReadOnly: true}

	res, _ := tool.Call(context.Background(),
		json.RawMessage(`{"query":"SELECT 1; DROP TABLE x"}`))
	if !res.IsError {
		t.Fatal("multi-statement should be rejected in readonly")
	}
	if fake.gotQuery != "" {
		t.Errorf("rejected query reached DB: %s", fake.gotQuery)
	}
}

func TestRunQuery_ReadWriteSkipsValidator(t *testing.T) {
	fake := &fakeQuerier{
		result: &chpkg.QueryResult{
			Columns: []string{"affected"},
			Rows:    [][]any{{1}},
		},
	}
	tool := &RunQuery{DB: fake, ReadOnly: false}

	// Validator is skipped: a DROP would normally be rejected, but in
	// ReadWrite mode it should reach the DB. We assert it was forwarded;
	// the fake just returns canned results.
	if _, err := tool.Call(context.Background(),
		json.RawMessage(`{"query":"DROP TABLE x"}`)); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(fake.gotQuery, "DROP TABLE x") {
		t.Errorf("query should reach DB in ReadWrite mode: %s", fake.gotQuery)
	}
}

func TestRunQuery_DBErrorPropagates(t *testing.T) {
	fake := &fakeQuerier{err: errors.New("syntax error")}
	tool := &RunQuery{DB: fake, ReadOnly: true}

	_, err := tool.Call(context.Background(),
		json.RawMessage(`{"query":"SELECT 1"}`))
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
	if !strings.Contains(err.Error(), "run_query") {
		t.Errorf("error should be wrapped: %v", err)
	}
}

func TestGetTableStats_Contract(t *testing.T) {
	tool := &GetTableStats{}
	if tool.Name() != "get_table_stats" {
		t.Errorf("Name = %q", tool.Name())
	}

	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("InputSchema invalid: %v", err)
	}
	required, _ := schema["required"].([]any)
	if len(required) != 2 {
		t.Errorf("required len = %d, want 2", len(required))
	}
}

func TestGetTableStats_MissingArguments(t *testing.T) {
	tool := &GetTableStats{DB: &fakeQuerier{}}

	cases := []string{
		`{}`,
		`{"database":"metrics"}`,
		`{"table":"events"}`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			res, err := tool.Call(context.Background(), json.RawMessage(c))
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			if !res.IsError {
				t.Errorf("expected IsError for %s", c)
			}
		})
	}
}

func TestGetTableStats_ForwardsArgs(t *testing.T) {
	fake := &fakeQuerier{
		result: &chpkg.QueryResult{
			Columns: []string{"rows", "disk_size"},
			Rows:    [][]any{{uint64(1234), "10 MiB"}},
		},
	}
	tool := &GetTableStats{DB: fake}

	res, err := tool.Call(context.Background(),
		json.RawMessage(`{"database":"metrics","table":"events"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.IsError {
		t.Errorf("unexpected IsError: %s", res.Content[0].Text)
	}
	if !strings.Contains(fake.gotQuery, "system.parts") {
		t.Errorf("query should target system.parts: %s", fake.gotQuery)
	}
	if len(fake.gotArgs) != 2 || fake.gotArgs[0] != "metrics" || fake.gotArgs[1] != "events" {
		t.Errorf("args = %v, want [metrics events]", fake.gotArgs)
	}
}
