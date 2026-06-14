# Distributed example — two services, one trace

A `gateway` service that calls a `billing` service over HTTP. Both run under rewynd and feed
the same core, so a single `POST /api/checkout` is recorded as **one trace** spanning both
services — the gateway is the root, and billing's work is tagged to billing.

No dependencies: plain Node `http` + `fetch`, instrumented automatically by the rewynd shim.

## Run

From this directory, in three terminals (or background the first two):

```bash
cd examples/distributed

# 1. the downstream service
REWYND_SERVICE=billing rewynd run -- node billing.mjs

# 2. the gateway (calls billing)
REWYND_SERVICE=gateway rewynd run -- node gateway.mjs

# 3. fire a request, then look
curl -s -XPOST localhost:3000/api/checkout
rewynd show $(rewynd ls --json | jq -r '.[0].id')
```

`REWYND_SERVICE` just sets each process's OpenTelemetry service name so the two are
distinguishable; in a real project that comes from your service's package name or config.

## What you see

```text
GET /api/checkout  ->  200
services gateway -> billing

OUTBOUND (1)
   …  POST http://localhost:3001/charge -> 200   [gateway]
```

The gateway's outbound call and billing's server span are the same trace: rewynd kept the
**entry** service as the request root and tagged each part with the service it ran in. Scale
this to a real stack — many Node/Python services, a worker, a queue consumer — and every
inbound request is recorded with the full cross-service story.
