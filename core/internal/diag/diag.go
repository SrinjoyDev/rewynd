// Package diag turns a correlated request into a short list of problems, shared by the CLI's
// `diagnose` command and the MCP server's diagnose tool.
package diag

import (
	"fmt"
	"strings"

	"github.com/rewyndhq/rewynd/internal/model"
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
		ps = append(ps, Problem{"exception", strings.TrimSpace(e.Type + ": " + oneLine(e.Message)), firstLine(e.Stack)})
	}
	for _, q := range r.Queries {
		if q.DurationMs >= 100 {
			ps = append(ps, Problem{"slow_query", fmt.Sprintf("slow query (%.0fms): %s", q.DurationMs, oneLine(q.Statement)), ""})
		}
	}
	return ps
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
