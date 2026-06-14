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
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/SrinjoyDev/rewynd/core/internal/config"
	"github.com/SrinjoyDev/rewynd/core/internal/daemon"
	"github.com/SrinjoyDev/rewynd/core/internal/diag"
	"github.com/SrinjoyDev/rewynd/core/internal/export"
	"github.com/SrinjoyDev/rewynd/core/internal/mcp"
	"github.com/SrinjoyDev/rewynd/core/internal/model"
	"github.com/SrinjoyDev/rewynd/core/internal/stats"
	"github.com/SrinjoyDev/rewynd/core/internal/store"
	"github.com/SrinjoyDev/rewynd/core/internal/tui"
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
		statsCmd(),
		exportCmd(),
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
			grpcAddr, _ := cmd.Flags().GetString("grpc-addr")
			if addr == "" {
				addr = config.DefaultOTLPAddr
			}
			if grpcAddr == "" {
				grpcAddr = config.DefaultOTLPGRPCAddr
			}
			fmt.Fprintf(os.Stderr, "rewynd core listening on %s (OTLP/HTTP) and %s (OTLP/gRPC) — db %s\n", addr, grpcAddr, config.DBPath())
			return daemon.Run(ctx, daemon.Options{Addr: addr, GRPCAddr: grpcAddr})
		},
	}
	cmd.Flags().String("addr", "", "OTLP/HTTP listen address (default "+config.DefaultOTLPAddr+")")
	cmd.Flags().String("grpc-addr", "", "OTLP/gRPC listen address (default "+config.DefaultOTLPGRPCAddr+")")
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

func statsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Load/performance summary: throughput, latency percentiles, error rate, by endpoint",
		Long: "Load/performance summary over the recorded window.\n\n" +
			"Compare runs to see if a fix helped:\n" +
			"  rewynd stats --save before     # snapshot the current numbers\n" +
			"  # ... change code, re-run the load ...\n" +
			"  rewynd stats --baseline before # show the delta (p95, error rate, by endpoint)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			last, _ := cmd.Flags().GetInt("last")
			save, _ := cmd.Flags().GetString("save")
			baseline, _ := cmd.Flags().GetString("baseline")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			limit := last
			if limit <= 0 {
				limit = config.MaxRequests()
			}
			reqs, err := st.ListRequests(store.ListOptions{Limit: limit})
			if err != nil {
				return err
			}
			s := stats.Compute(reqs)

			if baseline != "" {
				base, err := loadSnapshot(baseline)
				if err != nil {
					return err
				}
				diff := stats.Compare(base, s)
				if asJSON {
					return printJSON(diff)
				}
				printStatsDiff(baseline, diff)
				return nil
			}

			if asJSON {
				if err := printJSON(s); err != nil {
					return err
				}
			} else {
				printStats(s)
			}
			if save != "" {
				if err := saveSnapshot(save, s); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "saved snapshot %q\n", save)
			}
			return nil
		},
	}
	cmd.Flags().Int("last", 0, "summarize only the last N flows (default: the whole buffer)")
	cmd.Flags().String("save", "", "save this summary as a named baseline to compare against later")
	cmd.Flags().String("baseline", "", "show the delta against a previously saved baseline")
	cmd.Flags().Bool("json", false, "machine-readable output")
	return cmd
}

func exportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Export one request's full trace as a self-contained HTML file (share it / attach to CI)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, _ := cmd.Flags().GetString("out")
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			req, err := st.GetRequest(args[0])
			if err != nil {
				return err
			}
			doc := export.HTML(req)
			if out == "" || out == "-" {
				fmt.Print(doc)
				return nil
			}
			if err := os.WriteFile(out, []byte(doc), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "wrote %s (%s %s)\n", out, req.Method, req.Path)
			return nil
		},
	}
	cmd.Flags().StringP("out", "o", "", "write to a file instead of stdout")
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
				fmt.Printf("  - %s\n", p.Summary)
				if p.Suggestion != "" {
					fmt.Printf("    -> %s\n", p.Suggestion)
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
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			shortID(r.ID), r.Method, r.Path, statusCell(r),
			dur(r.DurationMs), r.Counts.Queries, flags(r))
	}
	tw.Flush()
}

func printStats(s stats.Stats) {
	if s.Total == 0 {
		fmt.Fprintln(os.Stderr, "no flows recorded")
		return
	}
	window := ""
	if s.WindowMs > 0 {
		window = fmt.Sprintf(" over %s — %.1f req/s", dur(s.WindowMs), s.ReqPerSec)
	}
	fmt.Printf("%d flows%s\n", s.Total, window)
	fmt.Printf("latency   p50 %s   p95 %s   p99 %s   max %s\n",
		dur(s.Latency.P50), dur(s.Latency.P95), dur(s.Latency.P99), dur(s.Latency.Max))
	fmt.Printf("errors    %.1f%% (%d)   5xx %d   4xx %d   failed jobs %d\n",
		s.ErrorRate*100, s.Errors, s.ServerErrors, s.ClientErrors, s.FailedJobs)
	fmt.Printf("issues    N+1 in %d   slow %d\n", s.NPlusOne, s.Slow)

	if len(s.Endpoints) == 0 {
		return
	}
	fmt.Println("\nBY ENDPOINT (worst first)")
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ERRORS\tP95\tMAX\tCOUNT\tENDPOINT")
	for i, e := range s.Endpoints {
		if i >= 15 {
			fmt.Fprintf(tw, "\t\t\t\t… +%d more\n", len(s.Endpoints)-i)
			break
		}
		flag := ""
		if e.NPlusOne {
			flag = "  [N+1]"
		}
		fmt.Fprintf(tw, "%.0f%%\t%s\t%s\t%d\t%s %s%s\n",
			e.ErrorRate*100, dur(e.P95Ms), dur(e.MaxMs), e.Count, e.Method, e.Route, flag)
	}
	tw.Flush()
}

func snapshotPath(label string) string {
	return filepath.Join(config.DataDir(), "snapshots", label+".json")
}

func saveSnapshot(label string, s stats.Stats) error {
	p := snapshotPath(label)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func loadSnapshot(label string) (stats.Stats, error) {
	var s stats.Stats
	b, err := os.ReadFile(snapshotPath(label))
	if err != nil {
		return s, fmt.Errorf("no baseline %q — save one first with `rewynd stats --save %s`", label, label)
	}
	return s, json.Unmarshal(b, &s)
}

// printStatsDiff renders the change from a saved baseline to the current run — the
// "did my fix help" view.
func printStatsDiff(label string, d stats.Diff) {
	fmt.Printf("baseline %q (%d flows)  ->  current (%d flows)\n", label, d.Base.Total, d.Cur.Total)
	fmt.Printf("latency   p50 %s   p95 %s   p99 %s\n",
		deltaDur(d.Base.Latency.P50, d.Cur.Latency.P50),
		deltaDur(d.Base.Latency.P95, d.Cur.Latency.P95),
		deltaDur(d.Base.Latency.P99, d.Cur.Latency.P99))
	fmt.Printf("errors    %.1f%% -> %.1f%% (%+.1fpp)\n", d.Base.ErrorRate*100, d.Cur.ErrorRate*100, (d.Cur.ErrorRate-d.Base.ErrorRate)*100)
	fmt.Printf("throughput %.1f -> %.1f req/s\n", d.Base.ReqPerSec, d.Cur.ReqPerSec)

	if len(d.Endpoints) == 0 {
		return
	}
	fmt.Println("\nBY ENDPOINT")
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, e := range d.Endpoints {
		name := e.Method + " " + e.Route
		switch {
		case e.New:
			fmt.Fprintf(tw, "%s\tnew\tp95 %s\terrors %.0f%%\n", name, dur(e.CurP95), e.CurErrRate*100)
		case e.Gone:
			fmt.Fprintf(tw, "%s\tgone\t\t\n", name)
		default:
			fmt.Fprintf(tw, "%s\t\tp95 %s\terrors %.0f%% -> %.0f%%\n",
				name, deltaDur(e.BaseP95, e.CurP95), e.BaseErrRate*100, e.CurErrRate*100)
		}
	}
	tw.Flush()
}

// deltaDur renders "340ms -> 120ms (-65%)" for a before/after duration.
func deltaDur(base, cur float64) string {
	s := dur(base) + " -> " + dur(cur)
	if base > 0 {
		s += fmt.Sprintf(" (%+.0f%%)", (cur-base)/base*100)
	}
	return s
}

// statusCell renders the status column for both flows: an HTTP code, or ok/fail for a job
// (which has no status code).
func statusCell(r model.Request) string {
	if r.Kind == model.KindJob {
		if r.Error {
			return "fail"
		}
		return "ok"
	}
	return fmt.Sprintf("%d", r.StatusCode)
}

func printRequestDetail(r *model.Request) {
	if r.Kind == model.KindJob {
		outcome := "ok"
		if r.Error {
			outcome = "fail"
		}
		fmt.Printf("JOB %s %s  ->  %s  (%s)\n", r.Method, r.Path, outcome, dur(r.DurationMs))
	} else {
		fmt.Printf("%s %s  ->  %d  (%s)\n", r.Method, r.Path, r.StatusCode, dur(r.DurationMs))
	}
	fmt.Printf("id %s   trace %s\n", shortID(r.ID), r.TraceID)
	svcs := requestServices(r)
	multi := len(svcs) > 1
	if multi {
		fmt.Printf("services %s\n", strings.Join(svcs, " -> "))
	}
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
	if r.Request != nil && r.Request.Body != "" {
		fmt.Println("\nREQUEST BODY")
		fmt.Printf("  %s\n", oneLine(r.Request.Body))
	}
	if len(r.Detections) > 0 {
		fmt.Println("\nDETECTIONS")
		for _, d := range r.Detections {
			fmt.Printf("  ! %s — %s\n", d.Type, d.Title)
		}
	}
	if len(r.Queries) > 0 {
		fmt.Printf("\nQUERIES (%d)\n", len(r.Queries))
		collapse(r.Queries,
			func(q model.Query) string { return q.Service + "\x00" + normStmt(q) },
			func(start, n int) {
				q := r.Queries[start]
				if n == 1 {
					fmt.Printf("  %7s  %s%s\n", dur(q.DurationMs), oneLine(q.Statement), svcSuffix(q.Service, multi))
					return
				}
				total := sumDur(r.Queries[start:start+n], func(q model.Query) float64 { return q.DurationMs })
				fmt.Printf("  %7s  %s   ×%d (avg %s)%s\n", dur(total), oneLine(normStmt(q)), n, dur(total/float64(n)), svcSuffix(q.Service, multi))
			})
	}
	if len(r.Outbound) > 0 {
		fmt.Printf("\nOUTBOUND (%d)\n", len(r.Outbound))
		collapse(r.Outbound,
			func(o model.Outbound) string { return fmt.Sprintf("%s\x00%s %s\x00%d", o.Service, o.Method, o.URL, o.StatusCode) },
			func(start, n int) {
				o := r.Outbound[start]
				if n == 1 {
					fmt.Printf("  %7s  %s %s -> %d%s\n", dur(o.DurationMs), o.Method, o.URL, o.StatusCode, svcSuffix(o.Service, multi))
					return
				}
				total := sumDur(r.Outbound[start:start+n], func(o model.Outbound) float64 { return o.DurationMs })
				fmt.Printf("  %7s  %s %s -> %d   ×%d (avg %s)%s\n", dur(total), o.Method, o.URL, o.StatusCode, n, dur(total/float64(n)), svcSuffix(o.Service, multi))
			})
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
			if e.Type != "" {
				fmt.Printf("  %s: %s\n", e.Type, oneLine(e.Message))
			} else {
				fmt.Printf("  %s\n", oneLine(e.Message))
			}
		}
	}
}

// requestServices lists the services in a request's spans, entry-first (spans are start-ordered).
func requestServices(r *model.Request) []string {
	seen := map[string]bool{}
	var out []string
	for _, sp := range r.Spans {
		if sp.Service != "" && !seen[sp.Service] {
			seen[sp.Service] = true
			out = append(out, sp.Service)
		}
	}
	return out
}

func svcSuffix(service string, multi bool) string {
	if multi && service != "" {
		return "  [" + service + "]"
	}
	return ""
}

// collapse walks xs in order and invokes emit once per maximal run of consecutive items that
// share key(item), passing the run's start index and length. It lets `show` fold an N+1's
// repeated queries — or a loop's repeated outbound calls — into one "×N" line while keeping
// every distinct step in its original place, so a 200-query request stays readable.
func collapse[T any](xs []T, key func(T) string, emit func(start, n int)) {
	for i := 0; i < len(xs); {
		j := i + 1
		for j < len(xs) && key(xs[j]) == key(xs[i]) {
			j++
		}
		emit(i, j-i)
		i = j
	}
}

// normStmt is the params-stripped statement (the N+1 group key); falls back to the raw text.
func normStmt(q model.Query) string {
	if q.StatementNormalized != "" {
		return q.StatementNormalized
	}
	return q.Statement
}

func sumDur[T any](xs []T, f func(T) float64) float64 {
	var s float64
	for _, x := range xs {
		s += f(x)
	}
	return s
}

func flags(r model.Request) string {
	var f []string
	if r.Kind == model.KindJob {
		f = append(f, "job")
	}
	if r.Error || r.StatusCode >= 500 {
		f = append(f, "error")
	}
	for _, d := range r.Detections {
		if d.Type == model.DetectNPlusOne {
			f = append(f, "N+1")
		}
	}
	if r.DurationMs >= 1000 {
		f = append(f, "slow")
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
		return s[:100] + "..."
	}
	return s
}
