# The hero TUI GIF. Generate with:  vhs demo/tui.tape
# Requires: vhs (https://github.com/charmbracelet/vhs), ttyd, ffmpeg.
# Prereq: seed the buffer first — start the example app under rewynd and fire a few requests
# (see demo/setup.sh), so the list has an N+1 and a 500 to show.

Output demo/tui.gif

Require rewynd

Set Shell bash
Set FontSize 18
Set Width 1360
Set Height 820
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 60ms

Hide
Type "clear" Enter
Show

Type "rewynd tui" Sleep 400ms Enter
# Opens on the newest request — the red 500: exception, failing INSERT, request +
# response body, the logs that led there. The whole story, no console.log.
Sleep 3s

# The detail pane scrolls — long traces are fully readable.
Ctrl+D Sleep 1.6s
Ctrl+D Sleep 1.8s
Ctrl+U Sleep 1.2s

# Down to the N+1: ten identical SELECTs stack into an obvious staircase, with a fix.
Down Sleep 1s
Down Sleep 3.2s

# Live search by path — filter the list as you type.
Type "/users" Sleep 1.8s
Enter Sleep 1.6s
Escape Sleep 1.2s

# A keybinding overlay is one key away.
Type "?" Sleep 2.2s
Escape Sleep 600ms

Type "q"
Sleep 800ms
