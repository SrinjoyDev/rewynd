# Go example — a service recorded by rewynd

A Go HTTP service using the [`rewynd` Go SDK](../../sdk/go/). `Start()` wires OpenTelemetry to
the local core; `otelhttp` records each request, and a child span tagged as a DB query shows up
correlated under it.

## Run

```bash
# start the core (or let `rewynd run` auto-start it)
rewynd serve &

# build + run the service (it uses a local replace for the SDK)
cd examples/go-service
go run .

# in another terminal
curl -s localhost:8090/api/widgets
rewynd show $(rewynd ls --json | jq -r '.[0].id')
```

## What you see

```text
GET /api/widgets  ->  200  (16ms)

QUERIES (1)
   15ms  SELECT id, name FROM widgets WHERE active = $1
```

The request and its query, recorded from a Go service with no print statements — the same
view Node and Python get. Swap the hand-rolled span for [`otelsql`](https://github.com/XSAM/otelsql)
and your real queries show up automatically.
