package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/budget"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/importer"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

// Service drives workflows. It holds no workflow state in memory between commands.
type Service struct {
	Store *store.Store
	Orch  *orchestrator.Orchestrator
	Cfg   *config.Config
	IDs   ids.Generator
	Now   func() time.Time
	Log   *slog.Logger
}

var (
	ErrOpenWorkflow = errors.New("workflow terbuka untuk input ini sudah ada")
	ErrInvalidInput = errors.New("input riset tidak valid")
)

func (s *Service) staleAfter() time.Duration {
	if d := time.Duration(s.Cfg.Limits.Caps.StaleAfterS) * time.Second; d > 0 {
		return d
	}
	return 10 * time.Minute
}

func (s *Service) ts() string { return s.Now().UTC().Format(time.RFC3339) }

func (s *Service) event(wfID, kind, stage, ref, detail string) *domain.WorkflowEvent {
	return &domain.WorkflowEvent{WorkflowID: wfID, Kind: kind, Stage: stage, RefID: ref, Detail: detail, CreatedAt: s.ts()}
}

func (s *Service) log() *slog.Logger {
	if s.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Log
}

// Start validates the inputs and creates the workflow record in one transaction. Invalid inputs
// leave nothing behind. A second open workflow for the same inputs is refused unless allowNew.
func (s *Service) Start(ctx context.Context, inputsDir, fixtureKey string, allowNew bool) (*domain.Workflow, error) {
	if inputsDir == "" {
		return nil, fmt.Errorf("%w: direktori input (--inputs) wajib diisi", ErrInvalidInput)
	}
	abs, err := filepath.Abs(inputsDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s bukan direktori yang dapat dibaca", ErrInvalidInput, abs)
	}
	res, err := importer.ImportDir(abs, s.Cfg.Sources.Imports.MaxFileBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if len(res.Errors) > 0 {
		var parts []string
		for _, e := range res.Errors {
			parts = append(parts, e.File+": "+e.Message)
		}
		return nil, fmt.Errorf("%w: %d berkas gagal validasi (%s); perbaiki lalu ulangi (mia validate --inputs %s)", ErrInvalidInput, len(res.Errors), strings.Join(parts, "; "), abs)
	}
	if len(res.Notes) == 0 {
		return nil, fmt.Errorf("%w: tidak ada berkas riset yang valid di %s", ErrInvalidInput, abs)
	}
	existing, err := s.Store.WorkflowsForInputs(ctx, abs)
	if err != nil {
		return nil, err
	}
	if !allowNew {
		for _, w := range existing {
			if w.Status != domain.WfCompleted {
				return nil, fmt.Errorf("%w: %s (status %s); lanjutkan dengan mia workflow run --id %s, atau mulai yang baru secara sengaja dengan --new", ErrOpenWorkflow, w.ID, w.Status, w.ID)
			}
		}
	}
	if fixtureKey == "" {
		fixtureKey = s.Cfg.Limits.Provider.Mock.FixtureKey
	}
	if fixtureKey == "" {
		fixtureKey = filepath.Base(abs)
	}
	now := s.ts()
	wf := &domain.Workflow{ID: s.IDs.New("wf"), Status: domain.WfCreated, InputsDir: abs, FixtureKey: fixtureKey, SummaryJSON: "{}", ConfigHash: s.Cfg.Hash, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.CreateWorkflow(ctx, wf, s.event(wf.ID, "created", "", "", fmt.Sprintf("input %s, %d catatan, fixture %s", abs, len(res.Notes), fixtureKey))); err != nil {
		return nil, err
	}
	return s.Store.GetWorkflow(ctx, wf.ID)
}

// Mode selects what an advance may do with failed or interrupted runs.
type Mode int

const (
	ModeRun    Mode = iota // start missing stages only; failed or interrupted runs stop the workflow
	ModeResume             // additionally resume failed or interrupted runs (each at most once per command)
)

// AdvanceOptions carries explicit operator requests.
type AdvanceOptions struct {
	NewContentRunFor string // opportunity ID: start a new content-run attempt after a failed one
}

// Result reports what one command did and the state afterwards.
type Result struct {
	Actions    []string
	Stopped    string
	Assessment *Assessment
}

type step struct {
	stage, desc string
	startsRun   bool
	do          func(ctx context.Context) error
}

// Advance runs the next safe automatic steps under the workflow lease, re-reading the database before
// every decision. It never approves, publishes or sends anything.
func (s *Service) Advance(ctx context.Context, id string, mode Mode, opts AdvanceOptions) (*Result, error) {
	wf, err := s.Store.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	owner := ids.TimeRandom{}.New("lease")
	until := s.Now().Add(time.Duration(s.Cfg.Workflow.LeaseSeconds) * time.Second).UTC().Format(time.RFC3339)
	ok, err := s.Store.AcquireWorkflowLease(ctx, id, owner, s.ts(), until)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w (lease sampai %s); coba lagi setelah proses itu selesai, atau gunakan mia workflow resume setelah lease kedaluwarsa", store.ErrLeaseHeld, wf.LeaseExpiresAt)
	}
	bctx := context.WithoutCancel(ctx)
	defer func() { _ = s.Store.ReleaseWorkflowLease(bctx, id, owner) }()
	res := &Result{}
	if mode == ModeResume {
		detail := "membaca ulang status dari basis data"
		if opts.NewContentRunFor != "" {
			detail += "; run konten baru diminta untuk " + opts.NewContentRunFor
		}
		if err := s.Store.AppendWorkflowEvent(ctx, s.event(id, "resume", "", "", detail)); err != nil {
			return nil, err
		}
	}
	resumed := map[string]bool{}
	newAttemptDone := false
	runs := 0
	for guard := 0; guard < s.Cfg.Workflow.MaxRunsPerCommand+5; guard++ {
		if wf, err = s.Store.GetWorkflow(ctx, id); err != nil {
			return nil, err
		}
		a, err := s.assess(ctx, wf, owner)
		if err != nil {
			return nil, err
		}
		st := s.nextStep(ctx, a, mode, opts, resumed, &newAttemptDone)
		if st == nil {
			break
		}
		if st.startsRun {
			if runs >= s.Cfg.Workflow.MaxRunsPerCommand {
				res.Stopped = fmt.Sprintf("batas %d run per perintah tercapai; jalankan perintah lagi untuk melanjutkan", s.Cfg.Workflow.MaxRunsPerCommand)
				_ = s.Store.AppendWorkflowEvent(ctx, s.event(id, "limit_reached", st.stage, "", res.Stopped))
				break
			}
			if reason, ok := s.budgetPrecheck(ctx); !ok {
				res.Stopped = reason
				_ = s.Store.AppendWorkflowEvent(ctx, s.event(id, "budget_stop", st.stage, "", reason))
				break
			}
			runs++
		}
		if err := st.do(ctx); err != nil {
			_ = s.Store.AppendWorkflowEvent(bctx, s.event(id, "stage_failed", st.stage, "", err.Error()))
			res.Actions = append(res.Actions, st.desc+" — gagal: "+err.Error())
			res.Stopped = "tahap " + StageLabels[st.stage] + " gagal; lihat mia workflow show --id " + id
			break
		}
		res.Actions = append(res.Actions, st.desc)
	}
	final, err := s.Sync(bctx, id, owner)
	if err != nil {
		return nil, err
	}
	res.Assessment = final
	return res, nil
}

// budgetPrecheck stops before a new stage when the monthly ceiling is already used up; per-call
// gating stays in the orchestrator.
func (s *Service) budgetPrecheck(ctx context.Context) (string, bool) {
	spent, err := s.Store.SpentMonth(ctx, budget.Period(s.Now(), budget.Location()))
	if err != nil {
		return "pemeriksaan anggaran gagal: " + err.Error(), false
	}
	if ceiling := s.Cfg.Limits.Caps.MonthlyCeilingUSD; spent >= ceiling {
		return fmt.Sprintf("plafon bulanan USD %.2f sudah terpakai (USD %.4f); tidak ada run baru yang dimulai", ceiling, spent), false
	}
	return "", true
}

// runAndRecord wraps an orchestrator call with stage events. A failed run is not an error here:
// its status is authoritative and the next assessment reports it.
func (s *Service) runAndRecord(ctx context.Context, wfID, stage, runID, desc string, call func() (*domain.RunSummary, error)) error {
	if err := s.Store.AppendWorkflowEvent(ctx, s.event(wfID, "stage_started", stage, runID, desc)); err != nil {
		return err
	}
	sum, err := call()
	if err != nil {
		return err
	}
	kind := "stage_completed"
	if sum.Status == domain.RunFailed || sum.Status == domain.RunPartial {
		kind = "stage_failed"
	}
	detail := "status " + sum.Status
	if len(sum.Errors) > 0 {
		detail += ": " + strings.Join(sum.Errors, "; ")
	}
	return s.Store.AppendWorkflowEvent(context.WithoutCancel(ctx), s.event(wfID, kind, stage, sum.RunID, detail))
}

func (s *Service) nextStep(ctx context.Context, a *Assessment, mode Mode, opts AdvanceOptions, resumed map[string]bool, newAttemptDone *bool) *step {
	wf := a.Workflow
	rs, ss, cgs := a.stage(StageResearch), a.stage(StageStrategy), a.stage(StageContentGeneration)
	resumeRun := func(stage, runID string, call func() (*domain.RunSummary, error)) *step {
		if mode != ModeResume || resumed[runID] {
			return nil
		}
		return &step{stage: stage, startsRun: true, desc: fmt.Sprintf("melanjutkan run %s (%s)", runID, StageLabels[stage]), do: func(ctx context.Context) error {
			resumed[runID] = true
			return s.runAndRecord(ctx, wf.ID, stage, runID, "resume", call)
		}}
	}
	switch rs.Status {
	case StPending:
		return &step{stage: StageResearch, startsRun: true, desc: "menjalankan riset (mock)", do: func(ctx context.Context) error {
			runID := wf.ResearchRunID
			if runID == "" {
				runID = s.IDs.New("run")
				if err := s.Store.ReserveWorkflowRun(ctx, wf.ID, StageResearch, runID, s.event(wf.ID, "reserved", StageResearch, runID, "ID run riset dicadangkan")); err != nil {
					return err
				}
			}
			return s.runAndRecord(ctx, wf.ID, StageResearch, runID, "run riset baru", func() (*domain.RunSummary, error) {
				return s.Orch.RunResearch(ctx, orchestrator.Options{InputsDir: wf.InputsDir, FixtureKey: wf.FixtureKey, RunID: runID})
			})
		}}
	case StFailed, StInterrupted, StBlocked:
		if a.Research != nil && a.Research.Kind == "research" {
			return resumeRun(StageResearch, a.Research.ID, func() (*domain.RunSummary, error) {
				return s.Orch.RunResearch(ctx, orchestrator.Options{ResumeID: a.Research.ID})
			})
		}
		return nil
	case StRunning:
		return nil
	}
	switch ss.Status {
	case StPending:
		return &step{stage: StageStrategy, startsRun: true, desc: "menjalankan Agen Strategi (mock)", do: func(ctx context.Context) error {
			runID := wf.StrategyRunID
			if runID == "" {
				runID = s.IDs.New("run")
				if err := s.Store.ReserveWorkflowRun(ctx, wf.ID, StageStrategy, runID, s.event(wf.ID, "reserved", StageStrategy, runID, "ID run strategi dicadangkan")); err != nil {
					return err
				}
			}
			return s.runAndRecord(ctx, wf.ID, StageStrategy, runID, "run strategi baru", func() (*domain.RunSummary, error) {
				return s.Orch.RunStrategy(ctx, orchestrator.Options{FromRun: wf.ResearchRunID, RunID: runID})
			})
		}}
	case StFailed, StInterrupted:
		if a.Strategy != nil && a.Strategy.Kind == "strategy" && a.Strategy.ParentRunID == wf.ResearchRunID {
			return resumeRun(StageStrategy, a.Strategy.ID, func() (*domain.RunSummary, error) {
				return s.Orch.RunStrategy(ctx, orchestrator.Options{ResumeID: a.Strategy.ID})
			})
		}
		return nil
	case StCompleted:
	default:
		return nil
	}
	if cgs.Status == StNotReached {
		return nil
	}
	for _, o := range a.Opportunities {
		if o.Status != domain.OppApproved {
			continue
		}
		o := o
		switch {
		case o.ContentRunID == "" && o.Eligible:
			drafts, err := s.Store.ContentDraftsForOpportunity(ctx, o.ID)
			if err != nil {
				return nil
			}
			if len(drafts) > 0 {
				// Drafts made outside this workflow are adopted, never regenerated silently.
				latest := drafts[len(drafts)-1].RunID
				return &step{stage: StageContentGeneration, desc: fmt.Sprintf("memakai run konten %s yang sudah ada untuk peluang %s (tanpa membuat ulang)", latest, o.ID), do: func(ctx context.Context) error {
					return s.Store.LinkWorkflowContentRun(ctx, &domain.WorkflowContentRun{WorkflowID: wf.ID, OpportunityID: o.ID, Attempt: 1, ContentRunID: latest, Origin: "adopted", CreatedAt: s.ts()},
						s.event(wf.ID, "adopted", StageContentGeneration, latest, "run konten yang sudah ada untuk peluang "+o.ID))
				}}
			}
			return s.contentStep(wf, o.ID, 1, false)
		case o.ContentRunID != "" && o.ContentStatus == StPending:
			// Reserved but not created (the previous process stopped before starting it).
			return &step{stage: StageContentGeneration, startsRun: true, desc: fmt.Sprintf("membuat draf untuk peluang %s (run %s yang sudah dicadangkan)", o.ID, o.ContentRunID), do: func(ctx context.Context) error {
				return s.runAndRecord(ctx, wf.ID, StageContentGeneration, o.ContentRunID, "run konten yang dicadangkan", func() (*domain.RunSummary, error) {
					return s.Orch.RunContent(ctx, orchestrator.Options{OpportunityID: o.ID, RunID: o.ContentRunID, Regenerate: o.ContentAttempt > 1})
				})
			}}
		case (o.ContentStatus == StFailed || o.ContentStatus == StInterrupted || o.ContentStatus == StBlocked) && opts.NewContentRunFor == o.ID && !*newAttemptDone && mode == ModeResume:
			*newAttemptDone = true
			return s.contentStep(wf, o.ID, o.ContentAttempt+1, true)
		case (o.ContentStatus == StFailed || o.ContentStatus == StInterrupted || o.ContentStatus == StBlocked) && !o.CallCapReached:
			if st := resumeRun(StageContentGeneration, o.ContentRunID, func() (*domain.RunSummary, error) {
				return s.Orch.RunContent(ctx, orchestrator.Options{ResumeID: o.ContentRunID})
			}); st != nil {
				return st
			}
		}
	}
	return nil
}

// contentStep reserves a content-run attempt for an opportunity and starts it.
func (s *Service) contentStep(wf *domain.Workflow, oppID string, attempt int, regenerate bool) *step {
	desc := fmt.Sprintf("membuat draf konten (mock) untuk peluang %s", oppID)
	if attempt > 1 {
		desc = fmt.Sprintf("membuat run konten baru (percobaan %d) untuk peluang %s atas permintaan eksplisit", attempt, oppID)
	}
	return &step{stage: StageContentGeneration, startsRun: true, desc: desc, do: func(ctx context.Context) error {
		runID := s.IDs.New("run")
		if err := s.Store.LinkWorkflowContentRun(ctx, &domain.WorkflowContentRun{WorkflowID: wf.ID, OpportunityID: oppID, Attempt: attempt, ContentRunID: runID, Origin: "workflow", CreatedAt: s.ts()},
			s.event(wf.ID, "reserved", StageContentGeneration, runID, fmt.Sprintf("ID run konten dicadangkan untuk peluang %s (percobaan %d)", oppID, attempt))); err != nil {
			return err
		}
		return s.runAndRecord(ctx, wf.ID, StageContentGeneration, runID, "run konten baru", func() (*domain.RunSummary, error) {
			return s.Orch.RunContent(ctx, orchestrator.Options{OpportunityID: oppID, RunID: runID, Regenerate: regenerate})
		})
	}}
}

// summary is the persisted orchestration summary of a workflow.
type summary struct {
	Status             string            `json:"status"`
	Reason             string            `json:"reason"`
	LastCompletedStage string            `json:"last_completed_stage"`
	FailedStage        string            `json:"failed_stage,omitempty"`
	Stages             map[string]string `json:"stages"`
	Opportunities      map[string]int    `json:"opportunities,omitempty"`
	Drafts             map[string]int    `json:"drafts,omitempty"`
	DraftsReady        *int              `json:"drafts_ready,omitempty"`
	Blockers           []string          `json:"blockers,omitempty"`
	HumanDecision      bool              `json:"human_decision_needed"`
	AssessedAt         string            `json:"assessed_at"`
}

// Sync recomputes the assessment and stores the summary (status, stages, counts) with an event when the
// status changes, in one transaction. leaseOwner is empty for commands that do not hold the lease.
func (s *Service) Sync(ctx context.Context, id, leaseOwner string) (*Assessment, error) {
	wf, err := s.Store.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	a, err := s.assess(ctx, wf, leaseOwner)
	if err != nil {
		return nil, err
	}
	sm := summary{Status: a.Status, Reason: a.Reason, LastCompletedStage: a.LastCompletedStage, FailedStage: a.FailedStage, Stages: map[string]string{},
		Opportunities: a.Counts.Opportunities, Drafts: a.Counts.Drafts, DraftsReady: a.Counts.DraftsReady, Blockers: a.Blockers, HumanDecision: a.HumanDecisionNeeded, AssessedAt: a.AssessedAt}
	for _, st := range a.Stages {
		sm.Stages[st.Name] = st.Status
	}
	b, _ := json.Marshal(sm)
	upd := *wf
	upd.Status, upd.LastCompletedStage, upd.FailedStage, upd.Reason, upd.SummaryJSON, upd.UpdatedAt = a.Status, a.LastCompletedStage, a.FailedStage, a.Reason, string(b), a.AssessedAt
	var ev *domain.WorkflowEvent
	if wf.Status != a.Status {
		ev = s.event(id, "status_changed", "", "", fmt.Sprintf("%s → %s: %s", wf.Status, a.Status, a.Reason))
	}
	if err := s.Store.SyncWorkflowSummary(ctx, &upd, leaseOwner, a.AssessedAt, ev); err != nil {
		if errors.Is(err, store.ErrLeaseHeld) && leaseOwner == "" {
			return a, nil // another process is advancing the workflow; the live assessment is still correct
		}
		return nil, err
	}
	a.Workflow, _ = s.Store.GetWorkflow(ctx, id)
	return a, nil
}

// RecordExport appends an export event for one draft version (the export itself writes no other table).
func (s *Service) RecordExport(ctx context.Context, wfID, draftID string, version int, relPath string) error {
	return s.Store.AppendWorkflowEvent(ctx, s.event(wfID, "export", StageHandoff, draftID, fmt.Sprintf("v%d %s", version, relPath)))
}
