// Package mcp exposes rewynd's recording to coding agents as MCP tools over stdio. The tools
// mirror the CLI and read the same store, so an agent gets the same correlated traces a human does.
package mcp

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SrinjoyDev/rewynd/core/internal/config"
	"github.com/SrinjoyDev/rewynd/core/internal/diag"
	"github.com/SrinjoyDev/rewynd/core/internal/model"
	"github.com/SrinjoyDev/rewynd/core/internal/stats"
	"github.com/SrinjoyDev/rewynd/core/internal/store"
)

// instructions are sent to the client on connect. They teach an agent the whole debugging
// loop once, so it reaches for rewynd instead of adding print statements or guessing.
const instructions = `rewynd is a local flight recorder for the backend you are working on. It has already
captured the HTTP requests this app served during development — and, correlated to each one,
the exact SQL (with params), outbound calls, log lines, and exceptions that request caused.
Read what actually happened instead of speculating, re-running blindly, or adding console.log.

The debugging loop you should run:
  1. clear              wipe the buffer so the next request is the only thing recorded
  2. (trigger)          you or the user hits the endpoint — a curl, a test, a UI click
  3. wait_for_request   block until that request lands; it returns the full correlated trace
  4. diagnose / get_request   read the failing SQL+params, the exception+stack, any N+1
  5. fix the code, then repeat from step 1 to confirm the request is now green

How to work with it well:
- Start a session with get_stats: it tells you what is already recorded and what is broken,
  so you can go straight to the offending request instead of paging through everything.
- To find failures fast, list_requests with status:"5xx" or has_error:true, or call
  get_last_error for the most recent 5xx in full.
- Anywhere an id is asked for, the first 8 characters of the request id are enough.
- Detections (N+1, slow query, slow request) and diagnose are deterministic — computed from
  the real recorded trace, not inferred. Trust them over guesses. If diagnose returns no
  problems, that request is clean; look elsewhere rather than inventing a cause.
- Everything is local to this machine and read-only. Calling these tools is free and has no
  side effects (except clear, which only wipes rewynd's own buffer — never your data).`

func RunStdio(ctx context.Context, st *store.Store, version string) error {
	s := sdk.NewServer(
		&sdk.Implementation{Name: "rewynd", Version: version},
		&sdk.ServerOptions{Instructions: instructions},
	)
	h := &handlers{st: st}

	sdk.AddTool(s, &sdk.Tool{Name: "get_stats",
		Description: "Get an at-a-glance overview of everything rewynd has recorded: total requests, " +
			"how many are 2xx/3xx vs 4xx vs 5xx, how many threw an exception or have an N+1, and a " +
			"short list of the broken/flagged requests (id, method, path, status, what's wrong) newest " +
			"first. Call this first to orient yourself, then jump straight to diagnose or get_request " +
			"on the ids it surfaces. Takes no arguments."}, h.getStats)
	sdk.AddTool(s, &sdk.Tool{Name: "get_load_stats",
		Description: "Get a load/performance summary over the recorded window: throughput (req/s), latency " +
			"percentiles (p50/p95/p99/max), error rate, and a per-endpoint breakdown ranked worst-first " +
			"(error rate, p95, count, N+1 flag). Use this to answer \"how is it performing\", \"which endpoint " +
			"is slow or erroring\", or to compare before/after a change. Takes no arguments."}, h.getLoadStats)
	sdk.AddTool(s, &sdk.Tool{Name: "list_requests",
		Description: "List recorded backend requests, newest first, each with method, path, status, " +
			"duration, and per-request counts (queries / outbound / logs / exceptions). Filter to narrow " +
			"in: status (\"2xx\"|\"4xx\"|\"5xx\"), path substring, slow (only slow requests), has_error " +
			"(only requests that errored), and last (limit to the most recent N). Returns summaries only — " +
			"call get_request for one request's full trace."}, h.listRequests)
	sdk.AddTool(s, &sdk.Tool{Name: "get_request",
		Description: "Get the complete correlated trace for one request id (full or first-8-chars prefix): " +
			"the request/response headers and body, the timed span waterfall, every SQL statement with its " +
			"params and duration, outbound HTTP calls, the correlated log lines, the exception with its " +
			"stack, and any detections. This is the ground truth for what the endpoint actually did."}, h.getRequest)
	sdk.AddTool(s, &sdk.Tool{Name: "wait_for_request",
		Description: "Block until a request matching the filters is recorded, then return its full trace. " +
			"Use this immediately after triggering an endpoint so you capture exactly that request without " +
			"polling: clear, trigger the endpoint, then wait_for_request with the path or status:\"5xx\" you " +
			"expect. Filters: status, path, timeout_seconds (default 10). Errors if nothing matches in time."}, h.waitForRequest)
	sdk.AddTool(s, &sdk.Tool{Name: "diagnose",
		Description: "Summarize what is wrong with one request id as a short, ordered list of problems — " +
			"N+1 queries, exceptions (type + message + first stack frame), and slow queries — each with a " +
			"concrete fix suggestion. Deterministic, computed from the recorded trace. Use it to get the " +
			"\"what's broken and why\" in one call before reading the full request. Empty list means clean."}, h.diagnose)
	sdk.AddTool(s, &sdk.Tool{Name: "get_last_error",
		Description: "Return the most recent 5xx request in full (same shape as get_request). The fastest " +
			"way to answer \"the endpoint just 500'd — what happened?\": it surfaces the failing SQL, the " +
			"exception and stack, and any detections without you needing an id."}, h.getLastError)
	sdk.AddTool(s, &sdk.Tool{Name: "clear",
		Description: "Wipe rewynd's recording buffer for a clean slate before triggering a test, so the next " +
			"request is unambiguous. Affects only rewynd's own buffer — never your database or app state. " +
			"Standard first step of the debugging loop."}, h.clear)

	return s.Run(ctx, &sdk.StdioTransport{})
}

type handlers struct{ st *store.Store }

type listInput struct {
	Status   string `json:"status,omitempty" jsonschema:"filter by class: 2xx, 4xx, or 5xx"`
	Path     string `json:"path,omitempty" jsonschema:"filter by path substring"`
	Slow     bool   `json:"slow,omitempty" jsonschema:"only slow requests"`
	HasError bool   `json:"has_error,omitempty" jsonschema:"only requests with an error"`
	Last     int    `json:"last,omitempty" jsonschema:"limit to the last N"`
}
type listOutput struct {
	Requests []model.Request `json:"requests"`
}

func (h *handlers) listRequests(_ context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, listOutput, error) {
	reqs, err := h.st.ListRequests(store.ListOptions{
		StatusClass: in.Status, PathLike: in.Path, Slow: in.Slow, HasError: in.HasError, Limit: in.Last,
	})
	return nil, listOutput{Requests: reqs}, err
}

type idInput struct {
	ID string `json:"id" jsonschema:"the request id (full or unambiguous prefix)"`
}
type requestOutput struct {
	Request *model.Request `json:"request"`
}

func (h *handlers) getRequest(_ context.Context, _ *sdk.CallToolRequest, in idInput) (*sdk.CallToolResult, requestOutput, error) {
	r, err := h.st.GetRequest(in.ID)
	return nil, requestOutput{Request: r}, err
}

type waitInput struct {
	Status         string `json:"status,omitempty" jsonschema:"filter by class: 2xx, 4xx, or 5xx"`
	Path           string `json:"path,omitempty" jsonschema:"filter by path substring"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"how long to wait (default 10)"`
}

func (h *handlers) waitForRequest(ctx context.Context, _ *sdk.CallToolRequest, in waitInput) (*sdk.CallToolResult, requestOutput, error) {
	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts := store.ListOptions{StatusClass: in.Status, PathLike: in.Path, Limit: 50}
	deadline := time.Now().Add(timeout)
	for {
		reqs, err := h.st.ListRequests(opts)
		if err != nil {
			return nil, requestOutput{}, err
		}
		if len(reqs) > 0 {
			full, err := h.st.GetRequest(reqs[0].ID)
			return nil, requestOutput{Request: full}, err
		}
		if !time.Now().Before(deadline) {
			return nil, requestOutput{}, fmt.Errorf("timed out after %s with no matching request", timeout)
		}
		select {
		case <-ctx.Done():
			return nil, requestOutput{}, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

type diagOutput struct {
	RequestID  string         `json:"request_id"`
	StatusCode int            `json:"status_code"`
	Problems   []diag.Problem `json:"problems"`
}

func (h *handlers) diagnose(_ context.Context, _ *sdk.CallToolRequest, in idInput) (*sdk.CallToolResult, diagOutput, error) {
	r, err := h.st.GetRequest(in.ID)
	if err != nil {
		return nil, diagOutput{}, err
	}
	return nil, diagOutput{RequestID: r.ID, StatusCode: r.StatusCode, Problems: diag.Diagnose(r)}, nil
}

type emptyInput struct{}

func (h *handlers) getLastError(_ context.Context, _ *sdk.CallToolRequest, _ emptyInput) (*sdk.CallToolResult, requestOutput, error) {
	reqs, err := h.st.ListRequests(store.ListOptions{StatusClass: "5xx", Limit: 1})
	if err != nil {
		return nil, requestOutput{}, err
	}
	if len(reqs) == 0 {
		return nil, requestOutput{}, fmt.Errorf("no 5xx requests recorded")
	}
	full, err := h.st.GetRequest(reqs[0].ID)
	return nil, requestOutput{Request: full}, err
}

type clearOutput struct {
	Cleared bool `json:"cleared"`
}

func (h *handlers) clear(_ context.Context, _ *sdk.CallToolRequest, _ emptyInput) (*sdk.CallToolResult, clearOutput, error) {
	if err := h.st.Clear(); err != nil {
		return nil, clearOutput{}, err
	}
	return nil, clearOutput{Cleared: true}, nil
}

func (h *handlers) getLoadStats(_ context.Context, _ *sdk.CallToolRequest, _ emptyInput) (*sdk.CallToolResult, stats.Stats, error) {
	reqs, err := h.st.ListRequests(store.ListOptions{Limit: config.MaxRequests()})
	if err != nil {
		return nil, stats.Stats{}, err
	}
	return nil, stats.Compute(reqs), nil
}

// problemRef is a one-line pointer to a broken or flagged request in the stats overview.
type problemRef struct {
	ID         string  `json:"id"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	StatusCode int     `json:"status_code"`
	DurationMs float64 `json:"duration_ms"`
	Issue      string  `json:"issue"`
}

type statsOutput struct {
	Total         int          `json:"total"`
	OK            int          `json:"ok_2xx_3xx"`
	ClientErrors  int          `json:"client_errors_4xx"`
	ServerErrors  int          `json:"server_errors_5xx"`
	FailedJobs    int          `json:"failed_jobs"`
	WithException int          `json:"with_exception"`
	WithNPlusOne  int          `json:"with_n_plus_one"`
	Slow          int          `json:"slow"`
	Problems      []problemRef `json:"problems"`
	Hint          string       `json:"hint"`
}

// slowRequestMs mirrors detect.DefaultSlowRequestMs for the stats summary.
const slowRequestMs = 1000

func (h *handlers) getStats(_ context.Context, _ *sdk.CallToolRequest, _ emptyInput) (*sdk.CallToolResult, statsOutput, error) {
	total, err := h.st.Count()
	if err != nil {
		return nil, statsOutput{}, err
	}
	// Sample the whole ring buffer for the breakdown; Count() above is the authoritative total.
	reqs, err := h.st.ListRequests(store.ListOptions{Limit: config.MaxRequests()})
	if err != nil {
		return nil, statsOutput{}, err
	}
	out := statsOutput{Total: total}
	for i := range reqs {
		r := &reqs[i]
		switch {
		case r.Kind == model.KindJob:
			if r.Error {
				out.FailedJobs++
			} else {
				out.OK++
			}
		case r.StatusCode >= 500:
			out.ServerErrors++
		case r.StatusCode >= 400:
			out.ClientErrors++
		case r.StatusCode >= 200:
			out.OK++
		}
		hasExc := r.Counts.Exceptions > 0
		hasNPlusOne := false
		for _, d := range r.Detections {
			if d.Type == model.DetectNPlusOne {
				hasNPlusOne = true
			}
		}
		isSlow := r.DurationMs >= slowRequestMs
		if hasExc {
			out.WithException++
		}
		if hasNPlusOne {
			out.WithNPlusOne++
		}
		if isSlow {
			out.Slow++
		}
		if issue := issueLabel(r, hasExc, hasNPlusOne, isSlow); issue != "" && len(out.Problems) < 25 {
			out.Problems = append(out.Problems, problemRef{
				ID: short(r.ID), Method: r.Method, Path: r.Path,
				StatusCode: r.StatusCode, DurationMs: r.DurationMs, Issue: issue,
			})
		}
	}
	out.Hint = statsHint(out)
	return nil, out, nil
}

// issueLabel names what's wrong with a request for the stats list, or "" if it looks clean.
func issueLabel(r *model.Request, hasExc, hasNPlusOne, isSlow bool) string {
	switch {
	case r.Kind == model.KindJob && r.Error:
		return "job failed"
	case r.StatusCode >= 500 && hasExc:
		return fmt.Sprintf("%d + exception", r.StatusCode)
	case r.StatusCode >= 500:
		return fmt.Sprintf("%d server error", r.StatusCode)
	case hasExc:
		return "exception"
	case hasNPlusOne:
		return fmt.Sprintf("N+1 (%d queries)", r.Counts.Queries)
	case isSlow:
		return fmt.Sprintf("slow %.0fms", r.DurationMs)
	default:
		return ""
	}
}

func statsHint(o statsOutput) string {
	switch {
	case o.Total == 0:
		return "Nothing recorded yet. Run the app under `rewynd run <cmd>`, trigger an endpoint, then call wait_for_request."
	case o.ServerErrors > 0:
		return "There are server errors. Call get_last_error, or diagnose the 5xx ids listed in problems."
	case o.FailedJobs > 0:
		return "A background job/consumer failed. Diagnose the 'job failed' ids listed in problems."
	case len(o.Problems) > 0:
		return "No 5xx, but some requests are flagged (N+1 / slow / exception). Call diagnose on the problem ids."
	default:
		return "Everything recorded looks clean. If you expect a failure, clear and re-trigger the endpoint."
	}
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
