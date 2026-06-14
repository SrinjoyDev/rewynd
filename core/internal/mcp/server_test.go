package mcp

import (
	"strings"
	"testing"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

func TestIssueLabel(t *testing.T) {
	cases := []struct {
		name             string
		r                model.Request
		exc, nplus, slow bool
		want             string
	}{
		{"clean", model.Request{StatusCode: 200, DurationMs: 12}, false, false, false, ""},
		{"5xx+exc", model.Request{StatusCode: 500}, true, false, false, "500 + exception"},
		{"5xx only", model.Request{StatusCode: 503}, false, false, false, "503 server error"},
		{"exc on 200", model.Request{StatusCode: 200}, true, false, false, "exception"},
		{"n+1", model.Request{StatusCode: 200, Counts: model.Counts{Queries: 12}}, false, true, false, "N+1 (12 queries)"},
		{"slow", model.Request{StatusCode: 200, DurationMs: 1400}, false, false, true, "slow 1400ms"},
	}
	for _, c := range cases {
		if got := issueLabel(&c.r, c.exc, c.nplus, c.slow); got != c.want {
			t.Errorf("%s: issueLabel = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestStatsHint(t *testing.T) {
	if h := statsHint(statsOutput{Total: 0}); !strings.Contains(h, "Nothing recorded") {
		t.Errorf("empty hint = %q", h)
	}
	if h := statsHint(statsOutput{Total: 5, ServerErrors: 2}); !strings.Contains(h, "server errors") {
		t.Errorf("5xx hint = %q", h)
	}
	if h := statsHint(statsOutput{Total: 5, Problems: []problemRef{{}}}); !strings.Contains(h, "flagged") {
		t.Errorf("flagged hint = %q", h)
	}
	if h := statsHint(statsOutput{Total: 5}); !strings.Contains(h, "clean") {
		t.Errorf("clean hint = %q", h)
	}
}

func TestShortID(t *testing.T) {
	if got := short("abcdef0123456789"); got != "abcdef01" {
		t.Errorf("short = %q", got)
	}
	if got := short("abc"); got != "abc" {
		t.Errorf("short of short id = %q", got)
	}
}
