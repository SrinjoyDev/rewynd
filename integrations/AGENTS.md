# rewynd — backend flight recorder for coding agents

This file teaches a coding agent how to debug and verify backend code using **rewynd**, a
local request flight recorder. Drop it into a project as `AGENTS.md` (or paste it into your
agent's rules / knowledge), and the agent will read what the backend *actually did* instead
of guessing from the source.

> If you are a coding agent reading this: rewynd has already recorded the HTTP requests this
> app served during development — and, correlated to each one, the exact SQL (with params),
> outbound calls, log lines, and exceptions it caused. When a backend endpoint is slow,
> broken, or behaving unexpectedly, **read the recording instead of speculating or adding
> print statements.**

## When to use rewynd

Reach for it whenever you would otherwise guess about runtime behavior:

- An endpoint returned a 500 or the wrong data — read what actually ran.
- You changed code that touches the database, an outbound API, or request handling — trigger
  the endpoint and confirm the real SQL / calls match your intent.
- A request is slow — see which query or outbound call cost the time.
- You suspect an N+1 or a duplicate call — rewynd detects these deterministically.
- Before claiming a fix works — re-trigger and verify the trace is now clean.

Do not add `console.log`/`print` to investigate, and do not re-run a request five times
squinting at stdout. rewynd already captured it, correlated per request.

## The loop

```
1. clear              wipe the buffer so the next request is the only thing recorded
2. trigger            hit the endpoint — a curl, a test, a UI action
3. wait_for_request   block until it lands; you get the full correlated trace back
4. diagnose / read    failing SQL+params, exception+stack, detected N+1 / slow query
5. fix, then repeat from 1 to confirm the request is now green
```

## Two ways to drive it

**MCP (preferred for agents).** rewynd ships an MCP server (`rewynd mcp`). Once it is wired
into your agent (see the project README under `integrations/`), you have these tools:

| Tool | Use it to |
|---|---|
| `get_stats` | Get oriented: totals by status, how many errored / have an N+1, and the broken request ids. Start here. |
| `list_requests` | List recent requests; filter by `status` (`2xx`/`4xx`/`5xx`), `path`, `slow`, `has_error`. |
| `get_request` | Full trace for one id: headers, body, span waterfall, SQL+params, outbound, logs, exception. |
| `wait_for_request` | Block until a matching request is recorded, then return it. Use right after triggering. |
| `diagnose` | One-call summary of what's wrong with a request, with a fix suggestion. |
| `get_last_error` | The most recent 5xx in full — no id needed. |
| `clear` | Wipe the buffer for a clean slate before a test. |

The MCP server also sends usage instructions on connect; follow them.

**CLI (works in any shell / any agent).** The same data, as JSON your agent reads directly:

```bash
rewynd clear                                   # clean slate
curl -s localhost:3000/api/orders              # trigger the endpoint
rewynd watch --status 5xx --timeout 10s --json # block until it lands, print the full trace
rewynd diagnose <id>                           # "what's wrong here", one line
rewynd ls --has-error --json                   # list failing requests
rewynd show <id> --json                        # one request's full correlated story
rewynd last-error --json                       # the most recent 5xx in full
```

A request id prefix (first 8 characters) is accepted anywhere an id is expected.

## Rules of thumb

- **Trust the detections.** N+1, slow-query, and slow-request flags and `diagnose` are
  computed from the real recorded trace, not inferred. Prefer them over speculation. If
  `diagnose` reports nothing, that request is clean — look elsewhere rather than inventing a
  cause.
- **Always `clear` before triggering** a request you want to inspect, so it is unambiguous.
- **It is local and read-only.** rewynd binds `127.0.0.1`, never phones home, and reading it
  has no side effects. `clear` only wipes rewynd's own buffer, never your data.
- **If nothing is recorded,** the app may not be running under rewynd. Start it with
  `rewynd run <your dev command>` (Node) or `rewynd-run <your command>` (Python).

## Worked example

A `POST /api/orders` returns 500. Instead of reading the handler and guessing:

```
clear
POST /api/orders            (you or the user triggers it)
wait_for_request status=5xx
```

The trace comes back with the exception `null value in column "total" violates not-null
constraint`, the exact `INSERT` that failed with its params, and the log lines leading up to
it. The fix is obvious — `total` was never computed — and re-triggering confirms a 201. No
print statements, no re-running blind, no hallucinated cause.
