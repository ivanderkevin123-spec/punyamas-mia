package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	agentstrategy "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/strategy"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
)

// Strategy run steps (Phase 3), executed in order; the report step is shared.
const (
	StepLoadEvidence  = "load_evidence"
	StepConflicts     = "conflicts"
	StepStrategyAgent = "strategy_agent"
	StepEvaluate      = "evaluate"
)

func (rc *runCtx) evidenceRunID() string {
	if rc.parentRunID != "" {
		return rc.parentRunID
	}
	return rc.run.ID
}

func (o *Orchestrator) strategyDefs() []stepDef {
	return []stepDef{
		{name: StepLoadEvidence, required: true, fn: o.stepLoadEvidence},
		{name: StepConflicts, required: true, fn: o.stepConflicts},
		{name: StepStrategyAgent, required: true, retry: true, fn: o.stepStrategyAgent},
		{name: StepEvaluate, required: true, fn: o.stepEvaluate},
		{name: StepReport, required: true, fn: o.stepReport},
	}
}

// StrategyStepNames lists the strategy workflow steps in order.
func StrategyStepNames() []string {
	return []string{StepLoadEvidence, StepConflicts, StepStrategyAgent, StepEvaluate, StepReport}
}

// execute runs the step list for a prepared run context (shared by research and strategy runs).
func (o *Orchestrator) execute(ctx context.Context, rc *runCtx, defs []stepDef) (*domain.RunSummary, error) {
	for _, def := range defs {
		st := rc.steps[def.name]
		if st == nil {
			return nil, fmt.Errorf("langkah %s tidak ada pada run %s", def.name, rc.run.ID)
		}
		// The report is always re-rendered so a resumed run never keeps a stale report.
		if st.Status == domain.StepCompleted && !rc.opts.Force && def.name != StepReport {
			rc.skipped[def.name] = true
			rc.sum.Steps = append(rc.sum.Steps, domain.StepSummary{Name: st.Name, Status: st.Status, Attempts: st.Attempts})
			continue
		}
		if (rc.failed || rc.stopped) && def.name != StepReport {
			if st.Status != domain.StepCompleted {
				st.Status = domain.StepSkipped
				_ = o.d.Store.UpdateStep(ctx, st)
			}
			rc.sum.Steps = append(rc.sum.Steps, domain.StepSummary{Name: st.Name, Status: st.Status, Attempts: st.Attempts})
			continue
		}
		if def.name == StepReport {
			if err := o.refreshCounts(ctx, rc); err != nil {
				return nil, err
			}
			if err := o.refreshCosts(ctx, rc); err != nil {
				return nil, err
			}
			rc.sum.Status = o.provisionalStatus(rc)
			rc.run.FinishedAt = o.ts()
		}
		o.execStep(ctx, rc, def)
		if rc.opts.Until != "" && def.name == rc.opts.Until {
			rc.stopped = true
		}
	}
	return o.finalize(ctx, rc)
}

type strategyInputs struct {
	FromRun    string `json:"from_run"`
	FixtureKey string `json:"fixture_key"`
}

// RunStrategy executes (or resumes) a strategy run over the validated items of a research run.
func (o *Orchestrator) RunStrategy(ctx context.Context, opts Options) (*domain.RunSummary, error) {
	now := o.ts()
	var run *domain.Run
	var steps []*domain.Step
	var prev *domain.RunSummary
	if opts.ResumeID != "" {
		r, err := o.d.Store.GetRun(ctx, opts.ResumeID)
		if err != nil {
			return nil, fmt.Errorf("run %s: %w", opts.ResumeID, err)
		}
		if r.Kind != "strategy" {
			return nil, fmt.Errorf("run %s bukan run strategi", r.ID)
		}
		var in strategyInputs
		_ = json.Unmarshal([]byte(r.InputsJSON), &in)
		if opts.FromRun == "" {
			opts.FromRun = in.FromRun
		}
		if opts.FixtureKey == "" {
			opts.FixtureKey = in.FixtureKey
		}
		var ps domain.RunSummary
		if json.Unmarshal([]byte(r.SummaryJSON), &ps) == nil && ps.RunID != "" {
			prev = &ps
		}
		steps, err = o.d.Store.StepsForRun(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		run = r
		run.Status = domain.RunRunning
		run.HeartbeatAt = now
		run.FinishedAt = ""
		if err := o.d.Store.UpdateRun(ctx, run); err != nil {
			return nil, err
		}
	} else {
		if opts.FromRun == "" {
			return nil, errors.New("run riset sumber (--from-run) wajib diisi")
		}
		parent, err := o.d.Store.GetRun(ctx, opts.FromRun)
		if err != nil {
			return nil, fmt.Errorf("run riset %s: %w", opts.FromRun, err)
		}
		if parent.Kind != "research" {
			return nil, fmt.Errorf("run %s bukan run riset", parent.ID)
		}
		if opts.FixtureKey == "" {
			opts.FixtureKey = o.d.Cfg.Limits.Provider.Mock.FixtureKey
		}
		if opts.FixtureKey == "" {
			var pin inputsJSON
			_ = json.Unmarshal([]byte(parent.InputsJSON), &pin)
			opts.FixtureKey = pin.FixtureKey
		}
		inJ, _ := json.Marshal(strategyInputs{FromRun: parent.ID, FixtureKey: opts.FixtureKey})
		run = &domain.Run{ID: o.d.IDs.New("run"), Kind: "strategy", Status: domain.RunRunning, CreatedAt: now, StartedAt: now, HeartbeatAt: now,
			ConfigHash: o.d.Cfg.Hash, ProviderName: o.d.Provider.Name(), InputsJSON: string(inJ), SummaryJSON: "{}", CostMeasurable: true, ParentRunID: parent.ID}
		if err := o.d.Store.CreateRun(ctx, run); err != nil {
			return nil, err
		}
		for i, def := range o.strategyDefs() {
			steps = append(steps, &domain.Step{ID: o.d.IDs.New("step"), RunID: run.ID, Name: def.name, Ordinal: i + 1, Status: domain.StepPending})
		}
		if err := o.d.Store.CreateSteps(ctx, steps); err != nil {
			return nil, err
		}
	}
	rc := &runCtx{run: run, steps: map[string]*domain.Step{}, opts: opts, prev: prev, skipped: map[string]bool{}, parentRunID: run.ParentRunID,
		sum: &domain.RunSummary{RunID: run.ID, Kind: run.Kind, CostMeasurable: true, ParentRunID: run.ParentRunID, ScoringConfigHash: o.d.Cfg.Scoring.Hash}}
	for _, st := range steps {
		rc.steps[st.Name] = st
	}
	if prev != nil {
		rc.sum.FixtureHash = prev.FixtureHash
		for _, w := range prev.Warnings {
			if !strings.HasPrefix(w, "[sebelumnya] ") {
				w = "[sebelumnya] " + w
			}
			rc.sum.Warnings = append(rc.sum.Warnings, w)
		}
	}
	o.d.Log.Info("strategy run started", "run_id", run.ID, "parent_run_id", run.ParentRunID, "resume", opts.ResumeID != "", "provider", run.ProviderName)
	return o.execute(ctx, rc, o.strategyDefs())
}

func (o *Orchestrator) stepLoadEvidence(ctx context.Context, rc *runCtx) error {
	parent, err := o.d.Store.GetRun(ctx, rc.parentRunID)
	if err != nil {
		return failf("run riset %s: %v", rc.parentRunID, err)
	}
	if parent.Status != domain.RunCompleted && parent.Status != domain.RunCompletedWithWarnings {
		return failf("run riset %s berstatus %s; selesaikan run riset (termasuk langkah validate) sebelum menjalankan strategi", parent.ID, parent.Status)
	}
	items, err := o.d.Store.ValidatedItemsForRun(ctx, parent.ID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return failf("run riset %s tidak memiliki butir tervalidasi", parent.ID)
	}
	return nil
}

// loadEvidence builds the evaluator's view of the parent run's validated items.
func (o *Orchestrator) loadEvidence(ctx context.Context, parentRunID string) ([]strategy.Evidence, map[string]strategy.Evidence, error) {
	items, err := o.d.Store.ValidatedItemsForRun(ctx, parentRunID)
	if err != nil {
		return nil, nil, err
	}
	evid, err := o.d.Store.EvidenceForItems(ctx, parentRunID)
	if err != nil {
		return nil, nil, err
	}
	srcNames := map[string]string{}
	var list []strategy.Evidence
	byRef := map[string]strategy.Evidence{}
	for _, it := range items {
		e := strategy.Evidence{ID: it.ID, Ref: it.Ref, Kind: it.Kind, Quality: it.Quality, TopicKey: it.TopicKey, Title: it.Title, Statement: it.Statement, ConflictGroup: it.ConflictGroup}
		for _, ev := range evid[it.ID] {
			if ev.SourceID != "" {
				e.SourceID = ev.SourceID
				e.Excerpt = ev.Excerpt
				name, ok := srcNames[ev.SourceID]
				if !ok {
					if s, err := o.d.Store.GetSource(ctx, ev.SourceID); err == nil {
						name = s.Name
					}
					srcNames[ev.SourceID] = name
				}
				e.SourceName = name
				break
			}
		}
		if e.SourceID == "" {
			// Agent-produced items: treat the producing execution as their "source" so independence counts are honest.
			e.SourceID = "agent:" + it.AgentExecutionID
			e.SourceName = "keluaran agen riset"
		}
		list = append(list, e)
		byRef[it.Ref] = e
	}
	return list, byRef, nil
}

func (o *Orchestrator) stepConflicts(ctx context.Context, rc *runCtx) error {
	list, _, err := o.loadEvidence(ctx, rc.parentRunID)
	if err != nil {
		return err
	}
	neg, err := regexp.Compile(o.d.Cfg.Scoring.Conflict.NegationPattern)
	if err != nil {
		return failf("pola negasi tidak valid: %v", err)
	}
	groups := strategy.DetectConflicts(list, neg, o.d.Cfg.Scoring.Conflict.MinSimilarity)
	if len(groups) == 0 {
		return nil
	}
	items, err := o.d.Store.ValidatedItemsForRun(ctx, rc.parentRunID)
	if err != nil {
		return err
	}
	changed := 0
	seen := map[string]bool{}
	for _, it := range items {
		g, ok := groups[it.ID]
		if !ok {
			continue
		}
		seen[g] = true
		if it.ConflictGroup != g {
			it.ConflictGroup = g
			if err := o.d.Store.UpdateItem(ctx, it); err != nil {
				return err
			}
			changed++
		}
	}
	rc.warn("%d butir fakta berada dalam %d kelompok konflik (pernyataan bertentangan dengan topik sama); peluang yang mengutipnya dibatasi tingkat C", len(groups), len(seen))
	return nil
}

func (o *Orchestrator) stepStrategyAgent(ctx context.Context, rc *runCtx) error {
	cfg := o.d.Cfg
	caps := cfg.Limits.Caps
	already, err := o.d.Store.CountExecutions(ctx, rc.run.ID, agentstrategy.Name)
	if err != nil {
		return err
	}
	if already >= agentstrategy.MaxCallsPerRun {
		// The cap counts every call (errors and timeouts included); report the last outcome honestly.
		last, status := "tidak diketahui", domain.StepFailed
		if execs, err := o.d.Store.ExecutionsForRun(ctx, rc.run.ID); err == nil {
			for _, x := range execs {
				if x.AgentName == agentstrategy.Name {
					last = x.Outcome
				}
			}
		}
		if last == "timeout" {
			status = domain.StepTimedOut
		}
		return &stepErr{status: status, err: fmt.Errorf("batas %d panggilan Agen Strategi per run sudah tercapai (%d tercatat; hasil terakhir: %s); mulai run strategi baru dengan --from-run", agentstrategy.MaxCallsPerRun, already, last)}
	}
	list, _, err := o.loadEvidence(ctx, rc.parentRunID)
	if err != nil {
		return err
	}
	var ev []agentstrategy.EvidenceForPrompt
	for _, e := range list {
		ev = append(ev, agentstrategy.EvidenceForPrompt{Ref: e.Ref, Kind: e.Kind, Quality: e.Quality, TopicKey: e.TopicKey, Title: e.Title, Statement: e.Statement, SourceName: e.SourceName, ConflictGroup: e.ConflictGroup})
	}
	req := agentstrategy.BuildRequest(cfg, ev, rc.opts.FixtureKey)
	req.Model = cfg.Limits.Provider.Model
	req.MaxOutputTokens = caps.MaxOutputTokens
	st := rc.steps[StepStrategyAgent]
	maxCalls := caps.MaxCallsPerStep
	if remaining := agentstrategy.MaxCallsPerRun - already; remaining < maxCalls {
		maxCalls = remaining
	}
	var problems []string
	for call := 1; call <= maxCalls; call++ {
		if call > 1 {
			req = agentstrategy.RepairRequest(req, problems)
		}
		estIn := provider.EstimateTokens(req.System + req.User)
		estOut := caps.MaxOutputTokens
		if estIn > caps.MaxInputTokens {
			return failf("masukan agen (%d token) melebihi batas %d token", estIn, caps.MaxInputTokens)
		}
		dec, err := o.d.Budget.Allow(ctx, rc.run.ID, agentstrategy.Name, req.Model, estIn, estOut)
		if err != nil {
			return failf("pemeriksaan anggaran gagal: %v", err)
		}
		if dec.Warn {
			rc.warn("anggaran bulanan mendekati plafon (terpakai USD %.4f dari USD %.2f)", dec.MonthSpent, caps.MonthlyCeilingUSD)
		}
		if !dec.Allowed {
			return &stepErr{status: domain.StepBudgetExhausted, err: errors.New(dec.Reason)}
		}
		start := time.Now()
		resp, usage, perr := o.d.Provider.Complete(ctx, req)
		now := o.ts()
		bctx := context.WithoutCancel(ctx)
		exec := &domain.AgentExecution{ID: o.d.IDs.New("exec"), RunID: rc.run.ID, StepID: st.ID, AgentName: agentstrategy.Name, ProviderName: o.d.Provider.Name(), Model: req.Model,
			SchemaVersion: agentstrategy.SchemaVersion, RequestHash: dedup.FileHash([]byte(req.System + "\n" + req.User)), InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
			CacheReadTokens: usage.CacheReadTokens, CostMeasurable: usage.Measurable, LatencyMS: time.Since(start).Milliseconds(), Attempt: already + call, Outcome: "pending", CreatedAt: now}
		exec.RequestPath, exec.ResponsePath = o.saveExchange(exec.ID, req, resp.Content)
		if perr != nil {
			exec.Outcome = "error"
			if errors.Is(perr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				exec.Outcome = "timeout"
			}
			exec.ErrorText = perr.Error()
			exec.CostMeasurable = false
			if err := o.d.Store.InsertExecution(bctx, exec); err != nil {
				return err
			}
			if provider.Fatal(perr) {
				return &stepErr{status: domain.StepFailed, err: fmt.Errorf("penyedia menolak (%v); tidak diulang", perr)}
			}
			rec, err := o.d.Budget.RecordUncertain(bctx, rc.run.ID, exec.ID, agentstrategy.Name, o.d.Provider.Name(), req.Model, estIn, estOut, exec.Outcome)
			if err != nil {
				return err
			}
			exec.CostUSD = rec.CostUSD
			_ = o.d.Store.UpdateExecution(bctx, exec)
			return &stepErr{status: domain.StepFailed, err: perr, retryable: provider.Retryable(perr)}
		}
		if err := o.d.Store.InsertExecution(bctx, exec); err != nil {
			return err
		}
		cost, err := o.d.Budget.Record(bctx, rc.run.ID, exec.ID, agentstrategy.Name, o.d.Provider.Name(), req.Model, usage, estIn, estOut)
		if err != nil {
			return err
		}
		exec.CostUSD = cost.CostUSD
		exec.CostMeasurable = !cost.Estimated
		if resp.FixtureUsed != "" {
			if b, err := os.ReadFile(resp.FixtureUsed); err == nil {
				rc.sum.FixtureHash = dedup.FileHash(b)
			}
		}
		for _, w := range resp.Warnings {
			rc.warn("%s", w)
		}
		out, perr := agentstrategy.ParseOutput(resp.Content)
		if perr != nil {
			exec.Outcome = "invalid_output"
			exec.ErrorText = perr.Error()
			_ = o.d.Store.UpdateExecution(bctx, exec)
			problems = []string{perr.Error()}
			if call < maxCalls {
				rc.warn("keluaran Agen Strategi tidak valid (percobaan %d): %v; meminta perbaikan", call, perr)
				continue
			}
			return &stepErr{status: domain.StepFailed, err: fmt.Errorf("keluaran Agen Strategi tidak valid setelah %d percobaan: %w", call, perr)}
		}
		valid, rejected := agentstrategy.Structural(out, cfg.Scoring)
		if len(valid) == 0 && len(rejected) > 0 {
			exec.Outcome = "invalid_output"
			problems = problems[:0]
			for _, r := range rejected {
				problems = append(problems, fmt.Sprintf("usulan %d (%s): %s", r.Index, r.Title, strings.Join(r.Reasons, "; ")))
			}
			exec.ErrorText = strings.Join(problems, " | ")
			_ = o.d.Store.UpdateExecution(bctx, exec)
			if call < maxCalls {
				rc.warn("semua usulan ditolak validasi struktural (percobaan %d); meminta perbaikan", call)
				continue
			}
			return &stepErr{status: domain.StepFailed, err: fmt.Errorf("semua usulan ditolak validasi struktural setelah %d percobaan", call)}
		}
		exec.Outcome = "ok"
		if call > 1 {
			exec.Outcome = "repaired"
		}
		if err := o.d.Store.UpdateExecution(bctx, exec); err != nil {
			return err
		}
		for _, r := range rejected {
			rc.warn("usulan ditolak validasi struktural: %q: %s", r.Title, strings.Join(r.Reasons, "; "))
		}
		if len(valid) == 0 {
			rc.warn("Agen Strategi tidak mengusulkan peluang apa pun")
		}
		if strings.TrimSpace(out.Notes) != "" {
			rc.warn("catatan agen: %s", strings.TrimSpace(out.Notes))
		}
		return nil
	}
	return failf("Agen Strategi tidak menghasilkan keluaran")
}

func (o *Orchestrator) stepEvaluate(ctx context.Context, rc *runCtx) error {
	exec, err := o.d.Store.LatestSuccessfulExecution(ctx, rc.run.ID, agentstrategy.Name)
	if errors.Is(err, store.ErrNotFound) {
		return failf("tidak ada keluaran Agen Strategi yang berhasil untuk dievaluasi")
	}
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(exec.ResponsePath)
	if err != nil {
		return failf("keluaran agen tidak dapat dibaca: %v", err)
	}
	out, err := agentstrategy.ParseOutput(string(raw))
	if err != nil {
		return failf("keluaran agen tidak valid: %v", err)
	}
	valid, _ := agentstrategy.Structural(out, o.d.Cfg.Scoring)
	_, byRef, err := o.loadEvidence(ctx, rc.parentRunID)
	if err != nil {
		return err
	}
	evaluator, err := strategy.New(o.d.Cfg, o.d.Checker)
	if err != nil {
		return failf("evaluator: %v", err)
	}
	results := evaluator.Evaluate(valid, byRef)
	existing, err := o.d.Store.OpportunitiesForRun(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	have := map[int]bool{}
	for _, e := range existing {
		have[e.ProposalIndex] = true
	}
	if len(existing) > 0 {
		rc.warn("evaluasi diulang: %d peluang sudah tersimpan untuk run ini; indeks usulan yang sama tidak ditulis ulang", len(existing))
	}
	fixtureHash := rc.sum.FixtureHash
	if fixtureHash == "" && rc.prev != nil {
		fixtureHash = rc.prev.FixtureHash
	}
	now := o.ts()
	blocked, rejected := 0, 0
	for _, r := range results {
		if have[r.ProposalIndex] {
			continue
		}
		dims, _ := json.Marshal(r.Dims)
		caps, _ := json.Marshal(r.Caps)
		gates, _ := json.Marshal(r.Gates)
		conflicts, _ := json.Marshal(r.Conflicts)
		brief, _ := json.Marshal(r.Brief)
		merged, _ := json.Marshal(r.MergedFrom)
		reasons, _ := json.Marshal(r.RejectReasons)
		opp := &domain.Opportunity{ID: o.d.IDs.New("opp"), RunID: rc.run.ID, ParentRunID: rc.parentRunID, ProposalIndex: r.ProposalIndex, Title: r.Title, Segment: r.Segment,
			TopicKey: r.TopicKey, Need: r.Need, Interpretation: r.Interpretation, Status: r.Status, Tier: r.Tier, Hypothesis: r.Hypothesis, RequiresStore: r.RequiresStore,
			DimensionsJSON: string(dims), OrdinalSum: r.OrdinalSum, CapsJSON: string(caps), GatesJSON: string(gates), ConflictsJSON: string(conflicts), BriefJSON: string(brief),
			MergedFromJSON: string(merged), RejectReasonsJSON: string(reasons), ConfigHash: o.d.Cfg.Hash, ScoringConfigHash: o.d.Cfg.Scoring.Hash, FixtureHash: fixtureHash,
			AgentExecutionID: exec.ID, CreatedAt: now}
		var links []domain.OpportunityEvidence
		for _, id := range r.EvidenceIDs {
			links = append(links, domain.OpportunityEvidence{OpportunityID: opp.ID, ItemID: id, Role: r.Roles[id]})
		}
		if err := o.d.Store.InsertOpportunity(ctx, opp, links); err != nil {
			return err
		}
		switch r.Status {
		case domain.OppBlocked:
			blocked++
		case domain.OppRejected:
			rejected++
		}
	}
	if blocked > 0 || rejected > 0 {
		rc.warn("%d usulan diblokir pemeriksa teks merek dan %d ditolak gerbang bukti atau klaim faktual; lihat laporan", blocked, rejected)
	}
	return nil
}

// strategyCounts derives the strategy summary from persisted opportunities and the parent run's evidence.
func (o *Orchestrator) strategyCounts(ctx context.Context, rc *runCtx) (*domain.StrategyCounts, error) {
	sc := &domain.StrategyCounts{}
	opps, err := o.d.Store.OpportunitiesForRun(ctx, rc.run.ID)
	if err != nil {
		return nil, err
	}
	for _, op := range opps {
		sc.Evaluated++
		var merged []int
		if op.MergedFromJSON != "" {
			_ = json.Unmarshal([]byte(op.MergedFromJSON), &merged)
		}
		sc.Merged += len(merged)
		switch op.Status {
		case domain.OppBlocked:
			sc.Blocked++
		case domain.OppRejected:
			sc.Rejected++
		}
		switch op.Tier {
		case domain.TierA:
			sc.TierA++
		case domain.TierB:
			sc.TierB++
		case domain.TierC:
			sc.TierC++
		}
		if op.Hypothesis {
			sc.Hypotheses++
		}
		if op.ConflictsJSON != "" && op.ConflictsJSON != "null" && op.ConflictsJSON != "[]" {
			sc.Conflicted++
		}
		if op.RequiresStore {
			sc.StoreDependent++
		}
	}
	sc.Proposed = sc.Evaluated + sc.Merged
	items, err := o.d.Store.ValidatedItemsForRun(ctx, rc.parentRunID)
	if err != nil {
		return nil, err
	}
	sc.EvidenceItems = len(items)
	groups := map[string]bool{}
	for _, it := range items {
		if it.ConflictGroup != "" {
			groups[it.ConflictGroup] = true
		}
	}
	sc.ConflictGroups = len(groups)
	return sc, nil
}
