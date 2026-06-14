package otlp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/SrinjoyDev/rewynd/core/internal/store"
)

// TestHTTPReceiverStoresTraces drives the OTLP/HTTP path: POST a marshalled trace export to the
// handler and assert the request is recorded (the gRPC path has its own test).
func TestHTTPReceiverStoresTraces(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	body, err := proto.Marshal(&coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strAttr("service.name", "api"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
				TraceId: []byte("trace-id-16bytes"), SpanId: []byte("span8byt"),
				Name: "GET /api/orders", Kind: tracepb.Span_SPAN_KIND_SERVER,
				StartTimeUnixNano: 1_000_000, EndTimeUnixNano: 4_000_000,
				Attributes: []*commonpb.KeyValue{
					strAttr("http.request.method", "GET"),
					strAttr("url.path", "/api/orders"),
					intAttr("http.response.status_code", 200),
				},
			}}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewReceiver(st).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	reqs, err := st.ListRequests(store.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Path != "/api/orders" || reqs[0].Service != "api" {
		t.Fatalf("expected the request recorded, got %+v", reqs)
	}
}

func TestHTTPReceiverRejectsGarbage(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	defer st.Close()
	srv := httptest.NewServer(NewReceiver(st).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/traces", "application/x-protobuf", bytes.NewReader([]byte("not protobuf")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("garbage body should be 400, got %d", resp.StatusCode)
	}
}
