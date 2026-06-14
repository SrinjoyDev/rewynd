package otlp

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/SrinjoyDev/rewynd/core/internal/store"
)

// TestGRPCExportLandsInStore drives the real gRPC path: it serves the OTLP/gRPC server, exports
// a server span through a genuine gRPC client, and asserts the request is recorded.
func TestGRPCExportLandsInStore(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewGRPCServer(st)
	go func() { _ = srv.Serve(ln) }()
	defer srv.Stop()

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = coltracepb.NewTraceServiceClient(conn).Export(ctx, &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strAttr("service.name", "checkout"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
				TraceId:           []byte("trace-id-16bytes"),
				SpanId:            []byte("span8byt"),
				Name:              "GET /api/cart",
				Kind:              tracepb.Span_SPAN_KIND_SERVER,
				StartTimeUnixNano: 1_000_000,
				EndTimeUnixNano:   6_000_000,
				Attributes: []*commonpb.KeyValue{
					strAttr("http.request.method", "GET"),
					strAttr("url.path", "/api/cart"),
					intAttr("http.response.status_code", 200),
				},
			}}}},
		}},
	})
	if err != nil {
		t.Fatalf("gRPC export failed: %v", err)
	}

	reqs, err := st.ListRequests(store.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 recorded request, got %d", len(reqs))
	}
	if reqs[0].Path != "/api/cart" || reqs[0].StatusCode != 200 || reqs[0].Service != "checkout" {
		t.Errorf("unexpected request: %+v", reqs[0])
	}
}

func strAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func intAttr(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}
