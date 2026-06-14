package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

func TestViewRendersNPlusOne(t *testing.T) {
	r := model.Request{
		ID: "abc123def456aa", TraceID: "abc123def456aa", Method: "GET", Path: "/api/users",
		StatusCode: 200, DurationMs: 15, StartedAt: 0, EndedAt: 15_000_000,
		Detections: []model.Detection{{Type: model.DetectNPlusOne, Title: "N+1 query — 10 identical statements"}},
	}
	detail := r
	for i := 0; i < 10; i++ {
		detail.Queries = append(detail.Queries, model.Query{
			Statement:  "SELECT id, title FROM posts WHERE user_id = $1",
			DurationMs: 1, StartedAt: int64(i) * 1_000_000,
		})
	}
	detail.Logs = []model.Log{{Level: "info", Message: "listing users"}}

	a := app{reqs: []model.Request{r}, detail: &detail, width: 120, height: 30}
	out := a.View()
	for _, want := range []string{"rewynd", "/api/users", "DETECTIONS", "WATERFALL", "LOGS"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestHelpOverlay(t *testing.T) {
	a := app{width: 100, height: 30, help: true}
	out := a.View()
	for _, want := range []string{"keys", "quit", "filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
}

func TestViewEmptyAndUnsized(t *testing.T) {
	if got := (app{}).View(); !strings.Contains(got, "starting") {
		t.Errorf("unsized view = %q", got)
	}
	a := app{width: 100, height: 24}
	if out := a.View(); !strings.Contains(out, "no requests yet") {
		t.Errorf("empty view should prompt to hit an endpoint, got: %q", out)
	}
}

func TestDetailRendersOutboundResponseAndSuggestion(t *testing.T) {
	r := model.Request{
		ID: "ab12cd34", TraceID: "ab12cd34", Method: "POST", Path: "/api/orders",
		StatusCode: 200, DurationMs: 40, EndedAt: 40_000_000,
		Detections: []model.Detection{{Type: model.DetectNPlusOne, Title: "N+1 query", Suggestion: "Batch into a single query"}},
		Outbound:   []model.Outbound{{Method: "GET", URL: "https://api.example.com/rates", StatusCode: 200, DurationMs: 12}},
		Response:   &model.HTTPPayload{Body: `{"ok":true}`},
	}
	a := app{width: 120, height: 60, reqs: []model.Request{r}, detail: &r}
	out := strings.Join(a.detailLines(&r, 80), "\n")
	for _, want := range []string{"OUTBOUND", "api.example.com/rates", "RESPONSE BODY", `{"ok":true}`, "Batch into a single query"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q", want)
		}
	}
}

func TestDetailScroll(t *testing.T) {
	r := model.Request{ID: "scroll01", TraceID: "scroll01", Method: "GET", Path: "/x", StatusCode: 200}
	for i := 0; i < 80; i++ {
		r.Logs = append(r.Logs, model.Log{Level: "info", Message: fmt.Sprintf("log-line-%02d", i)})
	}
	a := app{width: 120, height: 20, reqs: []model.Request{r}, detail: &r}
	_, detailW, bodyH := a.layout()
	all := a.detailLines(&r, detailW)
	if len(all) <= bodyH {
		t.Fatalf("need a detail taller than the pane to test scrolling (%d <= %d)", len(all), bodyH)
	}

	top := a.renderDetail(detailW, bodyH)
	if !strings.Contains(top, "log-line-00") || strings.Contains(top, "log-line-79") {
		t.Errorf("at scroll 0 the top should show but not the bottom")
	}
	if !strings.Contains(top, "▼") {
		t.Errorf("a scrollable detail should show the down indicator")
	}

	a.detailScroll = detailWindowMax(len(all), bodyH)
	bottom := a.renderDetail(detailW, bodyH)
	if !strings.Contains(bottom, "log-line-79") {
		t.Errorf("scrolled to the end the last log should be visible")
	}
	if !strings.Contains(bottom, "▲") {
		t.Errorf("at the bottom the up indicator should show")
	}
}

func TestDetailShowsServicesWhenDistributed(t *testing.T) {
	r := model.Request{
		ID: "tr01", TraceID: "tr01", Method: "GET", Path: "/checkout", StatusCode: 200, Service: "gateway",
		Spans: []model.Span{
			{Service: "gateway", Type: model.SpanHTTPServer},
			{Service: "billing", Type: model.SpanDBQuery},
		},
		Queries:  []model.Query{{Statement: "INSERT INTO charges (x) VALUES ($1)", Service: "billing", DurationMs: 3}},
		Outbound: []model.Outbound{{Method: "POST", URL: "http://billing/charge", StatusCode: 200, Service: "gateway"}},
	}
	a := app{width: 120, height: 60, reqs: []model.Request{r}, detail: &r}
	out := strings.Join(a.detailLines(&r, 90), "\n")
	for _, want := range []string{"services", "gateway", "billing", "[billing]"} {
		if !strings.Contains(out, want) {
			t.Errorf("distributed detail missing %q", want)
		}
	}

	// Single-service requests stay clean — no service labels.
	single := model.Request{ID: "s1", Method: "GET", Path: "/x", StatusCode: 200, Service: "app",
		Spans:   []model.Span{{Service: "app", Type: model.SpanHTTPServer}},
		Queries: []model.Query{{Statement: "SELECT 1", Service: "app", DurationMs: 1}}}
	b := app{width: 120, height: 60, reqs: []model.Request{single}, detail: &single}
	if strings.Contains(strings.Join(b.detailLines(&single, 90), "\n"), "[app]") {
		t.Errorf("single-service detail should not show service tags")
	}
}

func TestListOptsReflectsFilters(t *testing.T) {
	o := app{filter: "5xx", search: "/api/users", slowOnly: true}.listOpts()
	if o.StatusClass != "5xx" || o.PathLike != "/api/users" || !o.Slow {
		t.Errorf("listOpts did not carry filters: %+v", o)
	}
}

func TestFooterAndTitleReflectState(t *testing.T) {
	searching := app{width: 120, height: 30, searching: true, search: "ord"}
	if !strings.Contains(searching.footerText(), "search /ord") {
		t.Errorf("searching footer should echo the query, got %q", searching.footerText())
	}
	filtered := app{width: 120, height: 30, filter: "5xx", slowOnly: true, search: "ord"}
	title := filtered.titleText()
	for _, want := range []string{"5xx", "slow", "/ord"} {
		if !strings.Contains(title, want) {
			t.Errorf("title missing %q, got %q", want, title)
		}
	}
}
