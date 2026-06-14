# The agent-loop GIF — your coding agent debugging the backend over the CLI. Generate with:
#   vhs demo/agent-loop.tape
# Requires: vhs, ttyd, ffmpeg, jq. Prereq: run `demo/setup.sh` first.

Output demo/agent-loop.gif

Require rewynd

Set Shell bash
Set FontSize 18
Set Width 1320
Set Height 780
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 55ms

Hide
Type "clear" Enter
Show

Type "# the agent edits an endpoint, then runs the loop:" Enter
Sleep 1.2s

Type "rewynd clear" Enter
Sleep 1s

Type "curl -s -XPOST localhost:3000/api/orders >/dev/null" Enter
Sleep 1.5s

Type "rewynd watch --status 5xx --json | head -28" Enter
Sleep 3.5s

Type "rewynd diagnose $(rewynd last-error --json | jq -r .id)" Enter
Sleep 4s
