// Package model is rewynd's data contract, shared by the store, the JSON CLI, and the MCP
// server. It is public API: additive changes only, bump SchemaVersion on a breaking change.
package model

// SchemaVersion is the contract version embedded in every Request payload.
const SchemaVersion = 1

// SpanType is rewynd's normalized classification of an OTel span, derived from its
// attributes/kind so frontends don't have to re-derive it.
type SpanType string

const (
	SpanHTTPServer SpanType = "http_server" // the request root
	SpanDBQuery    SpanType = "db_query"
	SpanHTTPClient SpanType = "http_client" // outbound
	SpanInternal   SpanType = "internal"
	SpanOther      SpanType = "other"
)

// DetectionType enumerates the problems rewynd flags automatically.
type DetectionType string

const (
	DetectNPlusOne      DetectionType = "n_plus_one"
	DetectSlowQuery     DetectionType = "slow_query"
	DetectSlowRequest   DetectionType = "slow_request"
	DetectDuplicateCall DetectionType = "duplicate_outbound"
)

// Request is the correlation root: one inbound HTTP request and everything it caused.
type Request struct {
	ID            string `json:"id"`
	SchemaVersion int    `json:"schema_version"`
	TraceID       string `json:"trace_id"`
	Service       string `json:"service,omitempty"`

	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Route      string  `json:"route,omitempty"`
	StatusCode int     `json:"status_code"`
	StartedAt  int64   `json:"started_at"` // unix nanoseconds
	EndedAt    int64   `json:"ended_at"`   // unix nanoseconds
	DurationMs float64 `json:"duration_ms"`
	Error      bool    `json:"error"`

	Counts     Counts      `json:"counts"`
	Detections []Detection `json:"detections,omitempty"`

	// Detail — populated by `show`/`get_request`, omitted from list views.
	Request    *HTTPPayload `json:"request,omitempty"`
	Response   *HTTPPayload `json:"response,omitempty"`
	Spans      []Span       `json:"spans,omitempty"`
	Queries    []Query      `json:"queries,omitempty"`
	Outbound   []Outbound   `json:"outbound,omitempty"`
	Logs       []Log        `json:"logs,omitempty"`
	Exceptions []Exception  `json:"exceptions,omitempty"`
}

// Counts is the at-a-glance summary shown in list views.
type Counts struct {
	Queries    int `json:"queries"`
	Outbound   int `json:"outbound"`
	Logs       int `json:"logs"`
	Exceptions int `json:"exceptions"`
}

// HTTPPayload captures a redacted request or response body + headers.
type HTTPPayload struct {
	Headers     map[string]string `json:"headers,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Bytes       int               `json:"bytes,omitempty"`
	Truncated   bool              `json:"truncated,omitempty"`
}

// Span is one node in the request's waterfall.
type Span struct {
	SpanID       string         `json:"span_id"`
	ParentSpanID string         `json:"parent_span_id,omitempty"`
	TraceID      string         `json:"trace_id"`
	RequestID    string         `json:"request_id"`
	Name         string         `json:"name"`
	Type         SpanType       `json:"type"`
	StartedAt    int64          `json:"started_at"`
	EndedAt      int64          `json:"ended_at"`
	DurationMs   float64        `json:"duration_ms"`
	Status       string         `json:"status,omitempty"` // ok | error | unset
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// Query is a DB span enriched for the Queries tab + N+1 detection.
type Query struct {
	SpanID              string  `json:"span_id"`
	RequestID           string  `json:"request_id"`
	DBSystem            string  `json:"db_system,omitempty"`
	Statement           string  `json:"statement"`
	StatementNormalized string  `json:"statement_normalized"` // params stripped — N+1 group key
	DurationMs          float64 `json:"duration_ms"`
	StartedAt           int64   `json:"started_at"`
	Error               bool    `json:"error,omitempty"`
}

// Outbound is an outbound HTTP-client span.
type Outbound struct {
	SpanID     string  `json:"span_id"`
	RequestID  string  `json:"request_id"`
	Method     string  `json:"method,omitempty"`
	URL        string  `json:"url"`
	StatusCode int     `json:"status_code,omitempty"`
	DurationMs float64 `json:"duration_ms"`
	StartedAt  int64   `json:"started_at"`
	Error      bool    `json:"error,omitempty"`
}

// Log is a captured log line. RequestID may be empty (unattributed) — we never guess.
type Log struct {
	ID         string         `json:"id"`
	RequestID  string         `json:"request_id,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	SpanID     string         `json:"span_id,omitempty"`
	At         int64          `json:"at"` // unix nanoseconds
	Level      string         `json:"level,omitempty"`
	Message    string         `json:"message"`
	Source     string         `json:"source,omitempty"` // console | pino | winston
	Attributes map[string]any `json:"attributes,omitempty"`
}

// Exception is a captured error with its stack.
type Exception struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
	Type      string `json:"type,omitempty"`
	Message   string `json:"message"`
	Stack     string `json:"stack,omitempty"`
	At        int64  `json:"at"`
}

// Detection is a flagged problem (N+1, slow, etc.) with evidence + a suggestion.
type Detection struct {
	ID         string         `json:"id,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Type       DetectionType  `json:"type"`
	Severity   string         `json:"severity"` // info | warn | high
	Title      string         `json:"title"`
	Summary    string         `json:"summary"`
	Evidence   map[string]any `json:"evidence,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"`
}
