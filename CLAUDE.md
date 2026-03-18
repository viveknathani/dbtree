# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is dbtree?

A CLI tool written in Go that visualizes database schemas in the terminal. Supports PostgreSQL, MySQL, ClickHouse, and SQLite.

## Build & Test Commands

```bash
make build              # Build binary to ./bin/dbtree
make test               # Start test containers → run tests → stop containers
make test-local         # Start test containers → run tests → keep containers running
make test-ci            # Run tests only (assumes containers already running)
make test-up            # Start test database containers (docker-compose)
make test-down          # Stop test database containers
```

Run a single test:
```bash
go test ./database/ -run TestPostgresql
go test ./graph/ -run TestBuild
go test ./render/ -run TestRenderTree
```

Linter: `staticcheck ./...`

## Architecture

**Processing pipeline:** CLI flags → database introspection → graph building → rendering

### Modules

- **`cmd/dbtree/main.go`** — Entry point. Parses flags (`--conn`, `--format`, `--shape`), detects DB type, orchestrates pipeline. Subcommands: `version`, `update`, `open`.
- **`database/`** — Schema introspection. `InspectSchema()` auto-detects DB type and delegates to the appropriate inspector (`postgresql.go`, `mysql.go`, `clickhouse.go`, `sqlite.go`). Returns a `Database` struct containing tables, columns, and constraints.
- **`graph/`** — Converts `Database` → `SchemaGraph` (nodes + edges based on foreign keys).
- **`render/`** — Outputs graph as text or JSON in tree, flat, or chart shape. Handles circular references via DFS cycle detection. Chart mode uses D2 diagram syntax.
- **`tui/`** — Interactive terminal UI for managing connections and browsing schemas. Built with Bubble Tea.
- **`store/`** — Encrypted persistence for saved database connections. Uses AES-256-GCM with argon2id key derivation.
- **`updater/`** — Self-update mechanism that downloads latest GitHub release.

### Core types (database/database.go)

`Database` → `[]Table` → `[]Column` + `[]Constraint` (PRIMARY_KEY, FOREIGN_KEY, UNIQUE, CHECK)

## Testing

Tests use Docker Compose (`docker-compose.test.yml`) to spin up PostgreSQL 16, MySQL 8.0, and ClickHouse containers. SQLite tests use a temp file. Each database inspector has its own test file.
