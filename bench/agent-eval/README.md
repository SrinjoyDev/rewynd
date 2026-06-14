# Agent benchmark — does rewynd help a coding agent debug?

A reproducible eval of the core claim: **a coding agent diagnoses backend bugs more reliably
when it can read the recorded trace (rewynd) than when it has only the source and the logs** —
which is what it has today, and why it sometimes confidently invents a wrong root cause.

**→ [Results](./results.md):** correct root cause **4/4 with rewynd vs 3/4 without**; correct
*and* confident **4/4 vs 2/4**. The miss without rewynd was a silently-failing job the agent
*confidently misdiagnosed.*

## The tasks

Four realistic backend bugs (`tasks/`), each a different failure mode:

| Task | Bug | Where the cause hides |
|---|---|---|
| `checkout-500` | a `NOT NULL` violation (`total` never computed) | the exception + the failing INSERT |
| `users-nplus1` | an N+1 query in a loop | 50 identical SELECTs in the trace |
| `pay-502` | an outbound payments call returned 500 | the outbound span's status code |
| `email-job` | a queue job threw `SMTPConnectError` | a background-job flow (no HTTP request at all) |

Each task ships the **buggy source** and the **application logs** (what you'd have without a
recorder). The matching rewynd trace is produced by the seeder.

## Method

The same agent gets the same bug and prompt in both arms; the only difference is rewynd.

- **Without:** read `tasks/<id>/<source>` and `tasks/<id>/app.log`. Diagnose.
- **With:** the same, plus `rewynd ls / show / diagnose` over the recorded trace.

Each run ends with a single `ROOT CAUSE:` line, graded against `manifest.json`.

## Reproduce it

```bash
# 1. a clean core
REWYND_DB=/tmp/rw-eval.db rewynd serve &

# 2. seed the four scenarios as real traces
go -C core run ./cmd/eval-seed            # sends them over OTLP

# 3. confirm they recorded
REWYND_DB=/tmp/rw-eval.db rewynd ls
REWYND_DB=/tmp/rw-eval.db rewynd diagnose <id>

# 4. run your agent on each task, twice:
#    - WITHOUT: give it only tasks/<id>/<source> + tasks/<id>/app.log
#    - WITH:    also let it run `REWYND_DB=/tmp/rw-eval.db rewynd show/diagnose <id>`
#    grade the ROOT CAUSE line against manifest.json
```

Bring your own agent (Claude Code, Cursor, an SDK loop) — the harness is the tasks + the
grader, not a specific model. Add your own bugs to `tasks/` and a row to `manifest.json`.
