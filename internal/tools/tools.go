// Package tools defines the MCP Tool contract and a Registry.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Call(ctx context.Context, args json.RawMessage) (Result, error)
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Result struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

func TextResult(text string) Result {
	return Result{Content: []Content{{Type: "text", Text: text}}}
}

func ErrorResult(text string) Result {
	return Result{Content: []Content{{Type: "text", Text: text}}, IsError: true}
}

type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns registered tools sorted by name.
func (r *Registry) Definitions() []Definition {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Definition, 0, len(names))
	for _, n := range names {
		t := r.tools[n]
		out = append(out, Definition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return out
}

// FormatTable renders a QueryResult as a markdown table.
func FormatTable(r *chpkg.QueryResult) string {
	if r == nil || len(r.Rows) == 0 {
		return "(no results)"
	}

	var b strings.Builder

	b.WriteString("|")
	for _, c := range r.Columns {
		b.WriteString(" ")
		b.WriteString(escapeCell(c))
		b.WriteString(" |")
	}
	b.WriteString("\n|")
	for range r.Columns {
		b.WriteString("---|")
	}
	b.WriteString("\n")

	for _, row := range r.Rows {
		b.WriteString("|")
		for _, v := range row {
			b.WriteString(" ")
			b.WriteString(escapeCell(formatValue(v)))
			b.WriteString(" |")
		}
		b.WriteString("\n")
	}

	if r.Truncated {
		fmt.Fprintf(&b, "\n_(truncated to %d rows)_", len(r.Rows))
	}

	return b.String()
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
