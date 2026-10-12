CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE research_runs (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  heartbeat_at TEXT,
  config_hash TEXT NOT NULL,
  provider_name TEXT NOT NULL,
  inputs_json TEXT NOT NULL DEFAULT '{}',
  summary_json TEXT NOT NULL DEFAULT '{}',
  cost_usd_total REAL,
  cost_measurable INTEGER NOT NULL DEFAULT 1,
  parent_run_id TEXT REFERENCES research_runs(id)
);
CREATE INDEX idx_runs_kind_created ON research_runs(kind, created_at);

CREATE TABLE run_steps (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  name TEXT NOT NULL,
  ordinal INTEGER NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  started_at TEXT,
  finished_at TEXT,
  error_json TEXT,
  UNIQUE(run_id, name)
);

CREATE TABLE sources (
  id TEXT PRIMARY KEY,
  source_type TEXT NOT NULL,
  name TEXT NOT NULL,
  reference TEXT NOT NULL,
  canonical_url TEXT,
  publisher TEXT,
  retrieved_at TEXT NOT NULL,
  published_at TEXT,
  content_hash TEXT,
  snapshot_path TEXT,
  status TEXT NOT NULL,
  status_reason TEXT,
  allowlist_entry TEXT,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_sources_identity ON sources(reference, content_hash);
CREATE INDEX idx_sources_canonical ON sources(canonical_url);

CREATE TABLE run_sources (
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  source_id TEXT NOT NULL REFERENCES sources(id),
  PRIMARY KEY(run_id, source_id)
);

CREATE TABLE research_items (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  ref TEXT NOT NULL,
  origin TEXT NOT NULL,
  item_kind TEXT NOT NULL,
  topic_key TEXT,
  title TEXT NOT NULL,
  statement TEXT NOT NULL,
  rationale TEXT,
  language TEXT NOT NULL DEFAULT 'id',
  evidence_quality TEXT NOT NULL,
  status TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  duplicate_of TEXT REFERENCES research_items(id),
  conflict_group TEXT,
  claim_check_json TEXT,
  suspicious INTEGER NOT NULL DEFAULT 0,
  agent_execution_id TEXT,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_items_identity ON research_items(origin, ref, content_hash);
CREATE INDEX idx_items_run_kind ON research_items(run_id, item_kind);
CREATE INDEX idx_items_topic ON research_items(topic_key);

CREATE TABLE run_items (
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  item_id TEXT NOT NULL REFERENCES research_items(id),
  PRIMARY KEY(run_id, item_id)
);

CREATE TABLE evidence (
  id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL REFERENCES research_items(id),
  source_id TEXT REFERENCES sources(id),
  from_item_id TEXT REFERENCES research_items(id),
  excerpt TEXT,
  locator TEXT,
  retrieved_at TEXT,
  published_at TEXT,
  note TEXT,
  CHECK ((source_id IS NOT NULL) <> (from_item_id IS NOT NULL))
);
CREATE INDEX idx_evidence_item ON evidence(item_id);
CREATE INDEX idx_evidence_source ON evidence(source_id);

CREATE TABLE imported_assets (
  id TEXT PRIMARY KEY,
  asset_key TEXT NOT NULL,
  root TEXT NOT NULL,
  relative_path TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  mtime TEXT NOT NULL,
  workspace_commit TEXT,
  imported_at TEXT NOT NULL,
  parsed_summary_json TEXT,
  review_status TEXT NOT NULL,
  accepted_at TEXT,
  accepted_by_note TEXT
);
CREATE INDEX idx_assets_key ON imported_assets(asset_key, imported_at DESC);

CREATE TABLE agent_executions (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  step_id TEXT REFERENCES run_steps(id),
  agent_name TEXT NOT NULL,
  provider_name TEXT NOT NULL,
  model TEXT,
  schema_version TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  request_path TEXT,
  response_path TEXT,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  cache_read_tokens INTEGER NOT NULL DEFAULT 0,
  cost_usd REAL,
  cost_measurable INTEGER NOT NULL DEFAULT 1,
  latency_ms INTEGER NOT NULL DEFAULT 0,
  attempt INTEGER NOT NULL DEFAULT 1,
  outcome TEXT NOT NULL,
  error_text TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_exec_run ON agent_executions(run_id);

CREATE TABLE cost_records (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  agent_execution_id TEXT REFERENCES agent_executions(id),
  agent_name TEXT,
  category TEXT NOT NULL,
  provider_name TEXT NOT NULL,
  units_json TEXT NOT NULL,
  unit_price_source TEXT NOT NULL,
  cost_usd REAL,
  estimated INTEGER NOT NULL DEFAULT 0,
  period_month TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_cost_period ON cost_records(period_month);
CREATE INDEX idx_cost_run ON cost_records(run_id);

CREATE TABLE approval_records (
  id TEXT PRIMARY KEY,
  subject_type TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  decision TEXT NOT NULL,
  decided_by TEXT NOT NULL,
  note TEXT,
  decided_at TEXT NOT NULL
);
