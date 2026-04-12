-- 0001_init.sql: Initial schema for Aetronyx M1
-- Creates all 9 tables required for baseline functionality

CREATE TABLE IF NOT EXISTS runs (
  id              TEXT PRIMARY KEY,
  spec_path       TEXT NOT NULL,
  spec_name       TEXT NOT NULL,
  spec_hash       TEXT NOT NULL,
  workspace_path  TEXT NOT NULL,
  status          TEXT NOT NULL,
  started_at      INTEGER NOT NULL,
  completed_at    INTEGER,
  total_cost_usd  REAL NOT NULL DEFAULT 0,
  total_input_tokens  INTEGER NOT NULL DEFAULT 0,
  total_output_tokens INTEGER NOT NULL DEFAULT 0,
  iterations      INTEGER NOT NULL DEFAULT 0,
  budget_json     TEXT NOT NULL,
  user_id         TEXT NOT NULL DEFAULT 'local',
  exit_reason     TEXT,
  metadata_json   TEXT
);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at DESC);

CREATE TABLE IF NOT EXISTS iterations (
  id              TEXT PRIMARY KEY,
  run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  iter_number     INTEGER NOT NULL,
  started_at      INTEGER NOT NULL,
  completed_at    INTEGER,
  model           TEXT NOT NULL,
  provider        TEXT NOT NULL,
  input_tokens    INTEGER NOT NULL DEFAULT 0,
  output_tokens   INTEGER NOT NULL DEFAULT 0,
  cost_usd        REAL NOT NULL DEFAULT 0,
  status          TEXT NOT NULL,
  error           TEXT,
  UNIQUE(run_id, iter_number)
);
CREATE INDEX IF NOT EXISTS idx_iter_run ON iterations(run_id);

CREATE TABLE IF NOT EXISTS audit_events (
  id              TEXT PRIMARY KEY,
  run_id          TEXT REFERENCES runs(id),
  iteration_id    TEXT REFERENCES iterations(id),
  ts              INTEGER NOT NULL,
  event_type      TEXT NOT NULL,
  actor           TEXT NOT NULL,
  payload_json    TEXT NOT NULL,
  payload_hash    TEXT NOT NULL,
  prev_hash       TEXT NOT NULL,
  signature       TEXT NOT NULL,
  otel_trace_id   TEXT,
  otel_span_id    TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_run ON audit_events(run_id);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_events(ts);
CREATE INDEX IF NOT EXISTS idx_audit_type ON audit_events(event_type);

CREATE TABLE IF NOT EXISTS specs (
  id              TEXT PRIMARY KEY,
  path            TEXT NOT NULL,
  name            TEXT NOT NULL,
  current_hash    TEXT NOT NULL,
  first_seen_at   INTEGER NOT NULL,
  last_run_id     TEXT REFERENCES runs(id),
  UNIQUE(path)
);

CREATE TABLE IF NOT EXISTS checkpoints (
  id              TEXT PRIMARY KEY,
  run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  iteration_id    TEXT NOT NULL REFERENCES iterations(id) ON DELETE CASCADE,
  git_commit      TEXT NOT NULL,
  shadow_branch   TEXT NOT NULL,
  files_changed   INTEGER NOT NULL,
  created_at      INTEGER NOT NULL,
  label           TEXT
);
CREATE INDEX IF NOT EXISTS idx_ckpt_run ON checkpoints(run_id);

CREATE TABLE IF NOT EXISTS approvals (
  id              TEXT PRIMARY KEY,
  run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  iteration_id    TEXT REFERENCES iterations(id),
  gate            TEXT NOT NULL,
  decision        TEXT NOT NULL,
  rationale       TEXT,
  decided_by      TEXT NOT NULL,
  decided_at      INTEGER NOT NULL,
  diff_hash       TEXT
);

CREATE TABLE IF NOT EXISTS tool_calls (
  id              TEXT PRIMARY KEY,
  run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  iteration_id    TEXT REFERENCES iterations(id),
  ts              INTEGER NOT NULL,
  tool            TEXT NOT NULL,
  params_json     TEXT NOT NULL,
  result_json     TEXT,
  error           TEXT,
  duration_ms     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS file_changes (
  id              TEXT PRIMARY KEY,
  run_id          TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  iteration_id    TEXT NOT NULL REFERENCES iterations(id) ON DELETE CASCADE,
  path            TEXT NOT NULL,
  before_hash     TEXT,
  after_hash      TEXT NOT NULL,
  diff_text       TEXT,
  bytes_added     INTEGER NOT NULL,
  bytes_removed   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fc_run ON file_changes(run_id);

CREATE TABLE IF NOT EXISTS config_kv (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
