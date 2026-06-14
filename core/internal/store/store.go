// Package store is rewynd's SQLite-backed memory: a single writer, many readers (WAL).
// Raw spans/logs/exceptions go in; correlated Requests come out, assembled at read time.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/SrinjoyDev/rewynd/internal/detect"
	"github.com/SrinjoyDev/rewynd/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	wmu sync.Mutex // serialize writes; WAL handles concurrent reads
}

// Batch is one OTLP export decoded into rows. Requests carry only the http_server-derived
// summary; child spans/logs/exceptions are stored raw and re-joined on read.
type Batch struct {
	Requests   []model.Request
	Spans      []model.Span
	Logs       []model.Log
	Exceptions []model.Exception
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=OFF",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) WriteBatch(b Batch) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, r := range b.Requests {
		if _, err := tx.Exec(`
			INSERT INTO requests (id,trace_id,service,method,path,route,status_code,started_at,ended_at,duration_ms,error,req_json,resp_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
			  method=excluded.method, path=excluded.path, route=excluded.route,
			  status_code=excluded.status_code, started_at=excluded.started_at,
			  ended_at=excluded.ended_at, duration_ms=excluded.duration_ms, error=excluded.error`,
			r.ID, r.TraceID, r.Service, r.Method, r.Path, r.Route, r.StatusCode,
			r.StartedAt, r.EndedAt, r.DurationMs, boolToInt(r.Error),
			jsonOrNil(r.Request), jsonOrNil(r.Response),
		); err != nil {
			return fmt.Errorf("insert request: %w", err)
		}
	}

	for _, sp := range b.Spans {
		if _, err := tx.Exec(`
			INSERT INTO spans (span_id,request_id,trace_id,parent_span_id,name,type,started_at,ended_at,duration_ms,status,attrs_json,db_system,db_statement,db_statement_norm,http_method,http_url,http_status_code)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(span_id) DO NOTHING`,
			sp.SpanID, sp.RequestID, sp.TraceID, sp.ParentSpanID, sp.Name, string(sp.Type),
			sp.StartedAt, sp.EndedAt, sp.DurationMs, sp.Status, marshalAttrs(sp.Attributes),
			attr(sp, "db.system"), attr(sp, "db.statement"), attr(sp, "db.statement.norm"),
			attr(sp, "http.method"), attr(sp, "http.url"), attrInt(sp, "http.status_code"),
		); err != nil {
			return fmt.Errorf("insert span: %w", err)
		}
	}

	for _, l := range b.Logs {
		if _, err := tx.Exec(`
			INSERT INTO logs (id,request_id,trace_id,span_id,at,level,message,source,attrs_json)
			VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
			l.ID, l.RequestID, l.TraceID, l.SpanID, l.At, l.Level, l.Message, l.Source, marshalAttrs(l.Attributes),
		); err != nil {
			return fmt.Errorf("insert log: %w", err)
		}
	}

	for _, e := range b.Exceptions {
		if _, err := tx.Exec(`
			INSERT INTO exceptions (id,request_id,span_id,type,message,stack,at)
			VALUES (?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
			e.ID, e.RequestID, e.SpanID, e.Type, e.Message, e.Stack, e.At,
		); err != nil {
			return fmt.Errorf("insert exception: %w", err)
		}
	}

	return tx.Commit()
}

type ListOptions struct {
	StatusClass string // "2xx" | "4xx" | "5xx" | ""
	PathLike    string
	HasError    bool
	Slow        bool
	SlowMs      float64
	Limit       int
}

func (s *Store) ListRequests(opts ListOptions) ([]model.Request, error) {
	where, args := []string{}, []any{}
	switch opts.StatusClass {
	case "2xx":
		where = append(where, "status_code>=200 AND status_code<300")
	case "4xx":
		where = append(where, "status_code>=400 AND status_code<500")
	case "5xx":
		where = append(where, "status_code>=500")
	}
	if opts.HasError {
		where = append(where, "error=1")
	}
	if opts.PathLike != "" {
		where = append(where, "path LIKE ?")
		args = append(args, "%"+opts.PathLike+"%")
	}
	if opts.Slow {
		ms := opts.SlowMs
		if ms <= 0 {
			ms = 500
		}
		where = append(where, "duration_ms>=?")
		args = append(args, ms)
	}
	q := "SELECT id,trace_id,service,method,path,route,status_code,started_at,ended_at,duration_ms,error FROM requests"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	q += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []model.Request
	ids := []string{}
	for rows.Next() {
		r, err := scanRequestRow(rows)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(reqs) == 0 {
		return reqs, nil
	}
	if err := s.enrichSummaries(reqs, ids); err != nil {
		return nil, err
	}
	return reqs, nil
}

// enrichSummaries fills Counts and the N+1 detection flag for a page of requests using a
// handful of grouped queries (no per-request round trips).
func (s *Store) enrichSummaries(reqs []model.Request, ids []string) error {
	in, inArgs := inClause(ids)
	byID := map[string]*model.Request{}
	for i := range reqs {
		byID[reqs[i].ID] = &reqs[i]
	}

	spanRows, err := s.db.Query("SELECT request_id,type,COUNT(*) FROM spans WHERE request_id IN "+in+" GROUP BY request_id,type", inArgs...)
	if err != nil {
		return err
	}
	for spanRows.Next() {
		var rid, typ string
		var n int
		if err := spanRows.Scan(&rid, &typ, &n); err != nil {
			spanRows.Close()
			return err
		}
		if r := byID[rid]; r != nil {
			switch model.SpanType(typ) {
			case model.SpanDBQuery:
				r.Counts.Queries = n
			case model.SpanHTTPClient:
				r.Counts.Outbound = n
			}
		}
	}
	spanRows.Close()

	if err := s.countInto("SELECT request_id,COUNT(*) FROM logs WHERE request_id IN "+in+" GROUP BY request_id", inArgs, byID, func(r *model.Request, n int) { r.Counts.Logs = n }); err != nil {
		return err
	}
	if err := s.countInto("SELECT request_id,COUNT(*) FROM exceptions WHERE request_id IN "+in+" GROUP BY request_id", inArgs, byID, func(r *model.Request, n int) { r.Counts.Exceptions = n }); err != nil {
		return err
	}

	npRows, err := s.db.Query("SELECT request_id,db_statement_norm,COUNT(*) c FROM spans WHERE type=? AND db_statement_norm IS NOT NULL AND db_statement_norm!='' AND request_id IN "+in+" GROUP BY request_id,db_statement_norm HAVING c>=?",
		append(append([]any{string(model.SpanDBQuery)}, inArgs...), detect.DefaultNPlusOneThreshold)...)
	if err != nil {
		return err
	}
	for npRows.Next() {
		var rid, norm string
		var c int
		if err := npRows.Scan(&rid, &norm, &c); err != nil {
			npRows.Close()
			return err
		}
		if r := byID[rid]; r != nil {
			r.Detections = append(r.Detections, model.Detection{
				RequestID: rid, Type: model.DetectNPlusOne, Severity: "high",
				Title:    fmt.Sprintf("N+1 query — %d identical statements", c),
				Evidence: map[string]any{"statement_normalized": norm, "count": c},
			})
		}
	}
	npRows.Close()
	return nil
}

func (s *Store) countInto(query string, args []any, byID map[string]*model.Request, set func(*model.Request, int)) error {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rid string
		var n int
		if err := rows.Scan(&rid, &n); err != nil {
			return err
		}
		if r := byID[rid]; r != nil {
			set(r, n)
		}
	}
	return rows.Err()
}

// GetRequest returns the full correlated trace for an id (exact or unambiguous prefix).
func (s *Store) GetRequest(idOrPrefix string) (*model.Request, error) {
	id, err := s.resolveID(idOrPrefix)
	if err != nil {
		return nil, err
	}
	row := s.db.QueryRow("SELECT id,trace_id,service,method,path,route,status_code,started_at,ended_at,duration_ms,error FROM requests WHERE id=?", id)
	r, err := scanRequestRow(row)
	if err != nil {
		return nil, err
	}

	var reqJSON, respJSON sql.NullString
	if err := s.db.QueryRow("SELECT req_json, resp_json FROM requests WHERE id=?", id).Scan(&reqJSON, &respJSON); err == nil {
		if reqJSON.String != "" {
			_ = json.Unmarshal([]byte(reqJSON.String), &r.Request)
		}
		if respJSON.String != "" {
			_ = json.Unmarshal([]byte(respJSON.String), &r.Response)
		}
	}

	spanRows, err := s.db.Query("SELECT span_id,parent_span_id,trace_id,name,type,started_at,ended_at,duration_ms,status,attrs_json,db_system,db_statement,db_statement_norm,http_method,http_url,http_status_code FROM spans WHERE request_id=? ORDER BY started_at", id)
	if err != nil {
		return nil, err
	}
	defer spanRows.Close()
	for spanRows.Next() {
		var sp model.Span
		var attrs sql.NullString
		var dbSys, dbStmt, dbNorm, hMethod, hURL sql.NullString
		var hStatus sql.NullInt64
		if err := spanRows.Scan(&sp.SpanID, &sp.ParentSpanID, &sp.TraceID, &sp.Name, &sp.Type,
			&sp.StartedAt, &sp.EndedAt, &sp.DurationMs, &sp.Status, &attrs,
			&dbSys, &dbStmt, &dbNorm, &hMethod, &hURL, &hStatus); err != nil {
			return nil, err
		}
		sp.RequestID = id
		sp.Attributes = unmarshalAttrs(attrs.String)
		r.Spans = append(r.Spans, sp)
		switch sp.Type {
		case model.SpanDBQuery:
			r.Queries = append(r.Queries, model.Query{
				SpanID: sp.SpanID, RequestID: id, DBSystem: dbSys.String,
				Statement: dbStmt.String, StatementNormalized: dbNorm.String,
				DurationMs: sp.DurationMs, StartedAt: sp.StartedAt, Error: sp.Status == "error",
			})
		case model.SpanHTTPClient:
			r.Outbound = append(r.Outbound, model.Outbound{
				SpanID: sp.SpanID, RequestID: id, Method: hMethod.String, URL: hURL.String,
				StatusCode: int(hStatus.Int64), DurationMs: sp.DurationMs, StartedAt: sp.StartedAt,
				Error: sp.Status == "error",
			})
		}
	}

	if err := s.loadLogs(&r, id); err != nil {
		return nil, err
	}
	if err := s.loadExceptions(&r, id); err != nil {
		return nil, err
	}

	r.Detections = detect.NPlusOne(id, r.Queries, detect.DefaultNPlusOneThreshold)
	r.Detections = append(r.Detections, detect.SlowQueries(id, r.Queries, 0)...)
	r.Detections = append(r.Detections, detect.SlowRequest(id, r.DurationMs, 0)...)
	r.Counts = model.Counts{
		Queries: len(r.Queries), Outbound: len(r.Outbound),
		Logs: len(r.Logs), Exceptions: len(r.Exceptions),
	}
	return &r, nil
}

func (s *Store) loadLogs(r *model.Request, id string) error {
	rows, err := s.db.Query("SELECT id,trace_id,span_id,at,level,message,source,attrs_json FROM logs WHERE request_id=? ORDER BY at", id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var l model.Log
		var attrs sql.NullString
		if err := rows.Scan(&l.ID, &l.TraceID, &l.SpanID, &l.At, &l.Level, &l.Message, &l.Source, &attrs); err != nil {
			return err
		}
		l.RequestID = id
		l.Attributes = unmarshalAttrs(attrs.String)
		r.Logs = append(r.Logs, l)
	}
	return rows.Err()
}

func (s *Store) loadExceptions(r *model.Request, id string) error {
	rows, err := s.db.Query("SELECT id,span_id,type,message,stack,at FROM exceptions WHERE request_id=? ORDER BY at", id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e model.Exception
		if err := rows.Scan(&e.ID, &e.SpanID, &e.Type, &e.Message, &e.Stack, &e.At); err != nil {
			return err
		}
		e.RequestID = id
		r.Exceptions = append(r.Exceptions, e)
	}
	return rows.Err()
}

func (s *Store) resolveID(p string) (string, error) {
	var id string
	err := s.db.QueryRow("SELECT id FROM requests WHERE id=?", p).Scan(&id)
	if err == nil {
		return id, nil
	}
	rows, err := s.db.Query("SELECT id FROM requests WHERE id LIKE ? LIMIT 2", p+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var matches []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return "", err
		}
		matches = append(matches, m)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no request matches %q", p)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous id %q (matches multiple)", p)
	}
}

func (s *Store) Clear() error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	for _, t := range []string{"requests", "spans", "logs", "exceptions"} {
		if _, err := s.db.Exec("DELETE FROM " + t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Count() (int, error) {
	var n int
	return n, s.db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&n)
}

// Prune keeps the newest max requests (and their children), enforcing the ring buffer.
func (s *Store) Prune(max int) error {
	if max <= 0 {
		return nil
	}
	s.wmu.Lock()
	defer s.wmu.Unlock()
	rows, err := s.db.Query("SELECT id FROM requests ORDER BY started_at DESC LIMIT -1 OFFSET ?", max)
	if err != nil {
		return err
	}
	var old []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		old = append(old, id)
	}
	rows.Close()
	if len(old) == 0 {
		return nil
	}
	in, args := inClause(old)
	for _, t := range []string{"requests", "spans", "logs", "exceptions"} {
		col := "request_id"
		if t == "requests" {
			col = "id"
		}
		if _, err := s.db.Exec("DELETE FROM "+t+" WHERE "+col+" IN "+in, args...); err != nil {
			return err
		}
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanRequestRow(row rowScanner) (model.Request, error) {
	var r model.Request
	var service, route sql.NullString
	var errInt sql.NullInt64
	if err := row.Scan(&r.ID, &r.TraceID, &service, &r.Method, &r.Path, &route,
		&r.StatusCode, &r.StartedAt, &r.EndedAt, &r.DurationMs, &errInt); err != nil {
		return r, err
	}
	r.SchemaVersion = model.SchemaVersion
	r.Service = service.String
	r.Route = route.String
	r.Error = errInt.Int64 != 0
	return r, nil
}

func inClause(ids []string) (string, []any) {
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return "(" + strings.Join(ph, ",") + ")", args
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func marshalAttrs(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return string(b)
}

func unmarshalAttrs(s string) map[string]any {
	if s == "" {
		return nil
	}
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) != nil {
		return nil
	}
	return m
}

func jsonOrNil(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

func attr(sp model.Span, key string) any {
	if v, ok := sp.Attributes[key]; ok {
		return fmt.Sprint(v)
	}
	return nil
}

func attrInt(sp model.Span, key string) any {
	v, ok := sp.Attributes[key]
	if !ok {
		return nil
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return n
	case float64:
		return int(n)
	}
	return nil
}
