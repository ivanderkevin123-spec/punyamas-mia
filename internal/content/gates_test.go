package content_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	agent "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func setup(t *testing.T) (*content.Evaluator, *config.Config, *content.Source) {
	t.Helper()
	cfg, err := config.Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	checker, err := brand.New(cfg.Policy.Rules)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := content.New(cfg, checker)
	if err != nil {
		t.Fatal(err)
	}
	own := []strategy.Evidence{
		{ID: "it_a", Ref: "a.md#1", Kind: domain.KindFact, Quality: "A", Statement: "Petugas mencatat 12 pengunjung bertanya soal penimbangan.", SourceName: "s"},
		{ID: "it_h", Ref: "h.csv#2", Kind: domain.KindHypothesis, Quality: "D", Statement: "Calon penjual bertanya durasi.", SourceName: "s"},
		{ID: "it_d", Ref: "d.md#1", Kind: domain.KindFact, Quality: "D", Statement: "Catatan internal.", SourceName: "s"},
	}
	src := &content.Source{Opportunity: &domain.Opportunity{ID: "opp_1", RunID: "run_s", ScoringConfigHash: "sh"}, ResearchRunID: "run_r",
		ByRef: map[string]strategy.Evidence{}, ByID: map[string]strategy.Evidence{},
		ResearchRefs: map[string]string{"a.md#1": "validated", "h.csv#2": "validated", "d.md#1": "validated", "lain.md#3": "validated", "manual:x#1": "imported"}}
	for _, e := range own {
		src.Evidence = append(src.Evidence, e)
		src.ByRef[e.Ref], src.ByID[e.ID] = e, e
	}
	return ev, cfg, src
}

func clean() agent.Draft {
	return agent.Draft{ContentType: "instagram_caption", Variant: 1, Channel: "instagram", TargetSegment: "AU1", Objective: "o",
		Hook: "Ditimbang dan diuji di depan Anda.", Body: "Kami akan menimbang dan menguji barang di depan Bapak atau Ibu, dan harga akan datang dengan dasar hitungnya.",
		CallToAction: "Simpan unggahan ini.", SupportingEvidenceRefs: []string{"a.md#1"},
		Claims: []agent.Claim{{Text: "Pengunjung bertanya soal penimbangan.", Kind: agent.ClaimFact, EvidenceRefs: []string{"a.md#1"}}}}
}

func categories(r content.Result) map[string]string {
	m := map[string]string{}
	for _, f := range r.Findings {
		m[f.Category] = f.Severity
	}
	return m
}

func TestCleanDraftPassesEveryGate(t *testing.T) {
	ev, cfg, src := setup(t)
	r := ev.Evaluate(clean(), src)
	if r.Status != domain.DraftDraft || len(r.Findings) != 0 || len(r.Placeholders) != 0 || len(r.ClaimsToVerify) != 0 {
		t.Fatalf("status=%s findings=%+v", r.Status, r.Findings)
	}
	if strings.Join(r.EvidenceIDs, ",") != "it_a" || r.Roles["it_a"] != domain.KindFact || !r.Claims[0].Supported {
		t.Fatalf("evidence %v roles %v claims %+v", r.EvidenceIDs, r.Roles, r.Claims)
	}
	if len(r.Checklist) != len(cfg.Content.ReviewChecklist) || r.Checklist[0].Source != "code" {
		t.Fatalf("checklist %+v", r.Checklist)
	}
	if r.ContentHash != content.Hash("instagram_caption", "instagram", r.Draft.Hook, "", r.Draft.Body, r.Draft.CallToAction) {
		t.Fatal("content hash")
	}
	// Whitespace-only differences do not change the hash (no material change).
	if content.Hash("a", "b", "x  y", "", "z\n", "") != content.Hash("a", "b", "x y", "", " z", "") {
		t.Fatal("hash must ignore whitespace-only edits")
	}
}

func TestEvidenceRefsMustBelongToTheOpportunity(t *testing.T) {
	ev, _, src := setup(t)
	for ref, want := range map[string]string{"tidak-ada.md#9": "BUKTI_TIDAK_ADA", "lain.md#3": "BUKTI_LINTAS_PELUANG", "manual:x#1": "BUKTI_BELUM_VALID"} {
		d := clean()
		d.SupportingEvidenceRefs = []string{"a.md#1", ref}
		r := ev.Evaluate(d, src)
		if r.Status != domain.DraftBlocked || categories(r)[want] != content.SeverityBlock {
			t.Errorf("%s: status=%s findings=%+v", ref, r.Status, r.Findings)
		}
		for _, id := range r.EvidenceIDs {
			if id != "it_a" {
				t.Errorf("%s: invalid ref linked as %s", ref, id)
			}
		}
	}
	d := clean()
	d.BrandFactIDs = []string{"FB99"}
	if r := ev.Evaluate(d, src); r.Status != domain.DraftBlocked || categories(r)["FAKTA_TIDAK_ADA"] == "" {
		t.Fatalf("unknown business fact: %+v", r.Findings)
	}
	d = clean()
	d.SupportingEvidenceRefs, d.Claims = nil, nil
	if r := ev.Evaluate(d, src); r.Status != domain.DraftNeedsReview || categories(r)["TANPA_BUKTI"] != content.SeverityReview {
		t.Fatalf("a draft without evidence needs review: %+v", r.Findings)
	}
}

func TestClaimsWithoutSupportNeedVerification(t *testing.T) {
	ev, _, src := setup(t)
	d := clean()
	d.Claims = []agent.Claim{
		{Text: "tanpa referensi", Kind: agent.ClaimFact},
		{Text: "didukung hipotesis saja", Kind: agent.ClaimFact, EvidenceRefs: []string{"h.csv#2"}},
		{Text: "didukung fakta kualitas D", Kind: agent.ClaimFact, EvidenceRefs: []string{"d.md#1"}},
		{Text: "didukung fakta usaha", Kind: agent.ClaimFact, FactIDs: []string{"FB7"}},
		{Text: "bacaan pemasaran", Kind: agent.ClaimInterpretation},
	}
	r := ev.Evaluate(d, src)
	if r.Status != domain.DraftNeedsReview || strings.Join(r.ClaimsToVerify, "|") != "tanpa referensi|didukung hipotesis saja|didukung fakta kualitas D" {
		t.Fatalf("status=%s to verify=%v", r.Status, r.ClaimsToVerify)
	}
	if !r.Claims[3].Supported || r.Claims[4].Supported || !strings.Contains(r.Claims[4].Note, "bukan fakta") {
		t.Fatalf("claims %+v", r.Claims)
	}
}

func TestFactualAndLanguageRules(t *testing.T) {
	ev, _, src := setup(t)
	cases := []struct {
		text, category, severity string
	}{
		{"Kami membeli Rp 1.000.000 per gram.", "HARGA", content.SeverityBlock},
		{"Buka pukul 08.00 setiap hari.", "JAM_BUKA", content.SeverityBlock},
		{"Kunjungi kami di Jl. Contoh No. 5.", "ALAMAT", content.SeverityBlock},
		{"90% penjual puas.", "STATISTIK", content.SeverityBlock},
		{"Pelanggan kami mengatakan prosesnya jelas.", "KLAIM_PELANGGAN", content.SeverityBlock},
		{"Sudah ada 37 penjual yang datang.", "ANGKA", content.SeverityBlock},
		{"Menjual emas di sini bebas risiko.", "JANJI_UNTUNG", content.SeverityBlock},
		{"Harga emas sedang naik, jual sekarang.", "TREN_HARGA", content.SeverityBlock},
		{"Saat yang tepat untuk menjual adalah minggu depan.", "TREN_HARGA", content.SeverityBlock},
		{"Kata Bu Sari: prosesnya jelas sekali.", "TESTIMONI", content.SeverityBlock},
		{"Lihat testimoni penjual kami.", "TESTIMONI", content.SeverityBlock},
		{"Sudah dipercaya ribuan penjual.", "TESTIMONI", content.SeverityBlock},
		{"Hanya hari ini untuk Anda.", "URGENSI_PALSU", content.SeverityBlock},
		{"Kuota terbatas untuk penimbangan.", "URGENSI_PALSU", content.SeverityBlock},
		{"Datang sebelum kehabisan.", "URGENSI_PALSU", content.SeverityBlock},
		{"Datanglah!!", "URGENSI_PALSU", content.SeverityBlock},
		{"Dapatkan bonus untuk penjualan pertama.", "DISKON_PROMO", content.SeverityBlock},
		{"Ikuti promo pembukaan sampai tanggal 30.", "DISKON_PROMO", content.SeverityBlock},
		{"Uang dibayar dalam 15 menit.", "WAKTU_PROSES", content.SeverityBlock},
		{"Prosesnya 5 menit selesai.", "WAKTU_PROSES", content.SeverityBlock},
		{"Uang seketika ditransfer.", "WAKTU_PROSES", content.SeverityBlock},
		{"Segera hubungi kami.", "TEKANAN", content.SeverityReview},
		{"Kami menunggu Anda!", "TEKANAN", content.SeverityReview},
		{"Alamat: TODO", "PLACEHOLDER_TIDAK_BAKU", content.SeverityReview},
		{"Kontak: [ISI]", "PLACEHOLDER_TIDAK_BAKU", content.SeverityReview},
		{"Kontak: [ISI: ]", "PLACEHOLDER_TIDAK_BAKU", content.SeverityReview},
		{"Harga: [ISI: Rp 50.000 per gram]", "PLACEHOLDER_BERNILAI", content.SeverityBlock},
		{"Harga tertinggi di kota.", "SUPERLATIF", content.SeverityBlock},
	}
	for _, c := range cases {
		d := clean()
		d.Body = c.text
		r := ev.Evaluate(d, src)
		if got := categories(r)[c.category]; got != c.severity {
			t.Errorf("%q: %s severity %q, want %q (findings %+v)", c.text, c.category, got, c.severity, r.Findings)
		}
		want := domain.DraftNeedsReview
		if c.severity == content.SeverityBlock {
			want = domain.DraftBlocked
		}
		if r.Status != want {
			t.Errorf("%q: status %s, want %s", c.text, r.Status, want)
		}
	}
	// Clean controls: legitimate Indonesian text, supported numbers and placeholders pass.
	for _, ok := range []string{
		"Kami akan menimbang dan menguji di depan Anda; dasar hitung akan ditampilkan.",
		"Petugas mencatat 12 pengunjung bertanya soal penimbangan.",
		"Harga per gram yang berlaku: [ISI: harga buyback terverifikasi].",
		"Hubungi [ISI: kontak resmi Punyamas] untuk pertanyaan.",
		"Pertama, minta penimbangan di depan Anda.\n1. Tanyakan kadar.\n2. Minta rincian dasar hitung.",
	} {
		d := clean()
		d.Body = ok
		r := ev.Evaluate(d, src)
		for _, f := range r.Findings {
			if f.Severity != content.SeverityInfo {
				t.Errorf("%q: unexpected finding %+v", ok, f)
			}
		}
	}
	// Numbers backed by a cited business fact pass.
	d := clean()
	d.Body = "Penawaran berlaku 10 menit."
	d.BrandFactIDs = []string{"FB8"}
	if r := ev.Evaluate(d, src); categories(r)["ANGKA"] != "" {
		t.Fatalf("supported number flagged: %+v", r.Findings)
	}
}

func TestPlaceholdersAreRecordedAndBlockReadiness(t *testing.T) {
	ev, _, src := setup(t)
	d := clean()
	d.Body = "Alamat: [ISI: alamat cabang]. Kontak: [isi:  kontak resmi   Punyamas ]. Lagi: [ISI: alamat cabang]."
	d.Placeholders = []string{"[ISI: alamat cabang]", "[ISI: jam buka]"}
	r := ev.Evaluate(d, src)
	if strings.Join(r.Placeholders, "|") != "[ISI: alamat cabang]|[ISI: kontak resmi Punyamas]" || r.Status != domain.DraftNeedsReview {
		t.Fatalf("placeholders=%v status=%s", r.Placeholders, r.Status)
	}
	if categories(r)["PLACEHOLDER_TIDAK_DIPAKAI"] != content.SeverityInfo || categories(r)["STATUS"] != "" {
		t.Fatalf("findings %+v", r.Findings)
	}
	n := 0
	for _, c := range r.Checklist {
		if strings.HasPrefix(c.Text, "Isi [ISI:") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("each placeholder needs a checklist item: %+v", r.Checklist)
	}
	// Even an approved version with placeholders is never ready.
	dr, _, _ := r.Record("draft_1", src, content.Provenance{RunID: "run_c", ExecutionID: "exec_1", Now: "t"})
	dr.Status = domain.DraftApproved
	reviews := []*domain.ApprovalRecord{{Decision: "approved", SubjectVersion: 1, SubjectHash: dr.ContentHash}}
	rd := content.ReadinessOf(dr, reviews)
	if rd.Ready || !strings.Contains(strings.Join(rd.Reasons, " "), "[ISI: alamat cabang]") {
		t.Fatalf("readiness %+v", rd)
	}
}

func TestReadiness(t *testing.T) {
	ev, _, src := setup(t)
	r := ev.Evaluate(clean(), src)
	dr, links, v := r.Record("draft_1", src, content.Provenance{RunID: "run_c", ExecutionID: "exec_1", ConfigHash: "c", ContentConfigHash: "cc", FixtureHash: "f", Now: "t"})
	if len(links) != 1 || links[0].ItemID != "it_a" || v.Version != 1 || v.Author != "agent" || dr.ScoringConfigHash != "sh" || dr.Status != domain.DraftDraft {
		t.Fatalf("record: %+v %+v %+v", dr, links, v)
	}
	if content.ReadinessOf(dr, nil).Ready {
		t.Fatal("an unreviewed draft is not ready")
	}
	dr.Status = domain.DraftApproved
	ok := []*domain.ApprovalRecord{{Decision: "approved", SubjectVersion: 1, SubjectHash: dr.ContentHash}}
	if rd := content.ReadinessOf(dr, ok); !rd.Ready {
		t.Fatalf("approved clean draft: %+v", rd)
	}
	if content.ReadinessOf(dr, []*domain.ApprovalRecord{{Decision: "approved", SubjectVersion: 2, SubjectHash: dr.ContentHash}}).Ready {
		t.Fatal("approval of another version must not count")
	}
	tampered := *dr
	tampered.Body += " tambahan"
	if content.ReadinessOf(&tampered, ok).Ready {
		t.Fatal("text that no longer matches its hash must not be ready")
	}
}

func TestDuplicatesWithinRunAndAgainstExistingDrafts(t *testing.T) {
	ev, _, src := setup(t)
	a, b, c := clean(), clean(), clean()
	b.Variant = 2
	b.Body = a.Body + " Terima kasih."
	c.Variant = 3
	c.Hook, c.Body = "Harga datang dengan dasar hitungnya.", "Setiap komponen dasar hitung akan ditampilkan sebelum Anda memutuskan."
	res := ev.EvaluateAll([]agent.Indexed{{Index: 1, Draft: a}, {Index: 2, Draft: b}, {Index: 3, Draft: c}}, src, nil)
	if res[0].DuplicateOf != "" || res[1].DuplicateOf != "usulan #1" || res[2].DuplicateOf != "" {
		t.Fatalf("within run: %q %q %q", res[0].DuplicateOf, res[1].DuplicateOf, res[2].DuplicateOf)
	}
	other := clean()
	other.Channel = "instagram"
	existing := []*domain.ContentDraft{{ID: "draft_lama", ContentType: "instagram_caption", Channel: "instagram", Hook: a.Hook, Body: a.Body, ContentHash: "x"}}
	res = ev.EvaluateAll([]agent.Indexed{{Index: 1, Draft: a}, {Index: 2, Draft: c}}, src, existing)
	if res[0].DuplicateOf != "draft_lama" || res[1].DuplicateOf != "" {
		t.Fatalf("against existing: %q %q", res[0].DuplicateOf, res[1].DuplicateOf)
	}
	edu := a
	edu.ContentType, edu.Channel = "education", "situs"
	if r := ev.EvaluateAll([]agent.Indexed{{Index: 1, Draft: a}, {Index: 2, Draft: edu}}, src, nil); r[1].DuplicateOf != "" {
		t.Fatal("same text in another type or channel is not a duplicate")
	}
}

// The Content Agent path must not be able to reach the network: no package on it imports net, net/http or os/exec.
func TestNoNetworkImports(t *testing.T) {
	root := testutil.RepoRoot(t)
	for _, dir := range []string{"internal/content", "internal/agent/content", "internal/claims", "internal/safepath", "internal/orchestrator", "internal/provider/mock", "internal/report", "cmd/mia"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, dir, e.Name()), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if p == "net" || strings.HasPrefix(p, "net/http") || p == "os/exec" || strings.Contains(p, "anthropic") {
					t.Errorf("%s/%s imports %s", dir, e.Name(), p)
				}
			}
		}
	}
}
