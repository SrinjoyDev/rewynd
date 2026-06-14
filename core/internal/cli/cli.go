// Package cli builds rewynd's command-line surface. Every command is a thin read/write client
// over the store; the JSON output is a versioned contract that agents depend on.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/SrinjoyDev/rewynd/internal/config"
	"github.com/SrinjoyDev/rewynd/internal/daemon"
	"github.com/SrinjoyDev/rewynd/internal/diag"
	"github.com/SrinjoyDev/rewynd/internal/mcp"
	"github.com/SrinjoyDev/rewynd/internal/model"
	"github.com/SrinjoyDev/rewynd/internal/store"
	"github.com/SrinjoyDev/rewynd/internal/tui"
)

func Execute(version string) error {
	return newRoot(version).Execute()
}

func newRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "rewynd",
		Short:         "A zero-config, OTLP-native flight recorder for your backend.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		versionCmd(version),
		runCmd(),
		tuiCmd(),
		serveCmd(),
		statusCmd(),
		lsCmd(),
		showCmd(),
		diagnoseCmd(),
		lastErrorCmd(),
		tailCmd(),
		mcpCmd(version),
		clearCmd(),
		watchCmd(),
	)
	return root
}

func versionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the rewynd version",
		Run:   func(_ *cobra.Command, _ []string) { fmt.Println("rewynd", version) },
	}
}

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the rewynd core (OTLP receiver + store)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			addr, _ := cmd.Flags().GetString("addr")
			if addr == "" {
				addr = config.DefaultOTLPAddr
			}
			fmt.Fprintf(os.Stderr, "rewynd core listening on %s (OTLP/HTTP) — db %s\n", addr, config.DBPath())
			return daemon.Run(ctx, daemon.Options{Addr: addr})
		},
	}
	cmd.Flags().String("addr", "", "OTLP listen address (default "+config.DefaultOTLPAddr+")")
	return cmd
}

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Is the core running, and how many requests are buffered",
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			running := dialOK(config.DefaultOTLPAddr)
			count := 0
			if st, err := openStore(); err == nil {
				count, _ = st.Count()
				st.Close()
			}
			if asJSON {
				return printJSON(map[string]any{"running": running, "addr": config.DefaultOTLPAddr, "requests": count})
			}
			state := "not running"
			if running {
				state = "running"
			}
			fmt.Printf("core: %s (%s)\nrequests buffered: %d\n", state, config.DefaultOTLPAddr, count)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func lsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List recorded requests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := listOptsFromFlags(cmd)
			asJSON, _ := cmd.Flags().GetBool("json")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			reqs, err := st.ListRequests(opts)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(reqs)
			}
			printRequestTable(reqs)
			return nil
		},
	}
	addListFlags(cmd)
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func showCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show the full correlated trace for one request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			req, err := st.GetRequest(args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(req)
			}
			printRequestDetail(req)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func clearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Wipe the buffer (clean slate before a test)",
		RunE: func(_ *cobra.Command, _ []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "cleared")
			return nil
		},
	}
}

func watchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Block until a matching request is recorded, then print it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := listOptsFromFlags(cmd)
			opts.Limit = 50
			asJSON, _ := cmd.Flags().GetBool("json")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			deadline := time.Now().Add(timeout)
			for {
				reqs, err := st.ListRequests(opts)
				if err != nil {
					return err
				}
				if len(reqs) > 0 {
					full, err := st.GetRequest(reqs[0].ID) // newest match (ordered by started_at desc)
					if err != nil {
						return err
					}
					if asJSON {
						return printJSON(full)
					}
					printRequestDetail(full)
					return nil
				}
				if !time.Now().Before(deadline) {
					return fmt.Errorf("timed out after %s with no matching request", timeout)
				}
				time.Sleep(150 * time.Millisecond)
			}
		},
	}
	addListFlags(cmd)
	cmd.Flags().Bool("json", false, "machine-readable output")
	cmd.Flags().Duration("timeout", 10*time.Second, "how long to wait")
	return cmd
}

func diagnoseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnose <id>",
		Short: "Summarize what's wrong with a request (for humans and agents)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			req, err := st.GetRequest(args[0])
			if err != nil {
				return err
			}
			problems := diag.Diagnose(req)
			if asJSON {
				return printJSON(map[string]any{
					"request_id": req.ID, "status_code": req.StatusCode, "problems": problems,
				})
			}
			fmt.Printf("%s %s  ->  %d  (%s)\n", req.Method, req.Path, req.StatusCode, dur(req.DurationMs))
			if len(problems) == 0 {
				fmt.Println("no problems detected")
				return nil
			}
			fmt.Println("problems:")
			for _, p := range problems {
				fmt.Printf("  • %s\n", p.Summary)
				if p.Suggestion != "" {
					fmt.Printf("    ↳ %s\n", p.Suggestion)
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func lastErrorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "last-error",
		Short: "Show the most recent 5xx in full",
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			reqs, err := st.ListRequests(store.ListOptions{StatusClass: "5xx", Limit: 1})
			if err != nil {
				return err
			}
			if len(reqs) == 0 {
				return fmt.Errorf("no 5xx requests recorded")
			}
			full, err := st.GetRequest(reqs[0].ID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(full)
			}
			printRequestDetail(full)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func tailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream requests as they arrive (non-blocking monitor)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := listOptsFromFlags(cmd)
			opts.Limit = 50
			asJSON, _ := cmd.Flags().GetBool("json")
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			seen := map[string]bool{}
			if existing, _ := st.ListRequests(opts); existing != nil {
				for _, r := range existing {
					seen[r.ID] = true
				}
			}
			for {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				reqs, _ := st.ListRequests(opts)
				for i := len(reqs) - 1; i >= 0; i-- { // oldest-first among the page
					r := reqs[i]
					if seen[r.ID] {
						continue
					}
					seen[r.ID] = true
					if asJSON {
						printJSONLine(r)
					} else {
						fmt.Printf("%s  %-6s %-26s %d  %7s  %s\n",
							shortID(r.ID), r.Method, r.Path, r.StatusCode, dur(r.DurationMs), flags(r))
					}
				}
				time.Sleep(200 * time.Millisecond)
			}
		},
	}
	addListFlags(cmd)
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the live terminal UI",
		RunE: func(_ *cobra.Command, _ []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			return tui.Run(st)
		},
	}
}

func mcpCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP server (stdio) so coding agents can introspect the backend",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			return mcp.RunStdio(ctx, st, version)
		},
	}
}

func addListFlags(cmd *cobra.Command) {
	cmd.Flags().String("status", "", "filter by class: 2xx|4xx|5xx")
	cmd.Flags().Bool("slow", false, "only slow requests")
	cmd.Flags().Float64("slow-ms", 500, "threshold for --slow")
	cmd.Flags().Bool("has-error", false, "only requests with an error")
	cmd.Flags().String("path", "", "filter by path substring")
	cmd.Flags().Int("last", 0, "limit to the last N")
}

func listOptsFromFlags(cmd *cobra.Command) store.ListOptions {
	status, _ := cmd.Flags().GetString("status")
	slow, _ := cmd.Flags().GetBool("slow")
	slowMs, _ := cmd.Flags().GetFloat64("slow-ms")
	hasErr, _ := cmd.Flags().GetBool("has-error")
	path, _ := cmd.Flags().GetString("path")
	last, _ := cmd.Flags().GetInt("last")
	return store.ListOptions{
		StatusClass: status, Slow: slow, SlowMs: slowMs,
		HasError: hasErr, PathLike: path, Limit: last,
	}
}

func openStore() (*store.Store, error) {
	if err := os.MkdirAll(config.DataDir(), 0o755); err != nil {
		return nil, err
	}
	return store.Open(config.DBPath())
}

func dialOK(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printJSONLine(v any) {
	if b, err := json.Marshal(v); err == nil {
		os.Stdout.Write(append(b, '\n'))
	}
}

func printRequestTable(reqs []model.Request) {
	if len(reqs) == 0 {
		fmt.Fprintln(os.Stderr, "no requests recorded")
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tMETHOD\tPATH\tSTATUS\tDURATION\tQUERIES\tFLAGS")
	for _, r := range reqs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%d\t%s\n",
			shortID(r.ID), r.Method, r.Path, r.StatusCode,
			dur(r.DurationMs), r.Counts.Queries, flags(r))
	}
	tw.Flush()
}

func printRequestDetail(r *model.Request) {
	fmt.Printf("%s %s  ->  %d  (%s)\n", r.Method, r.Path, r.StatusCode, dur(r.DurationMs))
	fmt.Printf("id %s   trace %s\n", shortID(r.ID), r.TraceID)
	if r.Request != nil && len(r.Request.Headers) > 0 {
		fmt.Println("\nREQUEST HEADERS")
		keys := make([]string, 0, len(r.Request.Headers))
		for k := range r.Request.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, oneLine(r.Request.Headers[k]))
		}
	}
	if len(r.Detections) > 0 {
		fmt.Println("\nDETECTIONS")
		for _, d := range r.Detections {
			fmt.Printf("  ! %s — %s\n", d.Type, d.Title)
		}
	}
	if len(r.Queries) > 0 {
		fmt.Printf("\nQUERIES (%d)\n", len(r.Queries))
		for _, q := range r.Queries {
			fmt.Printf("  %7s  %s\n", dur(q.DurationMs), oneLine(q.Statement))
		}
	}
	if len(r.Outbound) > 0 {
		fmt.Printf("\nOUTBOUND (%d)\n", len(r.Outbound))
		for _, o := range r.Outbound {
			fmt.Printf("  %7s  %s %s -> %d\n", dur(o.DurationMs), o.Method, o.URL, o.StatusCode)
		}
	}
	if len(r.Logs) > 0 {
		fmt.Printf("\nLOGS (%d)\n", len(r.Logs))
		for _, l := range r.Logs {
			fmt.Printf("  [%s] %s\n", l.Level, oneLine(l.Message))
		}
	}
	if len(r.Exceptions) > 0 {
		fmt.Printf("\nEXCEPTIONS (%d)\n", len(r.Exceptions))
		for _, e := range r.Exceptions {
			fmt.Printf("  %s: %s\n", e.Type, oneLine(e.Message))
		}
	}
}

func flags(r model.Request) string {
	var f []string
	if r.Error || r.StatusCode >= 500 {
		f = append(f, "error")
	}
	for _, d := range r.Detections {
		if d.Type == model.DetectNPlusOne {
			f = append(f, "N+1")
		}
	}
	return strings.Join(f, ",")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func dur(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return fmt.Sprintf("%.0fms", ms)
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 100 {
		return s[:100] + "…"
	}
	return s
}
