package tui

import (
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
