# Contributing to rewynd

Thanks for your interest. rewynd is a pnpm + Go monorepo.

## Layout

- `core/` — the single Go binary: OTLP receiver (HTTP + gRPC), correlation, SQLite store,
  detections, and the CLI, TUI, and MCP server.
- `packages/shim-node/` — `@rewynd/shim`, the OpenTelemetry capture shim that runs in your app.
- `packages/cli/` — `@rewynd/cli`, the npm wrapper that ships the platform binary.
- `packages/shim-python/` — the Python shim (`rewynd-run`).
- `sdk/go/` — the Go SDK (`rewynd.Start(ctx)`).
- `examples/` — runnable demos: `express-postgres` (N+1 + a 500), `distributed`, `jobs`,
  `go-service`, `ruby-service`.
- `docs/` — `languages.md` (recording any OTel language) and the launch kit.

## Prerequisites

- Go 1.25+ (the toolchain is auto-managed)
- Node 18+ and pnpm 11+
- A local Postgres for the example app:

  ```bash
  docker run -d --name rewynd-pg \
    -e POSTGRES_PASSWORD=rewynd -e POSTGRES_USER=rewynd -e POSTGRES_DB=app \
    -p 5433:5432 postgres:16-alpine
  ```

## Build & run from source

```bash
pnpm install
go -C core build -o "$PWD/core/rewynd" ./cmd/rewynd

# Run the example app with recording on (auto-starts the core):
cd examples/express-postgres
DATABASE_URL=postgresql://rewynd:rewynd@localhost:5433/app \
  ../../core/rewynd run -- node_modules/.bin/tsx src/server.ts

# In another terminal:
../../core/rewynd ls
../../core/rewynd show <id>
```

## Tests

```bash
go -C core test ./...      # the core: store, ingest, detect, mcp, tui, stats, cli, otlp
go -C sdk/go build ./...   # the Go SDK
pnpm install --frozen-lockfile
```

## Conventions

- Go: `gofmt`, `go vet` clean. Comments are lean — only where the intent isn't obvious.
- The `--json` / MCP output is a **versioned contract**: additive changes only; bump
  `model.SchemaVersion` on anything breaking.
- **Correlation is the one invariant we never trade away.** When async context is ambiguous,
  show a log unattributed rather than attach it to the wrong request — show less, not wrong.
