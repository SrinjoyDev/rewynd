# Jobs example — background work, no HTTP request

Most backends do work that isn't an HTTP request: queue consumers, cron jobs, workers. rewynd
records those too. This worker drains an in-memory queue; each job runs inside an OpenTelemetry
**consumer** span, so rewynd records it as a first-class **job flow** — with the work it did
correlated under it, and a clear ok/fail outcome. One job fails on purpose.

## Run

From this directory:

```bash
cd examples/jobs

# the service the jobs call
REWYND_SERVICE=sink rewynd run -- node sink.mjs

# in another terminal: drain the queue
REWYND_SERVICE=worker rewynd run -- node worker.mjs

# then look — jobs show up alongside HTTP requests
rewynd ls
rewynd show $(rewynd ls --has-error --json | jq -r '.[0].id')   # the failed job
```

## What you see

```text
$ rewynd ls
ID        METHOD   PATH            STATUS  DURATION  QUERIES  FLAGS
a1b2c3d4  process  orders.created  fail    5ms       0        job,error
e5f6a7b8  process  orders.created  ok      7ms       0        job
f0a1b2c3  process  orders.created  ok      54ms      0        job

$ rewynd show a1b2c3d4
JOB process orders.created  ->  fail  (5ms)
services worker -> sink

OUTBOUND (1)
   3ms  POST http://localhost:3002/charge -> 422   [worker]

EXCEPTIONS (1)
  Error: charge failed: 422
```

A failed background job — the exact call that failed and the exception — without a single HTTP
request involved, and without one `console.log`. The same `clear → trigger → read → fix` loop
your agent already uses for HTTP works for jobs and consumers.
