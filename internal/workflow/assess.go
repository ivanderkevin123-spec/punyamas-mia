// Package workflow chains the existing research, strategy and content steps into one auditable,
// resumable workflow (Phase 5). It never approves opportunities or drafts, never publishes or sends
// anything, and derives every status, count, blocker and next action from the database.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

// Stages in pipeline order.
const (
	StageResearch          = "research"
	StageStrategy          = "strategy"
	StageOpportunityReview = "opportunity_review"
	StageContentGeneration = "content_generation"
	StageContentReview     = "content_review"
	StageHandoff           = "handoff"
)

// StageOrder lists the stages in pipeline order.
var StageOrder = []string{StageResearch, StageStrategy, StageOpportunityReview, StageContentGeneration, StageContentReview, StageHandoff}

// StageLabels are the Indonesian stage names shown to operators.
var StageLabels = map[string]string{
	StageResearch:          "Riset",
	StageStrategy:          "Strategi",
	StageOpportunityReview: "Tinjauan peluang",
	StageContentGeneration: "Pembuatan draf konten",
	StageContentReview:     "Tinjauan draf",
	StageHandoff:           "Serah terima (ekspor)",
}

// Stage statuses.
const (
	StPending       = "pending"        // an automatic step is available and not started
	StRunning       = "running"        // a run is in progress (recent heartbeat)
	StInterrupted   = "interrupted"    // a run stopped without finishing; resume needed
	StCompleted     = "completed"      // the stage is finished
	StFailed        = "failed"         // a run failed; resume needed
	StBlocked       = "blocked"        // a quality gate or the budget stops the stage
	StAwaitingHuman = "awaiting_human" // a human decision is required
	StNotReached    = "not_reached"    // an earlier stage is not finished
)

type StageView struct {
	Name, Label, Status string
	RunIDs              []string
	Detail              string
	Reasons             []string
}

// OppRow is one opportunity of the workflow's strategy run.
type OppRow struct {
	ID, Title, Segment, Tier, Status, LatestDecision string
	Reviewable                                       bool
	EvidenceIDs                                      []string
	ContentRunID                                     string
	ContentAttempt                                   int
	ContentOrigin                                    string
	ContentStatus                                    string
	CallCapReached                                   bool
	Eligible                                         bool
	IneligibleReasons                                []string
}

// DraftRow is one draft of a content run linked to the workflow.
type DraftRow struct {
	ID, OpportunityID, RunID, ContentType, Status string
	Variant, Version                              int
	Ready                                         bool
	ReadinessReasons                              []string
	Placeholders, ClaimsToVerify, EvidenceIDs     []string
	BlockingCategories                            []string
	Exported                                      bool
}

// ContentRunView is one linked content run attempt.
type ContentRunView struct {
	OpportunityID, RunID, Origin, Status string
	Attempt, Drafts                      int
	Exists                               bool
	ConfigHash, ContentConfigHash        string
	FixtureHash                          string
}

// Counts are read from the database; nil means "tidak tersedia" (the stage has not produced data yet).
type Counts struct {
	ValidatedEvidence, Facts, Interpretations, Hypotheses, Sources *int
	Opportunities                                                  map[string]int
	OpportunityTotal                                               *int
	Drafts                                                         map[string]int
	DraftTotal, DraftsReady, DraftsExported                        *int
	Placeholders, ClaimsToVerify                                   *int
	CostUSD, CostUncertainUSD                                      *float64
}

// Action is one concrete next step; Human marks steps only a person may take.
type Action struct {
	Text, Command string
	Human         bool
}

// Provenance collects the hashes and keys that make the workflow's artefacts traceable.
type Provenance struct {
	WorkflowConfigHash, CurrentConfigHash                  string
	ResearchConfigHash, ResearchFixtureKey                 string
	StrategyConfigHash, ScoringConfigHash, StrategyFixture string
}

// Assessment is the live state of a workflow, computed from the database only.
type Assessment struct {
	Workflow               *domain.Workflow
	Status, Reason         string
	LastCompletedStage     string
	FailedStage            string
	Stages                 []StageView
	Counts                 Counts
	Opportunities          []OppRow
	Drafts                 []DraftRow
	ContentRuns            []ContentRunView
	Blockers               []string
	NextActions            []Action
	HumanDecisionNeeded    bool
	AutomaticStepAvailable bool
	LeaseHeld              bool
	Research, Strategy     *domain.Run
	Provenance             Provenance
	Events                 []*domain.WorkflowEvent
	AssessedAt             string
}

func intp(v int) *int { return &v }

func (a *Assessment) stage(name string) *StageView {
	for i := range a.Stages {
		if a.Stages[i].Name == name {
			return &a.Stages[i]
		}
	}
	return nil
}

func (a *Assessment) block(format string, x ...any) {
	a.Blockers = append(a.Blockers, fmt.Sprintf(format, x...))
}

func (a *Assessment) act(human bool, text, cmd string) {
	a.NextActions = append(a.NextActions, Action{Text: text, Command: cmd, Human: human})
	if human {
		a.HumanDecisionNeeded = true
	}
}

// runStage maps an authoritative run status to a stage status and a reason.
func (s *Service) runStage(ctx context.Context, run *domain.Run) (string, string) {
	switch run.Status {
	case domain.RunCompleted, domain.RunCompletedWithWarnings:
		return StCompleted, ""
	case domain.RunRunning:
		if orchestrator.IsStale(run, s.Now(), s.staleAfter()) {
			return StInterrupted, fmt.Sprintf("run %s terhenti tanpa detak sejak %s", run.ID, run.HeartbeatAt)
		}
		return StRunning, fmt.Sprintf("run %s sedang berjalan", run.ID)
	case domain.RunPartial, domain.RunStale, domain.RunCreated:
		return StInterrupted, fmt.Sprintf("run %s belum lengkap (status %s)", run.ID, run.Status)
	}
	steps, _ := s.Store.StepsForRun(ctx, run.ID)
	var sum domain.RunSummary
	_ = json.Unmarshal([]byte(run.SummaryJSON), &sum)
	reason := strings.Join(sum.Errors, "; ")
	for _, st := range steps {
		if st.Status == domain.StepBudgetExhausted {
			if reason == "" {
				reason = "anggaran habis pada langkah " + st.Name
			}
			return StBlocked, fmt.Sprintf("run %s dihentikan anggaran: %s", run.ID, reason)
		}
	}
	if reason == "" {
		for _, st := range steps {
			if st.Status == domain.StepFailed || st.Status == domain.StepTimedOut {
				reason = fmt.Sprintf("langkah %s berstatus %s %s", st.Name, st.Status, st.ErrorJSON)
				break
			}
		}
	}
	return StFailed, fmt.Sprintf("run %s gagal: %s", run.ID, reason)
}

func (s *Service) getRun(ctx context.Context, id string) (*domain.Run, bool, error) {
	if id == "" {
		return nil, false, nil
	}
	r, err := s.Store.GetRun(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

func summaryOf(r *domain.Run) domain.RunSummary {
	var sum domain.RunSummary
	if r != nil {
		_ = json.Unmarshal([]byte(r.SummaryJSON), &sum)
	}
	return sum
}

// Assess computes the live state of a workflow from the database.
func (s *Service) Assess(ctx context.Context, id string) (*Assessment, error) {
	wf, err := s.Store.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.assess(ctx, wf, "")
}

// assess computes the assessment; self is the lease owner of the calling command (empty when it holds none).
func (s *Service) assess(ctx context.Context, wf *domain.Workflow, self string) (*Assessment, error) {
	now := s.Now()
	a := &Assessment{Workflow: wf, AssessedAt: now.UTC().Format(time.RFC3339),
		Provenance: Provenance{WorkflowConfigHash: wf.ConfigHash, CurrentConfigHash: s.Cfg.Hash}}
	a.LeaseHeld = wf.LeaseOwner != "" && wf.LeaseOwner != self && wf.LeaseExpiresAt >= a.AssessedAt
	for _, name := range StageOrder {
		a.Stages = append(a.Stages, StageView{Name: name, Label: StageLabels[name], Status: StNotReached})
	}
	var err error
	if a.Events, err = s.Store.WorkflowEvents(ctx, wf.ID); err != nil {
		return nil, err
	}
	var cost, uncertain float64
	costRun := func(runID string) error {
		rows, err := s.Store.CostsForRun(ctx, runID)
		if err != nil {
			return err
		}
		for _, c := range rows {
			if c.CostUSD != nil {
				cost += *c.CostUSD
				if c.Estimated {
					uncertain += *c.CostUSD
				}
			}
		}
		return nil
	}

	// Stage 1 — research, gate A.
	rs := a.stage(StageResearch)
	research, rexists, err := s.getRun(ctx, wf.ResearchRunID)
	if err != nil {
		return nil, err
	}
	gateA := false
	switch {
	case wf.ResearchRunID == "":
		rs.Status, rs.Detail = StPending, "run riset belum dibuat"
	case !rexists:
		rs.Status, rs.Detail, rs.RunIDs = StPending, "ID run riset sudah dicadangkan; run belum dibuat", []string{wf.ResearchRunID}
	default:
		a.Research, rs.RunIDs = research, []string{research.ID}
		_ = costRun(research.ID)
		var in struct {
			FixtureKey string `json:"fixture_key"`
		}
		_ = json.Unmarshal([]byte(research.InputsJSON), &in)
		a.Provenance.ResearchConfigHash, a.Provenance.ResearchFixtureKey = research.ConfigHash, in.FixtureKey
		if research.Kind != "research" {
			rs.Status = StFailed
			rs.Reasons = append(rs.Reasons, fmt.Sprintf("integritas: run %s berjenis %s, bukan research", research.ID, research.Kind))
			break
		}
		st, reason := s.runStage(ctx, research)
		rs.Status = st
		if reason != "" {
			rs.Reasons = append(rs.Reasons, reason)
		}
		if st == StCompleted {
			c, err := s.Orch.CountsFromStore(ctx, research.ID)
			if err != nil {
				return nil, err
			}
			validated := c.Facts + c.Interpretations + c.Hypotheses
			a.Counts.ValidatedEvidence, a.Counts.Facts, a.Counts.Interpretations, a.Counts.Hypotheses, a.Counts.Sources =
				intp(validated), intp(c.Facts), intp(c.Interpretations), intp(c.Hypotheses), intp(c.Sources)
			if validated == 0 {
				rs.Status = StBlocked
				rs.Reasons = append(rs.Reasons, "Gerbang A: run riset selesai tetapi tidak ada butir tervalidasi; strategi tidak dapat dijalankan")
			} else {
				gateA = true
				rs.Detail = fmt.Sprintf("%d butir tervalidasi (%d fakta, %d interpretasi, %d hipotesis) dari %d sumber", validated, c.Facts, c.Interpretations, c.Hypotheses, c.Sources)
			}
		}
	}

	// Stage 2 — strategy, gate B.
	ss := a.stage(StageStrategy)
	gateB := false
	var opps []*domain.Opportunity
	if gateA {
		strat, sexists, err := s.getRun(ctx, wf.StrategyRunID)
		if err != nil {
			return nil, err
		}
		switch {
		case wf.StrategyRunID == "":
			ss.Status, ss.Detail = StPending, "run strategi belum dibuat"
		case !sexists:
			ss.Status, ss.Detail, ss.RunIDs = StPending, "ID run strategi sudah dicadangkan; run belum dibuat", []string{wf.StrategyRunID}
		default:
			a.Strategy, ss.RunIDs = strat, []string{strat.ID}
			_ = costRun(strat.ID)
			sum := summaryOf(strat)
			a.Provenance.StrategyConfigHash, a.Provenance.ScoringConfigHash, a.Provenance.StrategyFixture = strat.ConfigHash, sum.ScoringConfigHash, sum.FixtureHash
			if strat.Kind != "strategy" || strat.ParentRunID != research.ID {
				ss.Status = StFailed
				ss.Reasons = append(ss.Reasons, fmt.Sprintf("integritas: run %s bukan run strategi atas run riset %s", strat.ID, research.ID))
				break
			}
			st, reason := s.runStage(ctx, strat)
			ss.Status = st
			if reason != "" {
				ss.Reasons = append(ss.Reasons, reason)
			}
			if st != StCompleted {
				break
			}
			if opps, err = s.Store.OpportunitiesForRun(ctx, strat.ID); err != nil {
				return nil, err
			}
			validated, err := s.Store.ValidatedItemsForRun(ctx, research.ID)
			if err != nil {
				return nil, err
			}
			ok := map[string]bool{}
			for _, it := range validated {
				ok[it.ID] = true
			}
			bad := 0
			for _, op := range opps {
				links, err := s.Store.EvidenceForOpportunity(ctx, op.ID)
				if err != nil {
					return nil, err
				}
				for _, l := range links {
					if !ok[l.ItemID] {
						bad++
						ss.Reasons = append(ss.Reasons, fmt.Sprintf("Gerbang B: peluang %s mengutip butir %s yang tidak (lagi) tervalidasi", op.ID, l.ItemID))
					}
				}
			}
			if bad > 0 {
				ss.Status = StBlocked
				break
			}
			gateB = true
			ss.Detail = fmt.Sprintf("%d peluang dievaluasi", len(opps))
		}
	}

	// Stage 3 — opportunity review, gate C (status and history must agree; nothing is approved here).
	ors := a.stage(StageOpportunityReview)
	links, err := s.Store.WorkflowContentRuns(ctx, wf.ID)
	if err != nil {
		return nil, err
	}
	latest := map[string]*domain.WorkflowContentRun{}
	for _, l := range links {
		latest[l.OpportunityID] = l
	}
	approved := 0
	if gateB {
		a.Counts.Opportunities = map[string]int{}
		a.Counts.OpportunityTotal = intp(len(opps))
		reviewable, proposed := 0, 0
		for _, op := range opps {
			a.Counts.Opportunities[op.Status]++
			row := OppRow{ID: op.ID, Title: op.Title, Segment: op.Segment, Tier: op.Tier, Status: op.Status, Reviewable: op.Tier != domain.TierNone && op.Tier != ""}
			ev, err := s.Store.EvidenceForOpportunity(ctx, op.ID)
			if err != nil {
				return nil, err
			}
			for _, l := range ev {
				row.EvidenceIDs = append(row.EvidenceIDs, l.ItemID)
			}
			sort.Strings(row.EvidenceIDs)
			apps, err := s.Store.ApprovalsFor(ctx, "opportunity", op.ID)
			if err != nil {
				return nil, err
			}
			if len(apps) > 0 {
				row.LatestDecision = apps[len(apps)-1].Decision
			}
			if row.Reviewable {
				reviewable++
				if op.Status == domain.OppProposed {
					proposed++
				}
			}
			if op.Status == domain.OppApproved {
				approved++
				if _, err := content.Load(ctx, s.Store, op.ID); err != nil {
					var ie *content.IneligibleError
					if !errors.As(err, &ie) {
						return nil, err
					}
					row.IneligibleReasons = ie.Reasons
				} else {
					row.Eligible = true
				}
			}
			a.Opportunities = append(a.Opportunities, row)
		}
		switch {
		case reviewable == 0:
			ors.Status = StBlocked
			ors.Reasons = append(ors.Reasons, fmt.Sprintf("tidak ada peluang yang layak ditinjau (%d peluang; diblokir %d, ditolak gerbang %d)", len(opps), a.Counts.Opportunities[domain.OppBlocked], a.Counts.Opportunities[domain.OppRejected]))
		case proposed > 0:
			ors.Status = StAwaitingHuman
			ors.Detail = fmt.Sprintf("%d peluang menunggu keputusan manusia", proposed)
		case approved == 0:
			ors.Status = StBlocked
			ors.Reasons = append(ors.Reasons, "tidak ada peluang yang disetujui (semua ditolak atau perlu bukti tambahan)")
		default:
			ors.Status = StCompleted
			ors.Detail = fmt.Sprintf("%d peluang disetujui", approved)
		}
	}

	// Stage 4 — content generation, gate D (eligibility through content.Load).
	cgs := a.stage(StageContentGeneration)
	var draftRuns []string
	runIndex := map[string]int{}
	if approved > 0 {
		pending, running, failed, interrupted, blocked, done := 0, 0, 0, 0, 0, 0
		for i := range a.Opportunities {
			row := &a.Opportunities[i]
			if row.Status != domain.OppApproved {
				continue
			}
			l := latest[row.ID]
			if l == nil {
				if row.Eligible {
					row.ContentStatus = StPending
					pending++
				} else {
					row.ContentStatus = StBlocked
					blocked++
					cgs.Reasons = append(cgs.Reasons, fmt.Sprintf("Gerbang D: peluang %s disetujui tetapi tidak memenuhi syarat: %s", row.ID, strings.Join(row.IneligibleReasons, "; ")))
				}
				continue
			}
			row.ContentRunID, row.ContentAttempt, row.ContentOrigin = l.ContentRunID, l.Attempt, l.Origin
			cgs.RunIDs = append(cgs.RunIDs, l.ContentRunID)
			run, exists, err := s.getRun(ctx, l.ContentRunID)
			if err != nil {
				return nil, err
			}
			if !exists {
				row.ContentStatus = StPending
				pending++
				continue
			}
			st, reason := s.runStage(ctx, run)
			if st == StCompleted && (run.Kind != "content" || run.ParentRunID != wf.StrategyRunID) {
				st, reason = StFailed, fmt.Sprintf("integritas: run %s bukan run konten atas run strategi %s", run.ID, wf.StrategyRunID)
			}
			row.ContentStatus = st
			if st != StCompleted {
				// The cap only blocks a resume that would need another agent call (agent step unfinished).
				agentDone := false
				if steps, err := s.Store.StepsForRun(ctx, run.ID); err == nil {
					for _, x := range steps {
						if x.Name == orchestrator.StepContentAgent && x.Status == domain.StepCompleted {
							agentDone = true
						}
					}
				}
				if n, err := s.Store.CountExecutions(ctx, run.ID, "content"); err == nil && n >= s.Cfg.Content.MaxCallsPerRun && !agentDone {
					row.CallCapReached = true
				}
			}
			switch st {
			case StCompleted:
				done++
			case StRunning:
				running++
			case StFailed:
				failed++
			case StInterrupted:
				interrupted++
			case StBlocked:
				blocked++
			}
			if reason != "" && st != StCompleted {
				cgs.Reasons = append(cgs.Reasons, fmt.Sprintf("peluang %s: %s", row.ID, reason))
			}
		}
		switch {
		case failed > 0 || interrupted > 0:
			cgs.Status = StFailed
			if failed == 0 {
				cgs.Status = StInterrupted
			}
		case running > 0:
			cgs.Status = StRunning
		case pending > 0:
			cgs.Status = StPending
		case done == 0 && blocked > 0:
			cgs.Status = StBlocked
		default:
			cgs.Status = StCompleted
		}
		cgs.Detail = fmt.Sprintf("run konten: selesai %d, tertunda %d, berjalan %d, gagal %d, terhenti %d, diblokir %d", done, pending, running, failed, interrupted, blocked)
	}
	for _, l := range links {
		run, exists, err := s.getRun(ctx, l.ContentRunID)
		if err != nil {
			return nil, err
		}
		v := ContentRunView{OpportunityID: l.OpportunityID, RunID: l.ContentRunID, Origin: l.Origin, Attempt: l.Attempt, Exists: exists, Status: "belum dibuat"}
		if exists {
			sum := summaryOf(run)
			v.Status, v.ConfigHash, v.ContentConfigHash, v.FixtureHash = run.Status, run.ConfigHash, sum.ContentConfigHash, sum.FixtureHash
			_ = costRun(run.ID)
			draftRuns = append(draftRuns, run.ID)
			runIndex[run.ID] = len(a.ContentRuns)
		}
		a.ContentRuns = append(a.ContentRuns, v)
	}

	// Stage 5 and 6 — content review and handoff, gate E (content.ReadinessOf).
	exported := map[string]int{}
	for _, e := range a.Events {
		if e.Kind == "export" {
			var v int
			fmt.Sscanf(e.Detail, "v%d", &v)
			if v > exported[e.RefID] {
				exported[e.RefID] = v
			}
		}
	}
	if len(draftRuns) > 0 {
		a.Counts.Drafts = map[string]int{}
		total, ready, nExported, placeholders, claims := 0, 0, 0, 0, 0
		pendingReview := 0
		for _, runID := range draftRuns {
			drafts, err := s.Store.ContentDraftsForRun(ctx, runID)
			if err != nil {
				return nil, err
			}
			a.ContentRuns[runIndex[runID]].Drafts = len(drafts)
			for _, d := range drafts {
				reviews, err := s.Store.ApprovalsFor(ctx, domain.SubjectContentDraft, d.ID)
				if err != nil {
					return nil, err
				}
				rd := content.ReadinessOf(d, reviews)
				dt := content.DecodeDetails(d)
				row := DraftRow{ID: d.ID, OpportunityID: d.OpportunityID, RunID: d.RunID, ContentType: d.ContentType, Status: d.Status, Variant: d.Variant, Version: d.Version,
					Ready: rd.Ready, ReadinessReasons: rd.Reasons, Placeholders: dt.Placeholders, ClaimsToVerify: dt.ClaimsToVerify(), Exported: exported[d.ID] >= d.Version && exported[d.ID] > 0}
				ev, err := s.Store.ContentEvidenceFor(ctx, d.ID)
				if err != nil {
					return nil, err
				}
				for _, l := range ev {
					row.EvidenceIDs = append(row.EvidenceIDs, l.ItemID)
				}
				for _, f := range dt.Findings {
					if f.Severity == content.SeverityBlock {
						row.BlockingCategories = append(row.BlockingCategories, f.Category)
					}
				}
				a.Drafts = append(a.Drafts, row)
				a.Counts.Drafts[d.Status]++
				total++
				// Open work only: placeholders and claims of rejected or blocked drafts are not counted.
				if d.Status == domain.DraftDraft || d.Status == domain.DraftNeedsReview || d.Status == domain.DraftApproved {
					placeholders += len(row.Placeholders)
					claims += len(row.ClaimsToVerify)
				}
				switch {
				case row.Ready:
					ready++
					if row.Exported {
						nExported++
					}
				case d.Status == domain.DraftDraft || d.Status == domain.DraftNeedsReview || d.Status == domain.DraftApproved:
					pendingReview++
				}
			}
		}
		a.Counts.DraftTotal, a.Counts.DraftsReady, a.Counts.DraftsExported = intp(total), intp(ready), intp(nExported)
		a.Counts.Placeholders, a.Counts.ClaimsToVerify = intp(placeholders), intp(claims)
		crs, hs := a.stage(StageContentReview), a.stage(StageHandoff)
		switch {
		case total == 0:
			crs.Status = StBlocked
			crs.Reasons = append(crs.Reasons, "run konten selesai tanpa draf tersimpan (semua ditolak struktural, duplikat, atau kosong)")
		case pendingReview > 0:
			crs.Status = StAwaitingHuman
			crs.Detail = fmt.Sprintf("%d draf menunggu tinjauan, revisi, atau pengisian [ISI: ...]", pendingReview)
		default:
			crs.Status = StCompleted
			crs.Detail = "semua draf sudah ditinjau atau diblokir"
		}
		switch {
		case ready == 0:
			if crs.Status == StCompleted {
				hs.Status = StBlocked
				hs.Reasons = append(hs.Reasons, "Gerbang E: tidak ada draf yang siap serah terima (semua ditolak, diblokir, atau belum lengkap)")
			}
		case nExported < ready:
			hs.Status = StAwaitingHuman
			hs.Detail = fmt.Sprintf("%d draf siap serah terima, %d sudah diekspor", ready, nExported)
		default:
			hs.Status = StCompleted
			hs.Detail = fmt.Sprintf("%d draf siap serah terima sudah diekspor (bukan publikasi)", ready)
		}
	}
	if research != nil || a.Strategy != nil || len(links) > 0 {
		a.Counts.CostUSD, a.Counts.CostUncertainUSD = &cost, &uncertain
	}
	s.decide(a)
	return a, nil
}

// decide applies the documented precedence to derive the workflow status, blockers and next actions.
func (s *Service) decide(a *Assessment) {
	wf := a.Workflow
	id := wf.ID
	for _, name := range StageOrder {
		st := a.stage(name)
		if st.Status == StCompleted {
			a.LastCompletedStage = name
			continue
		}
		break
	}
	for _, st := range a.Stages {
		if st.Status == StFailed || st.Status == StInterrupted {
			a.FailedStage = st.Name
			break
		}
	}
	rs, ss, ors, cgs, crs, hs := a.stage(StageResearch), a.stage(StageStrategy), a.stage(StageOpportunityReview), a.stage(StageContentGeneration), a.stage(StageContentReview), a.stage(StageHandoff)
	resume := "mia workflow resume --id " + id
	for _, st := range a.Stages {
		for _, r := range st.Reasons {
			a.block("%s: %s", st.Label, r)
		}
	}

	// Human actions are always listed, whatever the overall status.
	for _, o := range a.Opportunities {
		if o.Status == domain.OppProposed && o.Reviewable {
			a.act(true, fmt.Sprintf("Tinjau peluang %s (%q, tingkat %s) dan catat keputusan", o.ID, o.Title, o.Tier),
				fmt.Sprintf("mia approve opportunity --id %s --decision approved|rejected|needs_more_evidence --note \"...\"", o.ID))
		}
		if o.CallCapReached {
			a.act(true, fmt.Sprintf("Run konten %s untuk peluang %s mencapai batas panggilan; tidak dicoba ulang otomatis. Buat run konten baru secara sengaja", o.ContentRunID, o.ID),
				fmt.Sprintf("mia workflow resume --id %s --new-content-run %s", id, o.ID))
		}
	}
	for _, d := range a.Drafts {
		switch {
		case d.Status == domain.DraftDraft || d.Status == domain.DraftNeedsReview:
			a.act(true, fmt.Sprintf("Tinjau draf %s (%s varian %d, versi %d)", d.ID, d.ContentType, d.Variant, d.Version),
				fmt.Sprintf("mia content show --id %s && mia content review --id %s --decision approved|rejected --note \"...\"", d.ID, d.ID))
		case d.Status == domain.DraftApproved && !d.Ready:
			text := fmt.Sprintf("Draf %s disetujui tetapi belum siap serah terima: %s", d.ID, strings.Join(d.ReadinessReasons, "; "))
			if len(d.Placeholders) > 0 {
				text = fmt.Sprintf("Lengkapi isian draf %s (%s), lalu tinjau ulang", d.ID, strings.Join(d.Placeholders, ", "))
			}
			a.act(true, text, fmt.Sprintf("mia content revise --id %s --body-file ISI.txt --note \"...\"", d.ID))
		}
	}
	ready, exportedAll := 0, true
	for _, d := range a.Drafts {
		if d.Ready {
			ready++
			if !d.Exported {
				exportedAll = false
			}
		}
	}

	switch {
	case rs.Status == StFailed || rs.Status == StInterrupted || ss.Status == StFailed || ss.Status == StInterrupted || cgs.Status == StFailed || cgs.Status == StInterrupted:
		a.Status = domain.WfFailed
		a.Reason = fmt.Sprintf("tahap %s gagal atau terhenti", StageLabels[a.FailedStage])
		a.act(false, "Periksa kegagalan pada tahap "+StageLabels[a.FailedStage]+"; lanjutkan run yang sama dari titik aman terakhir (tanpa run duplikat)", resume)
	case rs.Status == StBlocked || ss.Status == StBlocked:
		a.Status = domain.WfBlocked
		a.Reason = "gerbang kualitas atau anggaran menghentikan tahap " + map[bool]string{true: StageLabels[StageResearch], false: StageLabels[StageStrategy]}[rs.Status == StBlocked]
		a.act(true, "Perbaiki penyebab blokir (lihat daftar blokir); bila anggaran yang menghentikan, tunggu periode berikutnya atau ubah plafon secara sengaja, lalu lanjutkan", resume)
	case rs.Status == StPending || rs.Status == StRunning:
		a.Status, a.AutomaticStepAvailable = domain.WfCreated, rs.Status == StPending
		a.Reason = "riset belum selesai"
		if rs.Status == StPending {
			a.act(false, "Jalankan riset (mock) untuk workflow ini", "mia workflow run --id "+id)
		}
	case ss.Status == StPending || ss.Status == StRunning:
		a.Status, a.AutomaticStepAvailable = domain.WfResearchCompleted, ss.Status == StPending
		a.Reason = "riset lolos Gerbang A; strategi belum selesai"
		if ss.Status == StPending {
			a.act(false, "Jalankan Agen Strategi (mock) atas bukti tervalidasi", "mia workflow run --id "+id)
		}
	case cgs.Status == StPending || cgs.Status == StRunning:
		a.Status, a.AutomaticStepAvailable = domain.WfStrategyCompleted, cgs.Status == StPending
		a.Reason = "ada peluang disetujui yang belum dibuatkan draf"
		if cgs.Status == StPending {
			a.act(false, "Buat draf konten (mock) untuk peluang yang disetujui dan memenuhi syarat", "mia workflow run --id "+id)
		}
	case cgs.Status == StBlocked && budgetBlocked(cgs):
		a.Status = domain.WfBlocked
		a.Reason = "anggaran menghentikan pembuatan draf"
		a.act(true, "Anggaran habis; tunggu periode berikutnya atau ubah plafon secara sengaja, lalu lanjutkan", resume)
	case ors.Status == StAwaitingHuman:
		a.Status = domain.WfAwaitingOpportunityReview
		a.Reason = ors.Detail
	case crs.Status == StAwaitingHuman:
		a.Status = domain.WfAwaitingContentReview
		a.Reason = crs.Detail
	case ready > 0:
		a.Status = domain.WfCompleted
		a.Reason = fmt.Sprintf("%d draf siap serah terima; tidak ada keputusan yang tertunda", ready)
	case ors.Status == StBlocked:
		a.Status = domain.WfBlocked
		a.Reason = strings.Join(ors.Reasons, "; ")
		a.act(true, "Tidak ada peluang yang dapat dilanjutkan; tambahkan bukti dan mulai workflow baru", fmt.Sprintf("mia workflow start --inputs %s --new", wf.InputsDir))
	case cgs.Status == StBlocked:
		a.Status = domain.WfBlocked
		a.Reason = "peluang yang disetujui tidak memenuhi syarat Agen Konten"
	default:
		a.Status = domain.WfBlocked
		a.Reason = "tidak ada draf yang siap serah terima"
		if crs.Status == StBlocked || hs.Status == StBlocked {
			a.act(true, "Revisi draf yang diblokir atau buat run konten baru secara sengaja", "mia content revise --id DRAFT_ID ... | "+resume+" --new-content-run OPP_ID")
		}
	}
	if ready > 0 && !exportedAll {
		a.act(true, "Ekspor draf siap serah terima ke drafts/ (serah terima manual, bukan publikasi)", "mia workflow export --id "+id)
	}
	if a.Status == domain.WfCompleted && exportedAll {
		a.act(false, "Workflow selesai untuk tahap saat ini; persetujuan publikasi dan penerbitan dilakukan manual di luar MIA", "")
	}
	if a.LeaseHeld {
		a.Blockers = append(a.Blockers, fmt.Sprintf("workflow sedang dimajukan oleh proses lain sampai %s", wf.LeaseExpiresAt))
	}
}

func budgetBlocked(st *StageView) bool {
	for _, r := range st.Reasons {
		if strings.Contains(r, "anggaran") {
			return true
		}
	}
	return false
}
