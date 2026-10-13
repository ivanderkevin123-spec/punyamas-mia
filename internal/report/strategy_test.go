package report

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
)

func hostileOpp() OppView {
	return OppView{ID: "opp_1", Title: "Judul | jahat <b>bold</b>", Segment: "AU1", TopicKey: "topik", Need: hostileStatement, Interpretation: readable,
		Status: domain.OppProposed, StatusLabel: "usulan", Tier: domain.TierA, OrdinalSum: 12,
		Dims:  []DimView{{Key: "evidence_strength", Label: "Kekuatan bukti", Value: 3, Rationale: "alasan | <i>x</i>", Source: "code"}},
		Caps:  []strategy.Cap{{Gate: "conflict", Reason: "alasan <script>alert(1)</script>"}},
		Gates: []strategy.GateResult{{Gate: "evidence", Passed: true, Details: []string{"detail <img src=x>"}}},
		Conflicts: []strategy.ConflictView{{Group: "cg:topik", Sides: []strategy.ConflictSide{{ID: "it_1", Ref: "a.md#1", Statement: hostileStatement, SourceName: hostileName, Cited: true}}}},
		Brief: strategy.Brief{Title: "t", Need: hostileStatement, Interpretation: readable, Facts: []strategy.BriefEvidence{{ID: "it_1", Ref: "a.md#1", Quality: "A", Statement: hostileStatement, SourceName: hostileName}},
			Message: readable, OfferConcept: "konsep <script>x</script>", Channel: "kanal", TestDesign: "uji", SuccessMetric: "O4", SuccessCriteria: "kriteria", Resources: "sumber", Effort: "rendah",
			Risks: []string{"risiko | <b>x</b>"}, NextAction: "tindakan"},
		RejectReasons: []string{"alasan <b>tolak</b>"},
		Approvals:     []ApprovalView{{Decision: "approved", By: "CEO | <b>x</b>", Note: "catatan <script>", At: "2026-10-12T10:00:00Z"}},
		ConfigHash:    "cfg", ScoringHash: "sc", FixtureHash: "fx", RunID: "run_s", ParentRunID: "run_r"}
}

func assertEscaped(t *testing.T, s, what string) {
	t.Helper()
	for _, bad := range []string{`<script>`, `<b>`, `<img`, `<i>`, `\]\(javascript:`, "`kode`", `\*\*tebal\*\*`} {
		if re := regexp.MustCompile(`(^|[^\\])` + bad); re.MatchString(s) {
			t.Errorf("%s still contains unescaped %q", what, bad)
		}
	}
	if !strings.Contains(s, readable) {
		t.Errorf("%s altered legitimate Indonesian text", what)
	}
}

func TestStrategyMarkdownAndBriefEscapeHostileValues(t *testing.T) {
	r, _, _ := seed(t)
	v := hostileOpp()
	d := &StrategyData{Title: "Laporan Strategi Pemasaran MIA", BrandName: "Punyamas", RunID: "run_s", ParentRunID: "run_r", Status: domain.RunCompleted, StatusLabel: "Selesai",
		Warnings: []string{"peringatan <b>x</b>"}, Steps: []domain.StepSummary{{Name: "evaluate", Status: "completed", Attempts: 1, Error: "err | <b>x</b>"}},
		ConfigHash: "cfg", ScoringHash: "sc", FixtureHash: "fx", Provider: "mock",
		TierA: []OppView{v}, Rejected: []OppView{v}, NextSteps: []string{"langkah <script>"}, CostTotal: "USD 0.0000", CostActual: "USD 0.0000", CostUncertain: "USD 0.0000", CostMeasurable: true, MonthlyCeiling: "USD 20.00"}
	md, err := r.RenderStrategyMarkdown(d)
	if err != nil {
		t.Fatal(err)
	}
	assertEscaped(t, string(md), "strategy markdown")
	for _, want := range []string{"Tingkat A (1)", "Ditolak gerbang bukti atau klaim faktual (1)", "Konflik bukti", "kedua sisi", "jumlah ordinal, bukan probabilitas", "Riwayat keputusan (hanya-tambah)", "`cfg`", "`sc`", "`fx`"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
	for _, line := range strings.Split(string(md), "\n") {
		if strings.HasPrefix(line, "| evaluate") {
			if n := len(unescapedPipe.FindAllString(line, -1)); n != 5 {
				t.Errorf("step row has %d unescaped pipes, want 5: %s", n, line)
			}
		}
	}
	brief, err := r.RenderBrief(&v)
	if err != nil {
		t.Fatal(err)
	}
	assertEscaped(t, string(brief), "brief")
	for _, want := range []string{"Brief Kampanye", "DRAF UNTUK SERAH TERIMA MANUAL", "2026-10-12T10:00:00Z", "Fakta pendukung", "Jumlah ordinal: **12 dari 15**"} {
		if !strings.Contains(string(brief), want) {
			t.Errorf("brief lacks %q", want)
		}
	}
	html, err := r.RenderStrategyHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	h := string(html)
	if strings.Contains(h, "<script>alert") || strings.Contains(h, "<img src=x>") || !strings.Contains(h, "&lt;script&gt;") {
		t.Error("html must escape hostile values")
	}
	if !strings.Contains(h, "tingkat A") || !strings.Contains(h, "Ditolak gerbang bukti") {
		t.Error("html sections missing")
	}
}
