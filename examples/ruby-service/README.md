# Ruby example — any OpenTelemetry language

Proof that rewynd records languages beyond its first-class shims. This Ruby script has **no
rewynd-specific code** — it uses standard OpenTelemetry. `rewynd run` sets the OTLP environment
variables, and Ruby's exporter sends to the local core. The same path covers Java, .NET, PHP,
Rust, and anything else that speaks OpenTelemetry (see [docs/languages.md](../../docs/languages.md)).

## Run

```bash
cd examples/ruby-service
gem install opentelemetry-sdk opentelemetry-exporter-otlp   # or: bundle install

rewynd run -- ruby ping.rb
rewynd show $(rewynd ls --json | jq -r '.[0].id')
```

## What you see

```text
GET /api/ping  ->  200  (11ms)

QUERIES (1)
   10ms  SELECT id, email FROM accounts WHERE id = $1
```

A Ruby request and its query, in the same TUI/CLI/MCP that Node, Python, and Go use — recorded
through the standard OpenTelemetry env vars, no shim required.
