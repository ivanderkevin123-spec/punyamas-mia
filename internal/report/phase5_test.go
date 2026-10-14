package report

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/workflow"
)

const hostileRef = "x](javascript:alert(1)) [klik"

var codeSpanRe = regexp.MustCompile("`[^`\n]*`")

// linkOutsideCode reports a Markdown link or raw HTML that survives outside code spans.
func linkOutsideCode(s string) bool {
	plain := codeSpanRe.ReplaceAllString(s, "")
	return regexp.MustCompile(`(^|[^\\])\]\(javascript:`).MatchString(plain) || regexp.MustCompile(`(^|[^\\])<script>`).MatchString(plain)
}

// Regression for the Phase 2/3 ref escaping finding: refs from import file names are rendered in code spans.
func TestEvidenceRefsAreEscapedInResearchStrategyAndBrief(t *testing.T) {
	r, _, _ := seed(t)
	item := ItemView{Ref: hostileRef, Title: "Judul", Statement: "s", Kind: "fact", Quality: "A"}
	md, err := r.RenderMarkdown(&Data{Title: "t", BrandName: "Punyamas", Facts: []ItemView{item}, Hyps: []ItemView{item}, Unvalidated: []ItemView{item}, Rejected: []ItemView{item}, Dups: []ItemView{item}, Susp: []ItemView{item}})
	if err != nil {
		t.Fatal(err)
	}
	ev := strategy.BriefEvidence{ID: "it_1", Ref: hostileRef, Quality: "A", Statement: "s"}
	opp := OppView{ID: "opp_1", Title: "t", Brief: strategy.Brief{Facts: []strategy.BriefEvidence{ev}, Interpretations: []strategy.BriefEvidence{ev}, Hypotheses: []strategy.BriefEvidence{ev}},
		Conflicts: []strategy.ConflictView{{Group: "cg:x", Sides: []strategy.ConflictSide{{ID: "it_1", Ref: hostileRef, Statement: "s"}}}}}
	smd, err := r.RenderStrategyMarkdown(&StrategyData{Title: "t", BrandName: "Punyamas", TierA: []OppView{opp}})
	if err != nil {
		t.Fatal(err)
	}
	brief, err := r.RenderBrief(&opp)
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{"research": string(md), "strategy": string(smd), "brief": string(brief)} {
		if linkOutsideCode(s) {
			t.Errorf("%s: a hostile ref formed a link outside a code span", name)
		}
		if !strings.Contains(s, "`"+strings.ReplaceAll(hostileRef, "`", "'")+"`") {
			t.Errorf("%s: ref not rendered as a code span", name)
		}
	}
}

func hostileAssessment() *workflow.Assessment {
	wf := &domain.Workflow{ID: "wf_0001", Status: domain.WfAwaitingContentReview, InputsDir: "/tmp/x](javascript:alert(1))", FixtureKey: "<script>alert(1)</script>", CreatedAt: "t", UpdatedAt: "t"}
	ready := 1
	return &workflow.Assessment{Workflow: wf, Status: domain.WfAwaitingContentReview, Reason: "alasan <b>x</b> | kolom",
		Stages:        []workflow.StageView{{Name: "research", Label: "Riset", Status: "completed", Detail: "detail <img src=x>", Reasons: []string{"[klik](javascript:x)"}}},
		Opportunities: []workflow.OppRow{{ID: "opp_1", Title: "Judul | <script>alert(1)</script> **tebal**", Status: "approved", Tier: "A", LatestDecision: "approved", EvidenceIDs: []string{"it_1"}}},
		Drafts:        []workflow.DraftRow{{ID: "draft_1", OpportunityID: "opp_1", ContentType: "education", Status: "needs_review", Placeholders: []string{"[ISI: x](javascript:y)"}, ClaimsToVerify: []string{"klaim <b>"}}},
		Blockers:      []string{"blokir <script>"},
		NextActions:   []workflow.Action{{Text: "Tinjau peluang \"<b>x</b>\"", Command: "mia approve opportunity --id opp_1 `rm -rf`", Human: true}},
		Events:        []*domain.WorkflowEvent{{Seq: 1, Kind: "created", Detail: "catatan <script> | x", CreatedAt: "t"}},
		Counts:        workflow.Counts{DraftsReady: &ready}}
}

// (21) The workflow report escapes every untrusted value (titles, refs, paths, notes, agent text).
func TestWorkflowReportEscapesUntrustedValues(t *testing.T) {
	r, _, _ := seed(t)
	d := r.BuildWorkflow(hostileAssessment())
	md, err := r.RenderWorkflowMarkdown(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(md)
	if linkOutsideCode(s) {
		t.Error("markdown contains a link or script outside code spans")
	}
	for _, bad := range []string{`<b>`, `<img`, `\*\*tebal\*\*`} {
		if regexp.MustCompile(`(^|[^\\])` + bad).MatchString(s) {
			t.Errorf("unescaped %q", bad)
		}
	}
	if !strings.Contains(s, "tidak tersedia") || !strings.Contains(s, "DRAF — BELUM DIPUBLIKASIKAN") || !strings.Contains(s, "rekomendasi, belum dilakukan") {
		t.Error("report must mark unavailable counts, the DRAF banner and that actions are only recommendations")
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "| `wf_0001`") || strings.HasPrefix(line, "| `opp_1`") {
			if n := len(regexp.MustCompile(`(^|[^\\])\|`).FindAllString(line, -1)); n != 9 {
				t.Errorf("table row has %d unescaped pipes: %s", n, line)
			}
		}
	}
	html, err := r.RenderWorkflowHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	h := string(html)
	if strings.Contains(h, "<script>alert") || strings.Contains(h, "<img src=x>") || strings.Contains(h, "<b>x</b>") || !strings.Contains(h, "&lt;script&gt;") {
		t.Error("html must escape untrusted values")
	}
}
