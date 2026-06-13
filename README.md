# rewynd

**A zero-config, OTLP-native flight recorder for your backend.** It silently records every
HTTP request, database query, outbound call, log line, and exception during local development,
**correlates them per request**, and lets you — or your coding agent — see exactly what
happened. No `console.log`, no re-running.

> Laravel Telescope, reborn for Node — terminal-native, and the first backend recorder your
> coding agent can actually drive. OpenTelemetry under the hood, zero config on top. Runs
> entirely on your machine.

> 🚧 **Early development.** The core, the CLI, and the agent loop work today (see below). The
> beautiful TUI and the MCP server are next. Stars and feedback welcome.

---

## The problem

Frontend devs have the Chrome DevTools network tab. Backend devs have `print`. When an
endpoint is slow or broken, you sprinkle `console.log`, re-run the request five times, and
squint at which SQL fired. And coding agents have it worse — they write backend code and fly
blind, unable to see what it actually did.

rewynd is the network tab for your backend — for humans **and** agents.

## Quick start

```bash
# In your Node project (Express, Fastify, …):
npm i -D @rewynd/shim          # (publishing soon; build from source today — see CONTRIBUTING)

# Run your normal dev command through rewynd — it auto-starts the recorder:
rewynd run npm run dev

# In another terminal:
rewynd ls                      # every request, color-coded, with query counts and flags
rewynd show <id>               # the full story of one request
```

```text
$ rewynd show 6928a80f
GET /api/users  ->  200  (15ms)

DETECTIONS
  ! n_plus_one — N+1 query — 10 identical statements

QUERIES (11)
      2ms  SELECT id, name FROM users ORDER BY id
      1ms  SELECT id, title FROM posts WHERE user_id = $1
      …  ×10  (the N+1, obvious at a glance)

LOGS (2)
  [info] listing users
  [info] assembled users with posts
```

## For your coding agent — the differentiator

rewynd gives agents a tight, structured **`clear → trigger → watch → read → fix`** loop so they
can debug a backend autonomously:

```bash
rewynd clear                                   # clean slate
curl localhost:3000/api/orders                 # the agent triggers the endpoint
rewynd watch --status 5xx --timeout 10s --json # blocks until it lands, prints the full trace
rewynd diagnose <id>                           # "what's wrong here" in one line
```

`watch` returns the failing SQL with its params, the exception and stack, and any detected
N+1 — as JSON the agent reads directly.

Or skip the CLI: rewynd ships an **MCP server** so agents introspect the backend natively.
Drop this into your Claude Code / Cursor MCP config:

```json
{
  "mcpServers": {
    "rewynd": { "command": "rewynd", "args": ["mcp"] }
  }
}
```

Tools: `list_requests`, `get_request`, `wait_for_request`, `diagnose`, `get_last_error`, `clear`.

## What it captures, correlated per request

| | |
|---|---|
| **HTTP requests** | method, path, status, timing |
| **DB queries** | SQL + params + duration, with **N+1 detection** |
| **Outbound calls** | method, URL, status, duration |
| **Logs** | `console` / `pino` / `winston`, stamped with the request's trace |
| **Exceptions** | type, message, stack |

## Commands

| Command | What it does |
|---|---|
| `rewynd run <cmd>` | Run your dev command with recording on (auto-starts the core) |
| `rewynd ls` | List requests (`--status 5xx`, `--slow`, `--has-error`, `--path`, `--json`) |
| `rewynd show <id>` | Full correlated trace for one request (`--json`) |
| `rewynd watch` | Block until a matching request is recorded, then print it (`--json`) |
| `rewynd tail` | Stream requests as they arrive |
| `rewynd diagnose <id>` | Summarize what's wrong (N+1, exceptions, slow queries) |
| `rewynd last-error` | The most recent 5xx, in full |
| `rewynd clear` | Wipe the buffer |
| `rewynd status` | Is the core running, how many requests buffered |

## Privacy — it's all local

No cloud, no account, no telemetry on you. The core binds `127.0.0.1` and never makes an
outbound connection. A hard **prod guard** refuses to start under `NODE_ENV=production`.
Secrets are redacted; bodies are size-capped. Your request data never leaves your machine.

## How it works

```
your app ─(stock OpenTelemetry, configured by the shim)→ OTLP ─→ rewynd core ─→ SQLite
                                                                    │
                                              TUI · CLI · MCP read the same recording
```

The shim stands on OpenTelemetry's auto-instrumentation (you never see OTel config). The Go
core is a single static binary: OTLP receiver → correlation → detections → embedded SQLite,
with the TUI, CLI, and MCP as thin clients over it.

## Roadmap

- [x] Zero-config capture + per-request correlation (Node, ESM/`tsx`)
- [x] Go core: OTLP receiver, SQLite store, N+1 detection
- [x] CLI + the agent `watch` loop, `rewynd run` launcher
- [x] MCP server (`rewynd mcp`) + `mcp.json` quickstart
- [ ] The beautiful TUI (live list + waterfall) — the hero
- [ ] Request/response body capture + redaction
- [ ] More frameworks (Fastify, Nest, Next), ORMs (Prisma, Drizzle), Python

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). The repo is a pnpm + Go monorepo:
`core/` (Go), `packages/shim-node/` (the shim), `examples/express-postgres/` (demo + fixture).

## License

[MIT](./LICENSE)
