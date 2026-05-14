package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

type fakeQuerier struct {
	gotQuery string
	gotArgs  []any
	result   *chpkg.QueryResult
	err      error
}

func (f *fakeQuerier) Query(_ context.Context, query string, args ...any) (*chpkg.QueryResult, error) {
	f.gotQuery = query
	f.gotArgs = args
	return f.result, f.err
}

func (f *fakeQuerier) Ping(_ context.Context) error { return nil }

type stubTool struct {
	name string
}

func (s *stubTool) Name() string                 { return s.name }
func (s *stubTool) Description() string          { return "stub for " + s.name }
func (s *stubTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (s *stubTool) Call(_ context.Context, _ json.RawMessage) (Result, error) {
	return TextResult("ok"), nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	if _, ok := r.Get("missing"); ok {
		t.Error("Get on empty registry returned ok=true")
	}

	a := &stubTool{name: "a"}
	r.Register(a)

	got, ok := r.Get("a")
	if !ok {
		t.Fatal("Get failed for registered tool")
	}
	if got != a {
		t.Errorf("Get returned %v, want %v", got, a)
	}
}

func TestRegistry_RegisterOverwrites(t *testing.T) {
	r := NewRegistry()
	first := &stubTool{name: "x"}
	second := &stubTool{name: "x"}

	r.Register(first)
	r.Register(second)

	got, _ := r.Get("x")
	if got != second {
		t.Error("second Register did not overwrite first")
	}
}

func TestRegistry_DefinitionsAreSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "charlie"})
	r.Register(&stubTool{name: "alpha"})
	r.Register(&stubTool{name: "bravo"})

	defs := r.Definitions()
	if len(defs) != 3 {
		t.Fatalf("got %d definitions, want 3", len(defs))
	}
	want := []string{"alpha", "bravo", "charlie"}
	for i, d := range defs {
		if d.Name != want[i] {
			t.Errorf("Definitions()[%d].Name = %q, want %q", i, d.Name, want[i])
		}
		if d.Description != "stub for "+want[i] {
			t.Errorf("Definitions()[%d].Description unexpected: %q", i, d.Description)
		}
		if string(d.InputSchema) != `{"type":"object"}` {
			t.Errorf("Definitions()[%d].InputSchema unexpected: %s", i, d.InputSchema)
		}
	}
}

func TestTextResultAndErrorResult(t *testing.T) {
	ok := TextResult("hello")
	if ok.IsError {
		t.Error("TextResult should not be marked as error")
	}
	if len(ok.Content) != 1 || ok.Content[0].Text != "hello" || ok.Content[0].Type != "text" {
		t.Errorf("TextResult content unexpected: %+v", ok.Content)
	}

	bad := ErrorResult("nope")
	if !bad.IsError {
		t.Error("ErrorResult should be marked as error")
	}
	if bad.Content[0].Text != "nope" {
		t.Errorf("ErrorResult content unexpected: %+v", bad.Content)
	}
}

func TestFormatTable_Empty(t *testing.T) {
	if got := FormatTable(nil); got != "(no results)" {
		t.Errorf("nil result = %q", got)
	}
	r := &chpkg.QueryResult{Columns: []string{"x"}}
	if got := FormatTable(r); got != "(no results)" {
		t.Errorf("empty rows = %q", got)
	}
}

func TestFormatTable_BasicMarkdown(t *testing.T) {
	r := &chpkg.QueryResult{
		Columns: []string{"name", "engine"},
		Rows: [][]any{
			{"users", "MergeTree"},
			{"events", "Distributed"},
		},
	}
	got := FormatTable(r)

	wantHeader := "| name | engine |"
	wantSep := "|---|---|"
	if !strings.Contains(got, wantHeader) {
		t.Errorf("missing header in output:\n%s", got)
	}
	if !strings.Contains(got, wantSep) {
		t.Errorf("missing separator in output:\n%s", got)
	}
	if !strings.Contains(got, "| users | MergeTree |") {
		t.Errorf("missing first row:\n%s", got)
	}
	if !strings.Contains(got, "| events | Distributed |") {
		t.Errorf("missing second row:\n%s", got)
	}
}

func TestFormatTable_TruncatedMarker(t *testing.T) {
	r := &chpkg.QueryResult{
		Columns:   []string{"id"},
		Rows:      [][]any{{1}, {2}},
		Truncated: true,
	}
	got := FormatTable(r)
	if !strings.Contains(got, "(truncated to 2 rows)") {
		t.Errorf("missing truncation marker:\n%s", got)
	}
}

func TestFormatTable_NullAndEscaping(t *testing.T) {
	r := &chpkg.QueryResult{
		Columns: []string{"raw", "with_pipe", "with_newline"},
		Rows: [][]any{
			{nil, "a|b", "first\nsecond"},
		},
	}
	got := FormatTable(r)
	if !strings.Contains(got, "| NULL |") {
		t.Errorf("nil should render as NULL:\n%s", got)
	}
	if !strings.Contains(got, `a\|b`) {
		t.Errorf("pipe should be escaped:\n%s", got)
	}
	if strings.Contains(got, "first\nsecond") {
		t.Errorf("newline in cell should be replaced:\n%s", got)
	}
}

func TestFormatTable_BytesAndStrings(t *testing.T) {
	r := &chpkg.QueryResult{
		Columns: []string{"col"},
		Rows: [][]any{
			{[]byte("from-bytes")},
			{"from-string"},
			{42},
			{3.14},
		},
	}
	got := FormatTable(r)
	for _, want := range []string{"from-bytes", "from-string", "42", "3.14"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestListDatabases_Contract(t *testing.T) {
	tool := &ListDatabases{DB: &fakeQuerier{}}
	if tool.Name() != "list_databases" {
		t.Errorf("Name = %q", tool.Name())
	}
	if !strings.Contains(tool.Description(), "databases") {
		t.Errorf("Description should mention databases: %q", tool.Description())
	}
	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("InputSchema.type = %v, want object", schema["type"])
	}
}

func TestListDatabases_Call(t *testing.T) {
	fake := &fakeQuerier{
		result: &chpkg.QueryResult{
			Columns: []string{"name", "engine", "comment"},
			Rows: [][]any{
				{"default", "Atomic", ""},
				{"metrics", "MergeTree", "event analytics"},
			},
		},
	}
	tool := &ListDatabases{DB: fake}

	res, err := tool.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.IsError {
		t.Error("expected non-error result")
	}
	if !strings.Contains(fake.gotQuery, "system.databases") {
		t.Errorf("query did not target system.databases: %s", fake.gotQuery)
	}
	if !strings.Contains(fake.gotQuery, "ORDER BY name") {
		t.Errorf("query missing ORDER BY: %s", fake.gotQuery)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "default") || !strings.Contains(text, "metrics") {
		t.Errorf("formatted output missing rows:\n%s", text)
	}
}

func TestListDatabases_PropagatesError(t *testing.T) {
	fake := &fakeQuerier{err: errors.New("connection refused")}
	tool := &ListDatabases{DB: fake}

	_, err := tool.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from failing Querier")
	}
	if !strings.Contains(err.Error(), "list_databases") {
		t.Errorf("error should be wrapped with tool name: %v", err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error should preserve underlying cause: %v", err)
	}
}
