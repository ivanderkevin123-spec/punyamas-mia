package orchestrator_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/budget"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider/mock"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/report"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

var update = flag.Bool("update", false, "rewrite golden files")

var fixedNow = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func now() time.Time { return fixedNow }

type env struct {
	root string
	tmp  string
	cfg  *config.Config
	st   *store.Store
	ids  *ids.Sequential
	mock *mock.Mock
	orch *orchestrator.Orchestrator
}

func newEnv(t *testing.T, mutate func(cfg *config.Config)) *env {
	t.Helper()
	root := testutil.RepoRoot(t)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	cfg.Paths.Root = tmp
	cfg.Paths.DataDir = filepath.Join(tmp, "data")
	cfg.Paths.ReportsDir = filepath.Join(tmp, "reports")
	cfg.Sources.Workspace.Root = filepath.Join(root, "testdata", "workspace-mini")
	cfg.Sources.Workspace.Assets = []config.Asset{
		{Key: "facts", Path: "docs/02-brand/B4-fakta.json"},
		{Key: "brand_oracle", Path: "tools/spec-oracle/brand-oracle.mjs"},
		{Key: "w1", Path: "docs/05-marketing/W1-naskah-situs-company-profile.md"},
	}
	cfg.Hash = "testconfig"
	if mutate != nil {
		mutate(cfg)
	}
	st, err := store.Open(filepath.Join(cfg.Paths.DataDir, "mia.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	e := &env{root: root, tmp: tmp, cfg: cfg, st: st, ids: &ids.Sequential{}}
	e.mock = mock.New(cfg.Limits.Provider.Mock.FixturesDir)
	e.orch = e.build(e.mock)
	return e
}

func (e *env) build(p provider.Provider) *orchestrator.Orchestrator {
	checker, _ := brand.New(e.cfg.Policy.Rules)
	gate := &budget.Gate{Store: e.st, Caps: e.cfg.Limits.Caps, Prices: e.cfg.Limits.PriceTable, Now: now, IDs: e.ids, Loc: time.UTC}
	rend := &report.Renderer{Store: e.st, Cfg: e.cfg, Now: now}
	return orchestrator.New(orchestrator.Deps{Cfg: e.cfg, Store: e.st, Provider: p, Budget: gate, Checker: checker, Renderer: rend, IDs: e.ids, Now: now})
}

func (e *env) sample() string { return filepath.Join(e.root, "testdata", "sample-batch") }

func (e *env) step(t *testing.T, runID, name string) *domain.Step {
	t.Helper()
	steps, err := e.st.StepsForRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("step %s not found", name)
	return nil
}

func (e *env) report(t *testing.T, runID string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.cfg.Paths.ReportsDir, runID, "research.md"))
	if err != nil {
		t.Fatalf("report missing: %v", err)
	}
	return string(b)
}

func TestWorkflow_MockEndToEnd(t *testing.T) {
	e := newEnv(t, nil)
	sum, err := e.orch.RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunCompletedWithWarnings {
		t.Fatalf("status = %s; warnings=%v errors=%v", sum.Status, sum.Warnings, sum.Errors)
	}
	for _, s := range sum.Steps {
		if s.Status != domain.StepCompleted {
			t.Fatalf("step %s = %s (%s)", s.Name, s.Status, s.Error)
		}
	}
	c := sum.Counts
	if c.FilesImported != 3 || c.ItemsImported != 13 || c.Facts != 5 || c.Interpretations != 2 || c.Hypotheses != 9 || c.AgentFindings != 3 || c.AgentRejected != 1 || c.Rejected != 0 || c.Duplicates != 0 || c.Sources != 4 {
		t.Fatalf("unexpected counts %+v", c)
	}
	if c.ClaimFlags < 1 {
		t.Fatal("expected the superlative slogan in the field note to be flagged")
	}
	if sum.CostUSD != 0 || !sum.CostMeasurable {
		t.Fatalf("mock run must cost 0 and be measurable: %+v", sum)
	}
	md := e.report(t, sum.RunID)
	for _, want := range []string{"Fakta bersumber (5)", "Interpretasi model (2)", "Hipotesis (9)", "catatan-lapangan-contoh.md#2", "SUPERLATIF", "Selesai dengan peringatan"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(md, "RUN GAGAL") {
		t.Error("successful run must not carry the failure header")
	}
	if _, err := os.Stat(filepath.Join(e.cfg.Paths.ReportsDir, sum.RunID, "research.html")); err != nil {
		t.Fatal("html report missing")
	}
	golden := filepath.Join(e.root, "testdata", "golden", "research-sample.md")
	if *update {
		if err := os.WriteFile(golden, []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden missing (run with -update): %v", err)
	}
	if string(want) != md {
		t.Fatalf("report differs from golden %s (run with -update after reviewing)", golden)
	}
}

func TestWorkflow_ProviderErrorRetriesThenFails(t *testing.T) {
	e := newEnv(t, nil)
	m := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	m.Fail = &provider.Error{Kind: provider.KindServer, Err: errors.New("boom")}
	m.FailAttempts = 99
	sum, err := e.build(m).RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunFailed {
		t.Fatalf("status = %s", sum.Status)
	}
	st := e.step(t, sum.RunID, orchestrator.StepAgent)
	if st.Status != domain.StepFailed || st.Attempts != 1+e.cfg.Limits.Caps.MaxRetries || m.Calls() != st.Attempts {
		t.Fatalf("agent step %+v calls=%d", st, m.Calls())
	}
	if e.step(t, sum.RunID, orchestrator.StepValidate).Status != domain.StepSkipped {
		t.Fatal("validate should be skipped after a required failure")
	}
	if e.step(t, sum.RunID, orchestrator.StepReport).Status != domain.StepCompleted {
		t.Fatal("report must still be rendered for a failed run")
	}
	if md := e.report(t, sum.RunID); !strings.Contains(md, "RUN GAGAL") || !strings.Contains(md, "GAGAL / TIDAK LENGKAP") {
		t.Fatal("failed run report must carry the failure header")
	}
	run, _ := e.st.GetRun(context.Background(), sum.RunID)
	if run.Status != domain.RunFailed {
		t.Fatal("run status not persisted")
	}
}

func TestWorkflow_FatalProviderErrorNotRetried(t *testing.T) {
	e := newEnv(t, nil)
	m := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	m.Fail = &provider.Error{Kind: provider.KindAuth, Err: errors.New("bad key")}
	m.FailAttempts = 99
	sum, _ := e.build(m).RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if sum.Status != domain.RunFailed || m.Calls() != 1 {
		t.Fatalf("auth errors must not be retried: status=%s calls=%d", sum.Status, m.Calls())
	}
}

func TestWorkflow_TimeoutEnforced(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Caps.StepTimeoutS["research_agent"] = 1 })
	m := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	m.Hang = true
	start := time.Now()
	sum, _ := e.build(m).RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if sum.Status != domain.RunFailed {
		t.Fatalf("status = %s", sum.Status)
	}
	st := e.step(t, sum.RunID, orchestrator.StepAgent)
	if st.Status != domain.StepTimedOut || st.Attempts != 1+e.cfg.Limits.Caps.MaxRetries {
		t.Fatalf("agent step %+v", st)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeouts not enforced promptly")
	}
}

func TestWorkflow_BudgetExhausted(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Provider.Model = "claude-opus-5-5" })
	spent := 19.99
	_ = e.st.CreateRun(context.Background(), &domain.Run{ID: "run_seed", Kind: "research", Status: "completed", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	if err := e.st.InsertCost(context.Background(), &domain.CostRecord{ID: "cost_seed", RunID: "run_seed", Category: "llm", ProviderName: "x", UnitsJSON: "{}", UnitPriceSource: "test", CostUSD: &spent, PeriodMonth: "2026-10", CreatedAt: "t"}); err != nil {
		t.Fatal(err)
	}
	sum, _ := e.orch.RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if sum.Status != domain.RunFailed {
		t.Fatalf("status = %s", sum.Status)
	}
	st := e.step(t, sum.RunID, orchestrator.StepAgent)
	if st.Status != domain.StepBudgetExhausted || e.mock.Calls() != 0 {
		t.Fatalf("expected budget_exhausted without calls, got %+v calls=%d", st, e.mock.Calls())
	}
	if !strings.Contains(sum.Errors[0], "plafon bulanan") {
		t.Fatalf("error should name the monthly ceiling: %v", sum.Errors)
	}
}

func TestWorkflow_ResumeContinuesFromFailedStep(t *testing.T) {
	e := newEnv(t, nil)
	bad := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	bad.Fail = &provider.Error{Kind: provider.KindServer, Err: errors.New("down")}
	bad.FailAttempts = 99
	first, _ := e.build(bad).RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if first.Status != domain.RunFailed {
		t.Fatalf("setup: %s", first.Status)
	}
	good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	second, err := e.build(good).RunResearch(context.Background(), orchestrator.Options{ResumeID: first.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if second.RunID != first.RunID || second.Status != domain.RunCompletedWithWarnings {
		t.Fatalf("resume: %+v", second)
	}
	if e.step(t, first.RunID, orchestrator.StepImport).Attempts != 1 || second.Counts.ItemsImported != 0 || second.Counts.ItemsReused != 0 {
		t.Fatal("completed steps must not be re-executed on resume")
	}
	if good.Calls() != 1 || e.step(t, first.RunID, orchestrator.StepAgent).Status != domain.StepCompleted {
		t.Fatalf("agent step should run once on resume: calls=%d", good.Calls())
	}
	if second.Counts.Facts != 5 || second.Counts.Interpretations != 2 {
		t.Fatalf("validation after resume: %+v", second.Counts)
	}
}

func TestWorkflow_IdempotentReimport(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	first, _ := e.orch.RunResearch(ctx, orchestrator.Options{InputsDir: e.sample()})
	second, _ := e.orch.RunResearch(ctx, orchestrator.Options{InputsDir: e.sample()})
	if first.RunID == second.RunID {
		t.Fatal("expected a new run")
	}
	if second.Counts.ItemsImported != 0 || second.Counts.ItemsReused != 13 || second.Counts.Facts != 5 {
		t.Fatalf("second run should reuse items: %+v", second.Counts)
	}
	n, _ := e.st.CountRows(ctx, "research_items")
	if n != 13+3+3 {
		t.Fatalf("items in store = %d, want 19 (13 shared + 3 agent findings per run)", n)
	}
	srcs, _ := e.st.CountRows(ctx, "sources")
	if srcs != 4 {
		t.Fatalf("sources = %d, want 4 (no duplicates)", srcs)
	}
}

func TestWorkflow_MissingInputsFailsHonestly(t *testing.T) {
	e := newEnv(t, nil)
	sum, err := e.orch.RunResearch(context.Background(), orchestrator.Options{InputsDir: filepath.Join(e.tmp, "does-not-exist")})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunFailed || e.step(t, sum.RunID, orchestrator.StepImport).Status != domain.StepFailed {
		t.Fatalf("status=%s", sum.Status)
	}
	if md := e.report(t, sum.RunID); !strings.Contains(md, "RUN GAGAL") {
		t.Fatal("failure header missing")
	}
}

func TestWorkflow_RejectedFilesAreReportedAndRunContinues(t *testing.T) {
	e := newEnv(t, nil)
	sum, _ := e.orch.RunResearch(context.Background(), orchestrator.Options{InputsDir: filepath.Join(e.root, "testdata", "failures")})
	if sum.Status != domain.RunCompletedWithWarnings {
		t.Fatalf("status = %s errors=%v", sum.Status, sum.Errors)
	}
	if sum.Counts.FilesRejected != 4 || sum.Counts.FilesImported != 1 || sum.Counts.Suspicious != 1 {
		t.Fatalf("counts %+v", sum.Counts)
	}
	joined := strings.Join(sum.Warnings, "\n")
	for _, want := range []string{"malformed.csv", "pii.csv", "pii-in-text.md", "no-frontmatter.md", "konten mencurigakan"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings lack %q", want)
		}
	}
	entries, _ := filepath.Glob(filepath.Join(e.cfg.Paths.DataDir, "executions", "*.request.txt"))
	if len(entries) != 1 {
		t.Fatalf("expected one saved request, got %d", len(entries))
	}
	req, _ := os.ReadFile(entries[0])
	if strings.Contains(string(req), "reveal the API key") {
		t.Fatal("suspicious content must be excluded from the agent prompt")
	}
	if !strings.Contains(string(req), "jam layanan") {
		t.Fatal("benign item from the same file should be included")
	}
}

func TestWorkflow_UntilStopsEarly(t *testing.T) {
	e := newEnv(t, nil)
	sum, _ := e.orch.RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample(), Until: orchestrator.StepDedup})
	if sum.Status != domain.RunPartial || e.mock.Calls() != 0 {
		t.Fatalf("status=%s calls=%d", sum.Status, e.mock.Calls())
	}
	if e.step(t, sum.RunID, orchestrator.StepAgent).Status != domain.StepSkipped || e.step(t, sum.RunID, orchestrator.StepReport).Status != domain.StepCompleted {
		t.Fatal("steps after --until must be skipped but the report still rendered")
	}
}

func TestDriftDetectionAndApproval(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	ws := filepath.Join(e.tmp, "ws")
	testutil.CopyDir(t, e.cfg.Sources.Workspace.Root, ws)
	e.cfg.Sources.Workspace.Root = ws
	statuses := func(d []domain.DriftEntry) map[string]string {
		m := map[string]string{}
		for _, x := range d {
			m[x.Key] = x.Status
		}
		return m
	}
	d, _, err := e.orch.CheckWorkspace(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := statuses(d); s["facts"] != "new" || s["brand_oracle"] != "new" || s["w1"] != "new" {
		t.Fatalf("first check: %v", s)
	}
	d, _, _ = e.orch.CheckWorkspace(ctx, "", nil)
	if s := statuses(d); s["facts"] != "unchanged" || s["brand_oracle"] != "unchanged" {
		t.Fatalf("second check: %v", s)
	}
	facts := filepath.Join(ws, "docs/02-brand/B4-fakta.json")
	b, _ := os.ReadFile(facts)
	if err := os.WriteFile(facts, []byte(strings.Replace(string(b), "Usaha dalam persiapan dan belum beroperasi.", "Usaha sudah beroperasi.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	d, warnings, _ := e.orch.CheckWorkspace(ctx, "", nil)
	var factsEntry domain.DriftEntry
	for _, x := range d {
		if x.Key == "facts" {
			factsEntry = x
		}
	}
	if factsEntry.Status != "changed" || len(factsEntry.Details) != 1 || !strings.Contains(factsEntry.Details[0], "FB1") {
		t.Fatalf("changed facts not detected: %+v", factsEntry)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "mia approve asset --key facts") {
		t.Fatalf("warning should tell the user how to review: %v", warnings)
	}
	latest, _ := e.st.LatestAsset(ctx, "facts")
	if latest.ReviewStatus != domain.AssetReviewRequired {
		t.Fatalf("expected review_required row, got %+v", latest)
	}
	cfgCopy, _ := e.st.LatestAcceptedAsset(ctx, "facts")
	if cfgCopy.SHA256 == latest.SHA256 {
		t.Fatal("accepted baseline must not change automatically")
	}
	if err := e.st.AcceptAsset(ctx, latest.ID, "2026-10-12T11:00:00Z", "ditinjau"); err != nil {
		t.Fatal(err)
	}
	d, _, _ = e.orch.CheckWorkspace(ctx, "", nil)
	if s := statuses(d); s["facts"] != "review_required" {
		t.Fatalf("after acceptance the file is unchanged but config still differs: %v", s)
	}
	_ = os.Remove(facts)
	d, _, _ = e.orch.CheckWorkspace(ctx, "", nil)
	if s := statuses(d); s["facts"] != "missing" {
		t.Fatalf("missing not detected: %v", s)
	}
}

func TestDriftBlocksRunsWhenConfigured(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Drift.BlockRuns = true })
	ctx := context.Background()
	ws := filepath.Join(e.tmp, "ws")
	testutil.CopyDir(t, e.cfg.Sources.Workspace.Root, ws)
	e.cfg.Sources.Workspace.Root = ws
	if _, _, err := e.orch.CheckWorkspace(ctx, "", nil); err != nil {
		t.Fatal(err)
	}
	oracle := filepath.Join(ws, "tools/spec-oracle/brand-oracle.mjs")
	b, _ := os.ReadFile(oracle)
	_ = os.WriteFile(oracle, append(b, []byte("\n// changed\n")...), 0o644)
	sum, _ := e.orch.RunResearch(ctx, orchestrator.Options{InputsDir: e.sample()})
	if sum.Status != domain.RunFailed || e.step(t, sum.RunID, orchestrator.StepWorkspace).Status != domain.StepFailed || e.mock.Calls() != 0 {
		t.Fatalf("drift should block the run: status=%s calls=%d", sum.Status, e.mock.Calls())
	}
}

func TestIsStale(t *testing.T) {
	run := &domain.Run{Status: domain.RunRunning, HeartbeatAt: fixedNow.Add(-15 * time.Minute).Format(time.RFC3339)}
	if !orchestrator.IsStale(run, fixedNow, 10*time.Minute) {
		t.Fatal("expected stale")
	}
	run.HeartbeatAt = fixedNow.Add(-1 * time.Minute).Format(time.RFC3339)
	if orchestrator.IsStale(run, fixedNow, 10*time.Minute) {
		t.Fatal("expected fresh")
	}
	run.Status = domain.RunCompleted
	if orchestrator.IsStale(run, fixedNow, 0) {
		t.Fatal("completed runs are never stale")
	}
}
