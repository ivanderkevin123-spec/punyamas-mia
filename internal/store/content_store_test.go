package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

// seedPhase3 creates a database with only migrations 0001 and 0002 applied (the Phase 3 schema) and
// Phase 3 data: research and strategy runs, two validated items, two opportunities with evidence and
// approval history, and the strategy execution.
func seedPhase3(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%v: %s", err, q)
		}
	}
	exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`)
	for _, name := range []string{"0001_init.sql", "0002_strategy.sql"} {
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(body))
		exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, 't')`, name)
	}
	exec(`INSERT INTO research_runs(id, kind, status, created_at, config_hash, provider_name) VALUES ('run_r','research','completed','t','h','mock')`)
	exec(`INSERT INTO research_runs(id, kind, status, created_at, config_hash, provider_name, parent_run_id) VALUES ('run_s','strategy','completed','t','h','mock','run_r')`)
	exec(`INSERT INTO agent_executions(id, run_id, agent_name, provider_name, schema_version, request_hash, outcome, created_at) VALUES ('exec_s','run_s','strategy','mock','strategy_proposals.v1','r','ok','t')`)
	for _, id := range []string{"item_a", "item_b"} {
		exec(`INSERT INTO research_items(id, run_id, ref, origin, item_kind, title, statement, evidence_quality, status, content_hash, created_at) VALUES (?, 'run_r', ?, 'import', 'fact', 't', 's', 'A', 'validated', ?, 't')`, id, id+"#1", id)
		exec(`INSERT INTO run_items(run_id, item_id) VALUES ('run_r', ?)`, id)
	}
	for _, o := range []struct{ id, status, item string }{{"opp_a", "approved", "item_a"}, {"opp_b", "proposed", "item_b"}} {
		exec(`INSERT INTO opportunities(id, run_id, parent_run_id, proposal_index, title, segment, need, interpretation, status, tier, dimensions_json, gates_json, brief_json, config_hash, scoring_config_hash, agent_execution_id, created_at)
VALUES (?, 'run_s', 'run_r', 1, 'judul', 'AU1', 'n', 'i', ?, 'A', '{}', '[]', '{}', 'h', 'sh', 'exec_s', 't')`, o.id, o.status)
		exec(`INSERT INTO opportunity_evidence(opportunity_id, item_id, role) VALUES (?, ?, 'fact')`, o.id, o.item)
	}
	exec(`INSERT INTO approval_records(id, subject_type, subject_id, decision, decided_by, note, decided_at) VALUES ('appr_old','opportunity','opp_a','approved','CEO','fase 3','t')`)
}

func newDraft(id, opp string) (*domain.ContentDraft, *domain.ContentDraftVersion) {
	d := &domain.ContentDraft{ID: id, RunID: "run_c", OpportunityID: opp, StrategyRunID: "run_s", ResearchRunID: "run_r", ProposalIndex: 1, ContentType: "education", Variant: 1,
		Channel: "situs", TargetSegment: "AU1", Objective: "o", Headline: "h", Body: "isi", Status: domain.DraftDraft, Version: 1, ContentHash: "hash_" + id,
		ExecutionID: "exec_c", ConfigHash: "c", ContentConfigHash: "cc", ScoringConfigHash: "sh", CreatedAt: "t", UpdatedAt: "t"}
	v := &domain.ContentDraftVersion{DraftID: id, Version: 1, Headline: "h", Body: "isi", ContentHash: d.ContentHash, Author: "agent", StatusAfter: d.Status, CreatedAt: "t"}
	return d, v
}

func TestMigrationFromPhase3DatabaseKeepsDataAndIntegrity(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mia.sqlite")
	seedPhase3(t, path)
	s, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer s.Close()
	if n, _ := s.CountRows(ctx, "schema_migrations"); n != 3 {
		t.Fatalf("migrations = %d", n)
	}
	if res, err := s.Integrity(ctx); err != nil || res != "ok" {
		t.Fatalf("integrity %q %v", res, err)
	}
	rows, err := s.DB().Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("foreign_key_check reports violations after the upgrade")
	}
	rows.Close()
	opp, err := s.GetOpportunity(ctx, "opp_a")
	if err != nil || opp.Status != domain.OppApproved {
		t.Fatalf("Phase 3 opportunity lost: %v %+v", err, opp)
	}
	apps, err := s.ApprovalsFor(ctx, "opportunity", "opp_a")
	if err != nil || len(apps) != 1 || apps[0].SubjectVersion != 0 || apps[0].SubjectHash != "" || apps[0].Note != "fase 3" {
		t.Fatalf("Phase 3 approval history: %v %+v", err, apps)
	}
	// The upgraded schema enforces the new rules on old data too.
	if _, err := s.DB().Exec(`UPDATE approval_records SET note = 'x' WHERE id = 'appr_old'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("old approval records become append-only: %v", err)
	}
	// Re-opening applies nothing twice.
	s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if n, _ := s2.CountRows(ctx, "schema_migrations"); n != 3 {
		t.Fatalf("migrations after reopen = %d", n)
	}
}

func TestContentDraftStorageRules(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mia.sqlite")
	seedPhase3(t, path)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB().Exec(`INSERT INTO research_runs(id, kind, status, created_at, config_hash, provider_name, parent_run_id) VALUES ('run_c','content','running','t','h','mock','run_s')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO agent_executions(id, run_id, agent_name, provider_name, schema_version, request_hash, outcome, created_at) VALUES ('exec_c','run_c','content','mock','content_drafts.v1','r','ok','t')`); err != nil {
		t.Fatal(err)
	}
	// A draft citing evidence of another opportunity is refused by the composite foreign key and rolled back entirely.
	d, v := newDraft("draft_x", "opp_a")
	err = s.InsertContentDraft(ctx, d, []domain.ContentDraftEvidence{{ItemID: "item_b", Role: "fact"}}, v)
	if err == nil {
		t.Fatal("cross-opportunity evidence must be refused")
	}
	if _, err := s.GetContentDraft(ctx, "draft_x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("failed insert must roll back the draft row")
	}
	if vs, _ := s.ContentDraftVersions(ctx, "draft_x"); len(vs) != 0 {
		t.Fatal("failed insert must roll back the version row")
	}
	// Drafts for an unapproved opportunity, or with a decision on creation, are refused by triggers.
	d, v = newDraft("draft_y", "opp_b")
	if err := s.InsertContentDraft(ctx, d, nil, v); err == nil || !strings.Contains(err.Error(), "approved opportunity") {
		t.Fatalf("unapproved opportunity: %v", err)
	}
	d, v = newDraft("draft_z", "opp_a")
	d.Status = domain.DraftApproved
	if err := s.InsertContentDraft(ctx, d, nil, v); err == nil || !strings.Contains(err.Error(), "review decision") {
		t.Fatalf("auto-approval: %v", err)
	}
	// A valid draft, then a stale review is refused without leaving an approval record behind.
	d, v = newDraft("draft_ok", "opp_a")
	if err := s.InsertContentDraft(ctx, d, []domain.ContentDraftEvidence{{ItemID: "item_a", Role: "fact"}}, v); err != nil {
		t.Fatal(err)
	}
	stale := &domain.ApprovalRecord{ID: "appr_stale", SubjectType: domain.SubjectContentDraft, SubjectID: "draft_ok", Decision: "approved", DecidedBy: "CEO", Note: "n", DecidedAt: "t", SubjectVersion: 2, SubjectHash: d.ContentHash}
	if err := s.ReviewContentDraft(ctx, stale, domain.DraftApproved, "t"); !errors.Is(err, ErrStale) {
		t.Fatalf("stale review: %v", err)
	}
	if apps, _ := s.ApprovalsFor(ctx, domain.SubjectContentDraft, "draft_ok"); len(apps) != 0 {
		t.Fatal("a refused review must not leave an approval record")
	}
	good := &domain.ApprovalRecord{ID: "appr_ok", SubjectType: domain.SubjectContentDraft, SubjectID: "draft_ok", Decision: "approved", DecidedBy: "CEO", Note: "n", DecidedAt: "t", SubjectVersion: 1, SubjectHash: d.ContentHash}
	if err := s.ReviewContentDraft(ctx, good, domain.DraftApproved, "t"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetContentDraft(ctx, "draft_ok")
	apps, _ := s.ApprovalsFor(ctx, domain.SubjectContentDraft, "draft_ok")
	if got.Status != domain.DraftApproved || len(apps) != 1 || apps[0].SubjectVersion != 1 || apps[0].SubjectHash != d.ContentHash {
		t.Fatalf("review: %+v %+v", got, apps)
	}
	// A stale revision (wrong expected version) is refused and rolled back.
	d.Body, d.Version, d.ContentHash = "isi baru", 2, "hash_baru"
	nv := &domain.ContentDraftVersion{DraftID: "draft_ok", Version: 2, Body: "isi baru", ContentHash: "hash_baru", Author: "human", StatusAfter: domain.DraftNeedsReview, CreatedAt: "t"}
	d.Status = domain.DraftNeedsReview
	if err := s.ReviseContentDraft(ctx, d, nv, 5); !errors.Is(err, ErrStale) {
		t.Fatalf("stale revision: %v", err)
	}
	if vs, _ := s.ContentDraftVersions(ctx, "draft_ok"); len(vs) != 1 {
		t.Fatal("a refused revision must not leave a version row")
	}
	if err := s.ReviseContentDraft(ctx, d, nv, 1); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetContentDraft(ctx, "draft_ok"); got.Version != 2 || got.Status != domain.DraftNeedsReview || got.Body != "isi baru" {
		t.Fatalf("revision: %+v", got)
	}
	// Exact duplicates (same opportunity, type, channel and hash) are refused by the unique index.
	dup, dv := newDraft("draft_dup", "opp_a")
	dup.ProposalIndex, dup.ContentHash, dv.ContentHash = 2, "hash_baru", "hash_baru"
	if err := s.InsertContentDraft(ctx, dup, nil, dv); err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("exact duplicate: %v", err)
	}
	// Evidence links and versions are append-only.
	if _, err := s.DB().Exec(`DELETE FROM content_draft_evidence WHERE draft_id = 'draft_ok'`); err == nil {
		t.Fatal("evidence links must not be deletable")
	}
	if _, err := s.DB().Exec(`DELETE FROM content_draft_versions WHERE draft_id = 'draft_ok'`); err == nil {
		t.Fatal("versions must not be deletable")
	}
}

func TestRecordOpportunityDecisionIsAtomic(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mia.sqlite")
	seedPhase3(t, path)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rec := &domain.ApprovalRecord{ID: "appr_missing", SubjectType: "opportunity", SubjectID: "opp_tidak_ada", Decision: "approved", DecidedBy: "CEO", DecidedAt: "t"}
	if err := s.RecordOpportunityDecision(ctx, rec, domain.OppApproved); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing opportunity: %v", err)
	}
	if apps, _ := s.ApprovalsFor(ctx, "opportunity", "opp_tidak_ada"); len(apps) != 0 {
		t.Fatal("decision for a missing opportunity must roll back")
	}
	rec = &domain.ApprovalRecord{ID: "appr_b", SubjectType: "opportunity", SubjectID: "opp_b", Decision: "approved", DecidedBy: "CEO", DecidedAt: "t"}
	if err := s.RecordOpportunityDecision(ctx, rec, domain.OppApproved); err != nil {
		t.Fatal(err)
	}
	if o, _ := s.GetOpportunity(ctx, "opp_b"); o.Status != domain.OppApproved {
		t.Fatal("status mirror")
	}
}
