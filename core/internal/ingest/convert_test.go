package ingest

import (
	"testing"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

func TestNormalizeSQL(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM posts WHERE user_id = $1": "SELECT * FROM posts WHERE user_id = ?",
		"SELECT * FROM t WHERE id = 42":          "SELECT * FROM t WHERE id = ?",
		"INSERT INTO a VALUES ('x', 1)":          "INSERT INTO a VALUES (?, ?)",
		"SELECT  a,\n  b":                        "SELECT a, b",
	}
	for in, want := range cases {
		if got := normalizeSQL(in); got != want {
			t.Errorf("normalizeSQL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name  string
		kind  tracepb.Span_SpanKind
		attrs map[string]any
		want  model.SpanType
	}{
		{"db query", tracepb.Span_SPAN_KIND_CLIENT, map[string]any{"db.system": "postgresql", "db.statement": "SELECT 1"}, model.SpanDBQuery},
		{"db connect (no statement)", tracepb.Span_SPAN_KIND_CLIENT, map[string]any{"db.system": "postgresql"}, model.SpanInternal},
		{"http server", tracepb.Span_SPAN_KIND_SERVER, map[string]any{"http.method": "GET", "url.path": "/x"}, model.SpanHTTPServer},
		{"http client", tracepb.Span_SPAN_KIND_CLIENT, map[string]any{"http.method": "GET", "http.url": "http://x"}, model.SpanHTTPClient},
		{"plain internal", tracepb.Span_SPAN_KIND_INTERNAL, map[string]any{"express.type": "middleware"}, model.SpanInternal},
	}
	for _, c := range cases {
		if got := classify(c.kind, c.attrs); got != c.want {
			t.Errorf("%s: classify = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestParseHeaders(t *testing.T) {
	m := parseHeaders(`{"content-type":"application/json","authorization":"«redacted»"}`)
	if m["content-type"] != "application/json" {
		t.Errorf("content-type = %q", m["content-type"])
	}
	if m["authorization"] != "«redacted»" {
		t.Errorf("redacted value not preserved: %q", m["authorization"])
	}
	if parseHeaders("") != nil || parseHeaders("not json") != nil {
		t.Error("empty/invalid input should yield nil")
	}
}

func TestFirstAttrInt(t *testing.T) {
	m := map[string]any{"a": float64(200), "b": "404", "c": int64(500)}
	for k, want := range map[string]int{"a": 200, "b": 404, "c": 500} {
		if got := firstAttrInt(m, k); got != want {
			t.Errorf("firstAttrInt(%q) = %d, want %d", k, got, want)
		}
	}
	st := map[string]any{"http.status_code": 0, "http.response.status_code": float64(200)}
	if got := firstAttrInt(st, "http.status_code", "http.response.status_code"); got != 200 {
		t.Errorf("zero should fall through to next key, got %d", got)
	}
}
