# rewynd for coding agents

rewynd is a backend flight recorder your coding agent can **drive**. Instead of guessing why
an endpoint failed — or writing `console.log` and re-running blind — the agent reads the real
recorded trace: the failing SQL with its params, the exception and stack, outbound calls,
correlated logs, and deterministic N+1 / slow detections.

This directory wires rewynd into the agent you use. Every integration teaches the same
**`clear → trigger → wait → read → fix`** loop; pick yours below.

## The canonical protocol

[`AGENTS.md`](./AGENTS.md) is the full agent-facing guide — when to use rewynd, the loop, the
tools, and the rules of thumb. The per-agent files below are thin wrappers around it. If your
agent reads an `AGENTS.md` (OpenCode, Codex, Gemini CLI, Jules, Amp, and a growing list),
just copy it into your project root and you are done.

## Two integration surfaces

Most agents support one or both of:

1. **MCP** — rewynd ships an MCP server (`rewynd mcp`) exposing `get_stats`, `list_requests`,
   `get_request`, `wait_for_request`, `diagnose`, `get_last_error`, `clear`. It also sends
   usage instructions on connect. This is the richest surface and the preferred one.
2. **A rules / knowledge file** — a markdown file that tells the agent rewynd exists and how
   to use its CLI. Use this for agents without MCP, or alongside MCP so the agent knows *when*
   to reach for it.

The MCP config is essentially identical everywhere — launch `rewynd` with the `mcp` argument:

```json
{
  "mcpServers": {
    "rewynd": { "command": "rewynd", "args": ["mcp"] }
  }
}
```

## Per-agent setup

### Claude Code

```bash
# MCP (run once in your project):
claude mcp add rewynd -- rewynd mcp

# Skill — copy into the project so Claude auto-loads it when debugging the backend:
cp -r integrations/claude-code/skills/rewynd .claude/skills/rewynd
```

The skill ([`claude-code/skills/rewynd/SKILL.md`](./claude-code/skills/rewynd/SKILL.md)) has a
trigger-rich description, so Claude pulls it in on its own when you hit a backend bug.

### Cursor

```bash
# Rule — agent-requested, loads when relevant:
mkdir -p .cursor/rules && cp integrations/cursor/rewynd.mdc .cursor/rules/rewynd.mdc
```

MCP — add to `.cursor/mcp.json` (the snippet above). Then Cursor's agent can call the tools
directly.

### Windsurf

- MCP: add the snippet above to `~/.codeium/windsurf/mcp_config.json`.
- Rules: copy [`AGENTS.md`](./AGENTS.md) to `.windsurf/rules/rewynd.md`.

### OpenCode / Codex CLI / Gemini CLI / Jules / Amp

These read `AGENTS.md` from the project root:

```bash
cp integrations/AGENTS.md AGENTS.md
```

For MCP: OpenCode uses an `mcp` block in `opencode.json`; Codex uses `~/.codex/config.toml`
(`[mcp_servers.rewynd]` with `command = "rewynd"`, `args = ["mcp"]`). Same command either way.

### Cline / Roo Code

- MCP: add the snippet above to the Cline/Roo MCP settings.
- Rules: copy [`AGENTS.md`](./AGENTS.md) to `.clinerules` (or `.roo/rules/rewynd.md`).

### Devin and other hosted agents

Paste the contents of [`AGENTS.md`](./AGENTS.md) into the agent's Knowledge / Playbook so it
learns the loop. Where the agent can run shell commands, the `rewynd` CLI (above) works
as-is.

### Any other agent

If it can run a shell command, it can use rewynd: give it [`AGENTS.md`](./AGENTS.md) as
context and let it use the CLI. If it speaks MCP, add the server. That is the whole point of
the OTLP-and-MCP design — one recorder, every agent.

## Prerequisite: record some traffic

None of this shows anything until the app runs under rewynd and serves a request:

```bash
rewynd run <your dev command>     # Node (Express, Fastify, Nest, ...)
rewynd-run <your command>         # Python (FastAPI, Flask, Django)
```

See the [project README](../README.md) for install and language details.
