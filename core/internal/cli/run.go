package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/SrinjoyDev/rewynd/core/internal/config"
)

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "run <command> [args...]",
		Short:              "Run your dev command with rewynd recording enabled (auto-starts the core)",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			if len(args) == 0 {
				return fmt.Errorf("usage: rewynd run <command> [args...]")
			}
			if err := ensureDaemon(); err != nil {
				return err
			}

			child := exec.Command(args[0], args[1:]...)
			child.Env = childEnv()
			child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := child.Start(); err != nil {
				return err
			}

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
			go func() {
				for s := range sigs {
					_ = child.Process.Signal(s)
				}
			}()
			err := child.Wait()
			signal.Stop(sigs)
			if exit, ok := err.(*exec.ExitError); ok {
				os.Exit(exit.ExitCode())
			}
			return err
		},
	}
}

// childEnv is the environment the recorded command runs in. It points any OpenTelemetry SDK or
// agent at the local rewynd core via the standard OTLP env vars — so a Java agent, a .NET or
// Ruby auto-instrumentation, a Rust service, anything OTel, exports to rewynd with no shim — and
// additionally injects the Node auto-instrumentation shim when it's installed. User-set values
// are never overridden.
func childEnv() []string {
	env := os.Environ()
	set := func(k, v string) {
		if os.Getenv(k) == "" {
			env = append(env, k+"="+v)
		}
	}
	set("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+config.DefaultOTLPAddr) // 127.0.0.1:4318
	set("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	set("OTEL_TRACES_EXPORTER", "otlp")
	set("OTEL_LOGS_EXPORTER", "otlp")
	set("OTEL_METRICS_EXPORTER", "none") // rewynd records traces + logs, not metrics

	if register := findShimRegister(); register != "" {
		env = append(env, "NODE_OPTIONS="+appendImport(os.Getenv("NODE_OPTIONS"), register))
	}
	return env
}

// findShimRegister walks up from the cwd looking for the installed @rewynd/shim entrypoint; it
// returns "" (not an error) when absent, since non-Node commands don't need it.
func findShimRegister() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		for _, rel := range []string{"src/register.mjs", "dist/register.mjs"} {
			cand := filepath.Join(dir, "node_modules", "@rewynd", "shim", rel)
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ensureDaemon starts the core in the background if it isn't already listening.
func ensureDaemon() error {
	if dialOK(config.DefaultOTLPAddr) {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(self, "serve")
	detachSysProcAttr(c) // platform-specific: own session/process group so the core outlives us
	if err := c.Start(); err != nil {
		return fmt.Errorf("start core: %w", err)
	}
	for i := 0; i < 30; i++ {
		if dialOK(config.DefaultOTLPAddr) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("rewynd core did not become ready")
}

func appendImport(existing, registerPath string) string {
	imp := "--import file://" + registerPath
	if existing == "" {
		return imp
	}
	return existing + " " + imp
}
