// Package mcp exposes rewynd's recording to coding agents as MCP tools over stdio. The tools
// mirror the CLI and read the same store, so an agent gets the same correlated traces a human does.
package mcp

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rewyndhq/rewynd/internal/diag"
	"github.com/rewyndhq/rewynd/internal/model"
	"github.com/rewyndhq/rewynd/internal/store"
)

func RunStdio(ctx context.Context, st *store.Store, version string) error {
	s := sdk.NewServer(&sdk.Implementation{Name: "rewynd", Version: version}, nil)
	h := &handlers{st: st}

	sdk.AddTool(s, &sdk.Tool{Name: "list_requests",
		Description: "List recorded backend requests (newest first), with optional filters."}, h.listRequests)
	sdk.AddTool(s, &sdk.Tool{Name: "get_request",
		Description: "Get the full correlated trace for one request id: queries, outbound calls, logs, exception, and detections."}, h.getRequest)
	sdk.AddTool(s, &sdk.Tool{Name: "wait_for_request",
		Description: "Block until a request matching the filters is recorded, then return it. Use right after triggering an endpoint."}, h.waitForRequest)
	sdk.AddTool(s, &sdk.Tool{Name: "diagnose",
		Description: "Summarize what's wrong with a request (N+1, exceptions, slow queries) with a fix suggestion."}, h.diagnose)
	sdk.AddTool(s, &sdk.Tool{Name: "get_last_error",
		Description: "Return the most recent 5xx request in full."}, h.getLastError)
	sdk.AddTool(s, &sdk.Tool{Name: "clear",
		Description: "Wipe the buffer for a clean slate before triggering a test."}, h.clear)

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
	RequestID  string        `json:"request_id"`
	StatusCode int           `json:"status_code"`
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
