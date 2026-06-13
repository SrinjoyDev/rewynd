# Contributing to rewynd

Thanks for your interest. rewynd is a pnpm + Go monorepo.

## Layout

- `core/` — the single Go binary: OTLP receiver, correlation, SQLite store, detections, CLI
  (and soon the TUI + MCP server).
- `packages/shim-node/` — `@rewynd/shim`, the OpenTelemetry capture shim that runs in your app.
- `examples/express-postgres/` — demo + integration fixture with two planted bugs (an N+1 and a
  contextual 500).

## Prerequisites

- Go 1.22+ (the toolchain is auto-managed)
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
go -C core test ./...
```

## Conventions

- Go: `gofmt`, `go vet` clean. Comments are lean — only where the intent isn't obvious.
- The `--json` / MCP output is a **versioned contract**: additive changes only; bump
  `model.SchemaVersion` on anything breaking.
- **Correlation is the one invariant we never trade away.** When async context is ambiguous,
  show a log unattributed rather than attach it to the wrong request — show less, not wrong.
