package ingest

import (
	"fmt"
	"strings"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
	"github.com/SrinjoyDev/rewynd/core/internal/store"
)

func DecodeLogs(req *collogspb.ExportLogsServiceRequest) store.Batch {
	var b store.Batch
	for _, rl := range req.ResourceLogs {
		service := resourceService(rl.Resource)
		for _, sl := range rl.ScopeLogs {
			scope := ""
			if sl.Scope != nil {
				scope = sl.Scope.Name
			}
			for _, lr := range sl.LogRecords {
				b.Logs = append(b.Logs, decodeLog(lr, scope, service))
			}
		}
	}
	return b
}

func decodeLog(lr *logspb.LogRecord, scope, service string) model.Log {
	at := int64(lr.TimeUnixNano)
	if at == 0 {
		at = int64(lr.ObservedTimeUnixNano)
	}
	traceID := hexID(lr.TraceId)
	attrs := attrsToMap(lr.Attributes)
	if service != "" {
		if attrs == nil {
			attrs = map[string]any{}
		}
		attrs["service.name"] = service
	}
	return model.Log{
		ID:         nextID("log"),
		RequestID:  traceID, // empty => unattributed; we never guess a parent
		Service:    service,
		TraceID:    traceID,
		SpanID:     hexID(lr.SpanId),
		At:         at,
		Level:      severityText(lr),
		Message:    bodyString(lr.Body),
		Source:     logSource(scope),
		Attributes: attrs,
	}
}

func severityText(lr *logspb.LogRecord) string {
	if lr.SeverityText != "" {
		return strings.ToLower(lr.SeverityText)
	}
	switch n := lr.SeverityNumber; {
	case n >= 21:
		return "fatal"
	case n >= 17:
		return "error"
	case n >= 13:
		return "warn"
	case n >= 9:
		return "info"
	case n >= 5:
		return "debug"
	case n >= 1:
		return "trace"
	}
	return ""
}

func bodyString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	if s, ok := v.Value.(*commonpb.AnyValue_StringValue); ok {
		return s.StringValue
	}
	return fmt.Sprint(anyValue(v))
}

func logSource(scope string) string {
	switch {
	case strings.Contains(scope, "pino"):
		return "pino"
	case strings.Contains(scope, "winston"):
		return "winston"
	case strings.Contains(scope, "bunyan"):
		return "bunyan"
	default:
		return "console"
	}
}
