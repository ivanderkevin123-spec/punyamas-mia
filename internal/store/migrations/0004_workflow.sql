-- Phase 5: workflow orchestration records. A workflow links the existing research, strategy and
-- content runs; it never replaces their authoritative statuses.
CREATE TABLE workflows (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN ('created','research_completed','strategy_completed','awaiting_opportunity_review','awaiting_content_review','completed','blocked','failed')),
  inputs_dir TEXT NOT NULL,
  fixture_key TEXT NOT NULL,
  -- Run IDs are reserved here before the run row exists (crash-safe idempotency), so they carry no
  -- foreign key; the workflow assessor checks that a reserved run has the expected kind and parent.
  research_run_id TEXT UNIQUE,
  strategy_run_id TEXT UNIQUE,
  last_completed_stage TEXT,
  failed_stage TEXT,
  reason TEXT,
  summary_json TEXT NOT NULL DEFAULT '{}',
  config_hash TEXT NOT NULL,
  lease_owner TEXT,
  lease_expires_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  CHECK (strategy_run_id IS NULL OR research_run_id IS NOT NULL)
);
CREATE INDEX idx_workflows_inputs ON workflows(inputs_dir);
CREATE TRIGGER workflows_run_links_immutable BEFORE UPDATE OF research_run_id, strategy_run_id ON workflows
WHEN (OLD.research_run_id IS NOT NULL AND NEW.research_run_id IS NOT OLD.research_run_id)
  OR (OLD.strategy_run_id IS NOT NULL AND NEW.strategy_run_id IS NOT OLD.strategy_run_id)
BEGIN SELECT RAISE(ABORT, 'workflow run links are immutable once set'); END;
CREATE TRIGGER workflows_no_delete BEFORE DELETE ON workflows
BEGIN SELECT RAISE(ABORT, 'workflows are kept for audit'); END;

-- One content run per approved opportunity and attempt; a new attempt is only created on explicit request.
CREATE TABLE workflow_content_runs (
  workflow_id TEXT NOT NULL REFERENCES workflows(id),
  opportunity_id TEXT NOT NULL REFERENCES opportunities(id),
  attempt INTEGER NOT NULL CHECK (attempt >= 1),
  content_run_id TEXT NOT NULL UNIQUE,
  origin TEXT NOT NULL CHECK (origin IN ('workflow','adopted')),
  created_at TEXT NOT NULL,
  PRIMARY KEY (workflow_id, opportunity_id, attempt)
);
CREATE TRIGGER workflow_content_runs_no_update BEFORE UPDATE ON workflow_content_runs
BEGIN SELECT RAISE(ABORT, 'workflow_content_runs is append-only'); END;
CREATE TRIGGER workflow_content_runs_no_delete BEFORE DELETE ON workflow_content_runs
BEGIN SELECT RAISE(ABORT, 'workflow_content_runs is append-only'); END;

-- Audit trail of every workflow command, stage start/end, gate stop, resume and export.
CREATE TABLE workflow_events (
  workflow_id TEXT NOT NULL REFERENCES workflows(id),
  seq INTEGER NOT NULL,
  kind TEXT NOT NULL,
  stage TEXT,
  ref_id TEXT,
  detail TEXT,
  created_at TEXT NOT NULL,
  PRIMARY KEY (workflow_id, seq)
);
CREATE TRIGGER workflow_events_no_update BEFORE UPDATE ON workflow_events
BEGIN SELECT RAISE(ABORT, 'workflow_events is append-only'); END;
CREATE TRIGGER workflow_events_no_delete BEFORE DELETE ON workflow_events
BEGIN SELECT RAISE(ABORT, 'workflow_events is append-only'); END;
