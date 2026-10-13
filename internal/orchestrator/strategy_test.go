package orchestrator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider/mock"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
)

// research runs the Phase 2 workflow on the sample batch and returns its summary.
func (e *env) research(t *testing.T) *domain.RunSummary {
	t.Helper()
	sum, err := e.orch.RunResearch(context.Background(), orchestrator.Options{InputsDir: e.sample()})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunCompletedWithWarnings {
		t.Fatalf("research status = %s errors=%v", sum.Status, sum.Errors)
	}
	return sum
}

func (e *env) strategyReport(t *testing.T, runID string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.cfg.Paths.ReportsDir, runID, "strategy.md"))
	if err != nil {
		t.Fatalf("strategy report missing: %v", err)
	}
	return string(b)
}

func (e *env) opps(t *testing.T, runID string) []*domain.Opportunity {
	t.Helper()
	opps, err := e.st.OpportunitiesForRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	return opps
}

func oppByIndex(opps []*domain.Opportunity, idx int) *domain.Opportunity {
	for _, o := range opps {
		if o.ProposalIndex == idx {
			return o
		}
	}
	return nil
}

func oppByTopic(opps []*domain.Opportunity, topic string) *domain.Opportunity {
	for _, o := range opps {
		if o.TopicKey == topic {
			return o
		}
	}
	return nil
}

func (e *env) validatedRefs(t *testing.T, runID string) map[string]*domain.Item {
	t.Helper()
	items, err := e.st.ValidatedItemsForRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]*domain.Item{}
	for _, it := range items {
		m[it.Ref] = it
	}
	return m
}

func TestStrategy_MockEndToEnd(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Scoring.Hash = "testscoring" })
	ctx := context.Background()
	r := e.research(t)
	sum, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunCompleted {
		t.Fatalf("status = %s warnings=%v errors=%v", sum.Status, sum.Warnings, sum.Errors)
	}
	if sum.Kind != "strategy" || sum.ParentRunID != r.RunID || sum.ScoringConfigHash != "testscoring" || sum.FixtureHash == "" {
		t.Fatalf("summary header: %+v", sum)
	}
	for _, s := range sum.Steps {
		if s.Status != domain.StepCompleted {
			t.Fatalf("step %s = %s (%s)", s.Name, s.Status, s.Error)
		}
	}
	if len(sum.Steps) != len(orchestrator.StrategyStepNames()) {
		t.Fatalf("steps %+v", sum.Steps)
	}
	c := sum.Strategy
	if c == nil {
		t.Fatal("strategy counts missing")
	}
	if c.Proposed != 5 || c.Merged != 1 || c.Evaluated != 4 || c.TierA != 3 || c.TierB != 0 || c.TierC != 1 || c.Blocked != 0 || c.Rejected != 0 || c.Hypotheses != 1 || c.StoreDependent != 1 || c.Conflicted != 0 || c.EvidenceItems != 16 || c.ConflictGroups != 0 {
		t.Fatalf("unexpected counts %+v", c)
	}
	if sum.CostUSD != 0 || !sum.CostMeasurable || e.mock.Calls() != 2 {
		t.Fatalf("mock run must cost 0 with one strategy call: cost=%v measurable=%v calls=%d", sum.CostUSD, sum.CostMeasurable, e.mock.Calls())
	}
	opps := e.opps(t, sum.RunID)
	if len(opps) != 4 {
		t.Fatalf("opportunities = %d", len(opps))
	}
	refs := e.validatedRefs(t, r.RunID)
	validIDs := map[string]bool{}
	for _, it := range refs {
		validIDs[it.ID] = true
	}
	for _, o := range opps {
		if o.ConfigHash != "testconfig" || o.ScoringConfigHash != "testscoring" || o.FixtureHash != sum.FixtureHash || o.AgentExecutionID == "" || o.ParentRunID != r.RunID {
			t.Fatalf("provenance missing on %s: %+v", o.ID, o)
		}
		links, err := e.st.EvidenceForOpportunity(ctx, o.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(links) == 0 {
			t.Fatalf("opportunity %s has no evidence links", o.ID)
		}
		for _, l := range links {
			if !validIDs[l.ItemID] {
				t.Fatalf("opportunity %s links item %s which is not a validated item of the parent run", o.ID, l.ItemID)
			}
		}
	}
	tr := oppByTopic(opps, "transparansi_penimbangan")
	if tr == nil || tr.Tier != domain.TierA || tr.MergedFromJSON != "[4]" {
		t.Fatalf("merged opportunity: %+v", tr)
	}
	links, _ := e.st.EvidenceForOpportunity(ctx, tr.ID)
	got := map[string]bool{}
	for _, l := range links {
		got[l.ItemID] = true
	}
	if !got[refs["catatan-lapangan-contoh.md#2"].ID] || !got[refs["artikel-edukasi-contoh.json#1"].ID] || len(links) != 2 {
		t.Fatalf("merged opportunity must keep both cited facts: %+v", links)
	}
	hyp := oppByTopic(opps, "durasi_proses")
	if hyp == nil || hyp.Tier != domain.TierC || !hyp.Hypothesis || !hyp.RequiresStore || hyp.Status != domain.OppProposed {
		t.Fatalf("hypothesis opportunity: %+v", hyp)
	}
	md := e.strategyReport(t, sum.RunID)
	for _, want := range []string{"Tingkat A (3)", "Tingkat C — hipotesis, konflik, atau bukti lemah (1)", "menggabungkan usulan #4", "jumlah ordinal, bukan probabilitas",
		"HIPOTESIS", "MEMERLUKAN KONFIRMASI OPERASIONAL", "`testscoring`", "`" + sum.FixtureHash + "`", "usulan — menunggu tinjauan manusia", "Interpretasi strategis (bukan fakta)", "Fakta pendukung (ID bukti)", "Selesai"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(md, "RUN GAGAL") || strings.Contains(md, "DISETUJUI") {
		t.Error("fresh report must not show failure or approval")
	}
	if _, err := os.Stat(filepath.Join(e.cfg.Paths.ReportsDir, sum.RunID, "strategy.html")); err != nil {
		t.Fatal("html report missing")
	}
	golden := filepath.Join(e.root, "testdata", "golden", "strategy-sample.md")
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
		t.Fatalf("strategy report differs from golden %s (run with -update after reviewing)", golden)
	}
}

func TestStrategy_HashesRecordedPerRun(t *testing.T) {
	e := newEnv(t, nil)
	r := e.research(t)
	sum, err := e.orch.RunStrategy(context.Background(), orchestrator.Options{FromRun: r.RunID})
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join(e.cfg.Limits.Provider.Mock.FixturesDir, "strategy", "sample-batch.json"))
	if err != nil {
		t.Fatal(err)
	}
	if sum.FixtureHash != dedup.FileHash(fixture) {
		t.Fatalf("fixture hash %q != hash of the replayed fixture %q", sum.FixtureHash, dedup.FileHash(fixture))
	}
	if len(e.cfg.Scoring.Hash) != 16 || sum.ScoringConfigHash != e.cfg.Scoring.Hash {
		t.Fatalf("scoring hash %q (cfg %q)", sum.ScoringConfigHash, e.cfg.Scoring.Hash)
	}
	run, _ := e.st.GetRun(context.Background(), sum.RunID)
	if run.ConfigHash != "testconfig" || run.ParentRunID != r.RunID || run.ProviderName != "mock" {
		t.Fatalf("run row: %+v", run)
	}
	for _, o := range e.opps(t, sum.RunID) {
		if o.FixtureHash != sum.FixtureHash || o.ScoringConfigHash != e.cfg.Scoring.Hash || o.ConfigHash != "testconfig" {
			t.Fatalf("opportunity %s hashes: %+v", o.ID, o)
		}
	}
}

func TestStrategy_GatesFixture(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	// An imported-but-unvalidated item exists in the parent run; it must never support a fact.
	srcs, err := e.st.SourcesForRun(ctx, r.RunID)
	if err != nil || len(srcs) == 0 {
		t.Fatalf("sources: %v", err)
	}
	unval := &domain.Item{ID: "item_unvalidated", RunID: r.RunID, Ref: "manual:unvalidated#1", Origin: domain.OriginImport, Kind: domain.KindFact, TopicKey: "uji", Title: "Fakta belum divalidasi",
		Statement: "Toko contoh buka setiap hari.", Language: "id", Quality: "A", Status: domain.ItemImported, ContentHash: "hash_unvalidated", CreatedAt: "t"}
	if err := e.st.InsertItemWithEvidence(ctx, r.RunID, unval, []*domain.Evidence{{ID: "ev_unvalidated", SourceID: srcs[0].ID, Excerpt: unval.Statement, Locator: "x"}}); err != nil {
		t.Fatal(err)
	}
	sum, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID, FixtureKey: "gates"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunCompletedWithWarnings {
		t.Fatalf("status = %s warnings=%v errors=%v", sum.Status, sum.Warnings, sum.Errors)
	}
	c := sum.Strategy
	// Rejected proposals are not counted as hypotheses; store terms in rejected texts are still labelled.
	if c.Evaluated != 8 || c.Proposed != 8 || c.Blocked != 1 || c.Rejected != 6 || c.Hypotheses != 0 || c.Conflicted != 0 || c.StoreDependent < 1 || c.TierA+c.TierB+c.TierC != 1 {
		t.Fatalf("counts %+v", c)
	}
	opps := e.opps(t, sum.RunID)
	reasons := func(i int) string {
		o := oppByIndex(opps, i)
		if o == nil {
			t.Fatalf("proposal %d missing", i)
		}
		return o.RejectReasonsJSON + " " + o.GatesJSON
	}
	for _, c := range []struct {
		idx    int
		status string
		want   []string
	}{
		{1, domain.OppRejected, []string{"tidak-ada.md#9", "Gerbang 1"}},
		{2, domain.OppRejected, []string{"manual:unvalidated#1", "belum divalidasi"}},
		{3, domain.OppRejected, []string{"HARGA"}},
		{4, domain.OppRejected, []string{"JAM_BUKA"}},
		{5, domain.OppRejected, []string{"ALAMAT"}},
		{6, domain.OppRejected, []string{"STATISTIK", "KLAIM_PELANGGAN"}},
		{7, domain.OppBlocked, []string{"SUPERLATIF", "ABSOLUT"}},
		{8, domain.OppProposed, nil},
	} {
		o := oppByIndex(opps, c.idx)
		if o == nil || o.Status != c.status {
			t.Fatalf("proposal %d: %+v", c.idx, o)
		}
		if c.status != domain.OppProposed && o.Tier != domain.TierNone {
			t.Fatalf("proposal %d must have no tier, got %s", c.idx, o.Tier)
		}
		for _, w := range c.want {
			if !strings.Contains(reasons(c.idx), w) {
				t.Errorf("proposal %d lacks %q: %s", c.idx, w, reasons(c.idx))
			}
		}
	}
	valid := oppByIndex(opps, 8)
	if !valid.RequiresStore || valid.Tier == domain.TierNone {
		t.Fatalf("store-dependent proposal must be labelled, not rejected: %+v", valid)
	}
	for _, o := range opps {
		links, _ := e.st.EvidenceForOpportunity(ctx, o.ID)
		for _, l := range links {
			if l.ItemID == unval.ID {
				t.Fatalf("unvalidated item linked as evidence of %s", o.ID)
			}
		}
	}
	md := e.strategyReport(t, sum.RunID)
	for _, want := range []string{"Diblokir pemeriksa teks merek (1)", "Ditolak gerbang bukti atau klaim faktual (6)", "Alasan penolakan/blokir", "Selesai dengan peringatan"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

func (e *env) seedConflictRun(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	runID := "run_conflict_parent"
	if err := e.st.CreateRun(ctx, &domain.Run{ID: runID, Kind: "research", Status: domain.RunCompleted, CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: `{"fixture_key":"conflict"}`, SummaryJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	mk := func(src, ref, statement string) {
		s := &domain.Source{ID: "src_" + src, SourceType: domain.SourceManualNote, Name: "catatan " + src, Reference: "imports/" + src + ".md", RetrievedAt: "2026-10-10", ContentHash: "h" + src, Status: "ok", CreatedAt: "t"}
		if err := e.st.InsertSource(ctx, s); err != nil {
			t.Fatal(err)
		}
		_ = e.st.LinkRunSource(ctx, runID, s.ID)
		it := &domain.Item{ID: "item_" + src, RunID: runID, Ref: ref, Origin: domain.OriginImport, Kind: domain.KindFact, TopicKey: "penimbangan_toko_x", Title: "Toko X " + src, Statement: statement, Language: "id", Quality: "A", Status: domain.ItemValidated, ContentHash: "hash_" + src, CreatedAt: "t"}
		if err := e.st.InsertItemWithEvidence(ctx, runID, it, []*domain.Evidence{{ID: "ev_" + src, SourceID: s.ID, Excerpt: statement, Locator: "bagian 1"}}); err != nil {
			t.Fatal(err)
		}
	}
	mk("a", "a.md#1", "Toko X menimbang di depan pembeli.")
	mk("b", "b.md#1", "Toko X tidak menimbang di depan pembeli.")
	return runID
}

func TestStrategy_ConflictingEvidenceCapsTierAndShowsBothSides(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	parent := e.seedConflictRun(t)
	sum, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: parent, FixtureKey: "conflict"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunCompletedWithWarnings || !strings.Contains(strings.Join(sum.Warnings, "\n"), "kelompok konflik") {
		t.Fatalf("status=%s warnings=%v errors=%v", sum.Status, sum.Warnings, sum.Errors)
	}
	for _, id := range []string{"item_a", "item_b"} {
		it, _ := e.st.GetItem(ctx, id)
		if it.ConflictGroup != "cg:penimbangan_toko_x" {
			t.Fatalf("item %s conflict group %q", id, it.ConflictGroup)
		}
	}
	c := sum.Strategy
	if c.ConflictGroups != 1 || c.Conflicted != 1 || c.TierC != 1 || c.TierA != 0 {
		t.Fatalf("counts %+v", c)
	}
	opps := e.opps(t, sum.RunID)
	if len(opps) != 1 || opps[0].Tier != domain.TierC || opps[0].Status != domain.OppProposed {
		t.Fatalf("opportunity: %+v", opps)
	}
	for _, want := range []string{"Toko X menimbang di depan pembeli.", "Toko X tidak menimbang di depan pembeli.", `"cited":true`, `"cited":true`} {
		if !strings.Contains(opps[0].ConflictsJSON, want) {
			t.Errorf("conflicts json lacks %q: %s", want, opps[0].ConflictsJSON)
		}
	}
	if !strings.Contains(opps[0].DimensionsJSON, `"evidence_strength":{"value":3`) {
		t.Fatalf("strength still computed from two A facts: %s", opps[0].DimensionsJSON)
	}
	md := e.strategyReport(t, sum.RunID)
	for _, want := range []string{"Konflik bukti `cg:penimbangan_toko_x` — kedua sisi", "Toko X menimbang di depan pembeli.", "Toko X tidak menimbang di depan pembeli.", "Tingkat C — hipotesis, konflik, atau bukti lemah (1)", "Selesaikan konflik bukti"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

func TestStrategy_ParentMustBeCompletedResearchRun(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	if _, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: "run_missing"}); err == nil {
		t.Fatal("missing parent must be an error")
	}
	if _, err := e.orch.RunStrategy(ctx, orchestrator.Options{}); err == nil {
		t.Fatal("--from-run is required")
	}
	partial, _ := e.orch.RunResearch(ctx, orchestrator.Options{InputsDir: e.sample(), Until: orchestrator.StepImport})
	if partial.Status != domain.RunPartial {
		t.Fatalf("setup: %s", partial.Status)
	}
	sum, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: partial.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunFailed || e.step(t, sum.RunID, orchestrator.StepLoadEvidence).Status != domain.StepFailed || !strings.Contains(strings.Join(sum.Errors, " "), "berstatus partial") {
		t.Fatalf("status=%s errors=%v", sum.Status, sum.Errors)
	}
	if e.mock.Calls() != 0 {
		t.Fatal("no agent call may happen without validated evidence")
	}
	if !strings.Contains(e.strategyReport(t, sum.RunID), "RUN GAGAL") {
		t.Fatal("failed run report must carry the failure header")
	}
	if _, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: sum.RunID}); err == nil || !strings.Contains(err.Error(), "bukan run riset") {
		t.Fatalf("a strategy run is not a valid parent: %v", err)
	}
}

func TestStrategy_AtMostTwoAgentCallsPerRun(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	bad := filepath.Join(t.TempDir(), "fixtures")
	if err := os.MkdirAll(filepath.Join(bad, "strategy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "strategy", "sample-batch.json"), []byte(`{"opportunities": "bukan daftar"`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := mock.New(bad)
	sum, err := e.build(m).RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	if err != nil {
		t.Fatal(err)
	}
	n, _ := e.st.CountExecutions(ctx, sum.RunID, "strategy")
	if sum.Status != domain.RunFailed || m.Calls() != 2 || n != 2 {
		t.Fatalf("status=%s calls=%d executions=%d errors=%v", sum.Status, m.Calls(), n, sum.Errors)
	}
	if st := e.step(t, sum.RunID, orchestrator.StepStrategyAgent); st.Status != domain.StepFailed {
		t.Fatalf("agent step %+v", st)
	}
	if !strings.Contains(strings.Join(sum.Errors, " "), "2 percobaan") {
		t.Fatalf("error should state the bounded repair: %v", sum.Errors)
	}
	// Resuming the same run cannot spend a third call.
	again, err := e.build(m).RunStrategy(ctx, orchestrator.Options{ResumeID: sum.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != domain.RunFailed || m.Calls() != 2 || !strings.Contains(strings.Join(again.Errors, " "), "batas 2 panggilan") {
		t.Fatalf("resume: status=%s calls=%d errors=%v", again.Status, m.Calls(), again.Errors)
	}
	if len(e.opps(t, sum.RunID)) != 0 {
		t.Fatal("no opportunity may be stored without a valid agent output")
	}
}

func TestStrategy_BudgetCeilingBlocksAgent(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	e.cfg.Limits.Provider.Model = "claude-opus-5-5"
	spent := 19.99
	_ = e.st.CreateRun(ctx, &domain.Run{ID: "run_seed", Kind: "research", Status: "completed", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	if err := e.st.InsertCost(ctx, &domain.CostRecord{ID: "cost_seed", RunID: "run_seed", Category: "llm", ProviderName: "x", UnitsJSON: "{}", UnitPriceSource: "test", CostUSD: &spent, PeriodMonth: "2026-10", CreatedAt: "t"}); err != nil {
		t.Fatal(err)
	}
	calls := e.mock.Calls()
	sum, _ := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	st := e.step(t, sum.RunID, orchestrator.StepStrategyAgent)
	if sum.Status != domain.RunFailed || st.Status != domain.StepBudgetExhausted || e.mock.Calls() != calls {
		t.Fatalf("status=%s step=%+v calls=%d", sum.Status, st, e.mock.Calls())
	}
	if !strings.Contains(strings.Join(sum.Errors, " "), "plafon bulanan") {
		t.Fatalf("error should name the monthly ceiling: %v", sum.Errors)
	}
}

func TestStrategy_TimeoutEnforced(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Caps.StepTimeoutS[orchestrator.StepStrategyAgent] = 1 })
	r := e.research(t)
	m := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	m.Hang = true
	start := time.Now()
	sum, _ := e.build(m).RunStrategy(context.Background(), orchestrator.Options{FromRun: r.RunID})
	if sum.Status != domain.RunFailed {
		t.Fatalf("status = %s", sum.Status)
	}
	st := e.step(t, sum.RunID, orchestrator.StepStrategyAgent)
	if st.Status != domain.StepTimedOut || m.Calls() != 2 {
		t.Fatalf("agent step %+v calls=%d (hard cap of 2 must hold even under retries)", st, m.Calls())
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeouts not enforced promptly")
	}
	execs, _ := e.st.ExecutionsForRun(context.Background(), sum.RunID)
	timeouts := 0
	for _, x := range execs {
		if x.Outcome == "timeout" && !x.CostMeasurable {
			timeouts++
		}
	}
	if timeouts != 2 || !strings.Contains(strings.Join(sum.Errors, " "), "hasil terakhir: timeout") {
		t.Fatalf("timeouts=%d errors=%v", timeouts, sum.Errors)
	}
	costs, _ := e.st.CostsForRun(context.Background(), sum.RunID)
	if len(costs) != 2 {
		t.Fatalf("each timed-out call must be booked as an uncertain cost, got %d rows", len(costs))
	}
}

func TestStrategy_ResumeRendersFreshReportAndNeverDuplicates(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	// A fatal provider error on the first call is not retried: one call is spent, one remains for the resume.
	bad := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	bad.Fail = &provider.Error{Kind: provider.KindAuth, Err: errors.New("401")}
	bad.FailAttempts = 1
	first, _ := e.build(bad).RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	if first.Status != domain.RunFailed || bad.Calls() != 1 || !strings.Contains(e.strategyReport(t, first.RunID), "RUN GAGAL") {
		t.Fatalf("setup: status=%s calls=%d", first.Status, bad.Calls())
	}
	if len(e.opps(t, first.RunID)) != 0 {
		t.Fatal("a failed run must not store opportunities")
	}
	good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	second, err := e.build(good).RunStrategy(ctx, orchestrator.Options{ResumeID: first.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if second.RunID != first.RunID || second.Status != domain.RunCompleted || good.Calls() != 1 {
		t.Fatalf("resume: status=%s calls=%d errors=%v", second.Status, good.Calls(), second.Errors)
	}
	if n, _ := e.st.CountExecutions(ctx, first.RunID, "strategy"); n != 2 {
		t.Fatalf("executions across resumes = %d, want 2 (the cap counts the failed call)", n)
	}
	if e.step(t, first.RunID, orchestrator.StepLoadEvidence).Attempts != 1 {
		t.Fatal("completed steps must not be re-executed on resume")
	}
	md := e.strategyReport(t, first.RunID)
	if strings.Contains(md, "RUN GAGAL") || !strings.Contains(md, "Tingkat A (3)") {
		t.Fatal("resumed run must re-render the report from the stored outcome")
	}
	if second.Strategy == nil || second.Strategy.Evaluated != 4 || second.FixtureHash == "" {
		t.Fatalf("resumed summary: strategy=%+v fixture=%q", second.Strategy, second.FixtureHash)
	}
	// Resuming a completed run re-renders only; opportunities are not duplicated and no call is made.
	third, err := e.build(good).RunStrategy(ctx, orchestrator.Options{ResumeID: first.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if third.Status != domain.RunCompleted || good.Calls() != 1 || len(e.opps(t, first.RunID)) != 4 || third.FixtureHash != second.FixtureHash {
		t.Fatalf("third: status=%s calls=%d opps=%d", third.Status, good.Calls(), len(e.opps(t, first.RunID)))
	}
	// Both calls of the resumed run are spent: forcing it again fails at the cap instead of calling a third time.
	capped, err := e.build(good).RunStrategy(ctx, orchestrator.Options{ResumeID: first.RunID, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if capped.Status != domain.RunFailed || good.Calls() != 1 || !strings.Contains(strings.Join(capped.Errors, " "), "batas 2 panggilan") {
		t.Fatalf("forced beyond the cap: status=%s calls=%d errors=%v", capped.Status, good.Calls(), capped.Errors)
	}
	// On a fresh run (one call spent), a forced re-evaluation uses the second call and keeps the stored proposals.
	fresh, err := e.build(good).RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	if err != nil || fresh.Status != domain.RunCompleted {
		t.Fatalf("fresh: %v %s", err, fresh.Status)
	}
	forced, err := e.build(good).RunStrategy(ctx, orchestrator.Options{ResumeID: fresh.RunID, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if good.Calls() != 3 || len(e.opps(t, fresh.RunID)) != 4 || !strings.Contains(strings.Join(forced.Warnings, "\n"), "evaluasi diulang") {
		t.Fatalf("forced: calls=%d opps=%d warnings=%v", good.Calls(), len(e.opps(t, fresh.RunID)), forced.Warnings)
	}
}

func TestStrategy_NoNetworkNoPaidProvider(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	sum, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if caps := e.mock.Capabilities(); caps.Network || !caps.MeasurableCost {
		t.Fatalf("mock capabilities %+v", caps)
	}
	execs, _ := e.st.ExecutionsForRun(ctx, sum.RunID)
	if len(execs) != 1 {
		t.Fatalf("executions = %d", len(execs))
	}
	for _, x := range execs {
		if x.ProviderName != "mock" || x.CostUSD == nil || *x.CostUSD != 0 || !x.CostMeasurable || x.AgentName != "strategy" || x.SchemaVersion != "strategy_proposals.v1" {
			t.Fatalf("execution %+v", x)
		}
		if _, err := os.Stat(x.RequestPath); err != nil {
			t.Fatalf("request not recorded: %v", err)
		}
	}
	if sum.CostUSD != 0 || !sum.CostMeasurable || sum.CostUncertainUSD != 0 {
		t.Fatalf("cost %+v", sum)
	}
}

func TestStrategy_ApprovalHistoryIsAppendOnly(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	sum, err := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	if err != nil {
		t.Fatal(err)
	}
	opp := e.opps(t, sum.RunID)[0]
	for i, d := range []string{"approved", "needs_more_evidence"} {
		rec := &domain.ApprovalRecord{ID: e.ids.New("appr"), SubjectType: "opportunity", SubjectID: opp.ID, Decision: d, DecidedBy: "CEO", Note: "catatan", DecidedAt: fixedNow.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)}
		if err := e.st.InsertApproval(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	hist, err := e.st.ApprovalsFor(ctx, "opportunity", opp.ID)
	if err != nil || len(hist) != 2 || hist[0].Decision != "approved" || hist[1].Decision != "needs_more_evidence" {
		t.Fatalf("history: %v %+v", err, hist)
	}
	if err := e.st.UpdateOpportunityStatus(ctx, opp.ID, domain.OppNeedsEvidence); err != nil {
		t.Fatal(err)
	}
	got, _ := e.st.GetOpportunity(ctx, opp.ID)
	if got.Status != domain.OppNeedsEvidence || got.Tier != opp.Tier || got.OrdinalSum != opp.OrdinalSum {
		t.Fatalf("status mirror must not touch the evaluation: %+v", got)
	}
	// Re-rendering the report shows the full history, not just the latest decision.
	again, err := e.orch.RunStrategy(ctx, orchestrator.Options{ResumeID: sum.RunID})
	if err != nil || again.Status != domain.RunCompleted {
		t.Fatalf("re-render: %v %s", err, again.Status)
	}
	md := e.strategyReport(t, sum.RunID)
	if !strings.Contains(md, "Riwayat keputusan (hanya-tambah)") || !strings.Contains(md, "approved oleh CEO") || !strings.Contains(md, `needs\_more\_evidence oleh CEO`) {
		t.Fatalf("report lacks the append-only history:\n%s", md)
	}
}
