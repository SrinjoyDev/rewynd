# Languages — recording any backend

rewynd's core speaks **OpenTelemetry over OTLP** (HTTP on `:4318`, gRPC on `:4317`). That is the
universal seam: *anything* that emits OpenTelemetry can feed rewynd, in any language. The TUI,
CLI, and MCP then work identically — same correlation, same N+1/slow detection, same load view.

`rewynd run -- <your command>` sets the standard OTLP environment variables for the process it
launches:

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_TRACES_EXPORTER=otlp   OTEL_LOGS_EXPORTER=otlp   OTEL_METRICS_EXPORTER=none
```

So for any language whose OpenTelemetry instrumentation reads those variables (most do),
`rewynd run` is all the wiring you need. Your own values are never overridden.

## First-class shims (the easiest)

| Language | Setup |
|---|---|
| **Node.js** | `npm i -D @rewynd/cli`, then `rewynd run <dev cmd>`. Zero code — auto-instruments http, pg/mysql, fetch/axios, pino/winston. |
| **Python** | `pip install rewynd`, then `rewynd-run <cmd>`. Zero code — FastAPI/Flask/Django, psycopg2/SQLAlchemy, requests/httpx, logging. |
| **Go** | `go get github.com/SrinjoyDev/rewynd/sdk/go`; `rewynd.Start(ctx)` + `otelhttp`/`otelsql`. Minimal code (Go has no runtime auto-instrumentation). See [`sdk/go`](../sdk/go/). |

## Any other language, via its OpenTelemetry auto-instrumentation

These need **no rewynd-specific code** — install the language's standard OTel auto-instrumentation
and run it under `rewynd run` (which points it at the core):

- **Java / Kotlin / Scala / JVM** — the [OpenTelemetry Java agent](https://opentelemetry.io/docs/zero-code/java/agent/) instruments Spring, JDBC, HTTP clients, and more with no code change:
  ```bash
  rewynd run -- java -javaagent:opentelemetry-javaagent.jar -jar app.jar
  ```
- **.NET** — the [OpenTelemetry .NET automatic instrumentation](https://opentelemetry.io/docs/zero-code/net/):
  ```bash
  rewynd run -- ./your-dotnet-app          # after enabling the auto-instrumentation
  ```
- **Ruby** — `opentelemetry-sdk` + `opentelemetry-instrumentation-all`:
  ```bash
  rewynd run -- ruby app.rb
  ```
  A runnable, verified example is in [`examples/ruby-service`](../examples/ruby-service/) — Ruby
  records through the standard OTLP env vars with no rewynd-specific code.
- **PHP** — the OpenTelemetry PHP extension + auto-instrumentation:
  ```bash
  rewynd run -- php -S localhost:8080
  ```
- **Rust** — set up `opentelemetry-otlp` in code (like Go); `rewynd run` provides the endpoint, or
  read `OTEL_EXPORTER_OTLP_ENDPOINT` yourself.

## Anything at all

If your service can emit OTLP — directly, through a language SDK, or through an existing
OpenTelemetry Collector you re-point — set:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318   # or :4317 for gRPC
```

and it shows up in rewynd. One local recorder, every backend — for you and the coding agents
debugging alongside you.

> rewynd is a **local development** tool: it binds `127.0.0.1`, never phones home, and refuses
> to run under `NODE_ENV=production`. Point your services at it on your machine, not in prod.
