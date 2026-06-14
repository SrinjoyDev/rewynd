// Command rewynd is the single static binary: the brain (OTLP receiver, correlation, store,
// detections) plus the TUI, CLI, and MCP frontends.
package main

import (
	"fmt"
	"os"

	"github.com/SrinjoyDev/rewynd/core/internal/cli"
)

// version is overridden at release time via -ldflags.
var version = "0.0.0-dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
