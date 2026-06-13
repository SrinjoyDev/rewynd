# rewynd — End-to-End Build Plan

> **Product:** `rewynd` — a zero-config, OTLP-native **flight recorder for your backend**.
> Records every request, query, outbound call, log, and exception during local dev,
> **correlates them per request**, and surfaces the whole story in a beautiful TUI, a
> structured CLI, and an MCP server your coding agent can drive.
>
> **Name status:** `rewynd` is free on npm + PyPI; GitHub org `rewyndhq`. The name is a
> single find/replace away from changeable — do not let it block building. (Backups:
> `reqcap`, `flightrec`.)
>
> **Companion docs:** `STRATEGY.md` (positioning + competitive reality), the Master Build
> Brief (original hypothesis). This file is the living plan — update it at every phase gate.

---

## 0. The thesis we are building (corrected, post-research)

PHP devs have Laravel Telescope / Symfony Web Profiler. Node/Python never got a great
local request profiler. OpenTelemetry now does the hard instrumentation but is built for
prod and miserable for local dev. **`rewynd` is the local-first DX layer on top of OTel —
for humans (TUI) and for agents (CLI/MCP).**

**The honest competitive picture (see `STRATEGY.md`):** the individual pillars are taken
— Sentry **Spotlight** ships a local `tail` + an `mcp` agent loop; **otel-tui** ships a
terminal OTLP viewer. **Our moat is the combination none of them have:**

> **OTLP-native (no SDK) + zero-config Node + _per-request_ correlation with N+1/exception
> detection + the agent loop + a real TUI — in one Go binary.**

Two-sentence wedge we must be able to defend on Hacker News:
- **vs Spotlight:** no Sentry SDK — stock OpenTelemetry, works on apps that never touched
  Sentry; and a per-request *flight-recorder* UX, not an error list.
- **vs otel-tui:** correlation + detections + the agent/MCP loop — the *story of one
  request*, not a raw signal browser.

---

## 1. Invariants (never trade these away)

1. **Correlation is never wrong.** A query/log/exception shown under a request truly
   belongs to it. When async context is ambiguous (worker threads, detached promises,
   some pool callbacks), show the log **unattributed** — never guess. *Show less, not wrong.*
2. **Zero-config is an honest promise.** Primary entry is a **launcher** (`rewynd run …`),
   not a fragile top-of-file import — this sidesteps the ESM ordering problem (see §3.3).
3. **Cannot leak prod data.** Hard prod guard + local-only bind + auto-redaction (§7).
4. **The `--json` / MCP schema is a versioned public contract** the moment an agent depends
   on it (§4). Never break it silently.
5. **Performance budgets hold** (§8): shim < ~1 ms p50 added latency, bounded memory;
   core startup < 100 ms, idle RAM single-digit MB, memory bounded forever; TUI smooth at
   thousands of requests. A CI benchmark enforces the shim budget.
6. **Anti-scope:** not an execution-replay debugger, not a prod APM, not a profiler, not a
   framework, not a web app (v0), not "all languages day one." Every feature must make the
   **human aha** or the **agent aha** land harder, or it's cut.

---

## 2. Architecture (committed)

```
  Your Node app  ──(stock OpenTelemetry, configured by the shim)──┐
   rewynd shim:  OTel auto-instrument (HTTP/pg/http-client)        │ OTLP/HTTP-protobuf
   + AsyncLocalStorage context + console/pino/winston correlation  │ over UDS (TCP fallback)
                                                                    ▼
  ┌───────────────────────── rewynd core (one Go binary) ─────────────────────────┐
  │  OTLP receiver ──▶ correlation engine ──▶ detections ──▶ SQLite (WAL) store    │
  │                                   │                                            │
  │                          live notify hub (pub/sub for new requests)           │
  └───────────────────────────────────────────────────────────────────────────────┘
        ▲ reads (SQLite WAL, multi-reader)        ▲ subscribes (notify socket)
   ┌────┴─────┐   ┌──────────┐   ┌──────────┐
   │   TUI    │   │   CLI    │   │   MCP    │     ← thin clients, NO business logic
   └──────────┘   └──────────┘   └──────────┘
```

**Topology decision (simplification over the brief's RPC daemon):**
- The **OTLP receiver is the only long-lived process** (`rewynd` daemon, started lazily).
  It is the **sole writer** to SQLite.
- TUI / CLI / MCP are **readers of the same SQLite file** (WAL = many readers + one writer,
  for free). This deletes a whole RPC layer and a class of stale-daemon / version-skew bugs.
- The only thing needing *push* is live streaming (`tui`, `tail`, `watch`). That's a **tiny
  notify channel** over the local socket: receiver publishes `{request_id}` on commit;
  clients get the nudge, then read the row from SQLite. No query results travel over RPC.
- Add a full RPC surface **only if** shared-reads provably can't deliver. (It won't be needed.)

**Stack (committed — not re-litigated):**
- Core: **Go**. TUI: **Charm** (Bubble Tea + Lip Gloss + Bubbles). Store: **`modernc.org/sqlite`**
  (pure-Go, no CGo). OTLP ingest: OpenTelemetry-Go collector/receiver libs. MCP: Go MCP SDK.
  Release: **goreleaser**.
- Shim: **TypeScript** on OpenTelemetry-JS (`@opentelemetry/sdk-node`,
  `auto-instrumentations-node`, OTLP-proto exporters). Distributed via npm; Go binary shipped
  through per-platform `optionalDependencies` (the esbuild model).
- Transport: OTLP/HTTP-protobuf over **Unix domain socket** (TCP loopback fallback / Windows).

---

## 3. The load-bearing technical decisions

### 3.1 Capture
Stand entirely on OTel auto-instrumentation for HTTP server, `pg`, and outbound HTTP. The
user/agent **never sees OTel config** — the shim/launcher sets endpoint, exporters, resource
attrs, sane batching. OTLP is the ingest seam → any OTLP language feeds the same core later.

### 3.2 Correlation (the magic)
- Join key = OTel **trace context** (`trace_id` / `span_id`). Don't invent a parallel system.
- The HTTP server span is the **request root**; all child spans (queries, outbound) join by
  `trace_id`. Logs are stamped with active `trace_id`/`span_id` via AsyncLocalStorage and
  OTel log instrumentation → joined to the request.
- **Conservative fallback:** a log with no active trace context is stored with
  `request_id = null` (unattributed) and shown in a separate "unattributed" lane — never
  attached to the wrong request.

### 3.3 The zero-config promise — resolved honestly (addresses the #1 risk)
OTel auto-instrumentation is reliable on **CommonJS** (`--require`) and **fragile on ESM**
(`import 'x/auto'` at top of file cannot patch modules imported afterward; needs
`--import`/`register()`). Bundlers (tsx, esbuild, Next standalone) break monkey-patching.

**Our DX, in priority order:**
1. **`rewynd run <your dev command>`** (PRIMARY, most robust). A launcher that injects the
   right flags (`--import`/`-r`) and OTLP endpoint, then spawns the app. Sidesteps ESM
   ordering entirely. The hero-GIF command: `rewynd run npm run dev`.
2. **`node --import @rewynd/register app.js`** (one flag) for ESM users who prefer it.
3. **`node -r @rewynd/register app.js`** / `import '@rewynd/register'` (top of entry) for CJS.

**Phase 0 GATE proves all three on a real ESM + `tsx` app**, not just CJS. If a path can't be
made reliable, we drop it from the docs rather than ship a broken promise.

---

## 4. The data model = the public contract (`schema_version: 1`)

Everything (`--json`, MCP, SQLite) serializes these. Versioned; additive changes only.

- **Request** (correlation root): `id, schema_version, trace_id, service, method, path,
  route, url, status_code, started_at, ended_at, duration_ms, request{headers,query,body,
  content_type,bytes}, response{headers,body,content_type,bytes}, error:bool,
  counts{queries,outbound,logs}, detections[]`
- **Span**: `span_id, parent_span_id, trace_id, request_id, name, kind, type(http_server|
  db_query|http_client|internal), started_at, ended_at, duration_ms, status, attributes{}`
- **Query** (enriched db span): `span_id, request_id, db_system, statement(redacted),
  statement_normalized(params stripped — N+1 group key), params(redacted), duration_ms,
  rows, error`
- **Outbound**: `span_id, request_id, method, url(redacted), status_code, duration_ms, error`
- **Log**: `id, request_id(nullable), trace_id, span_id(nullable), at, level, message,
  source(console|pino|winston), attributes{}`
- **Exception**: `id, request_id, span_id, type, message, stack[frames], at`
- **Detection**: `id, request_id, type(n_plus_one|slow_query|slow_request|duplicate_outbound),
  severity, title, summary, evidence{}, suggestion`

**N+1 definition (precise, to avoid false positives):** ≥ N (default 5) spans in one request
whose `statement_normalized` is identical. `evidence` = {normalized_sql, count, total_ms,
span_ids}. Tunable threshold; never fire on a single query.

**Contract tests** (Phase 1+): golden-JSON fixtures asserted by the integration test; any
schema change must bump `schema_version` or be additive.

---

## 5. Repository layout (monorepo: pnpm + Go module)

```
rewynd/
├─ PLAN.md  STRATEGY.md  README.md  LICENSE(Apache-2.0)  CONTRIBUTING.md
├─ go.work                              # Go workspace
├─ pnpm-workspace.yaml  package.json
├─ core/                                # the Go binary (brain + TUI + CLI + MCP)
│  ├─ cmd/rewynd/main.go                # cobra root; subcommands
│  ├─ internal/otlp/                    # OTLP receiver (traces + logs)
│  ├─ internal/correlate/              # spans+logs → requests; conservative fallback
│  ├─ internal/detect/                  # n+1, slow, dup-outbound
│  ├─ internal/store/                   # modernc sqlite, WAL, migrations, prune (ring buffer)
│  ├─ internal/notify/                  # live pub/sub hub + UDS socket
│  ├─ internal/api/                     # query layer the 3 frontends call (in-proc + read)
│  ├─ internal/tui/                     # Bubble Tea app (list, waterfall, tabs, theming)
│  ├─ internal/cli/                     # ls/show/watch/tail/diagnose/clear/last-error/status
│  ├─ internal/mcp/                     # MCP server (tools mirror the CLI)
│  ├─ internal/redact/                  # secret redaction, body truncation
│  ├─ internal/model/                   # the §4 contract structs + JSON tags
│  └─ internal/daemon/                  # lazy-start, pidfile, prod guard, lifecycle
├─ packages/
│  ├─ shim-node/                        # @rewynd/shim  (+ @rewynd/register, launcher)
│  │  ├─ src/register.ts  src/sdk.ts  src/log-correlation.ts  src/launcher.ts
│  └─ shim-python/   (Phase 4)
├─ examples/
│  └─ express-postgres/                 # the demo + integration fixture (2 planted bugs)
├─ docs/                                # docs site (Phase 3); mcp.json quickstart
└─ .github/workflows/                   # CI: lint, test, build (all OS), shim bench, release
```

---

## 6. Phase-by-phase plan (each phase ends runnable + demoable)

> **Iteration protocol per phase:** build → run the example app → meet the acceptance
> criteria (testable) → record the cast/GIF if applicable → update this PLAN → only then
> advance. Don't gold-plate; ship the phase, learn, iterate.

### Phase 0 — Research & de-risk  →  `PHASE_0_FINDINGS.md`
**Status:** competitive + naming research DONE (`STRATEGY.md`). Remaining = the spikes.
- [x] Landscape re-verified; positioning corrected; name chosen.
- [x] **GATE 1 (can force a pivot): ESM/`tsx` capture + correlation spike — PASSED 2026-06-13.**
      Real `tsx`-run ESM Express + `pg` app: HTTP+pg spans correlate by trace_id, N+1 (10
      identical SELECTs) captured under one request, pino logs carry trace_id — zero app code
      changes, injected via `--import`. Details in `PHASE_0_FINDINGS.md`. (`-r`/CJS path TODO.)
- [ ] **GATE 2:** Bubble Tea spike — live list + one horizontal waterfall bar row.
- [ ] **GATE 3:** `watch` spike — block until a matching request lands, emit stable JSON.
- [ ] Confirm §2 stack assumptions (Go OTLP receiver ingests the shim's OTLP; modernc
      SQLite WAL multi-reader; notify socket nudges a reader).
**Acceptance:** all three gates pass on macOS + Linux; `PHASE_0_FINDINGS.md` records the
real zero-config promise per module system. **If GATE 1 only works on CJS → rewrite the
README's install story before Phase 1.**

### Phase 1 — Walking skeleton — ✅ DONE 2026-06-13 (clean, not ugly)
Full pipeline verified: example app → shim (OTLP) → core daemon (:4318) → SQLite → CLI.
`ls/show/watch/clear/status` work; N+1 detected (10 identical SELECTs), logs correlated, 500 captured.
- [ ] `examples/express-postgres` with the **two planted bugs**: an **N+1** endpoint and a
      **contextual 500** (request → failing SQL+params → exception+stack → surrounding logs).
- [ ] `shim-node`: launcher + register; auto-capture requests, `pg` queries, console/pino
      logs; OTLP export over UDS; bounded drop-on-backpressure queue.
- [ ] `core`: OTLP receiver → correlation → SQLite (schema + migrations + ring-buffer prune);
      the `internal/api` query layer; notify hub.
- [ ] **CLI first** (simplest frontend, unblocks the agent loop): `ls`, `show <id> --json`,
      `clear`, `watch` (basic), `status`.
- [ ] Bare-bones TUI: live list + minimal detail pane.
**Acceptance:** `rewynd run` the example app, hit both endpoints; `rewynd ls --json` shows
both correlated requests; `rewynd show <id> --json` returns the full §4 shape (queries+logs
+exception under the right request); a basic `rewynd tui` lists them live. Integration test
boots the app, fires requests, asserts the recorded trace shape **and the JSON schema**.

### Phase 2 — Both ahas real (the two GIF gates — non-negotiable)
**Human aha (§5):**
- [ ] TUI: live streaming with sticky selection; color-coding (green 2xx / yellow 4xx /
      red 5xx); slow requests visibly loud (duration bar).
- [ ] **Waterfall** with proper span nesting + timing bars (btop aesthetic); **repeated
      identical queries grouped/stacked so N+1 is obvious at a glance.**
- [ ] **N+1 detection** + stacked-query view; **exception view** (stack + interleaved logs).
- [ ] Detail tabs: Request / Response / Queries / Outbound / Logs / Exception.
- [ ] Keybindings: `j/k Enter / g/G f e c q ? y Tab`. Polish until asciinema-worthy.
**Agent aha (§6):**
- [ ] Finalize `watch` / `tail` / `diagnose` / `last-error` + the **stable `--json` schema**
      (contract tests green).
- [ ] **MCP server** (`rewynd mcp`): tools `list_requests, get_request, wait_for_request,
      tail_requests, diagnose, get_last_error, clear`. Ship a copy-paste **`mcp.json`**.
- [ ] The loop works end-to-end: `clear → trigger → watch --status 5xx --json → read → fix
      → watch --status 2xx`.
**Non-negotiables (build now, not later):**
- [ ] Redaction (authorization/cookie/password/token/api_key/secret + patterns), body
      truncation, **hard prod guard** (`NODE_ENV==='production'` refuses unless overridden),
      `127.0.0.1`-only bind, retention ring buffer, one-key `clear`.
- [ ] **CI shim-overhead benchmark** that fails the build on regression past budget.
**Acceptance (THE GATES):** record the **§5 TUI GIF** and the **§6 agent-loop split-screen
GIF**, both of which make a stranger say "I need this." If you can't, the core isn't done.

### Phase 3 — DX & polish ("neat" → "loved")
- [ ] Frictionless install: `npx rewynd` / `rewynd run` / single-binary download; auto-detect
      + clear first-run message; great `--help`; shell completions; stable exit codes.
- [ ] TUI: search/filter, keybinding help overlay, **copy-as-cURL / SQL / stack**, themes
      (gorgeous default + presets; honor `NO_COLOR`), small-terminal/SSH graceful degrade.
- [ ] Broaden Node: **Fastify, Next.js API routes, NestJS**; **Prisma/Drizzle**;
      **`fetch`/axios**; **pino/winston**. (Each = a thin instrumentation + a fixture test.)
- [ ] Docs site + README (the two casts above the fold) + asciinema casts + `mcp.json` quickstart.
**Acceptance:** a stranger goes from install → "found a bug" in **< 2 min** unaided; an agent
runs the loop from a copy-pasted `mcp.json`.

### Phase 4 — Breadth & the multi-language unlock
- [ ] **Python shim** (FastAPI/Flask/Django + SQLAlchemy + requests/httpx) over the **same Go
      core** via OTLP + `contextvars` — second viral moment, doubles the audience.
- [ ] More detections (duplicate outbound, slow query); more drivers (Redis, BullMQ); GraphQL.
- [ ] (Optional) secondary **web UI** (React+Vite over the same daemon API) for richer
      visual waterfalls + marketing screenshots.
**Acceptance:** Python users get the same zero-config human + agent ahas from the same core,
with the same `--json`/MCP contract.

### Phase 5 — Launch
- [ ] Two headline assets (TUI GIF, agent-loop GIF) + `try-rewynd` 30-second demo repo.
- [ ] README per §14 of the brief; positioning line; privacy/local-only promise; supported
      stacks; `mcp.json` quickstart.
- [ ] **Show HN** ("Show HN: rewynd — a beautiful TUI backend recorder your coding agent can
      use"), r/node, r/Python, Lobsters, Charm/TUI community, AI-tooling Discords, X. Tue–Thu AM US.
- [ ] Two crowds, two angles: TUI crowd → beauty; AI crowd → "agents can finally debug
      backends." **Don't launch ugly** (the two GIF gates are the bar).

---

## 7. Privacy / redaction / prod guard (Phase 2, non-negotiable)
Auto-redact secrets (headers/body/params); truncate large bodies (configurable, per-route
opt-out); **hard prod guard**; bind `127.0.0.1`; zero outbound network; **no telemetry on the
user** (say it proudly in the README); ring-buffer retention (~1,000 reqs / 200 MB defaults,
survives restarts so you can inspect "right before the crash"); one-key clear.

## 8. Performance budgets + the CI benchmark
**Shim (sacred hot path):** async, batched, non-blocking export; bounded queue with
drop-oldest on backpressure; lazy, size-capped, truncated body capture; redaction off the hot
path. Budget: < ~1 ms p50 / low-single-digit ms p99 added latency; a few-MB queue cap.
**CI benchmark fails the build on regression.** **TUI render (sacred):** virtualize the list
(render only visible rows); rely on Bubble Tea diffing; stream deltas via the notify channel;
never re-query the whole DB per frame. **Core:** single static binary; startup < 100 ms; idle
RAM single-digit MB; memory bounded forever by the ring buffer; SQLite WAL + prepared stmts +
batched inserts + rowid-range prune. **Do NOT** architect for prod scale, HA, sampling, or
plugin frameworks.

## 9. Testing & CI/release
- Unit tests for the brittle bits: **correlation** (incl. the unattributed-fallback cases) and
  **detections** (N+1 thresholds, normalization).
- **Integration test**: boot the example app → fire requests → assert recorded trace shape +
  **golden `--json` schema** (agents depend on it).
- Cross-platform: macOS + Linux first, Windows soon; verify TUI over SSH + small terminals.
- CI on every PR: lint, typecheck, test, **build the binary (all OSes)**, build shims, **run
  the shim-overhead benchmark**. Release automation: **goreleaser + changesets**.
- **Contract versioning:** `schema_version` bumped only on breaking change; additive by default.

## 10. Definitions of done (the two gates that decide everything)
1. The **§5 TUI GIF** — live list, click the slow one, the N+1 stacked waterfall, the 500's
   request+SQL+stack+logs — that makes a stranger think *"I need this."*
2. The **§6 agent-loop GIF** — Claude Code edits code, runs `clear/trigger/watch`, reads the
   structured trace, fixes the N+1 / 500 by itself. *"Your agent can finally see the backend."*
When unsure whether to build something: *does it make one of these land harder?* If not, cut it.

## 11. Risks & mitigations (updated)
- **Spotlight / a vendor closes the gap** (e.g. Sentry adds an OTLP receiver) → move fast; own
  the no-SDK + zero-config + request-flight-recorder UX + local-first/no-account promise + TUI
  taste they won't prioritize. It's a combination moat — win on execution and speed.
- **ESM zero-config breaks** → the `rewynd run` launcher is the robust primary path; prove it
  in Phase 0; document the honest promise.
- **Correlation bug erodes trust** → conservative unattributed fallback; correlation tests early.
- **Unstable agent schema** → versioned, contract-tested `--json`/MCP.
- **TUI scope balloons** → ship lazygit-grade essentials first; themes/mouse later.
- **Scope creep into a debugger/APM** → re-read §1 anti-scope.

## 12. Immediate next actions (start here)
1. [ ] Scaffold the monorepo skeleton (§5): `go.work`, pnpm workspace, Apache-2.0, CI stub.
2. [ ] Build the **Phase 0 GATE 1 ESM/`tsx` spike** — the one thing that can force a pivot.
3. [ ] Write `PHASE_0_FINDINGS.md` with the real per-module-system zero-config promise.
4. [ ] Then Phase 1 walking skeleton (example app → shim → core → CLI → bare TUI).
```
```
