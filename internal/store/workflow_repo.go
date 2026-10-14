package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

// ErrLeaseHeld means another process currently holds the workflow lease.
var ErrLeaseHeld = errors.New("workflow sedang dimajukan oleh proses lain")

const wfCols = `id, status, inputs_dir, fixture_key, research_run_id, strategy_run_id, last_completed_stage, failed_stage, reason, summary_json, config_hash, lease_owner, lease_expires_at, created_at, updated_at`

func scanWorkflow(sc interface{ Scan(...any) error }) (*domain.Workflow, error) {
	var w domain.Workflow
	var rr, sr, lcs, fs, reason, owner, exp sql.NullString
	if err := sc.Scan(&w.ID, &w.Status, &w.InputsDir, &w.FixtureKey, &rr, &sr, &lcs, &fs, &reason, &w.SummaryJSON, &w.ConfigHash, &owner, &exp, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return nil, err
	}
	w.ResearchRunID, w.StrategyRunID, w.LastCompletedStage, w.FailedStage, w.Reason, w.LeaseOwner, w.LeaseExpiresAt = str(rr), str(sr), str(lcs), str(fs), str(reason), str(owner), str(exp)
	return &w, nil
}

func appendEvent(ctx context.Context, ex execer, ev *domain.WorkflowEvent) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO workflow_events(workflow_id, seq, kind, stage, ref_id, detail, created_at)
SELECT ?, COALESCE(MAX(seq), 0) + 1, ?, ?, ?, ?, ? FROM workflow_events WHERE workflow_id = ?`,
		ev.WorkflowID, ev.Kind, nz(ev.Stage), nz(ev.RefID), nz(ev.Detail), ev.CreatedAt, ev.WorkflowID)
	return err
}

// CreateWorkflow inserts the workflow and its first event in one transaction.
func (s *Store) CreateWorkflow(ctx context.Context, w *domain.Workflow, ev *domain.WorkflowEvent) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflows(id, status, inputs_dir, fixture_key, summary_json, config_hash, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
			w.ID, w.Status, w.InputsDir, w.FixtureKey, jsonObj(w.SummaryJSON), w.ConfigHash, w.CreatedAt, w.UpdatedAt); err != nil {
			return err
		}
		return appendEvent(ctx, tx, ev)
	})
}

func jsonObj(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

func (s *Store) GetWorkflow(ctx context.Context, id string) (*domain.Workflow, error) {
	w, err := scanWorkflow(s.db.QueryRowContext(ctx, `SELECT `+wfCols+` FROM workflows WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

func (s *Store) queryWorkflows(ctx context.Context, q string, args ...any) ([]*domain.Workflow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+wfCols+` FROM workflows `+q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Workflow
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) ListWorkflows(ctx context.Context, limit int) ([]*domain.Workflow, error) {
	return s.queryWorkflows(ctx, `ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
}

func (s *Store) WorkflowsForInputs(ctx context.Context, dir string) ([]*domain.Workflow, error) {
	return s.queryWorkflows(ctx, `WHERE inputs_dir = ? ORDER BY created_at, id`, dir)
}

// AcquireWorkflowLease takes the lease when it is free or expired (compare-and-set).
func (s *Store) AcquireWorkflowLease(ctx context.Context, id, owner, now, until string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE workflows SET lease_owner = ?, lease_expires_at = ? WHERE id = ? AND (lease_owner IS NULL OR lease_expires_at IS NULL OR lease_expires_at < ?)`, owner, until, id, now)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *Store) ReleaseWorkflowLease(ctx context.Context, id, owner string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE workflows SET lease_owner = NULL, lease_expires_at = NULL WHERE id = ? AND lease_owner = ?`, id, owner)
	return err
}

// ReserveWorkflowRun records the run ID of the research or strategy stage before the run starts.
// It fails with ErrStale when the stage already has a run.
func (s *Store) ReserveWorkflowRun(ctx context.Context, wfID, stage, runID string, ev *domain.WorkflowEvent) error {
	col := map[string]string{"research": "research_run_id", "strategy": "strategy_run_id"}[stage]
	if col == "" {
		return fmt.Errorf("unknown workflow stage %q", stage)
	}
	return s.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE workflows SET `+col+` = ?, updated_at = ? WHERE id = ? AND `+col+` IS NULL`, runID, ev.CreatedAt, wfID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrStale
		}
		return appendEvent(ctx, tx, ev)
	})
}

// LinkWorkflowContentRun records (reserves or adopts) a content run for an opportunity in one transaction.
func (s *Store) LinkWorkflowContentRun(ctx context.Context, l *domain.WorkflowContentRun, ev *domain.WorkflowEvent) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_content_runs(workflow_id, opportunity_id, attempt, content_run_id, origin, created_at) VALUES (?,?,?,?,?,?)`,
			l.WorkflowID, l.OpportunityID, l.Attempt, l.ContentRunID, l.Origin, l.CreatedAt); err != nil {
			return err
		}
		return appendEvent(ctx, tx, ev)
	})
}

// WorkflowContentRuns returns every content-run link of a workflow, ordered by opportunity and attempt.
func (s *Store) WorkflowContentRuns(ctx context.Context, wfID string) ([]*domain.WorkflowContentRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT workflow_id, opportunity_id, attempt, content_run_id, origin, created_at FROM workflow_content_runs WHERE workflow_id = ? ORDER BY opportunity_id, attempt`, wfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.WorkflowContentRun
	for rows.Next() {
		var l domain.WorkflowContentRun
		if err := rows.Scan(&l.WorkflowID, &l.OpportunityID, &l.Attempt, &l.ContentRunID, &l.Origin, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

func (s *Store) AppendWorkflowEvent(ctx context.Context, ev *domain.WorkflowEvent) error {
	return appendEvent(ctx, s.db, ev)
}

func (s *Store) WorkflowEvents(ctx context.Context, wfID string) ([]*domain.WorkflowEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT workflow_id, seq, kind, stage, ref_id, detail, created_at FROM workflow_events WHERE workflow_id = ? ORDER BY seq`, wfID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.WorkflowEvent
	for rows.Next() {
		var e domain.WorkflowEvent
		var stage, ref, detail sql.NullString
		if err := rows.Scan(&e.WorkflowID, &e.Seq, &e.Kind, &stage, &ref, &detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Stage, e.RefID, e.Detail = str(stage), str(ref), str(detail)
		out = append(out, &e)
	}
	return out, rows.Err()
}

// SyncWorkflowSummary stores the derived summary (and an event when the status changed) in one
// transaction. With a lease owner it requires that lease; without one it only writes while no other
// process holds a live lease. Returns ErrLeaseHeld otherwise.
func (s *Store) SyncWorkflowSummary(ctx context.Context, w *domain.Workflow, leaseOwner, now string, ev *domain.WorkflowEvent) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		q := `UPDATE workflows SET status = ?, last_completed_stage = ?, failed_stage = ?, reason = ?, summary_json = ?, updated_at = ? WHERE id = ? AND `
		args := []any{w.Status, nz(w.LastCompletedStage), nz(w.FailedStage), nz(w.Reason), jsonObj(w.SummaryJSON), w.UpdatedAt, w.ID}
		if leaseOwner != "" {
			q += `lease_owner = ?`
			args = append(args, leaseOwner)
		} else {
			q += `(lease_owner IS NULL OR lease_expires_at IS NULL OR lease_expires_at < ?)`
			args = append(args, now)
		}
		res, err := tx.ExecContext(ctx, q, args...)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrLeaseHeld
		}
		if ev != nil {
			return appendEvent(ctx, tx, ev)
		}
		return nil
	})
}
