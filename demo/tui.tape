# The hero TUI GIF. Generate with:  vhs demo/tui.tape
# Requires: vhs (https://github.com/charmbracelet/vhs), ttyd, ffmpeg.
# Prereq: seed the buffer first so the list has HTTP requests (an N+1 and a 500) and a
# couple of background jobs (see demo/setup.sh, or the seed used in development).

Output demo/tui.gif

Require rewynd

Set Shell bash
Set FontSize 18
Set Width 1380
Set Height 840
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 60ms

Hide
Type "clear" Enter
Show

Type "rewynd tui" Sleep 400ms Enter
# Opens on the N+1: ten identical SELECTs stack into an obvious staircase, with the fix.
Sleep 3s

# Down to the red 500 — the failing INSERT and the exception, no console.log.
Down Sleep 3s

# The list mixes HTTP requests and background jobs (queue consumers) as one stream.
Down Sleep 1s
Down Sleep 1.2s

# Press S for the load panel: throughput, latency percentiles, error rate, worst endpoints.
Type "S" Sleep 3.6s
Space Sleep 600ms

# Live search by path — filter the list as you type.
Type "/orders" Sleep 1.8s
Enter Sleep 1.4s
Escape Sleep 1s

# Every keybinding is one key away.
Type "?" Sleep 2.2s
Escape Sleep 600ms

Type "q"
Sleep 800ms
