// eval-seed populates a rewynd core with the agent-benchmark scenarios (see
// bench/agent-eval/). Each is a realistic backend bug whose root cause is recorded in the
// trace. Run a core first; then: go run ./cmd/eval-seed
package main

import (
	"context"
	"log"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const base int64 = 1_000_000_000_000

func s(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}
func i(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}
func ms(n int64) int64 { return n * 1_000_000 }

func id(n, sz int) []byte {
	b := make([]byte, sz)
	for j := range b {
		b[j] = byte(n + j)
	}
	return b
}

type span = tracepb.Span

var conn *grpc.ClientConn

func send(svc string, spans []*span) {
	c := coltracepb.NewTraceServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Export(ctx, &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource:   &resourcepb.Resource{Attributes: []*commonpb.KeyValue{s("service.name", svc)}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}},
	}}}); err != nil {
		log.Fatal(err)
	}
}

func httpRoot(tid int, method, path string, status int, dur int64) *span {
	sp := &span{TraceId: id(tid, 16), SpanId: id(tid+1, 8), Name: method + " " + path, Kind: tracepb.Span_SPAN_KIND_SERVER,
		StartTimeUnixNano: uint64(base), EndTimeUnixNano: uint64(base + ms(dur)),
		Attributes: []*commonpb.KeyValue{s("http.request.method", method), s("url.path", path), i("http.response.status_code", int64(status))}}
	if status >= 500 {
		sp.Status = &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR}
	}
	return sp
}

func dbSpan(tid, sid int, stmt string, start, dur int64) *span {
	return &span{TraceId: id(tid, 16), SpanId: id(sid, 8), ParentSpanId: id(tid+1, 8), Name: "query", Kind: tracepb.Span_SPAN_KIND_CLIENT,
		StartTimeUnixNano: uint64(base + ms(start)), EndTimeUnixNano: uint64(base + ms(start+dur)),
		Attributes: []*commonpb.KeyValue{s("db.system", "postgresql"), s("db.statement", stmt)}}
}

func exc(typ, msg, stack string, at int64) *tracepb.Span_Event {
	return &tracepb.Span_Event{Name: "exception", TimeUnixNano: uint64(base + ms(at)), Attributes: []*commonpb.KeyValue{
		s("exception.type", typ), s("exception.message", msg), s("exception.stacktrace", stack)}}
}

func main() {
	c, err := grpc.NewClient("127.0.0.1:4317", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	conn = c
	defer conn.Close()

	// Task 1 — checkout-500: a 500 whose logs only say "request failed". The trace has the
	// failing INSERT and the not-null-constraint exception.
	{
		root := httpRoot(10, "POST", "/api/checkout", 500, 95)
		root.Events = []*tracepb.Span_Event{exc("DatabaseError",
			`null value in column "total" of relation "orders" violates not-null constraint`,
			"at Query.run (db.ts:88)\nat async createOrder (checkout.ts:34)", 80)}
		send("api", []*span{root,
			dbSpan(10, 30, "SELECT id, price FROM items WHERE cart_id = $1", 5, 8),
			dbSpan(10, 31, "INSERT INTO orders (user_id, total) VALUES ($1, $2)", 70, 18)})
	}

	// Task 2 — users-nplus1: a slow endpoint. Logs say nothing useful. The trace shows the N+1.
	{
		spans := []*span{httpRoot(50, "GET", "/api/users", 200, 940)}
		spans = append(spans, dbSpan(50, 70, "SELECT id, name FROM users ORDER BY id", 2, 4))
		for k := 0; k < 50; k++ {
			spans = append(spans, dbSpan(50, 100+k, "SELECT id, title FROM posts WHERE user_id = $1", int64(8+k*18), 16))
		}
		send("api", spans)
	}

	// Task 3 — pay-502: a 502 whose logs say "charging". The trace shows the outbound call to the
	// payments API returned 500.
	{
		root := httpRoot(200, "POST", "/api/pay", 502, 310)
		out := &span{TraceId: id(200, 16), SpanId: id(201+1, 8), ParentSpanId: id(201, 8), Name: "POST charge",
			Kind: tracepb.Span_SPAN_KIND_CLIENT, StartTimeUnixNano: uint64(base + ms(20)), EndTimeUnixNano: uint64(base + ms(300)),
			Attributes: []*commonpb.KeyValue{s("http.request.method", "POST"), s("url.full", "https://payments.acme.com/v1/charge"), i("http.response.status_code", 500)},
			Status:     &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR}}
		send("api", []*span{root, out})
	}

	// Task 4 — email-job: a background job that fails silently (no HTTP request at all). The trace
	// has the consumer span and the SMTP exception.
	{
		job := &span{TraceId: id(300, 16), SpanId: id(301, 8), Name: "email.send process", Kind: tracepb.Span_SPAN_KIND_CONSUMER,
			StartTimeUnixNano: uint64(base), EndTimeUnixNano: uint64(base + ms(40)),
			Attributes: []*commonpb.KeyValue{s("messaging.system", "redis"), s("messaging.destination.name", "email.send"), s("messaging.operation", "process")},
			Status:     &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR},
			Events:     []*tracepb.Span_Event{exc("SMTPConnectError", "connection refused: smtp.mailer.internal:587", "at sendMail (mailer.ts:51)\nat processJob (worker.ts:22)", 35)}}
		send("worker", []*span{job})
	}

	log.Println("seeded 4 benchmark scenarios")
}
