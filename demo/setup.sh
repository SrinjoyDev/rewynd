#!/bin/sh
# Start the example app under rewynd and fire a few requests, so the TUI/CLI have something
# to show before recording the demo GIFs. Requires a `rewynd` on PATH and Docker Postgres
# (see CONTRIBUTING.md) on localhost:5433.
set -e

here=$(cd "$(dirname "$0")" && pwd)
app="$here/../examples/express-postgres"

echo "starting the example app under rewynd..."
( cd "$app" && DATABASE_URL="postgresql://rewynd:rewynd@localhost:5433/app" \
    rewynd run -- node_modules/.bin/tsx src/server.ts >/tmp/rewynd-demo-app.log 2>&1 & )

# wait for it to come up
for _ in $(seq 1 60); do
  curl -fsS -m 1 localhost:3000/api/feed >/dev/null 2>&1 && break
  sleep 1
done

rewynd clear
echo "firing sample requests..."
curl -s localhost:3000/api/feed >/dev/null
curl -s localhost:3000/api/users >/dev/null      # the N+1
curl -s localhost:3000/api/users >/dev/null
curl -s -XPOST localhost:3000/api/orders -H 'content-type: application/json' -d '{"userId":1,"note":"ship it"}' >/dev/null  # the 500

echo "ready. Now record:  vhs demo/tui.tape   and   vhs demo/agent-loop.tape"
