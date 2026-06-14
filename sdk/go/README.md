# rewynd for Go

Record a Go service to a local [rewynd](https://github.com/SrinjoyDev/rewynd) core.

Go has no runtime auto-instrumentation, so this is **minimal setup, not zero setup** (unlike the
Node and Python shims): you call `Start` once and add standard OpenTelemetry instrumentation
(`otelhttp`, `otelsql`, …). `Start` owns the painful part — exporter, resource, provider,
batching, and flush-on-exit.

```bash
go get github.com/SrinjoyDev/rewynd/sdk/go
```

```go
import (
    rewynd "github.com/SrinjoyDev/rewynd/sdk/go"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
    shutdown, _ := rewynd.Start(context.Background(), rewynd.WithService("widgets"))
    defer shutdown(context.Background())

    http.ListenAndServe(":8080", otelhttp.NewHandler(mux, "http.server"))
}
```

Wrap your database with [`otelsql`](https://github.com/XSAM/otelsql) and outbound calls with
`otelhttp.NewTransport` and they show up correlated under each request — the same TUI, CLI, and
MCP you get for Node and Python. A runnable example is in
[`examples/go-service`](../../examples/go-service/).

## Defaults

- **Service name**: `OTEL_SERVICE_NAME` → `REWYND_SERVICE` → the executable name.
- **Endpoint**: `OTEL_EXPORTER_OTLP_ENDPOINT` → `127.0.0.1:4317` (the rewynd OTLP/gRPC port).
- **Off switch**: set `REWYND_DISABLED` to make `Start` a no-op, so it is safe to leave in
  code that also runs in production.

Start the rewynd core first (`rewynd serve`, or it auto-starts under `rewynd run`).
