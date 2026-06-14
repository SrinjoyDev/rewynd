# Changelog

All notable changes to rewynd are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org).

## [Unreleased]

### Added

- Zero-config, OTLP-native request capture with per-request correlation for **Node**
  (Express, Fastify; ESM/`tsx`) and **Python** (FastAPI / Flask / Django).
- Go core: OTLP/HTTP receiver, embedded SQLite ring-buffer store, and detections
  (N+1, slow query, slow request).
- Three frontends over one core: a Bubble Tea **TUI**, a JSON **CLI**
  (`run/tui/serve/ls/show/watch/tail/diagnose/last-error/clear/status`), and an **MCP
  server** (`rewynd mcp`, 6 tools) for coding agents.
- One-command launchers: `rewynd run <cmd>` (Node) and `rewynd-run <cmd>` (Python); the
  core auto-starts on first use.
- Request header + body capture with **in-app redaction**; outbound HTTP capture.
- Privacy: binds `127.0.0.1`, no telemetry, a hard production guard.
- Distribution: cross-platform binaries (goreleaser), `rewynd` on **npm** (per-platform
  binary via `optionalDependencies`) and **PyPI**, plus a `curl | sh` installer and a
  tag-driven release workflow.

[Unreleased]: https://github.com/SrinjoyDev/rewynd/commits/main
