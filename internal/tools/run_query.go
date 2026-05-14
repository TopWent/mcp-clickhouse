package tools

import (
	"context"
	"encoding/json"
	"fmt"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
	"github.com/TopWent/mcp-clickhouse/internal/safety"
)

type RunQuery struct {
	DB       chpkg.Querier
	ReadOnly bool
}

func (q *RunQuery) Name() string { return "run_query" }

func (q *RunQuery) Description() string {
	return "Execute SQL against ClickHouse. In readonly mode only SELECT and EXPLAIN are accepted."
}

func (q *RunQuery) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"SQL to execute"}},"required":["query"],"additionalProperties":false}`)
}

func (q *RunQuery) Call(ctx context.Context, args json.RawMessage) (Result, error) {
	var a struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return Result{}, fmt.Errorf("run_query: parse args: %w", err)
	}
	if a.Query == "" {
		return ErrorResult("missing required argument: query"), nil
	}

	if q.ReadOnly {
		if err := safety.EnforceReadOnly(a.Query); err != nil {
			return ErrorResult(fmt.Sprintf("query rejected: %s", err)), nil
		}
	}

	res, err := q.DB.Query(ctx, a.Query)
	if err != nil {
		return Result{}, fmt.Errorf("run_query: %w", err)
	}
	return TextResult(FormatTable(res)), nil
}
