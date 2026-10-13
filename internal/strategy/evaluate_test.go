package strategy_test

import (
	"regexp"
	"strings"
	"testing"

	agent "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/strategy"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func newEval(t *testing.T) (*strategy.Evaluator, *config.Config) {
	t.Helper()
	cfg, err := config.Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	checker, err := brand.New(cfg.Policy.Rules)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := strategy.New(cfg, checker)
	if err != nil {
		t.Fatal(err)
	}
	return ev, cfg
}

func ev(id, ref, kind, quality, topic, statement, src string) strategy.Evidence {
	return strategy.Evidence{ID: id, Ref: ref, Kind: kind, Quality: quality, TopicKey: topic, Title: "Judul " + id, Statement: statement, Excerpt: statement, SourceID: src, SourceName: "sumber " + src}
}

// pool is the validated-evidence view handed to the evaluator (ref -> evidence).
func pool() map[string]strategy.Evidence {
	list := []strategy.Evidence{
		ev("it_a", "a.md#1", domain.KindFact, "A", "timbang", "Toko X menimbang di depan pembeli.", "src1"),
		ev("it_b", "b.md#1", domain.KindFact, "B", "timbang", "Artikel menyarankan penimbangan di depan penjual.", "src2"),
		ev("it_b2", "b2.md#1", domain.KindFact, "B", "timbang", "Panduan lain menyarankan penimbangan terbuka.", "src10"),
		ev("it_c1", "c.md#1", domain.KindFact, "C", "harga", "Catatan: penawaran berbeda antar toko.", "src3"),
		ev("it_c2", "c.md#2", domain.KindFact, "C", "harga", "Penawaran berbeda karena potongan.", "src4"),
		ev("it_c3", "c.md#3", domain.KindFact, "C", "harga", "Catatan lanjutan dari sumber yang sama.", "src3"),
		ev("it_d", "d.md#1", domain.KindFact, "D", "ktp", "Catatan internal tanpa sumber luar.", "src5"),
		ev("it_i", "i.md#1", domain.KindInterpretation, "B", "timbang", "Interpretasi: kejelasan proses dibutuhkan.", "src2"),
		ev("it_h", "h.csv#2", domain.KindHypothesis, "D", "durasi", "Calon penjual bertanya durasi proses.", "src6"),
		ev("it_x", "x.md#1", domain.KindFact, "A", "ktp", "Petugas mencatat 12 pengunjung bertanya soal KTP.", "src7"),
	}
	k1 := ev("it_k1", "k1.md#1", domain.KindFact, "A", "ktp", "Pengunjung perlu membawa KTP.", "src8")
	k1.ConflictGroup = "cg:ktp"
	k2 := ev("it_k2", "k2.md#1", domain.KindFact, "A", "ktp", "Pengunjung tidak perlu membawa KTP.", "src9")
	k2.ConflictGroup = "cg:ktp"
	list = append(list, k1, k2)
	m := map[string]strategy.Evidence{}
	for _, e := range list {
		m[e.Ref] = e
	}
	return m
}

func base(title, topic string, refs ...string) agent.Proposal {
	return agent.Proposal{Title: title, Segment: "AU1", TopicKey: topic, Need: "Penjual ingin melihat penimbangan.", Interpretation: "Interpretasi: kejelasan proses dibutuhkan.",
		FactRefs: refs, Dimensions: agent.Dimensions{EvidenceStrength: 3, CustomerIntent: 3, StrategicFit: 3, EaseOfTesting: 3, Measurability: 3},
		DimensionRationale: map[string]string{"customer_intent": "pengamatan", "strategic_fit": "pilar", "ease_of_testing": "konten", "measurability": "metrik"},
		FitPillars: []string{"PM1"}, Message: "Kami akan menimbang di depan Bapak atau Ibu.", OfferConcept: "Konten edukasi.", Channel: "Situs",
		TestDesign: "Uji konten.", SuccessMetric: "O4", SuccessCriteria: "Brief disetujui.", Resources: "Tim konten.", Effort: "rendah", NextAction: "Susun draf."}
}

func one(t *testing.T, e *strategy.Evaluator, p agent.Proposal) strategy.Result {
	t.Helper()
	res := e.Evaluate([]agent.Proposal{p}, pool())
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	return res[0]
}

func joined(r strategy.Result) string {
	var parts []string
	parts = append(parts, r.RejectReasons...)
	for _, g := range r.Gates {
		parts = append(parts, g.Details...)
	}
	return strings.Join(parts, " | ")
}

func TestValidEvidenceIsTraceableAndTierA(t *testing.T) {
	e, _ := newEval(t)
	r := one(t, e, base("Penimbangan terbuka", "timbang", "a.md#1", "b.md#1"))
	if r.Status != domain.OppProposed || r.Tier != domain.TierA {
		t.Fatalf("status=%s tier=%s reasons=%s", r.Status, r.Tier, joined(r))
	}
	if strings.Join(r.EvidenceIDs, ",") != "it_a,it_b" || r.Roles["it_a"] != domain.KindFact {
		t.Fatalf("evidence ids %v roles %v", r.EvidenceIDs, r.Roles)
	}
	if d := r.Dims["evidence_strength"]; d.Value != 3 || d.Source != "code" {
		t.Fatalf("evidence strength must be computed by code: %+v", d)
	}
	if r.OrdinalSum != 15 || r.Hypothesis || r.RequiresStore {
		t.Fatalf("sum=%d hypothesis=%v store=%v", r.OrdinalSum, r.Hypothesis, r.RequiresStore)
	}
	if len(r.Brief.Facts) != 2 || r.Brief.Facts[0].ID != "it_a" || r.Brief.Facts[0].Quality != "A" {
		t.Fatalf("brief facts %+v", r.Brief.Facts)
	}
	if len(r.Caps) != 0 || len(r.Conflicts) != 0 {
		t.Fatalf("unexpected caps %v conflicts %v", r.Caps, r.Conflicts)
	}
}

func TestGate1_NonexistentOrUnvalidatedRefs(t *testing.T) {
	e, _ := newEval(t)
	r := one(t, e, base("Tanpa bukti", "timbang", "tidak-ada.md#9", "manual:unvalidated#1"))
	if r.Status != domain.OppRejected || r.Tier != domain.TierNone || len(r.EvidenceIDs) != 0 {
		t.Fatalf("status=%s tier=%s ids=%v", r.Status, r.Tier, r.EvidenceIDs)
	}
	s := joined(r)
	for _, want := range []string{"Gerbang 1", `"tidak-ada.md#9"`, `"manual:unvalidated#1"`, "belum divalidasi"} {
		if !strings.Contains(s, want) {
			t.Errorf("gate output lacks %q: %s", want, s)
		}
	}
	// A valid ref next to an invalid one survives; the invalid one is reported and never linked.
	r = one(t, e, base("Campuran", "timbang", "a.md#1", "tidak-ada.md#9"))
	if r.Status != domain.OppProposed || strings.Join(r.EvidenceIDs, ",") != "it_a" || !strings.Contains(joined(r), "tidak-ada.md#9") {
		t.Fatalf("mixed refs: status=%s ids=%v gates=%s", r.Status, r.EvidenceIDs, joined(r))
	}
}

func TestGate1_HypothesisOnlyIsCappedAtTierC(t *testing.T) {
	e, _ := newEval(t)
	for _, refs := range [][]string{{"h.csv#2"}, {"i.md#1"}, {"d.md#1"}} {
		r := one(t, e, base("Hanya hipotesis", "durasi", refs...))
		if r.Status != domain.OppProposed || !r.Hypothesis || r.Tier != domain.TierC {
			t.Fatalf("refs %v: status=%s hypothesis=%v tier=%s", refs, r.Status, r.Hypothesis, r.Tier)
		}
		if r.Dims["evidence_strength"].Value != 0 {
			t.Fatalf("refs %v: strength %d", refs, r.Dims["evidence_strength"].Value)
		}
		if d := r.Dims["customer_intent"]; d.Value > 1 || !strings.Contains(d.Source, "cap") {
			t.Fatalf("refs %v: intent must be capped without facts: %+v", refs, d)
		}
		found := false
		for _, c := range r.Caps {
			if c.Gate == "evidence" {
				found = true
			}
		}
		if !found || !strings.Contains(strings.Join(r.Brief.MissingEvidence, " "), "Belum ada fakta tervalidasi") {
			t.Fatalf("refs %v: caps %v missing %v", refs, r.Caps, r.Brief.MissingEvidence)
		}
	}
}

func TestEvidenceStrengthRubricIsComputedByCode(t *testing.T) {
	e, _ := newEval(t)
	cases := []struct {
		refs []string
		want int
	}{
		{[]string{"a.md#1"}, 3},
		{[]string{"b.md#1", "b2.md#1"}, 3},
		{[]string{"b.md#1"}, 2},
		{[]string{"c.md#1", "c.md#2"}, 2},
		{[]string{"c.md#1", "c.md#3"}, 1}, // same source twice counts once
		{[]string{"c.md#1"}, 1},
		{[]string{"d.md#1"}, 0},
		{[]string{"b.md#1", "i.md#1", "h.csv#2"}, 2}, // interpretations and hypotheses do not add strength
	}
	for _, c := range cases {
		p := base("Uji rubrik", "timbang", c.refs...)
		p.Dimensions.EvidenceStrength = 3 // the agent's own value must be ignored
		r := one(t, e, p)
		if got := r.Dims["evidence_strength"].Value; got != c.want {
			t.Errorf("refs %v: strength %d, want %d (%s)", c.refs, got, c.want, r.Dims["evidence_strength"].Rationale)
		}
	}
}

func TestGate2_ConflictShowsBothSidesAndCapsTier(t *testing.T) {
	e, _ := newEval(t)
	r := one(t, e, base("KTP wajib", "ktp", "k1.md#1"))
	if r.Status != domain.OppProposed || r.Tier != domain.TierC || len(r.Conflicts) != 1 || len(r.Conflicts[0].Sides) != 2 {
		t.Fatalf("status=%s tier=%s conflicts=%+v", r.Status, r.Tier, r.Conflicts)
	}
	cited, uncited := 0, 0
	for _, s := range r.Conflicts[0].Sides {
		if s.Cited {
			cited++
		} else {
			uncited++
		}
	}
	if cited != 1 || uncited != 1 || r.Conflicts[0].Group != "cg:ktp" {
		t.Fatalf("both sides must be shown with the cited one marked: %+v", r.Conflicts)
	}
	if r.Dims["evidence_strength"].Value != 3 {
		t.Fatal("strength is still computed; only the tier is capped")
	}
	if !strings.Contains(strings.Join(r.Brief.Dependencies, " "), "cg:ktp") {
		t.Fatalf("brief must name the conflict to resolve: %v", r.Brief.Dependencies)
	}
}

func TestGate3_BrandFailureBlocks(t *testing.T) {
	e, _ := newEval(t)
	p := base("Harga terbaik", "timbang", "a.md#1", "b.md#1")
	p.Message = "Harga tertinggi dan dijamin puas."
	r := one(t, e, p)
	if r.Status != domain.OppBlocked || r.Tier != domain.TierNone {
		t.Fatalf("status=%s tier=%s", r.Status, r.Tier)
	}
	s := strings.Join(r.RejectReasons, " ")
	if !strings.Contains(s, "SUPERLATIF") || !strings.Contains(s, "ABSOLUT") {
		t.Fatalf("reject reasons should name the brand categories: %s", s)
	}
}

func TestGate3b_UnsupportedFactualClaimsRejected(t *testing.T) {
	e, _ := newEval(t)
	cases := []struct{ text, category string }{
		{"Kami membeli Rp 1.000.000 per gram.", "HARGA"},
		{"Buka pukul 08.00 sampai 17.00.", "JAM_BUKA"},
		{"Kunjungi kami di Jl. Contoh No. 5.", "ALAMAT"},
		{"90% penjual puas.", "STATISTIK"},
		{"Pelanggan kami mengatakan puas.", "KLAIM_PELANGGAN"},
		{"Proses selesai dalam 3 hari.", `angka "3"`},
	}
	for _, c := range cases {
		p := base("Klaim", "timbang", "a.md#1", "b.md#1")
		p.OfferConcept = c.text
		r := one(t, e, p)
		if r.Status != domain.OppRejected || r.Tier != domain.TierNone {
			t.Errorf("%q: status=%s tier=%s", c.text, r.Status, r.Tier)
			continue
		}
		if s := strings.Join(r.RejectReasons, " "); !strings.Contains(s, c.category) {
			t.Errorf("%q: reasons lack %s: %s", c.text, c.category, s)
		}
	}
}

func TestGate3b_NumbersSupportedByEvidenceOrIdentifiersPass(t *testing.T) {
	e, _ := newEval(t)
	p := base("Angka didukung", "ktp", "x.md#1")
	p.OfferConcept = "Dalam pengamatan contoh, 12 pengunjung bertanya soal KTP; minimal 10 jawaban survei."
	p.NextAction = "Mendukung PM1 dan O4; lihat FB5, CT3, Q61, M1 §2, aturan #5, D-019."
	r := one(t, e, p)
	if r.Status != domain.OppProposed {
		t.Fatalf("status=%s reasons=%s", r.Status, joined(r))
	}
}

func TestGate4_StoreDependencyIsLabelledNotRejected(t *testing.T) {
	e, _ := newEval(t)
	p := base("Layanan konter", "timbang", "a.md#1", "b.md#1")
	p.Resources = "Petugas konter dan kasir."
	r := one(t, e, p)
	if r.Status != domain.OppProposed || !r.RequiresStore {
		t.Fatalf("status=%s requires_store=%v", r.Status, r.RequiresStore)
	}
	if d := r.Dims["ease_of_testing"]; d.Value > 1 || !strings.Contains(d.Source, "cap") {
		t.Fatalf("ease must be capped when a store is required: %+v", d)
	}
	if r.Brief.OperationalNote == "" || !strings.Contains(strings.Join(r.Brief.Dependencies, " "), "Konfirmasi operasional") {
		t.Fatalf("brief lacks the operational label: %+v", r.Brief)
	}
	if !strings.Contains(joined(r), "konter") {
		t.Fatalf("gate details should name the operational terms: %s", joined(r))
	}
	// The explicit flag alone also labels the opportunity.
	p = base("Flag saja", "timbang", "a.md#1")
	p.RequiresStore = true
	if r := one(t, e, p); !r.RequiresStore {
		t.Fatal("requires_store flag ignored")
	}
}

func TestGate5_DedupMergesWithoutLosingCorroboration(t *testing.T) {
	e, _ := newEval(t)
	p1 := base("Penimbangan terbuka sebagai pembeda", "timbang", "a.md#1")
	p2 := base("Tunjukkan penimbangan", "timbang", "a.md#1", "b.md#1") // shares all of p1's evidence
	p3 := base("Edukasi penawaran berbeda antar toko", "harga", "c.md#1")
	p4 := base("Edukasi penawaran berbeda antar toko lain", "harga", "c.md#2") // title Jaccard ≥ 0.8, no shared evidence
	p5 := base("Penimbangan terbuka sebagai pembeda", "ktp", "x.md#1")        // same title, different topic: kept
	res := e.Evaluate([]agent.Proposal{p1, p2, p3, p4, p5}, pool())
	if len(res) != 3 {
		t.Fatalf("expected 3 results after dedup, got %d", len(res))
	}
	m := res[0]
	if m.ProposalIndex != 1 || len(m.MergedFrom) != 1 || m.MergedFrom[0] != 2 || strings.Join(m.EvidenceIDs, ",") != "it_a,it_b" {
		t.Fatalf("merge lost corroboration: idx=%d merged=%v ids=%v", m.ProposalIndex, m.MergedFrom, m.EvidenceIDs)
	}
	if m.Tier != domain.TierA || m.Dims["customer_intent"].Value != 3 {
		t.Fatalf("merged evidence should count toward independence: tier=%s intent=%+v", m.Tier, m.Dims["customer_intent"])
	}
	h := res[1]
	if h.ProposalIndex != 3 || len(h.MergedFrom) != 1 || h.MergedFrom[0] != 4 || strings.Join(h.EvidenceIDs, ",") != "it_c1,it_c2" {
		t.Fatalf("title-similar merge: idx=%d merged=%v ids=%v", h.ProposalIndex, h.MergedFrom, h.EvidenceIDs)
	}
	if h.Dims["evidence_strength"].Value != 2 {
		t.Fatalf("two C facts from different sources after merge should give strength 2, got %d", h.Dims["evidence_strength"].Value)
	}
	if res[2].ProposalIndex != 5 || len(res[2].MergedFrom) != 0 {
		t.Fatalf("different topic must not merge: %+v", res[2])
	}
}

func TestTiersAreRecomputedByCode(t *testing.T) {
	e, _ := newEval(t)
	// Tier B: one C fact (strength 1), fit ≥ 2.
	r := one(t, e, base("Tingkat B", "harga", "c.md#1"))
	if r.Tier != domain.TierB || r.Dims["evidence_strength"].Value != 1 {
		t.Fatalf("B: tier=%s strength=%d", r.Tier, r.Dims["evidence_strength"].Value)
	}
	// Tier C: strong evidence but no valid pillar → fit capped to 1.
	p := base("Tanpa pilar", "timbang", "a.md#1", "b.md#1")
	p.FitPillars = []string{"PMX"}
	r = one(t, e, p)
	if r.Tier != domain.TierC || r.Dims["strategic_fit"].Value != 1 {
		t.Fatalf("C via fit: tier=%s fit=%+v", r.Tier, r.Dims["strategic_fit"])
	}
	// Intent below 2 keeps an otherwise strong proposal out of Tier A.
	p = base("Niat rendah", "timbang", "a.md#1", "b.md#1")
	p.Dimensions.CustomerIntent = 1
	if r = one(t, e, p); r.Tier != domain.TierB {
		t.Fatalf("intent 1 should give Tier B, got %s", r.Tier)
	}
	// One source only caps intent at 2 (still Tier A) and values above 3 are clamped.
	p = base("Satu sumber", "timbang", "a.md#1")
	p.Dimensions.CustomerIntent = 9
	r = one(t, e, p)
	if r.Tier != domain.TierA || r.Dims["customer_intent"].Value != 2 {
		t.Fatalf("single source: tier=%s intent=%+v", r.Tier, r.Dims["customer_intent"])
	}
}

func TestMeasurabilityFollowsKPIAvailability(t *testing.T) {
	e, _ := newEval(t)
	p := base("Metrik asing", "timbang", "a.md#1")
	p.SuccessMetric = "ZZ9"
	r := one(t, e, p)
	if d := r.Dims["measurability"]; d.Value != 0 || !strings.HasPrefix(d.Source, "code") {
		t.Fatalf("unknown metric: %+v", d)
	}
	p.SuccessMetric = "B1"
	r = one(t, e, p)
	if d := r.Dims["measurability"]; d.Value != 1 || !strings.Contains(d.Source, "cap") {
		t.Fatalf("business KPI before opening must be capped: %+v", d)
	}
	if !strings.Contains(strings.Join(r.Brief.Dependencies, " "), "B1") {
		t.Fatalf("brief should state the KPI dependency: %v", r.Brief.Dependencies)
	}
}

func TestDetectConflicts(t *testing.T) {
	neg := regexp.MustCompile(`(?i)\b(tidak|bukan|belum|tanpa|jarang)\b`)
	items := []strategy.Evidence{
		ev("1", "a#1", domain.KindFact, "A", "t", "Toko X menimbang di depan pembeli.", "s1"),
		ev("2", "b#1", domain.KindFact, "A", "t", "Toko X tidak menimbang di depan pembeli.", "s2"),
		ev("3", "c#1", domain.KindFact, "A", "t", "Toko X belum menimbang terbuka.", "s1"), // same source as 1: ignored against 1, pairs with none
		ev("4", "d#1", domain.KindFact, "A", "u", "Toko Y tidak buka.", "s3"),
		ev("5", "e#1", domain.KindFact, "A", "u", "Toko Y buka.", "s3"), // same source: no conflict
		ev("6", "f#1", domain.KindInterpretation, "A", "t", "Tidak jelas.", "s4"),
	}
	g := strategy.DetectConflicts(items, neg, 0.5)
	if g["1"] != "cg:t" || g["2"] != "cg:t" {
		t.Fatalf("expected items 1 and 2 in cg:t, got %v", g)
	}
	if _, ok := g["4"]; ok {
		t.Fatal("same-source statements must not conflict")
	}
	if _, ok := g["6"]; ok {
		t.Fatal("interpretations are not facts and never conflict")
	}
	if _, ok := g["3"]; ok {
		t.Fatalf("item 3 shares a source with 1 and matches 2 in polarity; it must not be grouped: %v", g)
	}
	// Opposite polarity under one topic is not enough: unrelated statements are not conflicts.
	unrelated := []strategy.Evidence{
		ev("7", "g#1", domain.KindFact, "A", "v", "Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli.", "s5"),
		ev("8", "h#1", domain.KindFact, "B", "v", "Artikel menyarankan agar penjual meminta penimbangan dilakukan di depan mata.", "s6"),
	}
	if g := strategy.DetectConflicts(unrelated, neg, 0.5); len(g) != 0 {
		t.Fatalf("dissimilar statements must not be grouped: %v", g)
	}
	if g := strategy.DetectConflicts(unrelated, neg, 0); len(g) != 2 {
		t.Fatalf("threshold 0 falls back to polarity only: %v", g)
	}
}
