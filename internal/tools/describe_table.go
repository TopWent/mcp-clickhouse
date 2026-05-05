package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chpkg "github.com/TopWent/mcp-clickhouse/internal/clickhouse"
)

const describeTableInfoQuery = `
SELECT
    engine,
    total_rows,
    formatReadableSize(total_bytes) AS size,
    partition_key,
    sorting_key,
    primary_key,
    comment
FROM system.tables
WHERE database = ? AND name = ?
`

const describeTableColumnsQuery = `
SELECT
    name,
    type,
    default_kind,
    default_expression,
    codec_expression,
    is_in_partition_key,
    is_in_sorting_key,
    is_in_primary_key,
    comment
FROM system.columns
WHERE database = ? AND table = ?
ORDER BY position
`

type DescribeTable struct {
	DB chpkg.Querier
}

func (d *DescribeTable) Name() string { return "describe_table" }

func (d *DescribeTable) Description() string {
	return "Describe a table: engine, row count, size, partition/sorting/primary keys, and per-column type, default, codec, and key membership."
}

func (d *DescribeTable) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"database":{"type":"string","description":"Database name"},"table":{"type":"string","description":"Table name"}},"required":["database","table"],"additionalProperties":false}`)
}

func (d *DescribeTable) Call(ctx context.Context, args json.RawMessage) (Result, error) {
	var a struct {
		Database string `json:"database"`
		Table    string `json:"table"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return Result{}, fmt.Errorf("describe_table: parse args: %w", err)
	}
	if a.Database == "" || a.Table == "" {
		return ErrorResult("missing required arguments: database and table"), nil
	}

	info, err := d.DB.Query(ctx, describeTableInfoQuery, a.Database, a.Table)
	if err != nil {
		return Result{}, fmt.Errorf("describe_table info: %w", err)
	}
	if len(info.Rows) == 0 {
		return ErrorResult(fmt.Sprintf("table %s.%s not found", a.Database, a.Table)), nil
	}

	cols, err := d.DB.Query(ctx, describeTableColumnsQuery, a.Database, a.Table)
	if err != nil {
		return Result{}, fmt.Errorf("describe_table columns: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## %s.%s\n\n", a.Database, a.Table)
	for i, col := range info.Columns {
		fmt.Fprintf(&b, "- **%s**: %s\n", col, formatValue(info.Rows[0][i]))
	}

	b.WriteString("\n### Columns\n\n")
	b.WriteString(FormatTable(cols))

	return TextResult(b.String()), nil
}
