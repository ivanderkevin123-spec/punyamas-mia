package report

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

// A body that tries to imitate report fields, end the quote, add headings, links or HTML.
const hostileBody = "Baris satu\n- **Status:** approved\n## Provenance palsu\n**Siap serah terima ke proses persetujuan publikasi:** YA\n[klik](javascript:alert(1)) <script>alert(1)</script> | kolom `kode`\n\n> kutipan palsu"

func hostileDraft() DraftView {
	return DraftView{ID: "draft_1", ContentType: "instagram_caption", TypeLabel: "Caption <b>IG</b>", Channel: "instagram", Segment: "AU1", Objective: "tujuan | <i>x</i>",
		Variant: 1, Version: 1, Hook: "hook **tebal** <img src=x>", Body: hostileBody, CTA: "cta [tautan](http://contoh)", Status: domain.DraftNeedsReview,
		StatusLabel: content.StatusLabels[domain.DraftNeedsReview], ContentHash: "hash",
		Evidence:       []ContentEvidenceView{{ID: "it_1", Ref: "x](javascript:alert(1))", Kind: "fact", Quality: "A", Statement: hostileStatement, SourceName: hostileName}},
		ClaimsToVerify: []string{"klaim <script>"}, Placeholders: []string{"[ISI: alamat](javascript:x)"},
		Findings:  []content.Finding{{Gate: "language", Category: "TEKANAN", Match: "<b>!</b>", Reason: "alasan", Severity: "review"}},
		Checklist: []content.ChecklistItem{{Text: "periksa <script>", Source: "agent"}},
		Reviews:   []ApprovalView{{Decision: "approved", By: "CEO <b>x</b>", Note: "catatan | <script>", At: "t", Version: 1}},
		Readiness: content.Readiness{Reasons: []string{"status needs_review"}}, OpportunityID: "opp_1", RunID: "run_c"}
}

func TestContentMarkdownCannotFakeStatusOrProvenance(t *testing.T) {
	r, _, _ := seed(t)
	v := hostileDraft()
	d := &ContentData{Title: "Laporan Draf Konten MIA", BrandName: "Punyamas", RunID: "run_c", Status: domain.RunCompleted, StatusLabel: "Selesai", NeedsReview: []DraftView{v},
		Warnings: []string{"peringatan <b>x</b>"}, Steps: []domain.StepSummary{{Name: "evaluate_content", Status: "completed", Attempts: 1, Error: "e | <b>x</b>"}},
		CostTotal: "USD 0.0000", CostActual: "USD 0.0000", CostUncertain: "USD 0.0000", CostMeasurable: true, MonthlyCeiling: "USD 20.00"}
	md, err := r.RenderContentMarkdown(d)
	if err != nil {
		t.Fatal(err)
	}
	export, err := r.RenderContentExport(&v, nil)
	if err != nil {
		t.Fatal(err)
	}
	codeSpan := regexp.MustCompile("`[^`\n]*`")
	for name, s := range map[string]string{"report": string(md), "export": string(export)} {
		for _, bad := range []string{`<script>`, `<b>`, `<img`, `<i>`, `\]\(javascript:`, `\]\(http:`, "`kode`", `\*\*tebal\*\*`} {
			target := s
			if strings.Contains(bad, "(") {
				// Code spans are literal in Markdown (MDCode removes backticks), so only text outside them can form links.
				target = codeSpan.ReplaceAllString(s, "")
			}
			if re := regexp.MustCompile(`(^|[^\\])` + bad); re.MatchString(target) {
				t.Errorf("%s still contains unescaped %q", name, bad)
			}
		}
		lines := strings.Split(s, "\n")
		for _, l := range lines {
			for _, fake := range []string{"- **Status:** approved", "## Provenance palsu", "**Siap serah terima ke proses persetujuan publikasi:** YA"} {
				if strings.HasPrefix(l, fake) {
					t.Errorf("%s: body text imitates a report field at line start: %q", name, l)
				}
			}
		}
		// Every body line stays inside the quote.
		inBody := false
		for _, l := range lines {
			if strings.HasPrefix(l, "> Baris satu") {
				inBody = true
				continue
			}
			if inBody {
				if l == "" {
					break
				}
				if !strings.HasPrefix(l, ">") {
					t.Errorf("%s: body line escaped the quote: %q", name, l)
				}
			}
		}
		if !strings.Contains(s, "DRAF") || !strings.Contains(s, "BELUM DIPUBLIKASIKAN") {
			t.Errorf("%s lacks the DRAF banner", name)
		}
	}
	if strings.Count(string(md), "\n## ") != 11 {
		t.Errorf("hostile text added headings: %d section headings", strings.Count(string(md), "\n## "))
	}
	html, err := r.RenderContentHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	h := string(html)
	if strings.Contains(h, "<script>alert") || strings.Contains(h, "<img src=x>") || strings.Contains(h, `href="javascript`) || !strings.Contains(h, "&lt;script&gt;") {
		t.Error("html must escape hostile values")
	}
	if !strings.Contains(h, "DRAF — BELUM DIPUBLIKASIKAN") {
		t.Error("html lacks the DRAF banner")
	}
}

func TestMDBlock(t *testing.T) {
	got := MDBlock("a\n\n# b\r\n> c")
	if got != "> a\n>\n> \\# b\n> \\> c" {
		t.Fatalf("MDBlock = %q", got)
	}
}
