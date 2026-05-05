package tools

import (
	"context"
	"encoding/json"
	"fmt"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

const listDatabasesQuery = `
SELECT name, engine, comment
FROM system.databases
WHERE name NOT IN ('INFORMATION_SCHEMA', 'information_schema')
ORDER BY name
`

type ListDatabases struct {
	DB chpkg.Querier
}

func (l *ListDatabases) Name() string { return "list_databases" }

func (l *ListDatabases) Description() string {
	return "List all databases on the ClickHouse server with engine and comment."
}

func (l *ListDatabases) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
}

func (l *ListDatabases) Call(ctx context.Context, _ json.RawMessage) (Result, error) {
	res, err := l.DB.Query(ctx, listDatabasesQuery)
	if err != nil {
		return Result{}, fmt.Errorf("list_databases: %w", err)
	}
	return TextResult(FormatTable(res)), nil
}
