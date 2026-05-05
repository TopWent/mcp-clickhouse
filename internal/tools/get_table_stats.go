package tools

import (
	"context"
	"encoding/json"
	"fmt"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

const tableStatsQuery = `
SELECT
    sum(rows) AS rows,
    formatReadableSize(sum(bytes_on_disk)) AS disk_size,
    formatReadableSize(sum(data_compressed_bytes)) AS compressed_size,
    formatReadableSize(sum(data_uncompressed_bytes)) AS uncompressed_size,
    countIf(active) AS active_parts,
    countIf(NOT active) AS inactive_parts,
    count() AS total_parts
FROM system.parts
WHERE database = ? AND table = ?
`

type GetTableStats struct {
	DB chpkg.Querier
}

func (g *GetTableStats) Name() string { return "get_table_stats" }

func (g *GetTableStats) Description() string {
	return "Return row count, disk usage, compression ratio, and parts info for a MergeTree-family table."
}

func (g *GetTableStats) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"database":{"type":"string","description":"Database name"},"table":{"type":"string","description":"Table name"}},"required":["database","table"],"additionalProperties":false}`)
}

func (g *GetTableStats) Call(ctx context.Context, args json.RawMessage) (Result, error) {
	var a struct {
		Database string `json:"database"`
		Table    string `json:"table"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return Result{}, fmt.Errorf("get_table_stats: parse args: %w", err)
	}
	if a.Database == "" || a.Table == "" {
		return ErrorResult("missing required arguments: database and table"), nil
	}

	res, err := g.DB.Query(ctx, tableStatsQuery, a.Database, a.Table)
	if err != nil {
		return Result{}, fmt.Errorf("get_table_stats: %w", err)
	}
	return TextResult(FormatTable(res)), nil
}
