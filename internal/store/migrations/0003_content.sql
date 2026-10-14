-- Phase 4: Content Agent drafts, versions and evidence links; versioned, append-only review decisions.
-- Content runs reuse research_runs (kind = 'content', parent_run_id = strategy run), run_steps,
-- agent_executions and cost_records.

-- Review decisions stay in approval_records; content drafts record the version and hash they apply to.
ALTER TABLE approval_records ADD COLUMN subject_version INTEGER;
ALTER TABLE approval_records ADD COLUMN subject_hash TEXT;
CREATE INDEX idx_approval_subject ON approval_records(subject_type, subject_id);
CREATE TRIGGER approval_records_no_update BEFORE UPDATE ON approval_records
BEGIN SELECT RAISE(ABORT, 'approval_records is append-only'); END;
CREATE TRIGGER approval_records_no_delete BEFORE DELETE ON approval_records
BEGIN SELECT RAISE(ABORT, 'approval_records is append-only'); END;

CREATE TABLE content_drafts (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES research_runs(id),
  opportunity_id TEXT NOT NULL REFERENCES opportunities(id),
  strategy_run_id TEXT NOT NULL REFERENCES research_runs(id),
  research_run_id TEXT NOT NULL REFERENCES research_runs(id),
  proposal_index INTEGER NOT NULL,
  content_type TEXT NOT NULL CHECK (content_type IN ('instagram_caption','headline_variants','education','short_video_script','whatsapp_message','article_faq')),
  variant INTEGER NOT NULL CHECK (variant >= 1),
  channel TEXT NOT NULL,
  target_segment TEXT NOT NULL,
  objective TEXT NOT NULL,
  hook TEXT NOT NULL DEFAULT '',
  headline TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  call_to_action TEXT NOT NULL DEFAULT '',
  claims_json TEXT NOT NULL DEFAULT '[]',
  placeholders_json TEXT NOT NULL DEFAULT '[]',
  assumptions_json TEXT NOT NULL DEFAULT '[]',
  risks_json TEXT NOT NULL DEFAULT '[]',
  checklist_json TEXT NOT NULL DEFAULT '[]',
  findings_json TEXT NOT NULL DEFAULT '[]',
  brand_fact_ids_json TEXT NOT NULL DEFAULT '[]',
  status TEXT NOT NULL CHECK (status IN ('draft','needs_review','approved','rejected','blocked')),
  version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
  content_hash TEXT NOT NULL,
  execution_id TEXT NOT NULL REFERENCES agent_executions(id),
  config_hash TEXT NOT NULL,
  content_config_hash TEXT NOT NULL,
  scoring_config_hash TEXT NOT NULL,
  fixture_hash TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (id, opportunity_id),
  UNIQUE (run_id, proposal_index),
  UNIQUE (opportunity_id, content_type, channel, content_hash)
);
CREATE INDEX idx_content_drafts_run ON content_drafts(run_id);
CREATE INDEX idx_content_drafts_opp ON content_drafts(opportunity_id);

-- Eligibility at the storage layer: drafts exist only for approved opportunities.
CREATE TRIGGER content_drafts_require_approved_opportunity BEFORE INSERT ON content_drafts
WHEN (SELECT status FROM opportunities WHERE id = NEW.opportunity_id) IS NOT 'approved'
BEGIN SELECT RAISE(ABORT, 'content draft requires an approved opportunity'); END;
-- No automatic decisions: a new draft never starts approved or rejected.
CREATE TRIGGER content_drafts_no_auto_decision BEFORE INSERT ON content_drafts
WHEN NEW.status IN ('approved','rejected')
BEGIN SELECT RAISE(ABORT, 'a new content draft cannot carry a review decision'); END;
-- Approval only with a review record for the current version and content hash.
CREATE TRIGGER content_drafts_approval_needs_record BEFORE UPDATE OF status ON content_drafts
WHEN NEW.status = 'approved' AND NOT EXISTS (
  SELECT 1 FROM approval_records WHERE subject_type = 'content_draft' AND subject_id = NEW.id
    AND decision = 'approved' AND subject_version = NEW.version AND subject_hash = NEW.content_hash)
BEGIN SELECT RAISE(ABORT, 'approved status requires a review record for the current version'); END;
-- Blocked drafts cannot be reviewed.
CREATE TRIGGER content_drafts_blocked_not_reviewable BEFORE UPDATE OF status ON content_drafts
WHEN OLD.status = 'blocked' AND NEW.status IN ('approved','rejected')
BEGIN SELECT RAISE(ABORT, 'a blocked content draft cannot be reviewed'); END;
-- A text change after a review decision always returns the draft to needs_review.
CREATE TRIGGER content_drafts_change_after_review AFTER UPDATE OF hook, headline, body, call_to_action ON content_drafts
WHEN OLD.status IN ('approved','rejected') AND NEW.status = OLD.status
  AND (NEW.hook IS NOT OLD.hook OR NEW.headline IS NOT OLD.headline OR NEW.body IS NOT OLD.body OR NEW.call_to_action IS NOT OLD.call_to_action)
BEGIN UPDATE content_drafts SET status = 'needs_review' WHERE id = NEW.id; END;
CREATE TRIGGER content_drafts_no_delete BEFORE DELETE ON content_drafts
BEGIN SELECT RAISE(ABORT, 'content drafts are kept; reject a draft instead of deleting it'); END;

-- A draft may only cite evidence linked to its own opportunity (composite foreign keys).
CREATE TABLE content_draft_evidence (
  draft_id TEXT NOT NULL,
  opportunity_id TEXT NOT NULL,
  item_id TEXT NOT NULL,
  role TEXT NOT NULL,
  PRIMARY KEY (draft_id, item_id),
  FOREIGN KEY (draft_id, opportunity_id) REFERENCES content_drafts(id, opportunity_id),
  FOREIGN KEY (opportunity_id, item_id) REFERENCES opportunity_evidence(opportunity_id, item_id)
);
CREATE TRIGGER content_draft_evidence_no_update BEFORE UPDATE ON content_draft_evidence
BEGIN SELECT RAISE(ABORT, 'content_draft_evidence is append-only'); END;
CREATE TRIGGER content_draft_evidence_no_delete BEFORE DELETE ON content_draft_evidence
BEGIN SELECT RAISE(ABORT, 'content_draft_evidence is append-only'); END;

-- Every text version of a draft, agent-generated or human-revised (append-only).
CREATE TABLE content_draft_versions (
  draft_id TEXT NOT NULL REFERENCES content_drafts(id),
  version INTEGER NOT NULL CHECK (version >= 1),
  hook TEXT NOT NULL DEFAULT '',
  headline TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  call_to_action TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL,
  author TEXT NOT NULL CHECK (author IN ('agent','human')),
  note TEXT,
  status_after TEXT NOT NULL,
  findings_json TEXT NOT NULL DEFAULT '[]',
  placeholders_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  PRIMARY KEY (draft_id, version)
);
CREATE TRIGGER content_draft_versions_no_update BEFORE UPDATE ON content_draft_versions
BEGIN SELECT RAISE(ABORT, 'content_draft_versions is append-only'); END;
CREATE TRIGGER content_draft_versions_no_delete BEFORE DELETE ON content_draft_versions
BEGIN SELECT RAISE(ABORT, 'content_draft_versions is append-only'); END;
