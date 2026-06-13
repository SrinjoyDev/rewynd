# Phase 0 Findings — rewynd

Status as of 2026-06-13. Phase 0 de-risks the load-bearing assumptions before building
the product. **GATE 1 (the one that could force a pivot) is PASSED.**

## GATE 1 — zero-config capture + per-request correlation under ESM/`tsx` ✅ PASSED

**Claim tested:** the brief's "zero-config" promise silently assumes CommonJS; modern Node
apps are ESM + `tsx`, where a top-of-file `import` cannot patch later imports. **Result:
capture + correlation work under ESM/`tsx`** when the shim is injected via `--import`
(not a code import). No pivot needed; the `rewynd run` launcher just has to set `--import`.

**Evidence** (real run, Express + `pg` + pino, ESM via tsx, Docker Postgres):
- `/api/users` (trace `41abd7fa`): one `GET /api/users` root span → `request handler` →
  **10 identical `SELECT … FROM posts WHERE user_id = $1`** child spans (the N+1, captured
  and correlated) → log `assembled users with posts` carrying `trace_id=41abd7fa`.
- `/api/orders` (trace `f3995787`): `POST` → middleware spans → `creating order` log →
  failing `INSERT INTO orders …` span → `request failed` **error** log — all one trace_id.
- pino output now includes `trace_id`/`span_id`/`trace_flags`; bridged log records carry
  the active span context. **Log→request correlation works.**

**What made it work:**
- Shim = `@rewynd/shim` (`packages/shim-node`), entry `src/register.mjs`, loaded via
  `node --import` (we used `NODE_OPTIONS=--import file://…/register.mjs`).
- `register.mjs`: installs the OTel **ESM loader hook**
  (`module.register('@opentelemetry/instrumentation/hook.mjs', …)`) BEFORE the SDK, then
  starts `NodeSDK` with `getNodeAutoInstrumentations()` (http, express, pg, pino).
- **Log correlation needs `@opentelemetry/instrumentation` as a direct shim dep** (so the
  ESM hook resolves) + `logRecordProcessors` on `NodeSDK` (so pino log-bridging fires).
  Without the direct dep the hook failed and pino logs had no trace_id.
- OTel versions (compatible set): `api@1.9.1`, `api-logs/instrumentation/sdk-node/
  exporter-*-otlp-proto@0.219.0`, `auto-instrumentations-node@0.77.0`.

**The honest zero-config promise (write the README around THIS):**
1. PRIMARY: `rewynd run <dev command>` — launcher injects `--import` + OTLP endpoint. Robust.
2. `node --import @rewynd/register app.js` (ESM, one flag).
3. `node -r @rewynd/register app.js` / top-of-file import (CJS).

**Exact working invocation (reproduce):**
```bash
docker run -d --name rewynd-pg -e POSTGRES_PASSWORD=rewynd -e POSTGRES_USER=rewynd \
  -e POSTGRES_DB=app -p 5433:5432 postgres:16-alpine
export DATABASE_URL="postgresql://rewynd:rewynd@localhost:5433/app"
export NODE_OPTIONS="--import file:///home/srinjoy/hindsight/packages/shim-node/src/register.mjs"
/home/srinjoy/hindsight/examples/express-postgres/node_modules/.bin/tsx \
  /home/srinjoy/hindsight/examples/express-postgres/src/server.ts
# then: curl localhost:3000/api/users ; curl -XPOST localhost:3000/api/orders -d'{"userId":1}' -H'content-type: application/json'
```

## Remaining Phase 0 gates (low risk)
- [ ] GATE 2 — Bubble Tea spike: live list + one waterfall row.
- [ ] GATE 3 — `watch` primitive: block until a matching request lands, emit stable JSON.
- [ ] Wire shim → OTLP → minimal Go receiver (start of the core); confirm Go OTLP ingest.

## Environment quirks (CRITICAL for resume — these cost real time)
- **Docker**: native `docker` works ONLY when Docker Desktop is running with WSL
  integration. If `dockerDesktopLinuxEngine` pipe is "not found", Desktop is off. Postgres
  container `rewynd-pg` on host port **5433**.
- **No system Postgres, no passwordless sudo.** A no-Docker fallback exists:
  `embedded-postgres@18.4.0-beta.17` (real PG binary in-process). It needs
  `LD_LIBRARY_PATH=<pkg>/native/lib` AND soname symlinks (`libicui18n.so.60 ->
  …so.60.2`, etc.) created manually; the app prefers `DATABASE_URL` if set (use Docker).
- **pnpm 11 blocks build scripts.** `embedded-postgres`, `esbuild`, `protobufjs` show
  `ERR_PNPM_IGNORED_BUILDS`. A hook keeps re-adding an `allowBuilds:` placeholder to
  `pnpm-workspace.yaml`. Consequence: **`pnpm exec` runs a deps-check that exits 1** — so
  run the app with **node/tsx directly**, not `pnpm --filter … exec`.
- **Killing processes needs `dangerouslyDisableSandbox: true`** (plain `pkill` → exit 144,
  sandbox-blocked). Find PID via `ss -ltnp | grep :PORT`.
- **Foreground `sleep` is blocked.** Wait via `curl --retry --retry-connrefused` or Node
  `setTimeout` (added a DB connection retry in `db.ts` for this reason).
