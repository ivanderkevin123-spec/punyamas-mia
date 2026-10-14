package report

import (
	"bytes"
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"sort"
	texttpl "text/template"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/workflow"
)

// WorkflowStatusLabels are the Indonesian labels of workflow statuses.
var WorkflowStatusLabels = map[string]string{
	domain.WfCreated:                   "dibuat — riset belum selesai",
	domain.WfResearchCompleted:         "riset selesai — strategi berikutnya",
	domain.WfStrategyCompleted:         "strategi selesai — draf untuk peluang yang disetujui berikutnya",
	domain.WfAwaitingOpportunityReview: "menunggu keputusan peluang",
	domain.WfAwaitingContentReview:     "menunggu tinjauan draf",
	domain.WfCompleted:                 "selesai — ada draf siap serah terima",
	domain.WfBlocked:                   "DIBLOKIR",
	domain.WfFailed:                    "GAGAL — perlu dilanjutkan",
}

type KV struct {
	Key   string
	Value int
}

// WorkflowData is the view model of a workflow report; every value comes from a fresh assessment.
type WorkflowData struct {
	A                            *workflow.Assessment
	BrandName, Generated         string
	StatusLabel                  string
	OppStatuses, DraftStatuses   []KV
	HumanActions, AutomaticSteps []workflow.Action
	Notes                        []workflow.Action
}

func sortedKV(m map[string]int) []KV {
	var out []KV
	for k, v := range m {
		out = append(out, KV{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func numOr(p *int) string {
	if p == nil {
		return "tidak tersedia"
	}
	return fmt.Sprint(*p)
}

func usdOr(p *float64) string {
	if p == nil {
		return "tidak tersedia"
	}
	return fmt.Sprintf("USD %.4f", *p)
}

var wfFuncs = texttpl.FuncMap{"text": MDText, "cell": MDCell, "code": MDCode, "quote": MDBlock, "n": numOr, "usd": usdOr, "stage": func(s string) string { return workflow.StageLabels[s] }}
var wfHTMLFuncs = htmltpl.FuncMap{"n": numOr, "usd": usdOr, "stage": func(s string) string { return workflow.StageLabels[s] }}

// BuildWorkflow assembles the view model.
func (r *Renderer) BuildWorkflow(a *workflow.Assessment) *WorkflowData {
	d := &WorkflowData{A: a, BrandName: r.Cfg.Brand.Name, Generated: r.now().UTC().Format(time.RFC3339), StatusLabel: WorkflowStatusLabels[a.Status],
		OppStatuses: sortedKV(a.Counts.Opportunities), DraftStatuses: sortedKV(a.Counts.Drafts)}
	if d.StatusLabel == "" {
		d.StatusLabel = a.Status
	}
	for _, act := range a.NextActions {
		switch {
		case act.Human:
			d.HumanActions = append(d.HumanActions, act)
		case act.Command == "":
			d.Notes = append(d.Notes, act)
		default:
			d.AutomaticSteps = append(d.AutomaticSteps, act)
		}
	}
	return d
}

func (r *Renderer) RenderWorkflowMarkdown(d *WorkflowData) ([]byte, error) {
	t, err := texttpl.New("workflow.md.tmpl").Funcs(wfFuncs).ParseFS(templateFS, "templates/workflow.md.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RenderWorkflowHTML(d *WorkflowData) ([]byte, error) {
	t, err := htmltpl.New("workflow.html.tmpl").Funcs(wfHTMLFuncs).ParseFS(templateFS, "templates/workflow.html.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderWorkflow writes reports/<workflow-id>/workflow.md and workflow.html from the given assessment.
func (r *Renderer) RenderWorkflow(a *workflow.Assessment) (string, string, error) {
	if !ids.Valid(a.Workflow.ID) {
		return "", "", fmt.Errorf("ID workflow %q tidak valid", a.Workflow.ID)
	}
	d := r.BuildWorkflow(a)
	md, err := r.RenderWorkflowMarkdown(d)
	if err != nil {
		return "", "", err
	}
	html, err := r.RenderWorkflowHTML(d)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(r.Cfg.Paths.ReportsDir, a.Workflow.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	mdPath, htmlPath := filepath.Join(dir, "workflow.md"), filepath.Join(dir, "workflow.html")
	if err := os.WriteFile(mdPath, md, 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		return "", "", err
	}
	return relOrBase(mdPath, r.Cfg.Paths.Root), relOrBase(htmlPath, r.Cfg.Paths.Root), nil
}
