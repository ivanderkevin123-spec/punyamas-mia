package report

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

const hostileName = "Sumber | jahat <b>bold</b>\nbaris dua"
const hostileStatement = "Baris satu\nbaris dua | kolom [klik](javascript:alert(1)) `kode` **tebal** <script>alert(1)</script> & tag <img src=x>"
const readable = "Barang Bapak atau Ibu ditimbang di depan Anda: berat bersih 12,40 g, kadar 749 permil (Rp per gram ditampilkan)."

func seed(t *testing.T) (*Renderer, *domain.Run, *domain.RunSummary) {
	t.Helper()
	root := testutil.RepoRoot(t)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	cfg.Paths.Root, cfg.Paths.ReportsDir = tmp, filepath.Join(tmp, "reports")
	st, err := store.Open(filepath.Join(tmp, "data", "mia.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	run := &domain.Run{ID: "run_1", Kind: "research", Status: domain.RunPartial, CreatedAt: "2026-10-12T10:00:00Z", ConfigHash: "h", ProviderName: "mock", InputsJSON: `{"inputs_dir":"x"}`, SummaryJSON: "{}"}
	if err := st.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	src := &domain.Source{ID: "src_1", SourceType: "manual_note", Name: hostileName, Reference: "imports/x.md", RetrievedAt: "2026-10-10", ContentHash: "h", Status: "ok", CreatedAt: "t"}
	_ = st.InsertSource(ctx, src)
	_ = st.LinkRunSource(ctx, run.ID, src.ID)
	mk := func(id, kind, status, title, statement string) {
		it := &domain.Item{ID: id, RunID: run.ID, Ref: id + "#1", Origin: "import", Kind: kind, Title: title, Statement: statement, Language: "id", Quality: "A", Status: status, ContentHash: id, CreatedAt: "t"}
		ev := &domain.Evidence{ID: "ev_" + id, SourceID: src.ID, Excerpt: statement, Locator: "bagian 1"}
		if err := st.InsertItemWithEvidence(ctx, run.ID, it, []*domain.Evidence{ev}); err != nil {
			t.Fatal(err)
		}
	}
	mk("validated_fact", domain.KindFact, domain.ItemValidated, "Judul <b>fakta</b> | tervalidasi", hostileStatement)
	mk("validated_hyp", domain.KindHypothesis, domain.ItemValidated, "Hipotesis biasa", readable)
	mk("unvalidated_fact", domain.KindFact, domain.ItemImported, "Fakta belum divalidasi", readable)
	mk("rejected_fact", domain.KindFact, domain.ItemRejected, "Fakta ditolak", readable)
	sum := &domain.RunSummary{RunID: run.ID, Kind: "research", Status: domain.RunPartial, Warnings: []string{"peringatan | dengan <i>tag</i>"}, Steps: []domain.StepSummary{{Name: "import", Status: "completed", Attempts: 1, Error: "err | <b>x</b>"}}}
	return &Renderer{Store: st, Cfg: cfg, Now: func() time.Time { return time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC) }}, run, sum
}

var unescapedPipe = regexp.MustCompile(`(^|[^\\])\|`)

func TestMarkdownEscapesHostileValues(t *testing.T) {
	r, run, sum := seed(t)
	d, err := r.Build(context.Background(), run, sum)
	if err != nil {
		t.Fatal(err)
	}
	md, err := r.RenderMarkdown(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(md)
	// Hostile fragments may appear only in escaped form (preceded by a backslash).
	for _, bad := range []string{`<script>`, `<b>`, `<img`, `\]\(javascript:`, "`kode`", `\*\*tebal\*\*`} {
		if re := regexp.MustCompile(`(^|[^\\])` + bad); re.MatchString(s) {
			t.Errorf("markdown still contains unescaped %q", bad)
		}
	}
	// Every table row of the sources table keeps exactly 6 unescaped pipes (5 cells).
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "| Sumber") || strings.Contains(line, "jahat") && strings.HasPrefix(line, "|") {
			if n := len(unescapedPipe.FindAllString(line, -1)); n != 6 {
				t.Errorf("table row has %d unescaped pipes, want 6: %s", n, line)
			}
		}
	}
	if strings.Contains(s, "\nbaris dua") {
		t.Error("newline inside a value broke the structure")
	}
	if !strings.Contains(s, readable) {
		t.Errorf("legitimate Indonesian text was altered; got:\n%s", s)
	}
}

func TestMarkdownSectionsOnlyShowValidatedItems(t *testing.T) {
	r, run, sum := seed(t)
	d, err := r.Build(context.Background(), run, sum)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Facts) != 1 || len(d.Hyps) != 1 || len(d.Unvalidated) != 1 || len(d.Rejected) != 1 {
		t.Fatalf("grouping: facts=%d hyps=%d unvalidated=%d rejected=%d", len(d.Facts), len(d.Hyps), len(d.Unvalidated), len(d.Rejected))
	}
	md, _ := r.RenderMarkdown(d)
	s := string(md)
	for _, want := range []string{"RUN BELUM LENGKAP", "Fakta bersumber (1)", "Hipotesis (1)", "Belum divalidasi (1)", "status: belum divalidasi", "Ditolak (1)", "Sebagian (dihentikan sesuai permintaan)"} {
		if !strings.Contains(s, want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
	idxFacts := strings.Index(s, "## 3. Fakta bersumber")
	idxUnval := strings.Index(s, "Fakta belum divalidasi")
	idxSec7 := strings.Index(s, "## 7. Belum divalidasi")
	if idxUnval < idxSec7 || idxUnval < idxFacts {
		t.Error("unvalidated item appears before section 7")
	}
}

func TestHTMLEscapesAndSeparatesStatuses(t *testing.T) {
	r, run, sum := seed(t)
	d, _ := r.Build(context.Background(), run, sum)
	html, err := r.RenderHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if strings.Contains(s, "<script>alert") || !strings.Contains(s, "&lt;script&gt;") {
		t.Error("HTML not escaped")
	}
	if !strings.Contains(s, "Belum divalidasi (1)") || !strings.Contains(s, "RUN BELUM LENGKAP") {
		t.Error("HTML lacks the unvalidated section or incomplete banner")
	}
}

func TestMDHelpers(t *testing.T) {
	if got := MDText("a|b\nc_d"); got != `a\|b c\_d` {
		t.Fatalf("MDText = %q", got)
	}
	if got := MDCode("x`y\nz"); got != "x'y z" {
		t.Fatalf("MDCode = %q", got)
	}
	if got := MDText(readable); got != readable {
		t.Fatalf("readable text changed: %q", got)
	}
}
