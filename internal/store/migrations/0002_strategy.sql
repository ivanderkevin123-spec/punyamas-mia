CREATE TABLE opportunities (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  parent_run_id TEXT NOT NULL REFERENCES research_runs(id),
  proposal_index INTEGER NOT NULL,
  title TEXT NOT NULL,
  segment TEXT NOT NULL,
  topic_key TEXT,
  need TEXT NOT NULL,
  interpretation TEXT NOT NULL,
  status TEXT NOT NULL,
  tier TEXT NOT NULL,
  hypothesis INTEGER NOT NULL DEFAULT 0,
  requires_store INTEGER NOT NULL DEFAULT 0,
  dimensions_json TEXT NOT NULL,
  ordinal_sum INTEGER NOT NULL DEFAULT 0,
  caps_json TEXT,
  gates_json TEXT NOT NULL,
  conflicts_json TEXT,
  brief_json TEXT NOT NULL,
  merged_from_json TEXT,
  reject_reasons_json TEXT,
  config_hash TEXT NOT NULL,
  scoring_config_hash TEXT NOT NULL,
  fixture_hash TEXT,
  agent_execution_id TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_opp_run ON opportunities(run_id);
CREATE INDEX idx_opp_parent ON opportunities(parent_run_id);

CREATE TABLE opportunity_evidence (
  opportunity_id TEXT NOT NULL REFERENCES opportunities(id),
  item_id TEXT NOT NULL REFERENCES research_items(id),
  role TEXT NOT NULL,
  PRIMARY KEY(opportunity_id, item_id)
);
