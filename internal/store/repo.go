package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

var ErrNotFound = errors.New("not found")

// ---- runs ----

func (s *Store) CreateRun(ctx context.Context, r *domain.Run) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO research_runs(id, kind, status, created_at, started_at, finished_at, heartbeat_at, config_hash, provider_name, inputs_json, summary_json, cost_usd_total, cost_measurable, parent_run_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Kind, r.Status, r.CreatedAt, nz(r.StartedAt), nz(r.FinishedAt), nz(r.HeartbeatAt), r.ConfigHash, r.ProviderName, r.InputsJSON, r.SummaryJSON, nf(r.CostUSDTotal), b2i(r.CostMeasurable), nz(r.ParentRunID))
	return err
}

func (s *Store) UpdateRun(ctx context.Context, r *domain.Run) error {
	_, err := s.db.ExecContext(ctx, `UPDATE research_runs SET status=?, started_at=?, finished_at=?, heartbeat_at=?, summary_json=?, cost_usd_total=?, cost_measurable=? WHERE id=?`,
		r.Status, nz(r.StartedAt), nz(r.FinishedAt), nz(r.HeartbeatAt), r.SummaryJSON, nf(r.CostUSDTotal), b2i(r.CostMeasurable), r.ID)
	return err
}

func (s *Store) Heartbeat(ctx context.Context, runID, at string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE research_runs SET heartbeat_at=? WHERE id=?`, at, runID)
	return err
}

const runCols = `id, kind, status, created_at, started_at, finished_at, heartbeat_at, config_hash, provider_name, inputs_json, summary_json, cost_usd_total, cost_measurable, parent_run_id`

func scanRun(sc interface{ Scan(...any) error }) (*domain.Run, error) {
	var r domain.Run
	var started, finished, hb, parent sql.NullString
	var cost sql.NullFloat64
	var measurable int
	if err := sc.Scan(&r.ID, &r.Kind, &r.Status, &r.CreatedAt, &started, &finished, &hb, &r.ConfigHash, &r.ProviderName, &r.InputsJSON, &r.SummaryJSON, &cost, &measurable, &parent); err != nil {
		return nil, err
	}
	r.StartedAt, r.FinishedAt, r.HeartbeatAt, r.ParentRunID = str(started), str(finished), str(hb), str(parent)
	r.CostUSDTotal = fptr(cost)
	r.CostMeasurable = measurable == 1
	return &r, nil
}

func (s *Store) GetRun(ctx context.Context, id string) (*domain.Run, error) {
	r, err := scanRun(s.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM research_runs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]*domain.Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+runCols+` FROM research_runs ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- steps ----

func (s *Store) CreateSteps(ctx context.Context, steps []*domain.Step) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		for _, st := range steps {
			if _, err := tx.ExecContext(ctx, `INSERT INTO run_steps(id, run_id, name, ordinal, status, attempts, started_at, finished_at, error_json) VALUES (?,?,?,?,?,?,?,?,?)`,
				st.ID, st.RunID, st.Name, st.Ordinal, st.Status, st.Attempts, nz(st.StartedAt), nz(st.FinishedAt), nz(st.ErrorJSON)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) UpdateStep(ctx context.Context, st *domain.Step) error {
	_, err := s.db.ExecContext(ctx, `UPDATE run_steps SET status=?, attempts=?, started_at=?, finished_at=?, error_json=? WHERE id=?`,
		st.Status, st.Attempts, nz(st.StartedAt), nz(st.FinishedAt), nz(st.ErrorJSON), st.ID)
	return err
}

func (s *Store) StepsForRun(ctx context.Context, runID string) ([]*domain.Step, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, name, ordinal, status, attempts, started_at, finished_at, error_json FROM run_steps WHERE run_id=? ORDER BY ordinal`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Step
	for rows.Next() {
		var st domain.Step
		var started, finished, ej sql.NullString
		if err := rows.Scan(&st.ID, &st.RunID, &st.Name, &st.Ordinal, &st.Status, &st.Attempts, &started, &finished, &ej); err != nil {
			return nil, err
		}
		st.StartedAt, st.FinishedAt, st.ErrorJSON = str(started), str(finished), str(ej)
		out = append(out, &st)
	}
	return out, rows.Err()
}

// ---- sources ----

func (s *Store) FindSource(ctx context.Context, reference, contentHash string) (*domain.Source, error) {
	src, err := scanSource(s.db.QueryRowContext(ctx, `SELECT `+sourceCols+` FROM sources WHERE reference=? AND content_hash=?`, reference, contentHash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return src, err
}

const sourceCols = `id, source_type, name, reference, canonical_url, publisher, retrieved_at, published_at, content_hash, snapshot_path, status, status_reason, allowlist_entry, created_at`

func scanSource(sc interface{ Scan(...any) error }) (*domain.Source, error) {
	var x domain.Source
	var canon, pub, pubAt, hash, snap, reason, allow sql.NullString
	if err := sc.Scan(&x.ID, &x.SourceType, &x.Name, &x.Reference, &canon, &pub, &x.RetrievedAt, &pubAt, &hash, &snap, &x.Status, &reason, &allow, &x.CreatedAt); err != nil {
		return nil, err
	}
	x.CanonicalURL, x.Publisher, x.PublishedAt, x.ContentHash, x.SnapshotPath, x.StatusReason, x.AllowlistEntry = str(canon), str(pub), str(pubAt), str(hash), str(snap), str(reason), str(allow)
	return &x, nil
}

func (s *Store) InsertSource(ctx context.Context, x *domain.Source) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sources(`+sourceCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		x.ID, x.SourceType, x.Name, x.Reference, nz(x.CanonicalURL), nz(x.Publisher), x.RetrievedAt, nz(x.PublishedAt), nz(x.ContentHash), nz(x.SnapshotPath), x.Status, nz(x.StatusReason), nz(x.AllowlistEntry), x.CreatedAt)
	return err
}

func (s *Store) LinkRunSource(ctx context.Context, runID, sourceID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO run_sources(run_id, source_id) VALUES (?,?)`, runID, sourceID)
	return err
}

func (s *Store) SourcesForRun(ctx context.Context, runID string) ([]*domain.Source, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+sourceCols+` FROM sources WHERE id IN (SELECT source_id FROM run_sources WHERE run_id=?) ORDER BY reference, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Source
	for rows.Next() {
		x, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) GetSource(ctx context.Context, id string) (*domain.Source, error) {
	x, err := scanSource(s.db.QueryRowContext(ctx, `SELECT `+sourceCols+` FROM sources WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return x, err
}

// ---- items ----

const itemCols = `id, run_id, ref, origin, item_kind, topic_key, title, statement, rationale, language, evidence_quality, status, content_hash, duplicate_of, conflict_group, claim_check_json, suspicious, agent_execution_id, created_at`

func scanItem(sc interface{ Scan(...any) error }) (*domain.Item, error) {
	var it domain.Item
	var topic, rationale, dup, conflict, claim, exec sql.NullString
	var susp int
	if err := sc.Scan(&it.ID, &it.RunID, &it.Ref, &it.Origin, &it.Kind, &topic, &it.Title, &it.Statement, &rationale, &it.Language, &it.Quality, &it.Status, &it.ContentHash, &dup, &conflict, &claim, &susp, &exec, &it.CreatedAt); err != nil {
		return nil, err
	}
	it.TopicKey, it.Rationale, it.DuplicateOf, it.ConflictGroup, it.ClaimCheckJSON, it.AgentExecutionID = str(topic), str(rationale), str(dup), str(conflict), str(claim), str(exec)
	it.Suspicious = susp == 1
	return &it, nil
}

func (s *Store) FindItem(ctx context.Context, origin, ref, contentHash string) (*domain.Item, error) {
	it, err := scanItem(s.db.QueryRowContext(ctx, `SELECT `+itemCols+` FROM research_items WHERE origin=? AND ref=? AND content_hash=?`, origin, ref, contentHash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return it, err
}

func (s *Store) InsertItem(ctx context.Context, it *domain.Item) error { return insertItem(ctx, s.db, it) }

func (s *Store) UpdateItem(ctx context.Context, it *domain.Item) error {
	_, err := s.db.ExecContext(ctx, `UPDATE research_items SET status=?, duplicate_of=?, conflict_group=?, claim_check_json=?, suspicious=?, evidence_quality=? WHERE id=?`,
		it.Status, nz(it.DuplicateOf), nz(it.ConflictGroup), nz(it.ClaimCheckJSON), b2i(it.Suspicious), it.Quality, it.ID)
	return err
}

func (s *Store) LinkRunItem(ctx context.Context, runID, itemID string) error { return linkRunItem(ctx, s.db, runID, itemID) }

func (s *Store) ItemsForRun(ctx context.Context, runID string) ([]*domain.Item, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM research_items WHERE id IN (SELECT item_id FROM run_items WHERE run_id=?) ORDER BY origin, ref, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) GetItem(ctx context.Context, id string) (*domain.Item, error) {
	it, err := scanItem(s.db.QueryRowContext(ctx, `SELECT `+itemCols+` FROM research_items WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return it, err
}

// ---- evidence ----

func (s *Store) InsertEvidence(ctx context.Context, e *domain.Evidence) error { return insertEvidence(ctx, s.db, e) }

func (s *Store) EvidenceForItems(ctx context.Context, runID string) (map[string][]*domain.Evidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, item_id, source_id, from_item_id, excerpt, locator, retrieved_at, published_at, note FROM evidence WHERE item_id IN (SELECT item_id FROM run_items WHERE run_id=?) ORDER BY item_id, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]*domain.Evidence{}
	for rows.Next() {
		var e domain.Evidence
		var src, from, exc, loc, ra, pa, note sql.NullString
		if err := rows.Scan(&e.ID, &e.ItemID, &src, &from, &exc, &loc, &ra, &pa, &note); err != nil {
			return nil, err
		}
		e.SourceID, e.FromItemID, e.Excerpt, e.Locator, e.RetrievedAt, e.PublishedAt, e.Note = str(src), str(from), str(exc), str(loc), str(ra), str(pa), str(note)
		out[e.ItemID] = append(out[e.ItemID], &e)
	}
	return out, rows.Err()
}

// ---- imported assets ----

const assetCols = `id, asset_key, root, relative_path, sha256, size_bytes, mtime, workspace_commit, imported_at, parsed_summary_json, review_status, accepted_at, accepted_by_note`

func scanAsset(sc interface{ Scan(...any) error }) (*domain.ImportedAsset, error) {
	var a domain.ImportedAsset
	var commit, parsed, acc, note sql.NullString
	if err := sc.Scan(&a.ID, &a.AssetKey, &a.Root, &a.RelativePath, &a.SHA256, &a.SizeBytes, &a.MTime, &commit, &a.ImportedAt, &parsed, &a.ReviewStatus, &acc, &note); err != nil {
		return nil, err
	}
	a.WorkspaceCommit, a.ParsedSummaryJSON, a.AcceptedAt, a.AcceptedByNote = str(commit), str(parsed), str(acc), str(note)
	return &a, nil
}

func (s *Store) InsertAsset(ctx context.Context, a *domain.ImportedAsset) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO imported_assets(`+assetCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.AssetKey, a.Root, a.RelativePath, a.SHA256, a.SizeBytes, a.MTime, nz(a.WorkspaceCommit), a.ImportedAt, nz(a.ParsedSummaryJSON), a.ReviewStatus, nz(a.AcceptedAt), nz(a.AcceptedByNote))
	return err
}

// LatestAcceptedAsset returns the most recent accepted fingerprint for a key.
func (s *Store) LatestAcceptedAsset(ctx context.Context, key string) (*domain.ImportedAsset, error) {
	a, err := scanAsset(s.db.QueryRowContext(ctx, `SELECT `+assetCols+` FROM imported_assets WHERE asset_key=? AND review_status=? ORDER BY imported_at DESC, id DESC LIMIT 1`, key, domain.AssetAccepted))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// LatestAsset returns the most recent fingerprint row of any status for a key.
func (s *Store) LatestAsset(ctx context.Context, key string) (*domain.ImportedAsset, error) {
	a, err := scanAsset(s.db.QueryRowContext(ctx, `SELECT `+assetCols+` FROM imported_assets WHERE asset_key=? ORDER BY imported_at DESC, id DESC LIMIT 1`, key))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *Store) AcceptAsset(ctx context.Context, id, at, note string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE imported_assets SET review_status=?, accepted_at=?, accepted_by_note=? WHERE id=?`, domain.AssetAccepted, at, note, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- executions and costs ----

func (s *Store) InsertExecution(ctx context.Context, e *domain.AgentExecution) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_executions(id, run_id, step_id, agent_name, provider_name, model, schema_version, request_hash, request_path, response_path, input_tokens, output_tokens, cache_read_tokens, cost_usd, cost_measurable, latency_ms, attempt, outcome, error_text, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.RunID, nz(e.StepID), e.AgentName, e.ProviderName, nz(e.Model), e.SchemaVersion, e.RequestHash, nz(e.RequestPath), nz(e.ResponsePath), e.InputTokens, e.OutputTokens, e.CacheReadTokens, nf(e.CostUSD), b2i(e.CostMeasurable), e.LatencyMS, e.Attempt, e.Outcome, nz(e.ErrorText), e.CreatedAt)
	return err
}

func (s *Store) ExecutionsForRun(ctx context.Context, runID string) ([]*domain.AgentExecution, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, step_id, agent_name, provider_name, model, schema_version, request_hash, request_path, response_path, input_tokens, output_tokens, cache_read_tokens, cost_usd, cost_measurable, latency_ms, attempt, outcome, error_text, created_at FROM agent_executions WHERE run_id=? ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.AgentExecution
	for rows.Next() {
		var e domain.AgentExecution
		var step, model, rp, sp, et sql.NullString
		var cost sql.NullFloat64
		var measurable int
		if err := rows.Scan(&e.ID, &e.RunID, &step, &e.AgentName, &e.ProviderName, &model, &e.SchemaVersion, &e.RequestHash, &rp, &sp, &e.InputTokens, &e.OutputTokens, &e.CacheReadTokens, &cost, &measurable, &e.LatencyMS, &e.Attempt, &e.Outcome, &et, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.StepID, e.Model, e.RequestPath, e.ResponsePath, e.ErrorText = str(step), str(model), str(rp), str(sp), str(et)
		e.CostUSD = fptr(cost)
		e.CostMeasurable = measurable == 1
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (s *Store) InsertCost(ctx context.Context, c *domain.CostRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO cost_records(id, run_id, agent_execution_id, agent_name, category, provider_name, units_json, unit_price_source, cost_usd, estimated, period_month, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.RunID, nz(c.AgentExecutionID), nz(c.AgentName), c.Category, c.ProviderName, c.UnitsJSON, c.UnitPriceSource, nf(c.CostUSD), b2i(c.Estimated), c.PeriodMonth, c.CreatedAt)
	return err
}

// SpentMonth sums recorded (and estimated) cost for a period; NULL costs count as 0.
func (s *Store) SpentMonth(ctx context.Context, period string) (float64, error) {
	var v sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT SUM(COALESCE(cost_usd, 0)) FROM cost_records WHERE period_month=?`, period).Scan(&v)
	return v.Float64, err
}

func (s *Store) SpentRun(ctx context.Context, runID string) (float64, error) {
	var v sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT SUM(COALESCE(cost_usd, 0)) FROM cost_records WHERE run_id=?`, runID).Scan(&v)
	return v.Float64, err
}

func (s *Store) SpentAgent(ctx context.Context, runID, agent string) (float64, error) {
	var v sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT SUM(COALESCE(cost_usd, 0)) FROM cost_records WHERE run_id=? AND agent_name=?`, runID, agent).Scan(&v)
	return v.Float64, err
}

func (s *Store) CostsForRun(ctx context.Context, runID string) ([]*domain.CostRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, agent_execution_id, agent_name, category, provider_name, units_json, unit_price_source, cost_usd, estimated, period_month, created_at FROM cost_records WHERE run_id=? ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.CostRecord
	for rows.Next() {
		var c domain.CostRecord
		var exec, agent sql.NullString
		var cost sql.NullFloat64
		var est int
		if err := rows.Scan(&c.ID, &c.RunID, &exec, &agent, &c.Category, &c.ProviderName, &c.UnitsJSON, &c.UnitPriceSource, &cost, &est, &c.PeriodMonth, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.AgentExecutionID, c.AgentName = str(exec), str(agent)
		c.CostUSD = fptr(cost)
		c.Estimated = est == 1
		out = append(out, &c)
	}
	return out, rows.Err()
}

// ---- approvals ----

func (s *Store) InsertApproval(ctx context.Context, a *domain.ApprovalRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO approval_records(id, subject_type, subject_id, decision, decided_by, note, decided_at) VALUES (?,?,?,?,?,?,?)`,
		a.ID, a.SubjectType, a.SubjectID, a.Decision, a.DecidedBy, nz(a.Note), a.DecidedAt)
	return err
}

// CountRows is a test helper.
func (s *Store) CountRows(ctx context.Context, table string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(1) FROM %s`, table)).Scan(&n)
	return n, err
}

// UpdateExecution records the outcome and cost of an execution after the call.
func (s *Store) UpdateExecution(ctx context.Context, e *domain.AgentExecution) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agent_executions SET outcome=?, error_text=?, cost_usd=?, cost_measurable=? WHERE id=?`,
		e.Outcome, nz(e.ErrorText), nf(e.CostUSD), b2i(e.CostMeasurable), e.ID)
	return err
}

// execer is satisfied by *sql.DB and *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertItem(ctx context.Context, ex execer, it *domain.Item) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO research_items(`+itemCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		it.ID, it.RunID, it.Ref, it.Origin, it.Kind, nz(it.TopicKey), it.Title, it.Statement, nz(it.Rationale), it.Language, it.Quality, it.Status, it.ContentHash, nz(it.DuplicateOf), nz(it.ConflictGroup), nz(it.ClaimCheckJSON), b2i(it.Suspicious), nz(it.AgentExecutionID), it.CreatedAt)
	return err
}

func linkRunItem(ctx context.Context, ex execer, runID, itemID string) error {
	_, err := ex.ExecContext(ctx, `INSERT OR IGNORE INTO run_items(run_id, item_id) VALUES (?,?)`, runID, itemID)
	return err
}

func insertEvidence(ctx context.Context, ex execer, e *domain.Evidence) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO evidence(id, item_id, source_id, from_item_id, excerpt, locator, retrieved_at, published_at, note) VALUES (?,?,?,?,?,?,?,?,?)`,
		e.ID, e.ItemID, nz(e.SourceID), nz(e.FromItemID), nz(e.Excerpt), nz(e.Locator), nz(e.RetrievedAt), nz(e.PublishedAt), nz(e.Note))
	return err
}

// InsertItemWithEvidence persists an item, its run link and all its evidence
// rows in one transaction, so a failure leaves no orphan item behind (A2).
func (s *Store) InsertItemWithEvidence(ctx context.Context, runID string, it *domain.Item, evs []*domain.Evidence) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if err := insertItem(ctx, tx, it); err != nil {
			return fmt.Errorf("item %s: %w", it.Ref, err)
		}
		if runID != "" {
			if err := linkRunItem(ctx, tx, runID, it.ID); err != nil {
				return fmt.Errorf("run link %s: %w", it.Ref, err)
			}
		}
		for _, e := range evs {
			e.ItemID = it.ID
			if err := insertEvidence(ctx, tx, e); err != nil {
				return fmt.Errorf("evidence for %s: %w", it.Ref, err)
			}
		}
		return nil
	})
}

// FindCostByExecution returns the cost row booked for an execution, if any.
func (s *Store) FindCostByExecution(ctx context.Context, execID string) (*domain.CostRecord, error) {
	var c domain.CostRecord
	var exec, agent sql.NullString
	var cost sql.NullFloat64
	var est int
	err := s.db.QueryRowContext(ctx, `SELECT id, run_id, agent_execution_id, agent_name, category, provider_name, units_json, unit_price_source, cost_usd, estimated, period_month, created_at FROM cost_records WHERE agent_execution_id=? ORDER BY created_at, id LIMIT 1`, execID).
		Scan(&c.ID, &c.RunID, &exec, &agent, &c.Category, &c.ProviderName, &c.UnitsJSON, &c.UnitPriceSource, &cost, &est, &c.PeriodMonth, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.AgentExecutionID, c.AgentName = str(exec), str(agent)
	c.CostUSD = fptr(cost)
	c.Estimated = est == 1
	return &c, nil
}

// UpdateCost replaces the figures of an existing cost row (reconciliation; never a second row per execution).
func (s *Store) UpdateCost(ctx context.Context, c *domain.CostRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE cost_records SET units_json=?, unit_price_source=?, cost_usd=?, estimated=?, period_month=? WHERE id=?`,
		c.UnitsJSON, c.UnitPriceSource, nf(c.CostUSD), b2i(c.Estimated), c.PeriodMonth, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Phase 3: opportunities ----

const oppCols = `id, run_id, parent_run_id, proposal_index, title, segment, topic_key, need, interpretation, status, tier, hypothesis, requires_store, dimensions_json, ordinal_sum, caps_json, gates_json, conflicts_json, brief_json, merged_from_json, reject_reasons_json, config_hash, scoring_config_hash, fixture_hash, agent_execution_id, created_at`

func scanOpp(sc interface{ Scan(...any) error }) (*domain.Opportunity, error) {
	var o domain.Opportunity
	var topic, caps, conflicts, merged, rej, fixture, exec sql.NullString
	var hyp, store int
	if err := sc.Scan(&o.ID, &o.RunID, &o.ParentRunID, &o.ProposalIndex, &o.Title, &o.Segment, &topic, &o.Need, &o.Interpretation, &o.Status, &o.Tier, &hyp, &store, &o.DimensionsJSON, &o.OrdinalSum, &caps, &o.GatesJSON, &conflicts, &o.BriefJSON, &merged, &rej, &o.ConfigHash, &o.ScoringConfigHash, &fixture, &exec, &o.CreatedAt); err != nil {
		return nil, err
	}
	o.TopicKey, o.CapsJSON, o.ConflictsJSON, o.MergedFromJSON, o.RejectReasonsJSON, o.FixtureHash, o.AgentExecutionID = str(topic), str(caps), str(conflicts), str(merged), str(rej), str(fixture), str(exec)
	o.Hypothesis, o.RequiresStore = hyp == 1, store == 1
	return &o, nil
}

// InsertOpportunity stores an opportunity and its evidence links in one transaction.
func (s *Store) InsertOpportunity(ctx context.Context, o *domain.Opportunity, links []domain.OpportunityEvidence) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO opportunities(`+oppCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			o.ID, o.RunID, o.ParentRunID, o.ProposalIndex, o.Title, o.Segment, nz(o.TopicKey), o.Need, o.Interpretation, o.Status, o.Tier, b2i(o.Hypothesis), b2i(o.RequiresStore),
			o.DimensionsJSON, o.OrdinalSum, nz(o.CapsJSON), o.GatesJSON, nz(o.ConflictsJSON), o.BriefJSON, nz(o.MergedFromJSON), nz(o.RejectReasonsJSON), o.ConfigHash, o.ScoringConfigHash, nz(o.FixtureHash), nz(o.AgentExecutionID), o.CreatedAt); err != nil {
			return fmt.Errorf("opportunity %s: %w", o.Title, err)
		}
		for _, l := range links {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO opportunity_evidence(opportunity_id, item_id, role) VALUES (?,?,?)`, o.ID, l.ItemID, l.Role); err != nil {
				return fmt.Errorf("opportunity evidence %s: %w", l.ItemID, err)
			}
		}
		return nil
	})
}

func (s *Store) OpportunitiesForRun(ctx context.Context, runID string) ([]*domain.Opportunity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+oppCols+` FROM opportunities WHERE run_id=? ORDER BY proposal_index, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Opportunity
	for rows.Next() {
		o, err := scanOpp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetOpportunity(ctx context.Context, id string) (*domain.Opportunity, error) {
	o, err := scanOpp(s.db.QueryRowContext(ctx, `SELECT `+oppCols+` FROM opportunities WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

// UpdateOpportunityStatus mirrors the latest review decision; history stays in approval_records.
func (s *Store) UpdateOpportunityStatus(ctx context.Context, id, status string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE opportunities SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) EvidenceForOpportunity(ctx context.Context, oppID string) ([]domain.OpportunityEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT opportunity_id, item_id, role FROM opportunity_evidence WHERE opportunity_id=? ORDER BY item_id`, oppID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OpportunityEvidence
	for rows.Next() {
		var e domain.OpportunityEvidence
		if err := rows.Scan(&e.OpportunityID, &e.ItemID, &e.Role); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ValidatedItemsForRun returns items linked to a run whose status is validated.
func (s *Store) ValidatedItemsForRun(ctx context.Context, runID string) ([]*domain.Item, error) {
	items, err := s.ItemsForRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	var out []*domain.Item
	for _, it := range items {
		if it.Status == domain.ItemValidated {
			out = append(out, it)
		}
	}
	return out, nil
}

// CountExecutions counts provider calls made by an agent within a run (any outcome).
func (s *Store) CountExecutions(ctx context.Context, runID, agent string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM agent_executions WHERE run_id=? AND agent_name=?`, runID, agent).Scan(&n)
	return n, err
}

// LatestSuccessfulExecution returns the most recent ok/repaired execution of an agent in a run.
func (s *Store) LatestSuccessfulExecution(ctx context.Context, runID, agent string) (*domain.AgentExecution, error) {
	execs, err := s.ExecutionsForRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	var last *domain.AgentExecution
	for _, e := range execs {
		if e.AgentName == agent && (e.Outcome == "ok" || e.Outcome == "repaired") {
			last = e
		}
	}
	if last == nil {
		return nil, ErrNotFound
	}
	return last, nil
}

// ApprovalsFor returns the append-only decision history for a subject, oldest first.
func (s *Store) ApprovalsFor(ctx context.Context, subjectType, subjectID string) ([]*domain.ApprovalRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, subject_type, subject_id, decision, decided_by, note, decided_at FROM approval_records WHERE subject_type=? AND subject_id=? ORDER BY decided_at, id`, subjectType, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.ApprovalRecord
	for rows.Next() {
		var a domain.ApprovalRecord
		var note sql.NullString
		if err := rows.Scan(&a.ID, &a.SubjectType, &a.SubjectID, &a.Decision, &a.DecidedBy, &note, &a.DecidedAt); err != nil {
			return nil, err
		}
		a.Note = str(note)
		out = append(out, &a)
	}
	return out, rows.Err()
}
