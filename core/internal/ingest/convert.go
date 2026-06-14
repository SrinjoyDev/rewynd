// Package ingest decodes OTLP (the wire protocol) into rewynd's model. It owns the mapping
// from OTel semantic conventions to our canonical fields, so nothing downstream sees OTLP.
package ingest

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

func anyValue(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch x := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return x.DoubleValue
	case *commonpb.AnyValue_BytesValue:
		return string(x.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		out := make([]any, 0, len(x.ArrayValue.Values))
		for _, e := range x.ArrayValue.Values {
			out = append(out, anyValue(e))
		}
		return out
	case *commonpb.AnyValue_KvlistValue:
		m := map[string]any{}
		for _, kv := range x.KvlistValue.Values {
			m[kv.Key] = anyValue(kv.Value)
		}
		return m
	}
	return nil
}

func attrsToMap(kvs []*commonpb.KeyValue) map[string]any {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = anyValue(kv.Value)
	}
	return m
}

func hexID(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}

var (
	reSQLString = regexp.MustCompile(`'[^']*'`)
	reSQLParam  = regexp.MustCompile(`\$\d+`)
	reSQLNum    = regexp.MustCompile(`\b\d+\b`)
	reWS        = regexp.MustCompile(`\s+`)
)

// normalizeSQL strips literals and params so identical-shaped queries share a key (N+1).
func normalizeSQL(s string) string {
	s = reSQLString.ReplaceAllString(s, "?")
	s = reSQLParam.ReplaceAllString(s, "?")
	s = reSQLNum.ReplaceAllString(s, "?")
	return strings.TrimSpace(reWS.ReplaceAllString(s, " "))
}

func firstAttr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func firstAttrInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			return 0
		}
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return 0
}

func hasHTTP(m map[string]any) bool {
	return firstAttr(m, "http.method", "http.request.method", "url.path", "http.target", "http.route") != ""
}

func classify(kind tracepb.Span_SpanKind, m map[string]any) model.SpanType {
	// A query must carry a statement — connection/pool spans have db.system but no statement.
	if firstAttr(m, "db.statement", "db.query.text") != "" {
		return model.SpanDBQuery
	}
	switch kind {
	case tracepb.Span_SPAN_KIND_SERVER:
		if hasHTTP(m) {
			return model.SpanHTTPServer
		}
	case tracepb.Span_SPAN_KIND_CLIENT:
		if hasHTTP(m) {
			return model.SpanHTTPClient
		}
	}
	return model.SpanInternal
}
