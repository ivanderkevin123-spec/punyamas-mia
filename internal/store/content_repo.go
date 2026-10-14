package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

// ErrStale means the draft changed since the caller read it (optimistic concurrency).
var ErrStale = errors.New("draf berubah sejak dibaca; baca ulang lalu ulangi")

const draftCols = `id, run_id, opportunity_id, strategy_run_id, research_run_id, proposal_index, content_type, variant, channel, target_segment, objective,
hook, headline, body, call_to_action, claims_json, placeholders_json, assumptions_json, risks_json, checklist_json, findings_json, brand_fact_ids_json,
status, version, content_hash, execution_id, config_hash, content_config_hash, scoring_config_hash, fixture_hash, created_at, updated_at`

func scanDraft(sc interface{ Scan(...any) error }) (*domain.ContentDraft, error) {
	var d domain.ContentDraft
	var fixture sql.NullString
	if err := sc.Scan(&d.ID, &d.RunID, &d.OpportunityID, &d.StrategyRunID, &d.ResearchRunID, &d.ProposalIndex, &d.ContentType, &d.Variant, &d.Channel, &d.TargetSegment, &d.Objective,
		&d.Hook, &d.Headline, &d.Body, &d.CallToAction, &d.ClaimsJSON, &d.PlaceholdersJSON, &d.AssumptionsJSON, &d.RisksJSON, &d.ChecklistJSON, &d.FindingsJSON, &d.BrandFactIDsJSON,
		&d.Status, &d.Version, &d.ContentHash, &d.ExecutionID, &d.ConfigHash, &d.ContentConfigHash, &d.ScoringConfigHash, &fixture, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	d.FixtureHash = str(fixture)
	return &d, nil
}

func insertVersion(ctx context.Context, ex execer, v *domain.ContentDraftVersion) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO content_draft_versions(draft_id, version, hook, headline, body, call_to_action, content_hash, author, note, status_after, findings_json, placeholders_json, created_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.DraftID, v.Version, v.Hook, v.Headline, v.Body, v.CallToAction, v.ContentHash, v.Author, nz(v.Note), v.StatusAfter, jsonOr(v.FindingsJSON), jsonOr(v.PlaceholdersJSON), v.CreatedAt)
	return err
}

func jsonOr(s string) string {
	if s == "" || s == "null" {
		return "[]"
	}
	return s
}

// InsertContentDraft stores a draft, its first version and its evidence links in one transaction.
func (s *Store) InsertContentDraft(ctx context.Context, d *domain.ContentDraft, links []domain.ContentDraftEvidence, v *domain.ContentDraftVersion) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_drafts(`+draftCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.ID, d.RunID, d.OpportunityID, d.StrategyRunID, d.ResearchRunID, d.ProposalIndex, d.ContentType, d.Variant, d.Channel, d.TargetSegment, d.Objective,
			d.Hook, d.Headline, d.Body, d.CallToAction, jsonOr(d.ClaimsJSON), jsonOr(d.PlaceholdersJSON), jsonOr(d.AssumptionsJSON), jsonOr(d.RisksJSON), jsonOr(d.ChecklistJSON), jsonOr(d.FindingsJSON), jsonOr(d.BrandFactIDsJSON),
			d.Status, d.Version, d.ContentHash, d.ExecutionID, d.ConfigHash, d.ContentConfigHash, d.ScoringConfigHash, nz(d.FixtureHash), d.CreatedAt, d.UpdatedAt); err != nil {
			return fmt.Errorf("content draft %s: %w", d.ID, err)
		}
		if err := insertVersion(ctx, tx, v); err != nil {
			return fmt.Errorf("content draft version %s: %w", d.ID, err)
		}
		for _, l := range links {
			if _, err := tx.ExecContext(ctx, `INSERT INTO content_draft_evidence(draft_id, opportunity_id, item_id, role) VALUES (?,?,?,?)`, d.ID, d.OpportunityID, l.ItemID, l.Role); err != nil {
				return fmt.Errorf("content draft evidence %s: %w", l.ItemID, err)
			}
		}
		return nil
	})
}

func (s *Store) queryDrafts(ctx context.Context, where string, arg any) ([]*domain.ContentDraft, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+draftCols+` FROM content_drafts WHERE `+where+` ORDER BY created_at, proposal_index, id`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.ContentDraft
	for rows.Next() {
		d, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) ContentDraftsForRun(ctx context.Context, runID string) ([]*domain.ContentDraft, error) {
	return s.queryDrafts(ctx, "run_id = ?", runID)
}

func (s *Store) ContentDraftsForOpportunity(ctx context.Context, oppID string) ([]*domain.ContentDraft, error) {
	return s.queryDrafts(ctx, "opportunity_id = ?", oppID)
}

func (s *Store) GetContentDraft(ctx context.Context, id string) (*domain.ContentDraft, error) {
	d, err := scanDraft(s.db.QueryRowContext(ctx, `SELECT `+draftCols+` FROM content_drafts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

func (s *Store) ContentEvidenceFor(ctx context.Context, draftID string) ([]domain.ContentDraftEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT draft_id, opportunity_id, item_id, role FROM content_draft_evidence WHERE draft_id = ? ORDER BY item_id`, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ContentDraftEvidence
	for rows.Next() {
		var e domain.ContentDraftEvidence
		if err := rows.Scan(&e.DraftID, &e.OpportunityID, &e.ItemID, &e.Role); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ContentDraftVersions(ctx context.Context, draftID string) ([]*domain.ContentDraftVersion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT draft_id, version, hook, headline, body, call_to_action, content_hash, author, note, status_after, findings_json, placeholders_json, created_at
FROM content_draft_versions WHERE draft_id = ? ORDER BY version`, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.ContentDraftVersion
	for rows.Next() {
		var v domain.ContentDraftVersion
		var note sql.NullString
		if err := rows.Scan(&v.DraftID, &v.Version, &v.Hook, &v.Headline, &v.Body, &v.CallToAction, &v.ContentHash, &v.Author, &note, &v.StatusAfter, &v.FindingsJSON, &v.PlaceholdersJSON, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Note = str(note)
		out = append(out, &v)
	}
	return out, rows.Err()
}

// ReviewContentDraft appends the review decision (with the draft version and hash it applies to) and
// mirrors it on the draft, in one transaction. It fails with ErrStale when the draft changed.
func (s *Store) ReviewContentDraft(ctx context.Context, rec *domain.ApprovalRecord, status, updatedAt string) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if err := insertApproval(ctx, tx, rec); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE content_drafts SET status = ?, updated_at = ? WHERE id = ? AND version = ? AND content_hash = ?`,
			status, updatedAt, rec.SubjectID, rec.SubjectVersion, rec.SubjectHash)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrStale
		}
		return nil
	})
}

// ReviseContentDraft appends a new text version and updates the draft in one transaction,
// provided the draft is still at expectVersion.
func (s *Store) ReviseContentDraft(ctx context.Context, d *domain.ContentDraft, v *domain.ContentDraftVersion, expectVersion int) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if err := insertVersion(ctx, tx, v); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE content_drafts SET hook = ?, headline = ?, body = ?, call_to_action = ?, claims_json = ?, placeholders_json = ?, checklist_json = ?, findings_json = ?,
status = ?, version = ?, content_hash = ?, updated_at = ? WHERE id = ? AND version = ?`,
			d.Hook, d.Headline, d.Body, d.CallToAction, jsonOr(d.ClaimsJSON), jsonOr(d.PlaceholdersJSON), jsonOr(d.ChecklistJSON), jsonOr(d.FindingsJSON),
			d.Status, d.Version, d.ContentHash, d.UpdatedAt, d.ID, expectVersion)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrStale
		}
		return nil
	})
}

// RecordOpportunityDecision appends a review decision for an opportunity and mirrors it on the
// opportunity status in one transaction, so status and history cannot diverge.
func (s *Store) RecordOpportunityDecision(ctx context.Context, rec *domain.ApprovalRecord, status string) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if err := insertApproval(ctx, tx, rec); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE opportunities SET status = ? WHERE id = ?`, status, rec.SubjectID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ExecutionByID returns one agent execution.
func (s *Store) ExecutionByID(ctx context.Context, runID, id string) (*domain.AgentExecution, error) {
	execs, err := s.ExecutionsForRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	for _, e := range execs {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, ErrNotFound
}
