package workflow_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/budget"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider/mock"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/report"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/workflow"
)

var fixedNow = time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)

func now() time.Time { return fixedNow }

type env struct {
	root, dbPath string
	cfg          *config.Config
	st           *store.Store
	ids          *ids.Sequential
	mock         *mock.Mock
	svc          *workflow.Service
}

func newEnv(t *testing.T, mutate func(cfg *config.Config)) *env {
	t.Helper()
	root := testutil.RepoRoot(t)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	cfg.Paths.Root, cfg.Paths.DataDir, cfg.Paths.ReportsDir, cfg.Paths.DraftsDir = tmp, filepath.Join(tmp, "data"), filepath.Join(tmp, "reports"), filepath.Join(tmp, "drafts")
	cfg.Sources.Workspace.Root = filepath.Join(root, "testdata", "workspace-mini")
	cfg.Sources.Workspace.Assets = []config.Asset{{Key: "facts", Path: "docs/02-brand/B4-fakta.json"}, {Key: "brand_oracle", Path: "tools/spec-oracle/brand-oracle.mjs"}, {Key: "w1", Path: "docs/05-marketing/W1-naskah-situs-company-profile.md"}}
	cfg.Hash = "testconfig"
	if mutate != nil {
		mutate(cfg)
	}
	e := &env{root: root, cfg: cfg, ids: &ids.Sequential{}, dbPath: filepath.Join(cfg.Paths.DataDir, "mia.sqlite")}
	e.st = e.open(t)
	e.mock = mock.New(cfg.Limits.Provider.Mock.FixturesDir)
	e.svc = e.service(e.st, e.mock)
	return e
}

func (e *env) open(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// service builds a workflow service over a store handle and provider (a "process").
func (e *env) service(st *store.Store, p provider.Provider) *workflow.Service {
	checker, _ := brand.New(e.cfg.Policy.Rules)
	gate := &budget.Gate{Store: st, Caps: e.cfg.Limits.Caps, Prices: e.cfg.Limits.PriceTable, Now: now, IDs: e.ids, Loc: time.UTC}
	rend := &report.Renderer{Store: st, Cfg: e.cfg, Now: now}
	orch := orchestrator.New(orchestrator.Deps{Cfg: e.cfg, Store: st, Provider: p, Budget: gate, Checker: checker, Renderer: rend, IDs: e.ids, Now: now})
	return &workflow.Service{Store: st, Orch: orch, Cfg: e.cfg, IDs: e.ids, Now: now}
}

func (e *env) sample() string { return filepath.Join(e.root, "testdata", "sample-batch") }

func (e *env) start(t *testing.T, fixture string) *domain.Workflow {
	t.Helper()
	wf, err := e.svc.Start(context.Background(), e.sample(), fixture, true)
	if err != nil {
		t.Fatal(err)
	}
	return wf
}

func (e *env) advance(t *testing.T, s *workflow.Service, id string, mode workflow.Mode, o workflow.AdvanceOptions) *workflow.Result {
	t.Helper()
	res, err := s.Advance(context.Background(), id, mode, o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (e *env) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.st.DB().QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *env) runs(t *testing.T, kind string) int {
	return e.count(t, `SELECT COUNT(1) FROM research_runs WHERE kind = ?`, kind)
}

func (e *env) decide(t *testing.T, oppID, decision string) {
	t.Helper()
	status := map[string]string{"approved": domain.OppApproved, "rejected": domain.OppRejectedByReview, "needs_more_evidence": domain.OppNeedsEvidence}[decision]
	rec := &domain.ApprovalRecord{ID: e.ids.New("appr"), SubjectType: "opportunity", SubjectID: oppID, Decision: decision, DecidedBy: "CEO", Note: "uji", DecidedAt: fixedNow.Format(time.RFC3339)}
	if err := e.st.RecordOpportunityDecision(context.Background(), rec, status); err != nil {
		t.Fatal(err)
	}
}

func oppByTitle(a *workflow.Assessment, prefix string) workflow.OppRow {
	for _, o := range a.Opportunities {
		if strings.HasPrefix(o.Title, prefix) {
			return o
		}
	}
	return workflow.OppRow{}
}

func stage(a *workflow.Assessment, name string) workflow.StageView {
	for _, s := range a.Stages {
		if s.Name == name {
			return s
		}
	}
	return workflow.StageView{}
}

func hasAction(a *workflow.Assessment, substr string) bool {
	for _, act := range a.NextActions {
		if strings.Contains(act.Text, substr) || strings.Contains(act.Command, substr) {
			return true
		}
	}
	return false
}

func events(t *testing.T, e *env, id string) string {
	t.Helper()
	evs, err := e.st.WorkflowEvents(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, ev := range evs {
		kinds = append(kinds, ev.Kind+":"+ev.Stage+":"+ev.Detail)
	}
	return strings.Join(kinds, "\n")
}

// (1, 12) Clean research → strategy; a repeated run creates nothing new and approves nothing.
func TestWorkflow_CleanResearchToStrategyAndIdempotentRun(t *testing.T) {
	e := newEnv(t, nil)
	wf := e.start(t, "")
	if wf.Status != domain.WfCreated || wf.ResearchRunID != "" {
		t.Fatalf("start: %+v", wf)
	}
	res := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
	a := res.Assessment
	if a.Status != domain.WfAwaitingOpportunityReview || len(res.Actions) != 2 || e.mock.Calls() != 2 {
		t.Fatalf("status=%s actions=%v calls=%d", a.Status, res.Actions, e.mock.Calls())
	}
	if stage(a, workflow.StageResearch).Status != workflow.StCompleted || stage(a, workflow.StageStrategy).Status != workflow.StCompleted ||
		stage(a, workflow.StageOpportunityReview).Status != workflow.StAwaitingHuman || stage(a, workflow.StageContentGeneration).Status != workflow.StNotReached {
		t.Fatalf("stages %+v", a.Stages)
	}
	if *a.Counts.ValidatedEvidence != 16 || *a.Counts.OpportunityTotal != 4 || a.Counts.Opportunities[domain.OppProposed] != 4 || a.Counts.DraftTotal != nil || !a.HumanDecisionNeeded {
		t.Fatalf("counts %+v", a.Counts)
	}
	stored, _ := e.st.GetWorkflow(context.Background(), wf.ID)
	if stored.Status != a.Status || stored.ResearchRunID != a.Research.ID || stored.StrategyRunID != a.Strategy.ID || stored.LastCompletedStage != workflow.StageStrategy || stored.LeaseOwner != "" {
		t.Fatalf("stored summary %+v", stored)
	}
	for i := 0; i < 3; i++ {
		again := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
		if len(again.Actions) != 0 || again.Assessment.Status != domain.WfAwaitingOpportunityReview {
			t.Fatalf("repeat %d: %+v", i, again.Actions)
		}
	}
	if e.runs(t, "research") != 1 || e.runs(t, "strategy") != 1 || e.runs(t, "content") != 0 || e.mock.Calls() != 2 || e.count(t, `SELECT COUNT(1) FROM approval_records`) != 0 {
		t.Fatal("repeated runs must not create artefacts or approvals")
	}
	if !strings.Contains(events(t, e, wf.ID), "status_changed") {
		t.Fatal("status transitions must be recorded")
	}
}

// (2–6, 15–17, 22) Approval gates, draft review gates, readiness, live reports and counts.
func TestWorkflow_ApprovalAndReviewGates(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	wf := e.start(t, "")
	a := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	rend := &report.Renderer{Store: e.st, Cfg: e.cfg, Now: now}
	md := func() string {
		t.Helper()
		as, err := e.svc.Sync(ctx, wf.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		b, err := rend.RenderWorkflowMarkdown(rend.BuildWorkflow(as))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if r := md(); !strings.Contains(r, "proposed 4") || strings.Contains(r, "approved 1") {
		t.Fatal("report before approval")
	}
	// (2) A rejected opportunity never reaches the Content Agent.
	ktp := oppByTitle(a, "Jawab pertanyaan tentang KTP")
	e.decide(t, ktp.ID, "rejected")
	a = e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	if e.runs(t, "content") != 0 || a.Status != domain.WfAwaitingOpportunityReview {
		t.Fatalf("rejected opportunity must not produce content: %s", a.Status)
	}
	// (3) An approved opportunity proceeds to content generation.
	opp := oppByTitle(a, "Tunjukkan penimbangan")
	e.decide(t, opp.ID, "approved")
	if r := md(); !strings.Contains(r, "approved 1") || !strings.Contains(r, "rejected\\_by\\_review 1") {
		t.Fatal("(15) report must reflect the approval change after regeneration")
	}
	res := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
	a = res.Assessment
	if e.runs(t, "content") != 1 || len(res.Actions) != 1 || *a.Counts.DraftTotal != 10 {
		t.Fatalf("content: runs=%d actions=%v", e.runs(t, "content"), res.Actions)
	}
	if n := e.count(t, `SELECT COUNT(1) FROM content_drafts WHERE opportunity_id != ?`, opp.ID); n != 0 {
		t.Fatal("drafts only for the approved opportunity")
	}
	// Remaining opportunities are decided; drafts now gate the workflow (4).
	for _, o := range a.Opportunities {
		if o.Status == domain.OppProposed {
			e.decide(t, o.ID, "needs_more_evidence")
		}
	}
	a, _ = e.svc.Assess(ctx, wf.ID)
	if a.Status != domain.WfAwaitingContentReview || stage(a, workflow.StageHandoff).Status != workflow.StNotReached || !hasAction(a, "Tinjau draf") {
		t.Fatalf("(4) unreviewed drafts must block handoff: %s %+v", a.Status, stage(a, workflow.StageHandoff))
	}
	// (17) Counts equal the stored data.
	for status, n := range a.Counts.Drafts {
		if got := e.count(t, `SELECT COUNT(1) FROM content_drafts WHERE status = ? AND run_id = ?`, status, a.ContentRuns[0].RunID); got != n {
			t.Fatalf("draft count %s: %d vs %d", status, n, got)
		}
	}
	for status, n := range a.Counts.Opportunities {
		if got := e.count(t, `SELECT COUNT(1) FROM opportunities WHERE status = ? AND run_id = ?`, status, a.Strategy.ID); got != n {
			t.Fatalf("opportunity count %s: %d vs %d", status, n, got)
		}
	}
	// (22) Evidence IDs are traceable from the workflow to the stored links.
	links, _ := e.st.EvidenceForOpportunity(ctx, opp.ID)
	var want []string
	for _, l := range links {
		want = append(want, l.ItemID)
	}
	sort.Strings(want)
	row := oppByTitle(a, "Tunjukkan penimbangan")
	if strings.Join(row.EvidenceIDs, ",") != strings.Join(want, ",") || row.ContentRunID != a.ContentRuns[0].RunID {
		t.Fatalf("evidence ids %v vs %v", row.EvidenceIDs, want)
	}
	oppEvidence := map[string]bool{}
	for _, id := range want {
		oppEvidence[id] = true
	}
	var clean, withPH, other string
	for _, d := range a.Drafts {
		for _, id := range d.EvidenceIDs {
			if !oppEvidence[id] {
				t.Fatalf("draft %s cites %s outside the opportunity", d.ID, id)
			}
		}
		switch {
		case d.ContentType == "instagram_caption" && d.Variant == 1:
			clean = d.ID
		case d.ContentType == "instagram_caption" && d.Variant == 2:
			withPH = d.ID
		case other == "" && d.Status == domain.DraftNeedsReview && d.ContentType != "instagram_caption":
			other = d.ID
		}
	}
	// (5) A draft with placeholders is approved but never ready.
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, withPH, "approved", "isian menyusul", "CEO"); err != nil {
		t.Fatal(err)
	}
	// (6) A rejected draft is not ready and not exportable as approved content.
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, other, "rejected", "tidak dipakai", "CEO"); err != nil {
		t.Fatal(err)
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, clean, "approved", "ok", "CEO"); err != nil {
		t.Fatal(err)
	}
	a, _ = e.svc.Assess(ctx, wf.ID)
	for _, d := range a.Drafts {
		switch d.ID {
		case withPH:
			if d.Ready || !hasAction(a, "Lengkapi isian draf "+withPH) {
				t.Fatalf("(5) placeholder draft must not be ready: %+v", d)
			}
		case other:
			if d.Ready || d.Status != domain.DraftRejected {
				t.Fatalf("(6) rejected draft: %+v", d)
			}
		case clean:
			if !d.Ready {
				t.Fatalf("clean approved draft must be ready: %+v", d)
			}
		}
	}
	if a.Status != domain.WfAwaitingContentReview || *a.Counts.DraftsReady != 1 || !hasAction(a, "mia workflow export") {
		t.Fatalf("status %s ready %d", a.Status, *a.Counts.DraftsReady)
	}
	// (16) A revision of the approved draft returns it to review and shows in status and report.
	checker, _ := brand.New(e.cfg.Policy.Rules)
	ev, _ := content.New(e.cfg, checker)
	body := "Kami akan menimbang dan menguji barang di depan Bapak atau Ibu. Dasar hitung akan ditampilkan."
	if _, _, err := content.Revise(ctx, e.st, ev, e.cfg, fixedNow, clean, content.Edit{Body: &body}, "revisi"); err != nil {
		t.Fatal(err)
	}
	r := md()
	a, _ = e.svc.Assess(ctx, wf.ID)
	if *a.Counts.DraftsReady != 0 || !strings.Contains(r, "`"+clean+"`") || !hasAction(a, "Tinjau draf "+clean) {
		t.Fatal("(16) a revised draft must need review again in status and report")
	}
	for _, d := range a.Drafts {
		if d.ID == clean && (d.Version != 2 || d.Status != domain.DraftNeedsReview) {
			t.Fatalf("revised draft row %+v", d)
		}
	}
	if !strings.Contains(r, "| 2 | needs\\_review |") {
		t.Fatal("report must show the new version and status")
	}
}

// (7) A strategy run without any reviewable opportunity is reported honestly.
func TestWorkflow_NoValidOpportunity(t *testing.T) {
	e := newEnv(t, nil)
	wf := e.start(t, "kosong")
	a := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	if a.Status != domain.WfBlocked || *a.Counts.OpportunityTotal != 0 || !strings.Contains(a.Reason, "tidak ada peluang yang layak") || !hasAction(a, "--new") {
		t.Fatalf("status=%s reason=%q", a.Status, a.Reason)
	}
	if e.runs(t, "content") != 0 {
		t.Fatal("no content without opportunities")
	}
}

// (8, 11) A failed research run blocks strategy; run never retries it; resume (after a restart) continues the same run.
func TestWorkflow_FailedResearchAndRestartResume(t *testing.T) {
	e := newEnv(t, nil)
	wf := e.start(t, "")
	bad := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	bad.Fail = &provider.Error{Kind: provider.KindAuth, Err: errors.New("401")}
	bad.FailAttempts = 99
	a := e.advance(t, e.service(e.st, bad), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	if a.Status != domain.WfFailed || a.FailedStage != workflow.StageResearch || stage(a, workflow.StageStrategy).Status != workflow.StNotReached || !hasAction(a, "mia workflow resume") {
		t.Fatalf("failed research: %s %s", a.Status, a.FailedStage)
	}
	again := e.advance(t, e.service(e.st, bad), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
	if len(again.Actions) != 0 || e.runs(t, "research") != 1 || e.runs(t, "strategy") != 0 {
		t.Fatal("run must not retry a failed stage or skip gate A")
	}
	// A stale report on disk claiming completion is never used as state.
	_ = os.MkdirAll(filepath.Join(e.cfg.Paths.ReportsDir, wf.ID), 0o755)
	_ = os.WriteFile(filepath.Join(e.cfg.Paths.ReportsDir, wf.ID, "workflow.md"), []byte("status: completed"), 0o644)
	// Restart: a fresh store handle and service (another process) resume from the database.
	st2 := e.open(t)
	good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	res := e.advance(t, e.service(st2, good), wf.ID, workflow.ModeResume, workflow.AdvanceOptions{})
	if res.Assessment.Status != domain.WfAwaitingOpportunityReview || e.runs(t, "research") != 1 || e.runs(t, "strategy") != 1 || res.Assessment.Research.ID != a.Research.ID {
		t.Fatalf("resume: %s actions=%v", res.Assessment.Status, res.Actions)
	}
	if !strings.Contains(events(t, e, wf.ID), "resume:") {
		t.Fatal("resume must be recorded")
	}
}

// (9) An incomplete strategy run cannot pass gate B.
func TestWorkflow_IncompleteStrategyBlocksGateB(t *testing.T) {
	e := newEnv(t, nil)
	fx := filepath.Join(t.TempDir(), "fixtures")
	testutil.CopyDir(t, e.cfg.Limits.Provider.Mock.FixturesDir, fx)
	if err := os.WriteFile(filepath.Join(fx, "strategy", "sample-batch.json"), []byte(`{"opportunities": "bukan daftar"`), 0o644); err != nil {
		t.Fatal(err)
	}
	wf := e.start(t, "")
	a := e.advance(t, e.service(e.st, mock.New(fx)), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	if a.Status != domain.WfFailed || a.FailedStage != workflow.StageStrategy || stage(a, workflow.StageOpportunityReview).Status != workflow.StNotReached {
		t.Fatalf("status=%s failed=%s", a.Status, a.FailedStage)
	}
	// An interrupted (partial) strategy run is not complete either.
	if _, err := e.st.DB().Exec(`UPDATE research_runs SET status = 'partial' WHERE id = ?`, a.Strategy.ID); err != nil {
		t.Fatal(err)
	}
	a, _ = e.svc.Assess(context.Background(), wf.ID)
	if a.Status != domain.WfFailed || stage(a, workflow.StageStrategy).Status != workflow.StInterrupted || e.runs(t, "content") != 0 {
		t.Fatalf("partial strategy: %s %+v", a.Status, stage(a, workflow.StageStrategy))
	}
}

func approvedWorkflow(t *testing.T, e *env) (*domain.Workflow, workflow.OppRow) {
	t.Helper()
	wf := e.start(t, "")
	a := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	opp := oppByTitle(a, "Tunjukkan penimbangan")
	e.decide(t, opp.ID, "approved")
	return wf, opp
}

// (10) A failed content run is resumed without duplicating stored drafts; an explicit new attempt after the call cap.
func TestWorkflow_ContentFailureRecovery(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	wf, opp := approvedWorkflow(t, e)
	bad := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	bad.Fail = &provider.Error{Kind: provider.KindAuth, Err: errors.New("401")}
	bad.FailAttempts = 1
	a := e.advance(t, e.service(e.st, bad), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	if a.Status != domain.WfFailed || a.FailedStage != workflow.StageContentGeneration {
		t.Fatalf("content failure: %s", a.Status)
	}
	contentRun := a.ContentRuns[0].RunID
	good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	a = e.advance(t, e.service(e.st, good), wf.ID, workflow.ModeResume, workflow.AdvanceOptions{}).Assessment
	if *a.Counts.DraftTotal != 10 || len(a.ContentRuns) != 1 || a.ContentRuns[0].RunID != contentRun || good.Calls() != 1 {
		t.Fatalf("resume: drafts=%v runs=%d calls=%d", a.Counts.DraftTotal, len(a.ContentRuns), good.Calls())
	}
	// Simulate a crash during evaluation after drafts were stored: resume re-evaluates without duplicates or new calls.
	if _, err := e.st.DB().Exec(`UPDATE run_steps SET status = 'failed' WHERE run_id = ? AND name IN ('evaluate_content','report')`, contentRun); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.DB().Exec(`UPDATE research_runs SET status = 'failed' WHERE id = ?`, contentRun); err != nil {
		t.Fatal(err)
	}
	a = e.advance(t, e.service(e.st, good), wf.ID, workflow.ModeResume, workflow.AdvanceOptions{}).Assessment
	if n := e.count(t, `SELECT COUNT(1) FROM content_drafts WHERE run_id = ?`, contentRun); n != 10 || good.Calls() != 1 || stage(a, workflow.StageContentGeneration).Status != workflow.StCompleted {
		t.Fatalf("crash recovery: drafts=%d calls=%d", n, good.Calls())
	}
	execs, _ := e.st.CountExecutions(ctx, contentRun, "content")
	if execs != 2 {
		t.Fatalf("executions = %d (one failed, one ok)", execs)
	}
	_ = opp
}

// (19, 10) Timeouts are recorded in executions, the ledger and workflow events; after the call cap only an explicit new attempt runs.
func TestWorkflow_TimeoutAndExplicitNewContentRun(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Caps.StepTimeoutS[orchestrator.StepContentAgent] = 1 })
	ctx := context.Background()
	wf, opp := approvedWorkflow(t, e)
	hang := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	hang.Hang = true
	a := e.advance(t, e.service(e.st, hang), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	run := a.ContentRuns[0].RunID
	if a.Status != domain.WfFailed || hang.Calls() != 2 {
		t.Fatalf("timeout: %s calls=%d", a.Status, hang.Calls())
	}
	execs, _ := e.st.ExecutionsForRun(ctx, run)
	costs, _ := e.st.CostsForRun(ctx, run)
	if len(execs) != 2 || execs[0].Outcome != "timeout" || len(costs) != 2 || !costs[0].Estimated || *a.Counts.CostUncertainUSD != *a.Counts.CostUSD {
		t.Fatalf("executions/costs: %d %d", len(execs), len(costs))
	}
	if !strings.Contains(events(t, e, wf.ID), "stage_failed:content_generation") {
		t.Fatal("the failed stage must be recorded")
	}
	row := oppByTitle(a, "Tunjukkan penimbangan")
	if !row.CallCapReached || !hasAction(a, "--new-content-run "+opp.ID) {
		t.Fatalf("call cap must produce a recovery instruction, not a retry: %+v", row)
	}
	good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	res := e.advance(t, e.service(e.st, good), wf.ID, workflow.ModeResume, workflow.AdvanceOptions{})
	if good.Calls() != 0 || len(res.Actions) != 0 {
		t.Fatalf("resume must not retry past the call cap: %v", res.Actions)
	}
	res = e.advance(t, e.service(e.st, good), wf.ID, workflow.ModeResume, workflow.AdvanceOptions{NewContentRunFor: opp.ID})
	a = res.Assessment
	if good.Calls() != 1 || len(a.ContentRuns) != 2 || a.ContentRuns[1].Attempt != 2 || *a.Counts.DraftTotal != 10 {
		t.Fatalf("explicit new attempt: calls=%d runs=%+v", good.Calls(), a.ContentRuns)
	}
	if spent, _ := e.st.SpentRun(ctx, run); spent != *costs[0].CostUSD+*costs[1].CostUSD {
		t.Fatal("resume must not reset recorded cost")
	}
}

// (13) A failed transition transaction leaves no reserved run and no false progress.
func TestWorkflow_TransactionFailureLeavesNoFalseProgress(t *testing.T) {
	e := newEnv(t, nil)
	wf := e.start(t, "")
	if _, err := e.st.DB().Exec(`CREATE TRIGGER test_fail_events BEFORE INSERT ON workflow_events BEGIN SELECT RAISE(ABORT, 'simulated failure'); END;`); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.Advance(context.Background(), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := e.st.GetWorkflow(context.Background(), wf.ID)
	if stored.ResearchRunID != "" || stored.Status != domain.WfCreated || e.runs(t, "research") != 0 || e.mock.Calls() != 0 || res.Stopped == "" {
		t.Fatalf("after failed transaction: %+v runs=%d stopped=%q", stored, e.runs(t, "research"), res.Stopped)
	}
	if _, err := e.st.DB().Exec(`DROP TRIGGER test_fail_events`); err != nil {
		t.Fatal(err)
	}
	if a := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment; a.Status != domain.WfAwaitingOpportunityReview || e.runs(t, "research") != 1 {
		t.Fatalf("after recovery: %s", a.Status)
	}
}

type gatedProvider struct {
	inner provider.Provider
	open  chan struct{}
}

func (g *gatedProvider) Name() string                        { return g.inner.Name() }
func (g *gatedProvider) Capabilities() provider.Capabilities { return g.inner.Capabilities() }
func (g *gatedProvider) Complete(ctx interface {
	Done() <-chan struct{}
	Err() error
}, req provider.Request) (provider.Response, provider.Usage, error) {
	select {
	case <-g.open:
	case <-ctx.Done():
	}
	return g.inner.Complete(ctx, req)
}

// (14) Two processes advancing the same workflow: one holds the lease, artefacts are created once.
func TestWorkflow_ConcurrentAdvanceKeepsIntegrity(t *testing.T) {
	e := newEnv(t, nil)
	wf := e.start(t, "")
	gate := &gatedProvider{inner: mock.New(e.cfg.Limits.Provider.Mock.FixturesDir), open: make(chan struct{})}
	procA := e.service(e.open(t), gate)
	procB := e.service(e.open(t), mock.New(e.cfg.Limits.Provider.Mock.FixturesDir))
	done := make(chan error, 1)
	go func() {
		_, err := procA.Advance(context.Background(), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for e.runs(t, "research") == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := procB.Advance(context.Background(), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}); !errors.Is(err, store.ErrLeaseHeld) {
		t.Fatalf("second process must be refused while the lease is held: %v", err)
	}
	if a, _ := procB.Assess(context.Background(), wf.ID); !a.LeaseHeld {
		t.Fatal("show must report the foreign lease")
	}
	close(gate.open)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// Many concurrent advances afterwards: no duplicate artefacts, every result consistent.
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.service(e.open(t), mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)).Advance(context.Background(), wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
			if err != nil && !errors.Is(err, store.ErrLeaseHeld) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if e.runs(t, "research") != 1 || e.runs(t, "strategy") != 1 {
		t.Fatalf("duplicates: research=%d strategy=%d", e.runs(t, "research"), e.runs(t, "strategy"))
	}
	if stored, _ := e.st.GetWorkflow(context.Background(), wf.ID); stored.LeaseOwner != "" || stored.Status != domain.WfAwaitingOpportunityReview {
		t.Fatalf("lease must be released and status consistent: %+v", stored)
	}
	// An expired lease from a crashed process is taken over.
	if _, err := e.st.DB().Exec(`UPDATE workflows SET lease_owner = 'lease_mati', lease_expires_at = '2026-10-14T09:00:00Z' WHERE id = ?`, wf.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Advance(context.Background(), wf.ID, workflow.ModeResume, workflow.AdvanceOptions{}); err != nil {
		t.Fatalf("expired lease must be taken over: %v", err)
	}
}

// (18) The budget ceiling stops before any agent call; the ledger is never reset.
func TestWorkflow_BudgetStops(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Provider.Model = "claude-opus-5-5" })
	ctx := context.Background()
	seed := func(v float64, id string) {
		_ = e.st.CreateRun(ctx, &domain.Run{ID: "run_" + id, Kind: "research", Status: "completed", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
		if err := e.st.InsertCost(ctx, &domain.CostRecord{ID: "cost_" + id, RunID: "run_" + id, Category: "llm", ProviderName: "x", UnitsJSON: "{}", UnitPriceSource: "test", CostUSD: &v, PeriodMonth: "2026-10", CreatedAt: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	seed(19.99, "seed")
	wf := e.start(t, "")
	a := e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{}).Assessment
	if a.Status != domain.WfBlocked || e.mock.Calls() != 0 || !strings.Contains(strings.Join(a.Blockers, " "), "anggaran") {
		t.Fatalf("per-call budget gate: %s calls=%d blockers=%v", a.Status, e.mock.Calls(), a.Blockers)
	}
	// Precheck: with the monthly ceiling used up no new run is even created.
	seed(0.02, "seed2")
	wf2, err := e.svc.Start(ctx, e.sample(), "", true)
	if err != nil {
		t.Fatal(err)
	}
	res := e.advance(t, e.svc, wf2.ID, workflow.ModeRun, workflow.AdvanceOptions{})
	if !strings.Contains(res.Stopped, "plafon bulanan") || res.Assessment.Workflow.ResearchRunID != "" || e.mock.Calls() != 0 {
		t.Fatalf("precheck: %q", res.Stopped)
	}
	if !strings.Contains(events(t, e, wf2.ID), "budget_stop") {
		t.Fatal("budget stop must be recorded")
	}
}

// Start validates inputs and refuses half-made or duplicate open workflows.
func TestWorkflow_StartValidation(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	if _, err := e.svc.Start(ctx, filepath.Join(e.root, "testdata", "failures"), "", false); !errors.Is(err, workflow.ErrInvalidInput) {
		t.Fatalf("invalid inputs: %v", err)
	}
	if _, err := e.svc.Start(ctx, filepath.Join(e.root, "tidak-ada"), "", false); !errors.Is(err, workflow.ErrInvalidInput) {
		t.Fatalf("missing dir: %v", err)
	}
	if n := e.count(t, `SELECT COUNT(1) FROM workflows`); n != 0 {
		t.Fatal("invalid inputs must leave no workflow behind")
	}
	if _, err := e.svc.Start(ctx, e.sample(), "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Start(ctx, e.sample(), "", false); !errors.Is(err, workflow.ErrOpenWorkflow) {
		t.Fatalf("second open workflow: %v", err)
	}
	if _, err := e.svc.Start(ctx, e.sample(), "", true); err != nil {
		t.Fatal(err)
	}
	// Run links are immutable and history is append-only at the storage layer.
	wf := e.start(t, "")
	e.advance(t, e.svc, wf.ID, workflow.ModeRun, workflow.AdvanceOptions{})
	if _, err := e.st.DB().Exec(`UPDATE workflows SET research_run_id = 'run_lain' WHERE id = ?`, wf.ID); err == nil {
		t.Fatal("run links must be immutable")
	}
	if _, err := e.st.DB().Exec(`DELETE FROM workflow_events WHERE workflow_id = ?`, wf.ID); err == nil {
		t.Fatal("events must be append-only")
	}
	if _, err := e.st.DB().Exec(`DELETE FROM workflows WHERE id = ?`, wf.ID); err == nil {
		t.Fatal("workflows must not be deletable")
	}
}
