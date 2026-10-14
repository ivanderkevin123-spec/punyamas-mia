package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	agentcontent "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

// Content run steps (Phase 4), executed in order; the report step is shared.
const (
	StepLoadOpportunity = "load_opportunity"
	StepContentAgent    = "content_agent"
	StepEvaluateContent = "evaluate_content"
)

// ErrDraftsExist refuses a silent second generation for an opportunity that already has drafts.
var ErrDraftsExist = errors.New("draf sudah ada untuk peluang ini")

type contentState struct {
	src                                    *content.Source
	evaluated                              bool
	proposed, structuralRejected, dupSkips int
}

type contentInputs struct {
	OpportunityID string   `json:"opportunity_id"`
	FixtureKey    string   `json:"fixture_key"`
	ContentTypes  []string `json:"content_types"`
	Regenerate    bool     `json:"regenerate"`
}

func (o *Orchestrator) contentDefs() []stepDef {
	return []stepDef{
		{name: StepLoadOpportunity, required: true, fn: o.stepLoadOpportunity},
		{name: StepContentAgent, required: true, retry: true, fn: o.stepContentAgent},
		{name: StepEvaluateContent, required: true, fn: o.stepEvaluateContent},
		{name: StepReport, required: true, fn: o.stepReport},
	}
}

// ContentStepNames lists the content workflow steps in order.
func ContentStepNames() []string {
	return []string{StepLoadOpportunity, StepContentAgent, StepEvaluateContent, StepReport}
}

// RunContent executes (or resumes) a content run for one approved opportunity. Ineligible opportunities
// and silent regeneration are refused before any run is created or any agent call is made.
func (o *Orchestrator) RunContent(ctx context.Context, opts Options) (*domain.RunSummary, error) {
	now := o.ts()
	cfg := o.d.Cfg
	var run *domain.Run
	var steps []*domain.Step
	var prev *domain.RunSummary
	if opts.ResumeID != "" {
		r, err := o.d.Store.GetRun(ctx, opts.ResumeID)
		if err != nil {
			return nil, fmt.Errorf("run %s: %w", opts.ResumeID, err)
		}
		if r.Kind != "content" {
			return nil, fmt.Errorf("run %s bukan run konten", r.ID)
		}
		var in contentInputs
		_ = json.Unmarshal([]byte(r.InputsJSON), &in)
		opts.OpportunityID, opts.FixtureKey, opts.ContentTypes, opts.Regenerate = in.OpportunityID, in.FixtureKey, in.ContentTypes, in.Regenerate
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
		if opts.OpportunityID == "" {
			return nil, errors.New("peluang sumber (--opportunity) wajib diisi")
		}
		src, err := content.Load(ctx, o.d.Store, opts.OpportunityID)
		if err != nil {
			return nil, err
		}
		existing, err := o.d.Store.ContentDraftsForOpportunity(ctx, src.Opportunity.ID)
		if err != nil {
			return nil, err
		}
		if len(existing) > 0 && !opts.Regenerate {
			return nil, fmt.Errorf("%w: peluang %s sudah memiliki %d draf (run %s); gunakan --regenerate untuk membuat ulang secara sengaja", ErrDraftsExist, src.Opportunity.ID, len(existing), existing[0].RunID)
		}
		if len(opts.ContentTypes) == 0 {
			opts.ContentTypes = append([]string(nil), cfg.Content.DefaultTypes...)
		}
		for _, t := range opts.ContentTypes {
			if _, ok := cfg.Content.Types[t]; !ok {
				return nil, fmt.Errorf("jenis konten %q tidak dikenal (tersedia: %s)", t, strings.Join(cfg.Content.DefaultTypes, ", "))
			}
		}
		if opts.FixtureKey == "" {
			opts.FixtureKey = cfg.Limits.Provider.Mock.FixtureKey
		}
		if opts.FixtureKey == "" {
			opts.FixtureKey = src.StrategyFixtureKey + "-" + src.Opportunity.TopicKey
		}
		inJ, _ := json.Marshal(contentInputs{OpportunityID: src.Opportunity.ID, FixtureKey: opts.FixtureKey, ContentTypes: opts.ContentTypes, Regenerate: opts.Regenerate})
		run = &domain.Run{ID: o.newRunID(opts), Kind: "content", Status: domain.RunRunning, CreatedAt: now, StartedAt: now, HeartbeatAt: now,
			ConfigHash: cfg.Hash, ProviderName: o.d.Provider.Name(), InputsJSON: string(inJ), SummaryJSON: "{}", CostMeasurable: true, ParentRunID: src.Opportunity.RunID}
		if err := o.d.Store.CreateRun(ctx, run); err != nil {
			return nil, err
		}
		for i, def := range o.contentDefs() {
			steps = append(steps, &domain.Step{ID: o.d.IDs.New("step"), RunID: run.ID, Name: def.name, Ordinal: i + 1, Status: domain.StepPending})
		}
		if err := o.d.Store.CreateSteps(ctx, steps); err != nil {
			return nil, err
		}
	}
	rc := &runCtx{run: run, steps: map[string]*domain.Step{}, opts: opts, prev: prev, skipped: map[string]bool{}, parentRunID: run.ParentRunID, content: &contentState{},
		sum: &domain.RunSummary{RunID: run.ID, Kind: run.Kind, CostMeasurable: true, ParentRunID: run.ParentRunID, OpportunityID: opts.OpportunityID, ContentConfigHash: cfg.Content.Hash}}
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
	o.d.Log.Info("content run started", "run_id", run.ID, "opportunity_id", opts.OpportunityID, "resume", opts.ResumeID != "", "provider", run.ProviderName)
	return o.execute(ctx, rc, o.contentDefs())
}

// contentSource re-checks eligibility once per invocation, so a resumed run never continues for an
// opportunity whose approval or evidence changed in the meantime.
func (o *Orchestrator) contentSource(ctx context.Context, rc *runCtx) (*content.Source, error) {
	if rc.content.src != nil {
		return rc.content.src, nil
	}
	src, err := content.Load(ctx, o.d.Store, rc.opts.OpportunityID)
	if err != nil {
		if content.IsIneligible(err) {
			return nil, failf("%v", err)
		}
		return nil, err
	}
	if src.Opportunity.RunID != rc.parentRunID {
		return nil, failf("peluang %s tidak berasal dari run strategi %s", src.Opportunity.ID, rc.parentRunID)
	}
	rc.content.src = src
	return src, nil
}

func (o *Orchestrator) stepLoadOpportunity(ctx context.Context, rc *runCtx) error {
	_, err := o.contentSource(ctx, rc)
	return err
}

func (o *Orchestrator) stepContentAgent(ctx context.Context, rc *runCtx) error {
	cfg := o.d.Cfg
	src, err := o.contentSource(ctx, rc)
	if err != nil {
		return err
	}
	b := src.Brief
	opp := agentcontent.OpportunityForPrompt{Title: src.Opportunity.Title, Segment: src.Opportunity.Segment, TopicKey: src.Opportunity.TopicKey, Tier: src.Opportunity.Tier,
		Need: src.Opportunity.Need, Interpretation: src.Opportunity.Interpretation, Message: b.Message, OfferConcept: b.OfferConcept, Channel: b.Channel, TestDesign: b.TestDesign,
		SuccessMetric: b.SuccessMetric, RequiresStore: src.Opportunity.RequiresStore, MissingEvidence: b.MissingEvidence, Dependencies: b.Dependencies}
	var ev []agentcontent.EvidenceForPrompt
	for _, e := range src.Evidence {
		ev = append(ev, agentcontent.EvidenceForPrompt{Ref: e.Ref, Kind: e.Kind, Quality: e.Quality, Title: e.Title, Statement: e.Statement, SourceName: e.SourceName})
	}
	var facts []agentcontent.FactForPrompt
	for _, f := range cfg.Business.Facts {
		facts = append(facts, agentcontent.FactForPrompt{ID: f.ID, Text: f.Text, Status: f.Status})
	}
	types := rc.opts.ContentTypes
	return o.invokeAgent(ctx, rc, agentCall{
		agent: agentcontent.Name, label: "Agen Konten", unit: "draf", schema: agentcontent.SchemaVersion, step: StepContentAgent,
		maxPerRun: cfg.Content.MaxCallsPerRun, restartHint: "mulai run konten baru dengan run content --opportunity " + src.Opportunity.ID,
		request: agentcontent.BuildRequest(cfg, opp, ev, facts, types, rc.opts.FixtureKey), repair: agentcontent.RepairRequest,
		validate: func(raw string) agentValidation {
			out, perr := agentcontent.ParseOutput(raw)
			if perr != nil {
				return agentValidation{parseErr: perr}
			}
			valid, rejected, short := agentcontent.Structural(out, cfg.Content, cfg.Scoring.Segments, types)
			var v agentValidation
			if len(valid) == 0 && len(rejected) > 0 {
				for _, r := range rejected {
					v.problems = append(v.problems, fmt.Sprintf("draf %d (%s varian %d): %s", r.Index, r.ContentType, r.Variant, strings.Join(r.Reasons, "; ")))
				}
				return v
			}
			for _, r := range rejected {
				v.warnings = append(v.warnings, fmt.Sprintf("draf %d (%s varian %d) ditolak validasi struktural: %s", r.Index, r.ContentType, r.Variant, strings.Join(r.Reasons, "; ")))
			}
			v.warnings = append(v.warnings, short...)
			if len(valid) == 0 {
				v.warnings = append(v.warnings, "Agen Konten tidak menghasilkan draf apa pun")
			}
			if strings.TrimSpace(out.Notes) != "" {
				v.warnings = append(v.warnings, "catatan agen: "+strings.TrimSpace(out.Notes))
			}
			return v
		},
	})
}

func (o *Orchestrator) stepEvaluateContent(ctx context.Context, rc *runCtx) error {
	cfg := o.d.Cfg
	src, err := o.contentSource(ctx, rc)
	if err != nil {
		return err
	}
	exec, err := o.d.Store.LatestSuccessfulExecution(ctx, rc.run.ID, agentcontent.Name)
	if errors.Is(err, store.ErrNotFound) {
		return failf("tidak ada keluaran Agen Konten yang berhasil untuk dievaluasi")
	}
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(exec.ResponsePath)
	if err != nil {
		return failf("keluaran agen tidak dapat dibaca: %v", err)
	}
	out, err := agentcontent.ParseOutput(string(raw))
	if err != nil {
		return failf("keluaran agen tidak valid: %v", err)
	}
	valid, rejected, _ := agentcontent.Structural(out, cfg.Content, cfg.Scoring.Segments, rc.opts.ContentTypes)
	all, err := o.d.Store.ContentDraftsForOpportunity(ctx, src.Opportunity.ID)
	if err != nil {
		return err
	}
	have := map[int]bool{}
	var others []*domain.ContentDraft
	for _, d := range all {
		if d.RunID == rc.run.ID {
			have[d.ProposalIndex] = true
		} else {
			others = append(others, d)
		}
	}
	if len(have) > 0 {
		rc.warn("evaluasi diulang: %d draf sudah tersimpan untuk run ini; indeks usulan yang sama tidak ditulis ulang", len(have))
	}
	evaluator, err := content.New(cfg, o.d.Checker)
	if err != nil {
		return failf("evaluator konten: %v", err)
	}
	fixtureHash := rc.sum.FixtureHash
	if fixtureHash == "" && rc.prev != nil {
		fixtureHash = rc.prev.FixtureHash
	}
	prov := content.Provenance{RunID: rc.run.ID, ExecutionID: exec.ID, ConfigHash: cfg.Hash, ContentConfigHash: cfg.Content.Hash, FixtureHash: fixtureHash, Now: o.ts()}
	cs := rc.content
	cs.evaluated, cs.proposed, cs.structuralRejected, cs.dupSkips = true, len(out.Drafts), len(rejected), 0
	blocked, review := 0, 0
	for _, r := range evaluator.EvaluateAll(valid, src, others) {
		if have[r.Index] {
			continue
		}
		if r.DuplicateOf != "" {
			cs.dupSkips++
			rc.warn("draf %d (%s varian %d) hampir sama dengan %s; tidak disimpan", r.Index, r.Draft.ContentType, r.Draft.Variant, r.DuplicateOf)
			continue
		}
		d, links, v := r.Record(o.d.IDs.New("draft"), src, prov)
		if err := o.d.Store.InsertContentDraft(ctx, d, links, v); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				cs.dupSkips++
				rc.warn("draf %d (%s varian %d) identik dengan draf tersimpan; tidak disimpan", r.Index, r.Draft.ContentType, r.Draft.Variant)
				continue
			}
			return err
		}
		switch r.Status {
		case domain.DraftBlocked:
			blocked++
		case domain.DraftNeedsReview:
			review++
		}
	}
	if blocked > 0 {
		rc.warn("%d draf diblokir gerbang kode (bukti, klaim, atau bahasa); lihat laporan", blocked)
	}
	if review > 0 {
		rc.warn("%d draf memerlukan tinjauan (isian [ISI: ...], klaim yang perlu verifikasi, atau temuan)", review)
	}
	return nil
}

// contentCounts derives the content summary from stored drafts; agent-output counters come from this
// invocation's evaluation or, when evaluation was skipped on resume, from the previous summary.
func (o *Orchestrator) contentCounts(ctx context.Context, rc *runCtx) (*domain.ContentCounts, error) {
	cc := &domain.ContentCounts{}
	drafts, err := o.d.Store.ContentDraftsForRun(ctx, rc.run.ID)
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		cc.Stored++
		switch d.Status {
		case domain.DraftDraft:
			cc.Draft++
		case domain.DraftNeedsReview:
			cc.NeedsReview++
		case domain.DraftApproved:
			cc.Approved++
		case domain.DraftRejected:
			cc.Rejected++
		case domain.DraftBlocked:
			cc.Blocked++
		}
		dt := content.DecodeDetails(d)
		cc.Placeholders += len(dt.Placeholders)
		cc.ClaimsToVerify += len(dt.ClaimsToVerify())
		links, err := o.d.Store.ContentEvidenceFor(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		cc.EvidenceLinks += len(links)
	}
	switch {
	case rc.content != nil && rc.content.evaluated:
		cc.Proposed, cc.StructuralRejected, cc.DuplicatesSkipped = rc.content.proposed, rc.content.structuralRejected, rc.content.dupSkips
	case rc.prev != nil && rc.prev.Content != nil:
		cc.Proposed, cc.StructuralRejected, cc.DuplicatesSkipped = rc.prev.Content.Proposed, rc.prev.Content.StructuralRejected, rc.prev.Content.DuplicatesSkipped
	}
	return cc, nil
}
