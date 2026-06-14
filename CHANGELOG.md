# Changelog

All notable changes to rewynd are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org).

## [Unreleased]

### Added

- **`rewynd export <id>`** — render a request's full correlated trace as a single
  self-contained HTML file (`-o trace.html`, or stdout). Attach it to a PR, drop it in CI, or
  send it to a teammate; no rewynd needed to open it.
- **Agent-value benchmark** (`bench/agent-eval/`) — a reproducible eval showing a coding agent
  finds the root cause more reliably with the recorded trace than with only the source and
  logs (4/4 vs 3/4; correct *and confident* 4/4 vs 2/4). Includes the tasks, an OTLP seeder
  (`core/cmd/eval-seed`), the grader, and results.

### Improved

- `rewynd diagnose` now also reports **failed outbound calls** (any that errored or returned
  ≥400) and **DB queries that errored**, not just detections and exceptions. A 5xx caused by an
  upstream failure (e.g. a 502 from an outbound 500) no longer comes back as "no problems
  detected." It also reads cleaner on real telemetry (no leading colon for a typeless
  exception; no redundant hint when the stack is just the message).
- `rewynd show` **folds repeated queries and outbound calls** into one line — an N+1's 50
  identical statements become `800ms  SELECT … = ?  ×50 (avg 16ms)` instead of 50 raw rows.

## [0.2.1] - 2026-06-14

### Fixed

- npm packages now install cleanly for everyone. The first 0.2.0 publish shipped broken
  metadata; this release corrects it:
  - Pinned the OpenTelemetry experimental dependencies to the coherent **0.218.0** set
    (`api-logs`/`sdk-logs` had been referenced at a `0.219.0` version that was unpublished from
    npm, so a fresh `npm install` couldn't resolve the tree).
  - The CLI is published as **`@rewynd/cli`** — npm blocks the unscoped `rewynd` name as too
    similar to an existing package. The installed binary is still `rewynd`.
  - `@rewynd/shim` is no longer marked private, and the release's npm publish step is idempotent.

## [0.2.0] - 2026-06-14

### Added

- Zero-config, OTLP-native request capture with per-request correlation for **Node**
  (Express, Fastify; ESM/`tsx`) and **Python** (FastAPI / Flask / Django).
- **Go SDK** (`github.com/SrinjoyDev/rewynd/sdk/go`): `rewynd.Start(ctx)` wires a Go service's
  OpenTelemetry traces to the local core in one call (minimal setup, since Go has no runtime
  auto-instrumentation), with flush-on-exit and an off switch. See `examples/go-service/`.
- **Any OpenTelemetry language**: `rewynd run` now sets the standard OTLP env vars for the
  process it launches, so a Java agent, .NET / Ruby / PHP auto-instrumentation, or any
  OTel-emitting service records to rewynd with no rewynd-specific code (e.g.
  `rewynd run -- java -javaagent:opentelemetry-javaagent.jar -jar app.jar`). The Node shim is
  now optional rather than required. See `docs/languages.md`.
- Go core: OTLP/HTTP receiver, embedded SQLite ring-buffer store, and detections
  (N+1, slow query, slow request).
- Three frontends over one core: a Bubble Tea **TUI**, a JSON **CLI**
  (`run/tui/serve/ls/show/watch/tail/diagnose/last-error/clear/status`), and an **MCP
  server** (`rewynd mcp`) for coding agents.
- **OTLP/gRPC** intake on `:4317` alongside HTTP on `:4318` — most OpenTelemetry SDKs
  default to gRPC; the gRPC listener is best-effort so a busy port never blocks the core.
- **Distributed trace stitching**: a request that fans out across services is recorded as
  one trace, the entry service is the canonical root (earliest server span, regardless of
  export order), and every query / outbound / log is attributed to the service it ran in.
  See `examples/distributed/`.
- **Background jobs / queue consumers** as first-class flows: work with no HTTP root (a
  queue consumer, worker, or cron — any OTel consumer/RPC span) is recorded with its
  correlated queries, outbound calls, logs, and exceptions, and an ok/fail outcome. Shown
  across the TUI, CLI, and MCP. See `examples/jobs/`.
- **Load / performance view**: `rewynd stats` (and the MCP `get_load_stats` tool, and the
  TUI `S` panel) report throughput, latency percentiles (p50/p95/p99/max), error rate, and a
  per-endpoint breakdown ranked worst-first — for humans and agents to spot the slow or
  erroring endpoint and compare before/after a change.
- **Regression diff**: `rewynd stats --save <name>` snapshots a run; `rewynd stats --baseline
  <name>` shows the delta (latency, error rate, throughput, per-endpoint, new/gone) — the
  "did my fix help" answer.
- **Agent-native** integrations: connect-time MCP instructions, a `get_stats` orientation
  tool, richer tool descriptions, and drop-in setups under `integrations/` for Claude Code,
  Cursor, Windsurf, OpenCode, Codex, Cline, and Devin.
- **TUI control panel**: live path search (`/`), slow-only filter (`s`), a scrollable
  detail pane (`ctrl+d`/`ctrl+u`), and richer detail (detection suggestions, outbound,
  response body), plus the `?` keybinding overlay.
- One-command launchers: `rewynd run <cmd>` (Node) and `rewynd-run <cmd>` (Python); the
  core auto-starts on first use.
- Request header + body capture with **in-app redaction**; outbound HTTP capture.
- Configurable retention via `REWYND_MAX_REQUESTS` (default 1000).
- Privacy: binds `127.0.0.1`, no telemetry, a hard production guard.

### Fixed

- The Node shim now flushes recorded spans on normal process exit (`beforeExit`), not only on
  SIGTERM/SIGINT — so short-lived workers, scripts, and one-off jobs no longer lose their data.
- The Go module path now matches its directory (`github.com/SrinjoyDev/rewynd/core`), so
  `go install github.com/SrinjoyDev/rewynd/core/cmd/rewynd@latest` resolves (it previously
  could not — there was no go.mod at the path's root).
- Distribution: cross-platform binaries (goreleaser), `rewynd` on **npm** (per-platform
  binary via `optionalDependencies`) and **PyPI**, plus a `curl | sh` installer and a
  tag-driven release workflow.

[Unreleased]: https://github.com/SrinjoyDev/rewynd/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/SrinjoyDev/rewynd/releases/tag/v0.2.1
[0.2.0]: https://github.com/SrinjoyDev/rewynd/releases/tag/v0.2.0
