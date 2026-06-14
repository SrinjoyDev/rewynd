# Security Policy

## Scope

rewynd is a **local-only development tool**. The core binds to `127.0.0.1`, makes no
outbound network connections, collects no telemetry, and has a hard prod guard that refuses
to start under `NODE_ENV=production`. Recorded data stays on your machine. The threat surface
is therefore small, but we still take reports seriously.

## Supported Versions

Security fixes land on the **latest release**. Please confirm you can reproduce an issue on
the latest version before reporting.

## Reporting a Vulnerability

Please report security issues **privately** — do not open a public GitHub issue.

Email **dassrinjoy333@gmail.com** with:

- a description of the issue and its impact,
- steps to reproduce, and
- the rewynd version (`rewynd version`) and your environment.

We will acknowledge your report, investigate, and coordinate a fix and disclosure timeline
with you. Thanks for helping keep rewynd users safe.
