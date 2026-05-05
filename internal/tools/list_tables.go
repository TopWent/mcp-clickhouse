package tools

import (
	"context"
	"encoding/json"
	"fmt"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

const listTablesQuery = `
SELECT
    name,
    engine,
    total_rows,
    formatReadableSize(total_bytes) AS size,
    partition_key,
    sorting_key,
    comment
FROM system.tables
WHERE database = ?
ORDER BY name
`

type ListTables struct {
	DB chpkg.Querier
}

func (l *ListTables) Name() string { return "list_tables" }

func (l *ListTables) Description() string {
	return "List tables in a database with engine, rows, size, partition key, and sorting key."
}

func (l *ListTables) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"database":{"type":"string","description":"Database name"}},"required":["database"],"additionalProperties":false}`)
}

func (l *ListTables) Call(ctx context.Context, args json.RawMessage) (Result, error) {
	var a struct {
		Database string `json:"database"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return Result{}, fmt.Errorf("list_tables: parse args: %w", err)
	}
	if a.Database == "" {
		return ErrorResult("missing required argument: database"), nil
	}

	res, err := l.DB.Query(ctx, listTablesQuery, a.Database)
	if err != nil {
		return Result{}, fmt.Errorf("list_tables: %w", err)
	}
	return TextResult(FormatTable(res)), nil
}
