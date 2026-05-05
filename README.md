# ClickHouse MCP Server

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes a ClickHouse instance to MCP-compatible clients (Claude Desktop, Cursor, Continue). Lets an LLM list databases, describe tables, run read-only queries, and surface storage statistics with strict safety limits.

## Features

- Read-only by default; writes require explicit `MCP_MODE=readwrite`.
- SQL safety validator: rejects non-SELECT statements and multi-statement payloads with comment-aware tokenization.
- Per-query timeout and row cap enforced both client-side and via `max_execution_time`.
- Schema introspection: engine, partition/sorting/primary keys, codecs, comments.
- Connection pooling, health checks, graceful shutdown.
- Structured JSON logging via `log/slog`.
- Single static binary, distroless Docker image.

## Tools

| Tool | Required args | Returns |
|---|---|---|
| `list_databases` | none | name, engine, comment per database |
| `list_tables` | `database` | name, engine, rows, size, partition key, sorting key |
| `describe_table` | `database`, `table` | table info plus per-column type, default, codec, key membership |
| `run_query` | `query` | result set as a markdown table; rejects non-SELECT in readonly mode |
| `get_table_stats` | `database`, `table` | row count, disk usage, compression, parts info |

## Quick start

### Build

```bash
git clone https://github.com/TopWent/mcp-clickhouse.git
cd mcp-clickhouse
make build
```

Or with Docker:

```bash
docker build -t mcp-clickhouse:dev .
```

### Configure

```bash
export CLICKHOUSE_URL=localhost:9000
export CLICKHOUSE_USERNAME=default
export CLICKHOUSE_PASSWORD=
export MCP_MODE=readonly
```

### Connect to Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "clickhouse": {
      "command": "/path/to/mcp-clickhouse",
      "env": {
        "CLICKHOUSE_URL": "localhost:9000"
      }
    }
  }
}
```

Restart Claude and ask: "What databases are on this ClickHouse?"

## Configuration

| Env var | Default | Description |
|---|---|---|
| `CLICKHOUSE_URL` | required | host:port for the native protocol (typically 9000) |
| `CLICKHOUSE_USERNAME` | `default` | DB username |
| `CLICKHOUSE_PASSWORD` | empty | DB password |
| `CLICKHOUSE_DATABASE` | empty | optional default database |
| `MCP_QUERY_TIMEOUT` | `30s` | per-query timeout |
| `MCP_MAX_ROWS` | `1000` | client-side row cap |
| `MCP_MODE` | `readonly` | `readonly` blocks non-SELECT; `readwrite` for trusted setups |
| `MCP_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

## Architecture

```
+----------------+   stdio JSON-RPC 2.0   +----------------------+
|   MCP client   | <--------------------> | mcp-clickhouse       |
| (Claude / IDE) |                        |  - protocol (mcp)    |
+----------------+                        |  - registry (tools)  |
                                          |  - validator (safety)|
                                          |  - adapter (chpkg)   |
                                          +----------+-----------+
                                                     | clickhouse-go/v2
                                                     v
                                              +---------------+
                                              |  ClickHouse   |
                                              +---------------+
```

Stateless stdio MCP server. Each LLM tool call is parsed, dispatched to a registered tool, validated by the safety layer when applicable, and translated into a ClickHouse query bounded by timeout and row limits.

## Development

```bash
make test         # unit tests
make test-race    # with race detector
make lint         # golangci-lint
make cover        # generate coverage.html
make docker       # build the Docker image
```

## Roadmap

- [x] Config loading and lifecycle
- [x] MCP JSON-RPC 2.0 protocol with full dispatch logic
- [x] ClickHouse adapter with `Querier` interface for testability
- [x] SQL safety validator with comment-aware tokenization
- [x] Five core tools (list_databases, list_tables, describe_table, run_query, get_table_stats)
- [x] Tools registry, markdown table rendering
- [x] Dockerfile, GitHub Actions CI, Makefile, golangci-lint
- [ ] Integration tests against a live ClickHouse (with build tag)
- [ ] Cluster / replica awareness
- [ ] Materialized view discovery
- [ ] Query plan rendering for EXPLAIN

## License

MIT. See [LICENSE](LICENSE).

## Author

[@TopWent](https://github.com/TopWent). Backend AI Engineer.
