package orchestrator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider/mock"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/report"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
)

// decide records a review decision on an opportunity the way the CLI does (one transaction).
func (e *env) decide(t *testing.T, oppID, decision string) {
	t.Helper()
	status := map[string]string{"approved": domain.OppApproved, "rejected": domain.OppRejectedByReview, "needs_more_evidence": domain.OppNeedsEvidence}[decision]
	rec := &domain.ApprovalRecord{ID: e.ids.New("appr"), SubjectType: "opportunity", SubjectID: oppID, Decision: decision, DecidedBy: "CEO", Note: "uji", DecidedAt: fixedNow.Format(time.RFC3339)}
	if err := e.st.RecordOpportunityDecision(context.Background(), rec, status); err != nil {
		t.Fatal(err)
	}
}

// approved runs research and strategy on the sample batch and approves the transparency opportunity.
func (e *env) approved(t *testing.T) (*domain.RunSummary, *domain.Opportunity) {
	t.Helper()
	r := e.research(t)
	s, err := e.orch.RunStrategy(context.Background(), orchestrator.Options{FromRun: r.RunID})
	if err != nil || s.Status != domain.RunCompleted {
		t.Fatalf("strategy: %v %+v", err, s)
	}
	opp := oppByTopic(e.opps(t, s.RunID), "transparansi_penimbangan")
	e.decide(t, opp.ID, "approved")
	opp, _ = e.st.GetOpportunity(context.Background(), opp.ID)
	return r, opp
}

func (e *env) contentReport(t *testing.T, runID string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.cfg.Paths.ReportsDir, runID, "content.md"))
	if err != nil {
		t.Fatalf("content report missing: %v", err)
	}
	return string(b)
}

func (e *env) drafts(t *testing.T, runID string) []*domain.ContentDraft {
	t.Helper()
	ds, err := e.st.ContentDraftsForRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

func draftByIndex(ds []*domain.ContentDraft, idx int) *domain.ContentDraft {
	for _, d := range ds {
		if d.ProposalIndex == idx {
			return d
		}
	}
	return nil
}

func (e *env) contentRuns(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.st.DB().QueryRow(`SELECT COUNT(1) FROM research_runs WHERE kind = 'content'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestContent_MockEndToEnd(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Scoring.Hash = "testscoring"; cfg.Content.Hash = "testcontent" })
	ctx := context.Background()
	research, opp := e.approved(t)
	calls := e.mock.Calls()
	sum, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Status != domain.RunCompletedWithWarnings || sum.Kind != "content" || sum.ParentRunID != opp.RunID || sum.OpportunityID != opp.ID || sum.ContentConfigHash != "testcontent" {
		t.Fatalf("summary: %+v", sum)
	}
	for _, s := range sum.Steps {
		if s.Status != domain.StepCompleted {
			t.Fatalf("step %s = %s (%s)", s.Name, s.Status, s.Error)
		}
	}
	if len(sum.Steps) != len(orchestrator.ContentStepNames()) || e.mock.Calls() != calls+1 {
		t.Fatalf("steps=%d calls=%d", len(sum.Steps), e.mock.Calls()-calls)
	}
	c := sum.Content
	if c == nil || c.Proposed != 10 || c.StructuralRejected != 0 || c.DuplicatesSkipped != 0 || c.Stored != 10 || c.Draft != 5 || c.NeedsReview != 5 || c.Blocked != 0 || c.Approved != 0 ||
		c.Placeholders != 6 || c.ClaimsToVerify != 1 || c.EvidenceLinks != 8 {
		t.Fatalf("counts %+v", c)
	}
	fixture, _ := os.ReadFile(filepath.Join(e.cfg.Limits.Provider.Mock.FixturesDir, "content", "sample-batch-transparansi_penimbangan.json"))
	if sum.FixtureHash != dedup.FileHash(fixture) || sum.CostUSD != 0 || !sum.CostMeasurable {
		t.Fatalf("fixture hash or cost: %+v", sum)
	}
	oppItems := map[string]bool{}
	links, _ := e.st.EvidenceForOpportunity(ctx, opp.ID)
	for _, l := range links {
		oppItems[l.ItemID] = true
	}
	validated := e.validatedRefs(t, research.RunID)
	for _, d := range e.drafts(t, sum.RunID) {
		if d.Status == domain.DraftApproved || d.Status == domain.DraftRejected {
			t.Fatalf("draft %s must not start with a decision: %s", d.ID, d.Status)
		}
		if d.OpportunityID != opp.ID || d.StrategyRunID != opp.RunID || d.ResearchRunID != research.RunID || d.ExecutionID == "" || d.ConfigHash != "testconfig" ||
			d.ContentConfigHash != "testcontent" || d.ScoringConfigHash != "testscoring" || d.FixtureHash != sum.FixtureHash || d.Version != 1 ||
			d.ContentHash != content.Hash(d.ContentType, d.Channel, d.Hook, d.Headline, d.Body, d.CallToAction) {
			t.Fatalf("provenance of %s: %+v", d.ID, d)
		}
		dl, _ := e.st.ContentEvidenceFor(ctx, d.ID)
		for _, l := range dl {
			if !oppItems[l.ItemID] {
				t.Fatalf("draft %s cites %s which is not evidence of the opportunity", d.ID, l.ItemID)
			}
			found := false
			for _, it := range validated {
				found = found || it.ID == l.ItemID
			}
			if !found {
				t.Fatalf("draft %s cites unvalidated item %s", d.ID, l.ItemID)
			}
		}
		versions, _ := e.st.ContentDraftVersions(ctx, d.ID)
		if len(versions) != 1 || versions[0].Author != "agent" || versions[0].ContentHash != d.ContentHash {
			t.Fatalf("version history of %s: %+v", d.ID, versions)
		}
	}
	ds := e.drafts(t, sum.RunID)
	clean, withPlaceholder := draftByIndex(ds, 1), draftByIndex(ds, 2)
	if clean.Status != domain.DraftDraft || withPlaceholder.Status != domain.DraftNeedsReview || draftByIndex(ds, 3).Status != domain.DraftNeedsReview {
		t.Fatalf("statuses: %s %s %s", clean.Status, withPlaceholder.Status, draftByIndex(ds, 3).Status)
	}
	if ph := content.DecodeDetails(withPlaceholder).Placeholders; len(ph) != 1 || ph[0] != "[ISI: kontak resmi Punyamas]" {
		t.Fatalf("placeholders recorded: %v", ph)
	}
	md := e.contentReport(t, sum.RunID)
	for _, want := range []string{"DRAF — BELUM DIPUBLIKASIKAN", "Perlu tinjauan (5)", "Lolos gerbang kode, menunggu tinjauan (5)", "`[ISI: kontak resmi Punyamas]`", "Klaim perlu verifikasi:",
		"Siap serah terima ke proses persetujuan publikasi:** TIDAK", "`testcontent`", "`" + sum.FixtureHash + "`", "Interpretasi pemasaran (bukan fakta)", "Riwayat tinjauan (hanya-tambah):** belum ada"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(md, "RUN GAGAL") {
		t.Error("successful run must not carry the failure header")
	}
	if _, err := os.Stat(filepath.Join(e.cfg.Paths.ReportsDir, sum.RunID, "content.html")); err != nil {
		t.Fatal("html report missing")
	}
	golden := filepath.Join(e.root, "testdata", "golden", "content-sample.md")
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
		t.Fatalf("content report differs from golden %s (run with -update after reviewing)", golden)
	}

	// Human review: a clean draft becomes ready for handoff; a draft with a placeholder is approved but never ready.
	approvedClean, err := content.Review(ctx, e.st, e.ids, fixedNow, clean.ID, "approved", "Draf diperiksa", "CEO")
	if err != nil {
		t.Fatal(err)
	}
	reviews, _ := e.st.ApprovalsFor(ctx, domain.SubjectContentDraft, clean.ID)
	if r := content.ReadinessOf(approvedClean, reviews); approvedClean.Status != domain.DraftApproved || !r.Ready || len(reviews) != 1 || reviews[0].SubjectVersion != 1 || reviews[0].SubjectHash != clean.ContentHash {
		t.Fatalf("clean approval: status=%s readiness=%+v reviews=%+v", approvedClean.Status, r, reviews)
	}
	approvedPH, err := content.Review(ctx, e.st, e.ids, fixedNow, withPlaceholder.ID, "approved", "Isian akan dilengkapi", "CEO")
	if err != nil {
		t.Fatal(err)
	}
	reviews, _ = e.st.ApprovalsFor(ctx, domain.SubjectContentDraft, withPlaceholder.ID)
	if r := content.ReadinessOf(approvedPH, reviews); r.Ready || !strings.Contains(strings.Join(r.Reasons, " "), "[ISI: kontak resmi Punyamas]") {
		t.Fatalf("a draft with placeholders must never be ready: %+v", r)
	}
	again, err := e.orch.RunContent(ctx, orchestrator.Options{ResumeID: sum.RunID})
	if err != nil || again.Content.Approved != 2 || !strings.Contains(e.contentReport(t, sum.RunID), "Disetujui peninjau — bukan izin publikasi (2)") {
		t.Fatalf("re-render after review: %v %+v", err, again.Content)
	}
}

func TestContent_IneligibleOpportunitiesRefused(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	r := e.research(t)
	s, _ := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID})
	opps := e.opps(t, s.RunID)
	pending := oppByTopic(opps, "transparansi_penimbangan")
	rejected := oppByTopic(opps, "pertanyaan_identitas")
	e.decide(t, rejected.ID, "rejected")
	needs := oppByTopic(opps, "praktik_harga_pesaing")
	e.decide(t, needs.ID, "needs_more_evidence")
	gates, _ := e.orch.RunStrategy(ctx, orchestrator.Options{FromRun: r.RunID, FixtureKey: "gates"})
	gopps := e.opps(t, gates.RunID)
	blocked := oppByIndex(gopps, 7)
	factual := oppByIndex(gopps, 3)
	// A status set without an approval record (history and status disagree) is not eligible either.
	tampered := oppByTopic(opps, "durasi_proses")
	if _, err := e.st.DB().Exec(`UPDATE opportunities SET status = 'approved' WHERE id = ?`, tampered.ID); err != nil {
		t.Fatal(err)
	}
	calls := e.mock.Calls()
	for _, c := range []struct{ id, want string }{
		{pending.ID, "status peluang proposed"},
		{rejected.ID, "status peluang rejected_by_review"},
		{needs.ID, "status peluang needs_more_evidence"},
		{blocked.ID, "status peluang blocked"},
		{factual.ID, "status peluang rejected"},
		{tampered.ID, "keputusan terakhir: tidak ada"},
		{"opp_tidak_ada", "peluang tidak ditemukan"},
	} {
		_, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: c.id})
		if !content.IsIneligible(err) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err=%v, want ineligible with %q", c.id, err, c.want)
		}
	}
	if e.mock.Calls() != calls || e.contentRuns(t) != 0 {
		t.Fatalf("refusals must not create runs or call the agent: calls=%d runs=%d", e.mock.Calls()-calls, e.contentRuns(t))
	}
	// The storage layer refuses drafts for unapproved opportunities even if application checks were bypassed.
	if _, err := e.st.DB().Exec(`INSERT INTO content_drafts(id, run_id, opportunity_id, strategy_run_id, research_run_id, proposal_index, content_type, variant, channel, target_segment, objective, body, status, content_hash, execution_id, config_hash, content_config_hash, scoring_config_hash, created_at, updated_at)
VALUES ('draft_x', ?, ?, ?, ?, 1, 'education', 1, 'situs', 'AU1', 'o', 'b', 'draft', 'h', ?, 'c', 'c', 'c', 't', 't')`, s.RunID, pending.ID, s.RunID, r.RunID, pending.AgentExecutionID); err == nil || !strings.Contains(err.Error(), "requires an approved opportunity") {
		t.Fatalf("trigger must refuse drafts for an unapproved opportunity: %v", err)
	}
}

func TestContent_InvalidProvenanceRefused(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	_, opp := e.approved(t)
	links, _ := e.st.EvidenceForOpportunity(ctx, opp.ID)
	// Evidence that is no longer validated makes the opportunity ineligible.
	if _, err := e.st.DB().Exec(`UPDATE research_items SET status = 'rejected' WHERE id = ?`, links[0].ItemID); err != nil {
		t.Fatal(err)
	}
	_, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if !content.IsIneligible(err) || !strings.Contains(err.Error(), links[0].ItemID) {
		t.Fatalf("unvalidated evidence: %v", err)
	}
	if _, err := e.st.DB().Exec(`UPDATE research_items SET status = 'validated' WHERE id = ?`, links[0].ItemID); err != nil {
		t.Fatal(err)
	}
	// A strategy run that is not completed breaks provenance.
	if _, err := e.st.DB().Exec(`UPDATE research_runs SET status = 'failed' WHERE id = ?`, opp.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID}); !content.IsIneligible(err) || !strings.Contains(err.Error(), "berstatus failed") {
		t.Fatalf("failed strategy run: %v", err)
	}
	if _, err := e.st.DB().Exec(`UPDATE research_runs SET status = 'completed' WHERE id = ?`, opp.RunID); err != nil {
		t.Fatal(err)
	}
	// An opportunity whose approval is withdrawn after a failed attempt cannot be resumed.
	bad := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	bad.Fail = &provider.Error{Kind: provider.KindAuth, Err: errors.New("401")}
	bad.FailAttempts = 1
	first, _ := e.build(bad).RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if first.Status != domain.RunFailed {
		t.Fatalf("setup: %s", first.Status)
	}
	e.decide(t, opp.ID, "rejected")
	good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
	resumed, err := e.build(good).RunContent(ctx, orchestrator.Options{ResumeID: first.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != domain.RunFailed || good.Calls() != 0 || len(e.drafts(t, first.RunID)) != 0 || !strings.Contains(strings.Join(resumed.Errors, " "), "rejected_by_review") {
		t.Fatalf("resume after withdrawn approval: status=%s calls=%d errors=%v", resumed.Status, good.Calls(), resumed.Errors)
	}
}

func TestContent_GatesFixture(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	research, opp := e.approved(t)
	srcs, _ := e.st.SourcesForRun(ctx, research.RunID)
	unval := &domain.Item{ID: "item_unvalidated", RunID: research.RunID, Ref: "manual:unvalidated#1", Origin: domain.OriginImport, Kind: domain.KindFact, TopicKey: "uji", Title: "Belum divalidasi",
		Statement: "Toko contoh buka setiap hari.", Language: "id", Quality: "A", Status: domain.ItemImported, ContentHash: "hash_unvalidated", CreatedAt: "t"}
	if err := e.st.InsertItemWithEvidence(ctx, research.RunID, unval, []*domain.Evidence{{ID: "ev_unvalidated", SourceID: srcs[0].ID, Excerpt: unval.Statement, Locator: "x"}}); err != nil {
		t.Fatal(err)
	}
	sum, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID, FixtureKey: "gates"})
	if err != nil {
		t.Fatal(err)
	}
	c := sum.Content
	if sum.Status != domain.RunCompletedWithWarnings || c.Proposed != 11 || c.StructuralRejected != 1 || c.DuplicatesSkipped != 1 || c.Stored != 9 || c.Blocked != 9 {
		t.Fatalf("status=%s counts=%+v warnings=%v", sum.Status, c, sum.Warnings)
	}
	ds := e.drafts(t, sum.RunID)
	for _, x := range []struct {
		idx  int
		want []string
	}{
		{1, []string{"HARGA", "ANGKA"}},
		{2, []string{"TESTIMONI"}},
		{3, []string{"BUKTI_TIDAK_ADA"}},
		{5, []string{"BUKTI_LINTAS_PELUANG"}},
		{6, []string{"BUKTI_BELUM_VALID"}},
		{8, []string{"URGENSI_PALSU"}},
		{9, []string{"WAKTU_PROSES", "ANGKA"}},
		{10, []string{"STATISTIK", "ALAMAT"}},
		{11, []string{"JANJI_UNTUNG", "TREN_HARGA"}},
	} {
		d := draftByIndex(ds, x.idx)
		if d == nil || d.Status != domain.DraftBlocked {
			t.Fatalf("proposal %d: %+v", x.idx, d)
		}
		for _, w := range x.want {
			if !strings.Contains(d.FindingsJSON, `"category":"`+w+`"`) {
				t.Errorf("proposal %d lacks finding %s: %s", x.idx, w, d.FindingsJSON)
			}
		}
		links, _ := e.st.ContentEvidenceFor(ctx, d.ID)
		for _, l := range links {
			if l.ItemID == unval.ID {
				t.Fatalf("unvalidated item linked to %s", d.ID)
			}
		}
		if _, err := content.Review(ctx, e.st, e.ids, fixedNow, d.ID, "approved", "coba", "CEO"); !errors.Is(err, content.ErrNotReviewable) {
			t.Fatalf("blocked draft %d must not be reviewable: %v", x.idx, err)
		}
	}
	if draftByIndex(ds, 4) != nil || draftByIndex(ds, 7) != nil {
		t.Fatal("structurally rejected and duplicate drafts must not be stored")
	}
	warn := strings.Join(sum.Warnings, "\n")
	if !strings.Contains(warn, "variant 4 di luar batas 1..3") || !strings.Contains(warn, "hampir sama dengan usulan #6") {
		t.Fatalf("warnings should report the structural rejection and the duplicate: %v", sum.Warnings)
	}
	md := e.contentReport(t, sum.RunID)
	if !strings.Contains(md, "Diblokir gerbang kode (9)") || !strings.Contains(md, "BUKTI\\_LINTAS\\_PELUANG") {
		t.Fatal("report must list blocked drafts with their findings")
	}
}

func TestContent_RegenerateAndDuplicates(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	_, opp := e.approved(t)
	first, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if err != nil || first.Content.Stored != 10 {
		t.Fatalf("first: %v", err)
	}
	calls := e.mock.Calls()
	if _, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID}); !errors.Is(err, orchestrator.ErrDraftsExist) || !strings.Contains(err.Error(), "--regenerate") {
		t.Fatalf("silent regeneration must be refused: %v", err)
	}
	if e.mock.Calls() != calls || e.contentRuns(t) != 1 {
		t.Fatal("refusal must not call the agent or create a run")
	}
	second, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID, Regenerate: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Content.Stored != 0 || second.Content.DuplicatesSkipped != 10 || len(e.drafts(t, second.RunID)) != 0 {
		t.Fatalf("deterministic regeneration must not store duplicates: %+v", second.Content)
	}
	firstIDs := map[string]bool{}
	for _, d := range e.drafts(t, first.RunID) {
		firstIDs[d.ID] = true
	}
	matched := 0
	for _, w := range second.Warnings {
		for id := range firstIDs {
			if strings.Contains(w, "hampir sama dengan "+id) {
				matched++
			}
		}
	}
	if matched != 10 {
		t.Fatalf("each skipped duplicate must name the existing draft: %v", second.Warnings)
	}
	all, _ := e.st.ContentDraftsForOpportunity(ctx, opp.ID)
	if len(all) != 10 {
		t.Fatalf("drafts for the opportunity = %d", len(all))
	}
}

func TestContent_CallCapTimeoutBudgetAndResume(t *testing.T) {
	t.Run("call cap across retries and resume", func(t *testing.T) {
		e := newEnv(t, nil)
		ctx := context.Background()
		_, opp := e.approved(t)
		bad := filepath.Join(t.TempDir(), "fixtures")
		_ = os.MkdirAll(filepath.Join(bad, "content"), 0o755)
		if err := os.WriteFile(filepath.Join(bad, "content", "sample-batch-transparansi_penimbangan.json"), []byte(`{"drafts": "bukan daftar"`), 0o644); err != nil {
			t.Fatal(err)
		}
		m := mock.New(bad)
		sum, _ := e.build(m).RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
		n, _ := e.st.CountExecutions(ctx, sum.RunID, "content")
		if sum.Status != domain.RunFailed || m.Calls() != 2 || n != 2 || !strings.Contains(strings.Join(sum.Errors, " "), "2 percobaan") {
			t.Fatalf("status=%s calls=%d execs=%d errors=%v", sum.Status, m.Calls(), n, sum.Errors)
		}
		again, _ := e.build(m).RunContent(ctx, orchestrator.Options{ResumeID: sum.RunID})
		if again.Status != domain.RunFailed || m.Calls() != 2 || !strings.Contains(strings.Join(again.Errors, " "), "batas 2 panggilan Agen Konten") {
			t.Fatalf("resume must not exceed the cap: calls=%d errors=%v", m.Calls(), again.Errors)
		}
		if len(e.drafts(t, sum.RunID)) != 0 {
			t.Fatal("no draft without a valid output")
		}
	})
	t.Run("timeout booked as uncertain cost", func(t *testing.T) {
		e := newEnv(t, func(cfg *config.Config) { cfg.Limits.Caps.StepTimeoutS[orchestrator.StepContentAgent] = 1 })
		_, opp := e.approved(t)
		m := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
		m.Hang = true
		start := time.Now()
		sum, _ := e.build(m).RunContent(context.Background(), orchestrator.Options{OpportunityID: opp.ID})
		st := e.step(t, sum.RunID, orchestrator.StepContentAgent)
		costs, _ := e.st.CostsForRun(context.Background(), sum.RunID)
		if sum.Status != domain.RunFailed || st.Status != domain.StepTimedOut || m.Calls() != 2 || len(costs) != 2 || sum.CostMeasurable {
			t.Fatalf("status=%s step=%+v calls=%d costs=%d", sum.Status, st, m.Calls(), len(costs))
		}
		for _, c := range costs {
			if !c.Estimated || !strings.HasPrefix(c.UnitPriceSource, "uncertain:timeout") {
				t.Fatalf("cost row %+v", c)
			}
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("timeouts not enforced promptly")
		}
	})
	t.Run("budget ceiling stops before any call", func(t *testing.T) {
		e := newEnv(t, nil)
		ctx := context.Background()
		_, opp := e.approved(t)
		e.cfg.Limits.Provider.Model = "claude-opus-5-5"
		spent := 19.99
		_ = e.st.CreateRun(ctx, &domain.Run{ID: "run_seed", Kind: "research", Status: "completed", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
		if err := e.st.InsertCost(ctx, &domain.CostRecord{ID: "cost_seed", RunID: "run_seed", Category: "llm", ProviderName: "x", UnitsJSON: "{}", UnitPriceSource: "test", CostUSD: &spent, PeriodMonth: "2026-10", CreatedAt: "t"}); err != nil {
			t.Fatal(err)
		}
		calls := e.mock.Calls()
		sum, _ := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
		if st := e.step(t, sum.RunID, orchestrator.StepContentAgent); sum.Status != domain.RunFailed || st.Status != domain.StepBudgetExhausted || e.mock.Calls() != calls {
			t.Fatalf("status=%s step=%+v", sum.Status, st)
		}
	})
	t.Run("fatal error then resume without duplicates", func(t *testing.T) {
		e := newEnv(t, nil)
		ctx := context.Background()
		_, opp := e.approved(t)
		bad := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
		bad.Fail = &provider.Error{Kind: provider.KindAuth, Err: errors.New("401")}
		bad.FailAttempts = 1
		first, _ := e.build(bad).RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
		if first.Status != domain.RunFailed || bad.Calls() != 1 || !strings.Contains(e.contentReport(t, first.RunID), "RUN GAGAL") {
			t.Fatalf("setup: %s", first.Status)
		}
		good := mock.New(e.cfg.Limits.Provider.Mock.FixturesDir)
		second, err := e.build(good).RunContent(ctx, orchestrator.Options{ResumeID: first.RunID})
		if err != nil || second.Status != domain.RunCompletedWithWarnings || good.Calls() != 1 || second.Content.Stored != 10 {
			t.Fatalf("resume: %v status=%s calls=%d", err, second.Status, good.Calls())
		}
		if strings.Contains(e.contentReport(t, first.RunID), "RUN GAGAL") {
			t.Fatal("resumed run must re-render the report")
		}
		third, _ := e.build(good).RunContent(ctx, orchestrator.Options{ResumeID: first.RunID})
		if third.Status != domain.RunCompletedWithWarnings || good.Calls() != 1 || len(e.drafts(t, first.RunID)) != 10 || third.Content.Proposed != 10 {
			t.Fatalf("re-render: status=%s calls=%d drafts=%d counts=%+v", third.Status, good.Calls(), len(e.drafts(t, first.RunID)), third.Content)
		}
		forced, _ := e.build(good).RunContent(ctx, orchestrator.Options{ResumeID: first.RunID, Force: true})
		if forced.Status != domain.RunFailed || good.Calls() != 1 || len(e.drafts(t, first.RunID)) != 10 || !strings.Contains(strings.Join(forced.Errors, " "), "batas 2 panggilan") {
			t.Fatalf("forced beyond the cap: status=%s calls=%d errors=%v", forced.Status, good.Calls(), forced.Errors)
		}
	})
}

func TestContent_NoNetworkNoPaidProvider(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	_, opp := e.approved(t)
	sum, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if err != nil {
		t.Fatal(err)
	}
	if caps := e.mock.Capabilities(); caps.Network {
		t.Fatal("mock provider must not use the network")
	}
	execs, _ := e.st.ExecutionsForRun(ctx, sum.RunID)
	if len(execs) != 1 {
		t.Fatalf("executions = %d", len(execs))
	}
	x := execs[0]
	if x.ProviderName != "mock" || x.AgentName != "content" || x.SchemaVersion != "content_drafts.v1" || x.CostUSD == nil || *x.CostUSD != 0 || !x.CostMeasurable {
		t.Fatalf("execution %+v", x)
	}
	req, err := os.ReadFile(x.RequestPath)
	if err != nil || !strings.Contains(string(req), "DATA TIDAK TEPERCAYA — MULAI") || !strings.Contains(string(req), "catatan-lapangan-contoh.md#2") || strings.Contains(string(req), "catatan-lapangan-contoh.md#3") {
		t.Fatal("request must hold only the opportunity's own evidence inside the untrusted block")
	}
	if sum.CostUSD != 0 || sum.CostUncertainUSD != 0 {
		t.Fatalf("cost %+v", sum)
	}
}

func TestContent_ReviewAppendOnlyAndRevision(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	_, opp := e.approved(t)
	sum, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if err != nil {
		t.Fatal(err)
	}
	d := draftByIndex(e.drafts(t, sum.RunID), 1)
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, d.ID, "approved", "", "CEO"); err == nil {
		t.Fatal("a review without a note must be refused")
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, d.ID, "publish", "x", "CEO"); err == nil {
		t.Fatal("unknown decisions must be refused")
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, d.ID, "approved", "Draf diperiksa", "CEO"); err != nil {
		t.Fatal(err)
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, d.ID, "approved", "lagi", "CEO"); err == nil {
		t.Fatal("the same decision twice for one version must be refused")
	}
	// Review history is append-only at the storage layer.
	if _, err := e.st.DB().Exec(`UPDATE approval_records SET decision = 'rejected' WHERE subject_id = ?`, d.ID); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("approval_records update must fail: %v", err)
	}
	if _, err := e.st.DB().Exec(`DELETE FROM approval_records WHERE subject_id = ?`, d.ID); err == nil {
		t.Fatal("approval_records delete must fail")
	}
	// A human revision after approval returns the draft to review with a new version.
	checker, _ := brand.New(e.cfg.Policy.Rules)
	ev, err := content.New(e.cfg, checker)
	if err != nil {
		t.Fatal(err)
	}
	body := d.Body + " Rincian dasar hitung akan dijelaskan sebelum Anda memutuskan."
	revised, r, err := content.Revise(ctx, e.st, ev, e.cfg, fixedNow, d.ID, content.Edit{Body: &body}, "Menambah kalimat penjelasan")
	if err != nil {
		t.Fatal(err)
	}
	reviews, _ := e.st.ApprovalsFor(ctx, domain.SubjectContentDraft, d.ID)
	if revised.Status != domain.DraftNeedsReview || revised.Version != 2 || r.Status != domain.DraftNeedsReview || len(reviews) != 1 || content.ReadinessOf(revised, reviews).Ready {
		t.Fatalf("revision: status=%s version=%d reviews=%d", revised.Status, revised.Version, len(reviews))
	}
	versions, _ := e.st.ContentDraftVersions(ctx, d.ID)
	if len(versions) != 2 || versions[1].Author != "human" || versions[1].Note != "Menambah kalimat penjelasan" || versions[0].Body != d.Body {
		t.Fatalf("versions: %+v", versions)
	}
	if _, _, err := content.Revise(ctx, e.st, ev, e.cfg, fixedNow, d.ID, content.Edit{Body: &body}, "sama"); !errors.Is(err, content.ErrNoChange) {
		t.Fatalf("a non-material edit must be refused: %v", err)
	}
	bad := "Harga emas sedang naik, jual sekarang."
	blockedRev, _, err := content.Revise(ctx, e.st, ev, e.cfg, fixedNow, d.ID, content.Edit{Body: &bad}, "uji")
	if err != nil || blockedRev.Status != domain.DraftBlocked || blockedRev.Version != 3 {
		t.Fatalf("a revision that fails a hard gate must be blocked: %v %+v", err, blockedRev)
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, d.ID, "approved", "x", "CEO"); !errors.Is(err, content.ErrNotReviewable) {
		t.Fatalf("blocked revision must not be reviewable: %v", err)
	}
	// Approval needs a review record for the current version; direct status changes are refused.
	other := draftByIndex(e.drafts(t, sum.RunID), 4)
	if _, err := e.st.DB().Exec(`UPDATE content_drafts SET status = 'approved' WHERE id = ?`, other.ID); err == nil || !strings.Contains(err.Error(), "review record") {
		t.Fatalf("approval without a review record must fail: %v", err)
	}
	// A text change made outside MIA on an approved draft returns it to review (trigger) and breaks readiness.
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, other.ID, "approved", "ok", "CEO"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.DB().Exec(`UPDATE content_drafts SET body = body || ' tambahan' WHERE id = ?`, other.ID); err != nil {
		t.Fatal(err)
	}
	tampered, _ := e.st.GetContentDraft(ctx, other.ID)
	reviews, _ = e.st.ApprovalsFor(ctx, domain.SubjectContentDraft, other.ID)
	if r := content.ReadinessOf(tampered, reviews); tampered.Status != domain.DraftNeedsReview || r.Ready || !strings.Contains(strings.Join(r.Reasons, " "), "di luar MIA") {
		t.Fatalf("out-of-band edit: status=%s readiness=%+v", tampered.Status, r)
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, other.ID, "approved", "ok", "CEO"); err == nil || !strings.Contains(err.Error(), "di luar MIA") {
		t.Fatalf("review of an out-of-band edit must be refused until revised: %v", err)
	}
	if _, err := e.st.DB().Exec(`DELETE FROM content_drafts WHERE id = ?`, other.ID); err == nil {
		t.Fatal("drafts must not be deletable")
	}
	if _, err := e.st.DB().Exec(`UPDATE content_draft_versions SET body = 'x' WHERE draft_id = ?`, d.ID); err == nil {
		t.Fatal("versions must be append-only")
	}
}

// Regression (Phase 5): re-rendering a content report after reviews shows current counts, not the
// counts captured in the run summary when the run finished.
func TestContent_ReportCountsReflectReviewsAfterRun(t *testing.T) {
	e := newEnv(t, nil)
	ctx := context.Background()
	_, opp := e.approved(t)
	sum, err := e.orch.RunContent(ctx, orchestrator.Options{OpportunityID: opp.ID})
	if err != nil || sum.Content.Approved != 0 {
		t.Fatalf("setup: %v %+v", err, sum.Content)
	}
	ds := e.drafts(t, sum.RunID)
	for _, idx := range []int{1, 4} {
		if _, err := content.Review(ctx, e.st, e.ids, fixedNow, draftByIndex(ds, idx).ID, "approved", "ok", "CEO"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := content.Review(ctx, e.st, e.ids, fixedNow, draftByIndex(ds, 2).ID, "rejected", "tidak", "CEO"); err != nil {
		t.Fatal(err)
	}
	run, _ := e.st.GetRun(ctx, sum.RunID)
	rend := &report.Renderer{Store: e.st, Cfg: e.cfg, Now: now}
	if _, _, err := rend.RenderContent(ctx, run, sum); err != nil { // the old summary, as `mia report <run>` uses it
		t.Fatal(err)
	}
	md := e.contentReport(t, sum.RunID)
	if !strings.Contains(md, "Status: perlu tinjauan 4 · lolos gerbang 3 · disetujui 2 · ditolak 1 · diblokir 0") || !strings.Contains(md, "diusulkan agen: 10") {
		t.Fatal("re-rendered report must show the stored statuses, not the run-time snapshot")
	}
}
