# The agent-loop GIF — a coding agent debugging a real backend over the CLI. Generate with:
#   vhs demo/agent-loop.tape
# Requires: vhs, ttyd, ffmpeg, jq. Prereq: run `demo/setup.sh` first (starts the app on :3000
# and seeds some traffic, so `stats` and the N+1 endpoint have something to show).

Output demo/agent-loop.gif

Require rewynd

Set Shell bash
Set FontSize 18
Set Width 1380
Set Height 840
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 50ms

Hide
Type "clear" Enter
Show

Type "# 1) the agent hits the endpoint it just changed, then reads what actually happened:" Enter
Sleep 900ms
Type "curl -s -XPOST localhost:3000/api/orders -H 'content-type: application/json' -d '{}' >/dev/null" Enter
Sleep 800ms
Type "rewynd watch --status 5xx" Enter
Sleep 4s

Type "# 2) one line — the root cause, and the exact failing SQL:" Enter
Sleep 800ms
Type "rewynd diagnose $(rewynd last-error --json | jq -r .id)" Enter
Sleep 3.8s

Type "# 3) a slow endpoint — its N+1 query, folded into one line:" Enter
Sleep 800ms
Type "rewynd show $(rewynd ls --path /api/users --json | jq -r '.[0].id')" Enter
Sleep 4s

Type "# 4) zoom out — the whole app's load picture, by endpoint:" Enter
Sleep 800ms
Type "rewynd stats" Enter
Sleep 4.2s
