package ingest

import (
	"fmt"
	"sync/atomic"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/rewyndhq/rewynd/internal/model"
	"github.com/rewyndhq/rewynd/internal/store"
)

var seq uint64

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, atomic.AddUint64(&seq, 1))
}

func DecodeTraces(req *coltracepb.ExportTraceServiceRequest) store.Batch {
	var b store.Batch
	for _, rs := range req.ResourceSpans {
		service := resourceService(rs.Resource)
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				decodeSpan(service, sp, &b)
			}
		}
	}
	return b
}

func decodeSpan(service string, sp *tracepb.Span, b *store.Batch) {
	attrs := attrsToMap(sp.Attributes)
	if attrs == nil {
		attrs = map[string]any{}
	}
	traceID := hexID(sp.TraceId)
	spanID := hexID(sp.SpanId)
	typ := classify(sp.Kind, attrs)
	statusErr := sp.Status != nil && sp.Status.Code == tracepb.Status_STATUS_CODE_ERROR

	switch typ {
	case model.SpanDBQuery:
		stmt := firstAttr(attrs, "db.statement", "db.query.text")
		attrs["db.statement"] = stmt
		attrs["db.statement.norm"] = normalizeSQL(stmt)
	case model.SpanHTTPClient:
		attrs["http.method"] = firstAttr(attrs, "http.method", "http.request.method")
		attrs["http.url"] = firstAttr(attrs, "http.url", "url.full")
		attrs["http.status_code"] = firstAttrInt(attrs, "http.status_code", "http.response.status_code")
	}

	started, ended := int64(sp.StartTimeUnixNano), int64(sp.EndTimeUnixNano)
	b.Spans = append(b.Spans, model.Span{
		SpanID: spanID, RequestID: traceID, TraceID: traceID, ParentSpanID: hexID(sp.ParentSpanId),
		Name: sp.Name, Type: typ, StartedAt: started, EndedAt: ended,
		DurationMs: durationMs(started, ended), Status: statusString(sp.Status), Attributes: attrs,
	})

	if typ == model.SpanHTTPServer {
		method := firstAttr(attrs, "http.method", "http.request.method")
		route := firstAttr(attrs, "http.route")
		path := firstAttr(attrs, "url.path", "http.target")
		if path == "" {
			path = route
		}
		status := firstAttrInt(attrs, "http.status_code", "http.response.status_code")
		b.Requests = append(b.Requests, model.Request{
			ID: traceID, SchemaVersion: model.SchemaVersion, TraceID: traceID, Service: service,
			Method: method, Path: path, Route: route, StatusCode: status,
			StartedAt: started, EndedAt: ended, DurationMs: durationMs(started, ended),
			Error: statusErr || status >= 500,
		})
	}

	for _, ev := range sp.Events {
		if ev.Name != "exception" {
			continue
		}
		ea := attrsToMap(ev.Attributes)
		b.Exceptions = append(b.Exceptions, model.Exception{
			ID: nextID("exc"), RequestID: traceID, SpanID: spanID,
			Type:    firstAttr(ea, "exception.type"),
			Message: firstAttr(ea, "exception.message"),
			Stack:   firstAttr(ea, "exception.stacktrace"),
			At:      int64(ev.TimeUnixNano),
		})
	}
}

func resourceService(r *resourcepb.Resource) string {
	if r == nil {
		return ""
	}
	return firstAttr(attrsToMap(r.Attributes), "service.name")
}

func durationMs(start, end int64) float64 {
	if end <= start {
		return 0
	}
	return float64(end-start) / 1e6
}

func statusString(s *tracepb.Status) string {
	if s == nil {
		return "unset"
	}
	switch s.Code {
	case tracepb.Status_STATUS_CODE_OK:
		return "ok"
	case tracepb.Status_STATUS_CODE_ERROR:
		return "error"
	}
	return "unset"
}
