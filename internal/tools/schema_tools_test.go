package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

// queryRecorder records each Query call for tools that issue more than one.
type queryRecorder struct {
	calls []recordedCall
	// results queues up responses in order; if exhausted, returns errResultsExhausted.
	results []*chpkg.QueryResult
	errs    []error
	idx     int
}

type recordedCall struct {
	query string
	args  []any
}

func (q *queryRecorder) Query(_ context.Context, query string, args ...any) (*chpkg.QueryResult, error) {
	q.calls = append(q.calls, recordedCall{query: query, args: args})
	if q.idx >= len(q.results) {
		return nil, errors.New("queryRecorder: results exhausted")
	}
	r := q.results[q.idx]
	var err error
	if q.idx < len(q.errs) {
		err = q.errs[q.idx]
	}
	q.idx++
	return r, err
}

func (q *queryRecorder) Ping(_ context.Context) error { return nil }

func TestListTables_Contract(t *testing.T) {
	tool := &ListTables{}
	if tool.Name() != "list_tables" {
		t.Errorf("Name = %q", tool.Name())
	}

	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("InputSchema invalid: %v", err)
	}
	required, _ := schema["required"].([]any)
	if len(required) != 1 || required[0] != "database" {
		t.Errorf("required = %v, want [database]", required)
	}
}

func TestListTables_MissingArgument(t *testing.T) {
	tool := &ListTables{DB: &fakeQuerier{}}

	res, err := tool.Call(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError for missing database")
	}
	if !strings.Contains(res.Content[0].Text, "database") {
		t.Errorf("error message should mention database: %q", res.Content[0].Text)
	}
}

func TestListTables_InvalidJSON(t *testing.T) {
	tool := &ListTables{DB: &fakeQuerier{}}

	_, err := tool.Call(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse args") {
		t.Errorf("err should mention parse args: %v", err)
	}
}

func TestListTables_PassesDatabaseAsArg(t *testing.T) {
	fake := &fakeQuerier{
		result: &chpkg.QueryResult{
			Columns: []string{"name", "engine"},
			Rows:    [][]any{{"users", "MergeTree"}},
		},
	}
	tool := &ListTables{DB: fake}

	if _, err := tool.Call(context.Background(), json.RawMessage(`{"database":"metrics"}`)); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(fake.gotQuery, "system.tables") {
		t.Errorf("query should target system.tables: %s", fake.gotQuery)
	}
	if len(fake.gotArgs) != 1 || fake.gotArgs[0] != "metrics" {
		t.Errorf("args = %v, want [metrics]", fake.gotArgs)
	}
}

func TestDescribeTable_Contract(t *testing.T) {
	tool := &DescribeTable{}
	if tool.Name() != "describe_table" {
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

func TestDescribeTable_MissingArguments(t *testing.T) {
	tool := &DescribeTable{DB: &queryRecorder{}}

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

func TestDescribeTable_TableNotFound(t *testing.T) {
	rec := &queryRecorder{
		results: []*chpkg.QueryResult{
			{Columns: []string{"engine"}, Rows: [][]any{}},
		},
	}
	tool := &DescribeTable{DB: rec}

	res, err := tool.Call(context.Background(),
		json.RawMessage(`{"database":"metrics","table":"missing"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError for missing table")
	}
	if !strings.Contains(res.Content[0].Text, "missing") {
		t.Errorf("error text should mention table name: %q", res.Content[0].Text)
	}
	// Only the info query should have run; columns query is skipped.
	if len(rec.calls) != 1 {
		t.Errorf("expected 1 query, got %d", len(rec.calls))
	}
}

func TestDescribeTable_RendersInfoAndColumns(t *testing.T) {
	rec := &queryRecorder{
		results: []*chpkg.QueryResult{
			{
				Columns: []string{"engine", "total_rows", "size"},
				Rows:    [][]any{{"MergeTree", uint64(1000000), "45 MiB"}},
			},
			{
				Columns: []string{"name", "type", "is_in_primary_key"},
				Rows: [][]any{
					{"id", "UInt64", uint8(1)},
					{"created_at", "DateTime", uint8(0)},
				},
			},
		},
	}
	tool := &DescribeTable{DB: rec}

	res, err := tool.Call(context.Background(),
		json.RawMessage(`{"database":"metrics","table":"events"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.IsError {
		t.Errorf("unexpected IsError: %s", res.Content[0].Text)
	}

	text := res.Content[0].Text
	wantStrings := []string{
		"## metrics.events",
		"engine",
		"MergeTree",
		"45 MiB",
		"### Columns",
		"id",
		"UInt64",
		"created_at",
		"DateTime",
	}
	for _, want := range wantStrings {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}

	if len(rec.calls) != 2 {
		t.Errorf("expected 2 queries, got %d", len(rec.calls))
	}
	if !strings.Contains(rec.calls[0].query, "system.tables") {
		t.Errorf("first query should hit system.tables: %s", rec.calls[0].query)
	}
	if !strings.Contains(rec.calls[1].query, "system.columns") {
		t.Errorf("second query should hit system.columns: %s", rec.calls[1].query)
	}
}

func TestDescribeTable_InfoQueryError(t *testing.T) {
	rec := &queryRecorder{
		results: []*chpkg.QueryResult{nil},
		errs:    []error{errors.New("boom")},
	}
	tool := &DescribeTable{DB: rec}

	_, err := tool.Call(context.Background(),
		json.RawMessage(`{"database":"x","table":"y"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "describe_table info") {
		t.Errorf("error should be wrapped with describe_table info: %v", err)
	}
}
