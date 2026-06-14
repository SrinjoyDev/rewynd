# The hero TUI GIF. Generate with:  vhs demo/tui.tape
# Requires: vhs (https://github.com/charmbracelet/vhs), ttyd, ffmpeg.
# Prereq: run `demo/setup.sh` first to start the example app and seed a few requests.

Output demo/tui.gif

Require rewynd

Set Shell bash
Set FontSize 18
Set Width 1320
Set Height 780
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 65ms

Hide
Type "clear" Enter
Show

Type "rewynd tui" Sleep 400ms Enter
Sleep 2s

# Arrow down through the live requests — the detail pane updates with each.
Down Sleep 1.3s
Down Sleep 1.3s

# Land on the N+1 request: the stacked SELECT waterfall makes it obvious.
Sleep 3s
Down Sleep 1.3s

# The red 500: request body, the failing query, the exception, the logs.
Sleep 3.5s

Type "q"
Sleep 1s
