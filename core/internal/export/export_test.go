package export

import (
	"strings"
	"testing"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

func TestHTMLContainsTheTrace(t *testing.T) {
	r := &model.Request{
		Method: "POST", Path: "/api/orders", StatusCode: 500, Error: true, DurationMs: 120,
		TraceID: "t1", Service: "api", EndedAt: 120_000_000,
		Detections: []model.Detection{{Title: "N+1 query", Suggestion: "Batch it"}},
		Exceptions: []model.Exception{{Type: "DatabaseError", Message: "null value in column total", Stack: "at run"}},
		Queries:    []model.Query{{Statement: "SELECT 1", DurationMs: 2, StartedAt: 1_000_000}},
		Outbound:   []model.Outbound{{Method: "GET", URL: "https://x/y", StatusCode: 200, DurationMs: 5}},
		Logs:       []model.Log{{Level: "error", Message: "boom"}},
		Request:    &model.HTTPPayload{Headers: map[string]string{"authorization": "[redacted]"}, Body: `{"a":1}`},
	}
	out := HTML(r)
	for _, want := range []string{
		"<!doctype html>", "POST", "/api/orders", "500", "N+1 query", "Batch it",
		"DatabaseError", "null value in column total", "wf-bar", "https://x/y", "boom",
		"[redacted]", "github.com/SrinjoyDev/rewynd",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("export HTML missing %q", want)
		}
	}
	// User content must be HTML-escaped (no raw injection).
	if strings.Contains(HTML(&model.Request{Method: "GET", Path: "/<script>"}), "/<script>") {
		t.Errorf("path was not HTML-escaped")
	}
}

func TestHTMLJobFlow(t *testing.T) {
	r := &model.Request{Kind: model.KindJob, Method: "process", Path: "orders.created", Error: true, DurationMs: 9}
	out := HTML(r)
	if !strings.Contains(out, "JOB") || !strings.Contains(out, "orders.created") || !strings.Contains(out, "fail") {
		t.Errorf("job export missing JOB/label/outcome")
	}
}
