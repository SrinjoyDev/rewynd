// Package diag turns a correlated request into a short list of problems, shared by the CLI's
// `diagnose` command and the MCP server's diagnose tool.
package diag

import (
	"strconv"
	"strings"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

type Problem struct {
	Type       string `json:"type"`
	Summary    string `json:"summary"`
	Suggestion string `json:"suggestion,omitempty"`
}

func Diagnose(r *model.Request) []Problem {
	var ps []Problem
	for _, d := range r.Detections {
		s := d.Summary
		if s == "" {
			s = d.Title
		}
		ps = append(ps, Problem{string(d.Type), s, d.Suggestion})
	}
	seen := map[string]bool{}
	for _, e := range r.Exceptions {
		key := e.Type + e.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		summary := oneLine(e.Message)
		if e.Type != "" {
			summary = e.Type + ": " + summary
		}
		// Show the top stack frame as the "where", but not when the stack is just the error
		// message repeated (DB drivers often carry no real frame).
		hint := firstLine(e.Stack)
		if strings.Contains(hint, oneLine(e.Message)) {
			hint = ""
		}
		ps = append(ps, Problem{"exception", summary, hint})
	}
	// A failed outbound call is often the real cause of a 5xx the local code never threw.
	for _, o := range r.Outbound {
		if !o.Error && o.StatusCode < 400 {
			continue
		}
		key := "out:" + o.Method + o.URL + strconv.Itoa(o.StatusCode)
		if seen[key] {
			continue
		}
		seen[key] = true
		ps = append(ps, Problem{"outbound_error", outboundSummary(o),
			"The upstream call failed — the handler likely surfaced this as the request's error."})
	}
	// A DB query that errored but didn't surface as an exception.
	for _, q := range r.Queries {
		if !q.Error {
			continue
		}
		key := "q:" + q.StatementNormalized
		if seen[key] {
			continue
		}
		seen[key] = true
		ps = append(ps, Problem{"query_error", "query failed: " + oneLine(q.Statement), ""})
	}
	return ps
}

func outboundSummary(o model.Outbound) string {
	m := o.Method
	if m == "" {
		m = "GET"
	}
	s := m + " " + o.URL
	if o.StatusCode > 0 {
		s += " -> " + strconv.Itoa(o.StatusCode)
	}
	return s
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
