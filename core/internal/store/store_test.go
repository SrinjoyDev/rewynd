package store

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func nPlusOneBatch(traceID string, posts int) Batch {
	norm := "SELECT * FROM posts WHERE user_id=?"
	spans := []model.Span{{
		SpanID: "root", RequestID: traceID, TraceID: traceID, Type: model.SpanHTTPServer,
		Name: "GET", StartedAt: 1, EndedAt: 1_000_000, DurationMs: 1,
	}}
	for i := 0; i < posts; i++ {
		spans = append(spans, model.Span{
			SpanID: fmt.Sprintf("q%d", i), RequestID: traceID, TraceID: traceID, ParentSpanID: "root",
			Type: model.SpanDBQuery, Name: "pg.query", DurationMs: 1,
			Attributes: map[string]any{"db.statement": norm, "db.statement.norm": norm},
		})
	}
	return Batch{
		Requests: []model.Request{{
			ID: traceID, TraceID: traceID, Method: "GET", Path: "/api/users",
			StatusCode: 200, StartedAt: 1, EndedAt: 1_000_000, DurationMs: 1,
		}},
		Spans: spans,
	}
}

func TestRoundTripAndDetect(t *testing.T) {
	st := newTestStore(t)
	if err := st.WriteBatch(nPlusOneBatch("trace_aaa111", 6)); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := st.GetRequest("trace_aaa111")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Queries) != 6 {
		t.Fatalf("queries = %d, want 6", len(got.Queries))
	}
	if len(got.Detections) != 1 || got.Detections[0].Type != model.DetectNPlusOne {
		t.Fatalf("want one n_plus_one detection, got %+v", got.Detections)
	}
	if got.SchemaVersion != model.SchemaVersion {
		t.Fatalf("schema version = %d", got.SchemaVersion)
	}
}

func TestListEnrichesCountsAndFlags(t *testing.T) {
	st := newTestStore(t)
	if err := st.WriteBatch(nPlusOneBatch("trace_bbb222", 5)); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListRequests(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d, want 1", len(list))
	}
	if list[0].Counts.Queries != 5 {
		t.Fatalf("counts.queries = %d, want 5", list[0].Counts.Queries)
	}
	if len(list[0].Detections) != 1 {
		t.Fatalf("expected N+1 flag in list view")
	}
}

func TestPrefixResolveAndClear(t *testing.T) {
	st := newTestStore(t)
	if err := st.WriteBatch(nPlusOneBatch("trace_ccc333", 5)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetRequest("trace_ccc"); err != nil {
		t.Fatalf("prefix resolve: %v", err)
	}
	if err := st.Clear(); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.Count(); n != 0 {
		t.Fatalf("count after clear = %d, want 0", n)
	}
}

func TestStatusFilter(t *testing.T) {
	st := newTestStore(t)
	_ = st.WriteBatch(Batch{Requests: []model.Request{
		{ID: "ok1", TraceID: "ok1", Method: "GET", Path: "/a", StatusCode: 200, StartedAt: 2},
		{ID: "err1", TraceID: "err1", Method: "POST", Path: "/b", StatusCode: 500, Error: true, StartedAt: 3},
	}})
	five, err := st.ListRequests(ListOptions{StatusClass: "5xx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(five) != 1 || five[0].ID != "err1" {
		t.Fatalf("5xx filter returned %+v", five)
	}
}
