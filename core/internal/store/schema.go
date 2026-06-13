package store

const schema = `
CREATE TABLE IF NOT EXISTS requests (
  id           TEXT PRIMARY KEY,
  trace_id     TEXT NOT NULL,
  service      TEXT,
  method       TEXT,
  path         TEXT,
  route        TEXT,
  status_code  INTEGER,
  started_at   INTEGER,
  ended_at     INTEGER,
  duration_ms  REAL,
  error        INTEGER,
  req_json     TEXT,
  resp_json    TEXT
);
CREATE INDEX IF NOT EXISTS idx_requests_started ON requests(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_requests_status  ON requests(status_code);

CREATE TABLE IF NOT EXISTS spans (
  span_id            TEXT PRIMARY KEY,
  request_id         TEXT NOT NULL,
  trace_id           TEXT NOT NULL,
  parent_span_id     TEXT,
  name               TEXT,
  type               TEXT,
  started_at         INTEGER,
  ended_at           INTEGER,
  duration_ms        REAL,
  status             TEXT,
  attrs_json         TEXT,
  db_system          TEXT,
  db_statement       TEXT,
  db_statement_norm  TEXT,
  http_method        TEXT,
  http_url           TEXT,
  http_status_code   INTEGER
);
CREATE INDEX IF NOT EXISTS idx_spans_request ON spans(request_id);
CREATE INDEX IF NOT EXISTS idx_spans_type    ON spans(request_id, type);

CREATE TABLE IF NOT EXISTS logs (
  id          TEXT PRIMARY KEY,
  request_id  TEXT,
  trace_id    TEXT,
  span_id     TEXT,
  at          INTEGER,
  level       TEXT,
  message     TEXT,
  source      TEXT,
  attrs_json  TEXT
);
CREATE INDEX IF NOT EXISTS idx_logs_request ON logs(request_id);

CREATE TABLE IF NOT EXISTS exceptions (
  id          TEXT PRIMARY KEY,
  request_id  TEXT,
  span_id     TEXT,
  type        TEXT,
  message     TEXT,
  stack       TEXT,
  at          INTEGER
);
CREATE INDEX IF NOT EXISTS idx_exceptions_request ON exceptions(request_id);
`
