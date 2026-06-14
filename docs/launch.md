# Launch kit

Copy-paste assets for launching rewynd. The only prerequisites are publishing the packages
(see [RELEASING.md](../RELEASING.md)) so the install commands work for people who arrive.

## One-line pitch

> A local flight recorder for your backend — every request's SQL, calls, logs, and exceptions,
> correlated, in a TUI/CLI/MCP your coding agent can drive. So agents (and you) read what
> actually happened instead of guessing.

## Show HN

**Title**

```
Show HN: Rewynd – a local flight recorder for your backend that AI agents can drive
```

**Body**

```
Backend devs have `print`; frontend devs have the network tab. When a local endpoint is slow
or 500s, you sprinkle console.logs and re-run it five times. Coding agents have it worse — they
can't see what the code did, so they *guess* the root cause and sometimes go dangerously deep
on the wrong thing.

Rewynd records every local request — the SQL with params, outbound calls, logs, exceptions —
correlated per request, with N+1/slow detection. There's a terminal UI, a JSON CLI, and an MCP
server so Claude Code / Cursor read the actual trace instead of hallucinating. It also handles
background jobs/queue consumers, distributed traces across services, and a load view (p50/p95,
error rate by endpoint, before/after a fix).

It's OpenTelemetry under the hood, so Node/Python/Go/Ruby/Java/.NET all feed it; runs fully
local (binds 127.0.0.1, refuses to start in production). MIT.

Repo: https://github.com/SrinjoyDev/rewynd

Would love feedback — especially from people whose agents debug backends.
```

## X / Twitter

```
Your AI coding agent debugs your backend blind — it guesses the bug instead of seeing it.

rewynd is a local flight recorder: every request's SQL, calls, logs & exceptions, correlated —
in a TUI, a CLI, and an MCP tool agents drive. Node/Python/Go/Ruby/Java. Local. MIT.

⭐ github.com/SrinjoyDev/rewynd
```

**Thread (optional)**

```
1/ When Claude Code or Cursor verifies a backend change live and finds a bug, it goes deep —
which is good — but often *assumes* the root cause instead of seeing it. And going network/
system-level deep on your machine is risky + a skill most people don't have.

2/ rewynd fixes the seeing part. It records every request you trigger while testing: the route,
the SQL + params, outbound calls, logs, the exception + stack — correlated per request.

3/ Three ways to read it: a beautiful TUI, a JSON CLI, and an MCP server. The agent calls
`get_stats` → `wait_for_request` → `diagnose` and reads ground truth. No more hallucinated
root causes.

4/ It detects N+1s and slow queries deterministically, stitches distributed traces across
services, records background jobs/queue consumers, and shows load stats (p50/p95, error rate)
— even a before/after diff to confirm a fix helped.

5/ OpenTelemetry under the hood, so Node/Python/Go/Ruby/Java all work. Runs 100% local. MIT.
A star genuinely helps: github.com/SrinjoyDev/rewynd
```

## Reddit (r/programming, r/node, r/golang, r/javascript)

**Title**

```
I built a local flight recorder for backends that coding agents can drive (TUI + CLI + MCP, MIT)
```

**Body** — the Show HN body works; add a sentence inviting critique of the approach.

## Blog — tightened opening

Keep your original blog; replace the opening and trim the claims to what ships today:

> AI agents debug your backend blind. When something breaks, a coding agent goes deep — which
> is good — but it often *assumes* the bug instead of seeing it, and "going network-and-system-
> level deep on someone's machine" is both risky and a skill most people don't have.
>
> So I built **rewynd** — a rewind button for your requests. What route got hit, which table the
> query touched, how many ms it took, what failed and why — captured during local testing and
> replayed in a beautiful TUI, a CLI, and an MCP tool agents drive. Now humans *and* agents read
> what actually happened instead of guessing.
>
> It's still early, with a lot of benchmarks already passing. Feedback, ideas, and a GitHub star
> would mean a lot.

> Edit note: keep claims to what's shipped (HTTP requests, DB/N+1, jobs, distributed, load
> stats). Frame anything beyond that as "where it's headed."

## Posting order

1. Publish the packages (RELEASING.md) so `npm i -D @rewynd/cli` / `pip install rewynd` work.
2. Post the blog.
3. Show HN (Tue–Thu morning ET tends to do best), then share the HN link on X/Reddit.
4. Reply to every comment for the first few hours — engagement drives ranking.
