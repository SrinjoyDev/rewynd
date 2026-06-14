---
name: rewynd-backend-debugging
description: Use when debugging, verifying, or investigating backend runtime behavior — HTTP 500s, wrong responses, slow endpoints, suspected N+1 or duplicate queries, "why did this request do that" — in a project running under rewynd. Reads the actual recorded request trace (SQL with params, outbound calls, logs, exceptions, detections) instead of guessing from source or adding print statements.
---

# Debugging backends with rewynd

rewynd is a local flight recorder for the backend. It has already recorded the HTTP requests
this app served during development and, correlated to each one, the exact SQL (with params),
outbound calls, log lines, and exceptions it caused. **Read what actually happened — do not
speculate from the source, and do not add `console.log`/`print` to investigate.**

## When to use this skill

- An endpoint returned a 500 or the wrong data.
- You changed code touching the DB, an outbound call, or request handling and want to confirm
  the real runtime behavior matches your intent.
- A request is slow, or you suspect an N+1 / duplicate query.
- Before telling the user a fix works — re-trigger and confirm the trace is clean.

## The loop

1. **clear** the buffer so the next request is unambiguous.
2. **trigger** the endpoint (curl it, run the test, or ask the user to click it).
3. **wait** for the request to land — you get the full correlated trace back.
4. **read / diagnose** — the failing SQL+params, the exception+stack, any detected N+1.
5. **fix** the code, then repeat from step 1 to confirm it is now green.

## How to drive it

**If the rewynd MCP server is connected** (tools prefixed `rewynd` / named `get_stats`,
`list_requests`, `get_request`, `wait_for_request`, `diagnose`, `get_last_error`, `clear`):
prefer these tools. Start with `get_stats` to orient, then `wait_for_request` right after
triggering, then `diagnose` or `get_request` on the id. The server sends usage instructions
on connect — follow them.

**Otherwise use the CLI** (works in any shell):

```bash
rewynd clear                                   # clean slate
curl -s localhost:3000/api/orders              # trigger
rewynd watch --status 5xx --timeout 10s --json # block until it lands, print full trace
rewynd diagnose <id>                           # what's wrong, one line
rewynd ls --has-error --json                   # list failing requests
rewynd show <id> --json                        # one request's full story
rewynd last-error --json                       # most recent 5xx, in full
```

A request id prefix (first 8 chars) works anywhere an id is expected.

## Rules

- **Trust the detections and `diagnose`** — they are computed from the real trace, not
  inferred. If `diagnose` is empty, the request is clean; look elsewhere rather than inventing
  a cause.
- **Always `clear` before triggering** the request you want to inspect.
- If nothing is recorded, the app is probably not running under rewynd — start it with
  `rewynd run <dev command>` (Node) or `rewynd-run <command>` (Python).
- rewynd is local and read-only; `clear` only wipes rewynd's buffer, never your data.

## Setup (if rewynd is not yet wired in)

Install: `npm i -D rewynd` (Node) or `pip install rewynd` (Python), or
`curl -fsSL https://raw.githubusercontent.com/SrinjoyDev/rewynd/main/scripts/install.sh | sh`.

Connect the MCP server (run once in the project):

```bash
claude mcp add rewynd -- rewynd mcp
```

Then run the app under rewynd (`rewynd run <dev command>`) so requests are recorded.
