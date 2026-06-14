# Demo GIFs

The README's hero GIFs are generated with [vhs](https://github.com/charmbracelet/vhs).

## Regenerate

```bash
# one-time: install the tools
go install github.com/charmbracelet/vhs@latest      # vhs
#   plus ttyd + ffmpeg (apt install ttyd ffmpeg, or static binaries)

# start the example app + seed a few requests, then record:
./demo/setup.sh
vhs demo/tui.tape          # -> demo/tui.gif
vhs demo/agent-loop.tape   # -> demo/agent-loop.gif
```

The tapes (`*.tape`) are declarative — tweak the keystrokes, sizes, and theme there.
