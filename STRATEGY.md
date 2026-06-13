# Hindsight — Strategy Correction (post-research, 2026-06-13)

> This document corrects the Master Build Brief after a competitive + feasibility
> research pass. The brief's architecture and stack survive. Its **positioning** and
> one **silent technical assumption** do not. Read this before Phase 0.

## Verdict

**Build it — repositioned, and ESM-first.** The idea is sound and the specific
combination is unowned. But two of the brief's load-bearing premises were falsified
by research and must be corrected before any product code.

## What the research falsified

1. **"Nobody has built the agent-native backend debugger."** FALSE.
   **Sentry Spotlight** (`getsentry/spotlight`, v4.11.6 2026-06-10, shipping monthly)
   ships `spotlight tail` (terminal stream) **and** `spotlight mcp` (MCP server:
   `search_errors`, `search_logs`, `search_traces`, `get_traces` for Claude Code /
   Cursor). This is our §6 agent loop, already shipping, from a resourced vendor.
   **Spotlight is the #1 competitive threat.**

2. **"Beautiful TUI over OTLP is an open niche."** FALSE.
   **`ymtdzzz/otel-tui`** (~1,034★, Go, v0.7.3 2026-05-13, active) is a terminal
   OTLP viewer / receiver on 4317/4318. The raw "TUI over OTLP" real estate is taken.

3. **The name is taken 3×.** npm `hindsight` = dead 2020 squat; PyPI `hindsight` =
   dead 2013; **Vectorize ships a live AI product "Hindsight" (agent memory),
   npm 2026-06-12** — an adjacent brand collision in our own audience. Rename.

## Competitive reality (and the gap to exploit)

| Tool | What it is | Our exploitable gap |
|---|---|---|
| **Sentry Spotlight** ⚠️ | Local dev observability; `tail` + `mcp` + web/Electron UI | Requires the **Sentry SDK** (ingests Sentry envelopes :8969, **not native OTLP**); web UI not TUI; error-shaped, not a **per-request flight recorder** |
| **otel-tui** ⚠️ | Go terminal OTLP viewer, 1k★ | No per-request correlation, no MCP/agent loop, no SQLite, no Node zero-config shim — a generic signal browser, not "click the broken request, see its story" |
| otel-desktop-viewer / Aspire dashboard | Local OTLP → **browser** | Browser, ops-shaped, not agent-native |
| traceloop/opentelemetry-mcp-server (~190★) | MCP over **prod** OTel backends | Not zero-config local Node dev |
| Sentry Seer | Cloud AI debug agent | **Cloud-only, not self-hostable** |
| node-telescope clones | Hobby React dashboards, abandoned | The Node local-profiler slot has **no loved incumbent** (real) |

**Moat = the combination, not any single pillar.** Every individual pillar is taken;
the *combination* is not: **OTLP-native (no SDK) + zero-config Node + per-request
correlation (N+1/exception "story") + agent loop + real TUI, in one binary.** This is
a combination moat (shallow) — won on execution, DX, and speed, not on category novelty.
Assume Sentry *could* add an OTLP receiver to the Sidecar; our edge is taste + the
local-first/no-account promise + the request-flight-recorder UX they won't prioritize.

## Corrected positioning

- **Internal framing to KILL:** "the first backend debugger your agent can use" /
  "the slot nobody owns." Walking into the "isn't this Spotlight + otel-tui?" question
  with no answer is the failure mode.
- **New one-liner:** *"A zero-config, OTLP-native flight recorder for your backend —
  the per-request story (every query, call, log, and the N+1) in a beautiful TUI, and
  the first one your coding agent can drive. No SDK, no config, one binary, runs local."*
- **The two-sentence wedge vs the field:**
  - vs **Spotlight**: no Sentry SDK — stock OpenTelemetry, so it works on apps that
    never adopted Sentry; and a true per-request flight-recorder UX, not an error list.
  - vs **otel-tui**: correlation + detections + the agent/MCP loop — the *story of one
    request*, not a raw signal browser.

## Naming decision

- Bare evocative single words are all squatted (verified: hindsight, lookback,
  periscope, traceback, glass, gander, spyglass, loupe, otello, wiretap, tracewell,
  rearview, blackbird, scry, recce — taken or risky). `loupe` and `tracewell` are
  **actively** maintained — avoid. Free but utilitarian: **`flightrec`**, **`reqcorder`**.
- **Do NOT put "otel" in the name** — the brief (§8.4) says hide the plumbing; the name
  must not advertise it.
- **Strategy:** choose a short, beautiful brand for the **CLI command + GitHub repo** on
  taste; ship the npm shim **scoped** (`@<org>/<name>`) if the bare npm name is a dead
  squat. The npm name is not the viral surface — the typed command in the GIF is.
- **DECIDED (2026-06-13): `rewynd`.** Free on **npm + PyPI** (rare — secures the Python
  future); short, trendy, on-theme ("rewind your backend = hindsight"); hero command
  `npx rewynd run`. GitHub org `rewyndhq` (bare `rewynd` org is a private empty squat).
  Honest caveat: phonetic echo of consumer app *Rewind.ai* (different category). Swappable
  via find/replace in a fresh repo. Backups: `reqcap`, `flightrec` (free on npm, plainer).
- **Active-product collisions to AVOID (verified):** blip (Block/CashApp MySQL monitor),
  spanly (MCP-monitoring SDK), vantage (vantage.sh), retrace (Stackify APM), tracelet
  (AI-agent tracing lib, Apr 2026), loupe, tracewell, lark, miru, hindsight (Vectorize).

## Revised Phase 0 — the gate that can kill the plan

The brief's Phase 0 spike validates the *easy* half (CommonJS). The risk is ESM.

- **GATE 1 (lead, can force a pivot): zero-config capture + correct log↔request
  correlation on a real ESM + `tsx` Express + `pg` app** — not just CommonJS.
  - Why: OTel auto-instrumentation is reliable on CJS (`--require`) but fragile on ESM.
    `import '<tool>/auto'` at the top of an entry file **cannot reliably patch modules
    imported afterward** under ESM — needs `--import` / `register()` loader hook. Bundlers
    (tsx, esbuild, Next.js standalone) break monkey-patching. Modern target user = ESM+tsx.
  - If it fails: the honest promise becomes **`npx <tool> run app.js`** (we own the
    launcher and inject the loader) or a one-line `node --import <tool>/register`.
    Decide and write the README around the *honest* promise **before** building.
- **Correlation invariant:** best-effort with conservative fallback — if async context
  is ambiguous (detached promises, worker threads, some pool callbacks), show the log
  **unattributed** rather than attach it to the wrong request. "Show less, not wrong."
- GATE 2 (low risk): Bubble Tea pane renders a live list + simple waterfall.
- GATE 3 (low risk): `watch` blocks until a request matches, emits stable JSON.

## Architecture note to revisit (not a blocker)

Consider simplifying the daemon: make the **OTLP receiver the only long-lived process,
writing to SQLite (WAL)**; let TUI/CLI/MCP be **readers of the same SQLite file** (WAL =
multi-reader/single-writer for free) + a tiny notify channel for live push. Deletes an
RPC layer and a class of stale-daemon/version-skew bugs. Add RPC only if shared reads
provably can't deliver. Stack (Go + Charm + TS/OTel) is unchanged.

## Immediate next actions

1. [ ] Owner: pick brand name + GitHub org (taste-driven; scoped npm is fine).
2. [ ] Build the GATE 1 ESM/`tsx` spike — decide the real "zero-config" promise.
3. [ ] Then: walking skeleton per brief Phase 1 (CLI → minimal TUI → MCP).
