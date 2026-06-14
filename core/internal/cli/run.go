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

	"github.com/SrinjoyDev/rewynd/internal/config"
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
			register, err := findShimRegister()
			if err != nil {
				return err
			}
			if err := ensureDaemon(); err != nil {
				return err
			}

			child := exec.Command(args[0], args[1:]...)
			child.Env = append(os.Environ(), "NODE_OPTIONS="+appendImport(os.Getenv("NODE_OPTIONS"), register))
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
			err = child.Wait()
			signal.Stop(sigs)
			if exit, ok := err.(*exec.ExitError); ok {
				os.Exit(exit.ExitCode())
			}
			return err
		},
	}
}

// findShimRegister walks up from the cwd looking for the installed @rewynd/shim entrypoint.
func findShimRegister() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		for _, rel := range []string{"src/register.mjs", "dist/register.mjs"} {
			cand := filepath.Join(dir, "node_modules", "@rewynd", "shim", rel)
			if _, err := os.Stat(cand); err == nil {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find @rewynd/shim — run `npm i -D @rewynd/shim` in your project")
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
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
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
