package store_test

import (
	"path/filepath"
	"testing"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/SrinjoyDev/rewynd/internal/ingest"
	"github.com/SrinjoyDev/rewynd/internal/store"
)

// TestDistributedTraceStitching simulates a gateway calling a downstream service in one trace,
// with the downstream export arriving first, and asserts: the two services collapse into one
// recorded request, the entry (earliest) service is the canonical root, and every query /
// outbound / span is attributed to the service it ran in.
func TestDistributedTraceStitching(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	trace := []byte("distributed-tr16")

	gateway := exportFor("gateway", &tracepb.Span{
		TraceId: trace, SpanId: []byte("gwsrv001"), Name: "GET /checkout",
		Kind: tracepb.Span_SPAN_KIND_SERVER, StartTimeUnixNano: 1000, EndTimeUnixNano: 9000,
		Attributes: kv(s("http.request.method", "GET"), s("url.path", "/checkout"), i("http.response.status_code", 200)),
	}, &tracepb.Span{
		TraceId: trace, SpanId: []byte("gwcli001"), ParentSpanId: []byte("gwsrv001"), Name: "POST billing",
		Kind: tracepb.Span_SPAN_KIND_CLIENT, StartTimeUnixNano: 2000, EndTimeUnixNano: 8000,
		Attributes: kv(s("http.request.method", "POST"), s("url.full", "http://billing/charge"), i("http.response.status_code", 200)),
	})

	billing := exportFor("billing", &tracepb.Span{
		TraceId: trace, SpanId: []byte("blsrv001"), ParentSpanId: []byte("gwcli001"), Name: "POST /charge",
		Kind: tracepb.Span_SPAN_KIND_SERVER, StartTimeUnixNano: 3000, EndTimeUnixNano: 7000,
		Attributes: kv(s("http.request.method", "POST"), s("url.path", "/charge"), i("http.response.status_code", 200)),
	}, &tracepb.Span{
		TraceId: trace, SpanId: []byte("bldb0001"), ParentSpanId: []byte("blsrv001"), Name: "INSERT",
		Kind: tracepb.Span_SPAN_KIND_CLIENT, StartTimeUnixNano: 4000, EndTimeUnixNano: 5000,
		Attributes: kv(s("db.system", "postgresql"), s("db.statement", "INSERT INTO charges (amount) VALUES ($1)")),
	})

	// Downstream service's export arrives first — the entry root must still win.
	if err := st.WriteBatch(ingest.DecodeTraces(billing)); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteBatch(ingest.DecodeTraces(gateway)); err != nil {
		t.Fatal(err)
	}

	reqs, err := st.ListRequests(store.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("a distributed trace should collapse to one request, got %d", len(reqs))
	}

	r, err := st.GetRequest(reqs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Service != "gateway" || r.Path != "/checkout" {
		t.Errorf("root should be the entry service (gateway /checkout), got %s %s", r.Service, r.Path)
	}
	if len(r.Queries) != 1 || r.Queries[0].Service != "billing" {
		t.Errorf("the query should be attributed to billing, got %+v", r.Queries)
	}
	if len(r.Outbound) != 1 || r.Outbound[0].Service != "gateway" {
		t.Errorf("the outbound call should be attributed to gateway, got %+v", r.Outbound)
	}
	services := map[string]bool{}
	for _, sp := range r.Spans {
		services[sp.Service] = true
	}
	if !services["gateway"] || !services["billing"] {
		t.Errorf("both services' spans should be present, got %v", services)
	}
}

func exportFor(service string, spans ...*tracepb.Span) *coltracepb.ExportTraceServiceRequest {
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource:   &resourcepb.Resource{Attributes: kv(s("service.name", service))},
			ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}},
		}},
	}
}

func kv(attrs ...*commonpb.KeyValue) []*commonpb.KeyValue { return attrs }

func s(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func i(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}
