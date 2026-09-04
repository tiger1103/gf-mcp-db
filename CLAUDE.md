# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**gf-mcp-db** is a Universal Database MCP (Model Context Protocol) Server built with GoFrame and mcp-go. It provides database introspection and query execution tools for MySQL, PostgreSQL, SQLite, SQL Server, Oracle, and DM (DaMeng) via MCP protocol, with unified cross-database tool output.

## Quick Commands

```bash
# Install dependencies
go mod tidy

# Build for all platforms (configured in hack/config.yaml)
gf build

# Build for specific platform
gf build -a amd64 -s linux

# Run in stdio mode (for MCP client integration)
go run main.go --type mysql --host localhost --port 3306 --user root --password 123456 --database test

# Run in HTTP SSE mode (default, listens on :8000)
go run main.go
```

Binary output goes to `./bin/gf-mcp-db` (or `gf-mcp-db.exe` on Windows).

## Architecture

### Entry Point

`main.go` imports all six GoFrame database drivers (mysql, pgsql, sqlite, mssql, oracle, dm — all pure Go, no CGO), the boot package, and calls `cmd.Main.Run()`.

### Connection Modes

The server auto-detects the mode at startup (`internal/cmd/cmd.go`):

- **Stdio mode**: Triggered by `--stdio` flag or database connection args (`--type`, `--host`, `--database`). Uses `server.NewStdioServer` from mcp-go. Database config is parsed from CLI args.
- **HTTP SSE mode**: Default. Runs a GoFrame HTTP server on `:8000` with MCP SSE endpoint at `/mcp`. Database config comes from URL query params or `X-DB-*` headers.

### Tool Registration Pattern

Tools follow a registration pattern using reflection (`internal/mcp/register/register.go`):

1. Each tool is a struct with `ReturnTool() mcp.Tool` and `Handler() func(...)` methods
2. Tools register themselves via `Register*` methods on the `tools.Reg` struct (e.g., `RegisterExecuteQuery`, `RegisterGetSchema`)
3. `register.DoRegister(&tools.Reg{})` uses reflection to call all `Register*` methods
4. Each `Register*` method calls `register.AddHandler()` which appends to a handler list
5. `register.DoHandler(mcpServer)` iterates handlers and adds tools to the MCPServer

### Available Tools (in `internal/mcp/tools/`)

| File | Tool Name | Purpose |
|------|-----------|---------|
| `execute_query.go` | `execute_query` | Run SQL queries/commands |
| `get_table_list.go` | `get_table_list` | List database tables |
| `get_schema.go` | `get_schema` | Get full database schema |
| `get_table_info.go` | `get_table_info` | Get detailed table info |
| `get_enum_values.go` | `get_enum_values` | Get unique column values |
| `get_sample_data.go` | `get_sample_data` | Get sample data (sanitized) |
| `clear_cache.go` | `clear_cache` | Clear schema cache |

### Key Packages

- `internal/cmd/` - CLI handling, mode detection, stdio entry
- `internal/dbconn/` - Unified connection config normalization/validation and `Init` (used by both stdio and HTTP paths; fills `gdb.ConfigNode` fields directly, short-circuits identical consecutive configs, restores last-good config on failed init)
- `internal/mcp/dialect/` - Per-database dialect implementations (identifier quoting, LIMIT/TOP/FETCH FIRST pagination, indexes/foreign keys/table stats via catalog views, primary keys) behind a registry
- `internal/mcp/register/` - MCP handler registration via reflection
- `internal/mcp/tools/` - All MCP tool implementations (dialect-driven; `getDB` reads `gdb.Instance`, NOT `g.DB`)
- `internal/mcp/router/` - HTTP route registration for SSE mode
- `internal/mcp/model/` - Config struct definitions
- `internal/consts/` - Constants and error codes
- `library/liberr/` - Error handling utilities (panic-based error flow)
- `library/libUtils/` - Shared utilities (cache, slice tree, general utils)

### Database Connection

All tools obtain the connection via `gdb.Instance("default")` (see `internal/mcp/tools/helpers.go`). The "default" group is configured at startup (stdio) or per-request (HTTP mode) through `internal/dbconn.Init`, which fills `gdb.ConfigNode` fields directly (never Link strings). Note: `g.DB("default")` must NOT be used — gins caches instances permanently and ignores `gdb.SetConfig`, which causes stale connections after config changes.

### Error Handling

Uses panic-based error flow through `library/liberr/`. Tools wrap logic in `g.Try()` and recover via the `returnRes()` method in `tools.go`. Error codes are defined in `internal/consts/error_code.go`.

### Important Notes

- All MySQL-specific SQL has been removed; table/column metadata comes from `gdb.Tables()`/`TableFields()` and per-dialect catalog queries in `internal/mcp/dialect/`.
- Tool output uses a unified cross-database shape (columns: column_name/data_type/nullable/primary_key/default/extra/comment; unified keys for indexes/foreign keys/table stats). Error keys (`columns_error`, `primary_keys_error`, `indexes_error`, `foreign_keys_error`, `table_stat_error`) expose per-entry query failures.
- There is duplication no longer: `internal/cmd/cmd.go` and `internal/mcp/router/router.go` both delegate connection setup to `internal/dbconn`.
- Adding a new database: implement an `IDialect` in `internal/mcp/dialect/` with `init(){Register(...)}`, add the contrib driver import in `main.go`, and extend `internal/dbconn` type aliases/defaults.
- Integration tests: `internal/mcp/tools/integration_test.go` (SQLite always runs; MySQL/PG gated by `GF_MCP_TEST_MYSQL`/`GF_MCP_TEST_PG` env vars, format `host|port|user|password|database`).
