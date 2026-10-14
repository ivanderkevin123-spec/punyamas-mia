package report

import (
	"bytes"
	"context"
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	texttpl "text/template"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
)

type ContentEvidenceView struct {
	ID, Ref, Kind, Quality, Statement, SourceName string
}

type BrandFactView struct {
	ID, Status, Text string
}

type VersionView struct {
	Version                                           int
	Author, Note, StatusAfter, CreatedAt, ContentHash string
}

// DraftView is one stored draft as reports, the CLI and exports show it.
type DraftView struct {
	ID, ContentType, TypeLabel, Channel, Segment, Objective string
	Variant, Version                                        int
	Hook, Headline, Body, CTA                               string
	Status, StatusLabel, ContentHash                        string
	Evidence                                                []ContentEvidenceView
	BrandFacts                                              []BrandFactView
	SupportedClaims, OtherClaims                            []content.ClaimView
	ClaimsToVerify, Placeholders, Assumptions, Risks        []string
	Checklist                                               []content.ChecklistItem
	Findings                                                []content.Finding
	Versions                                                []VersionView
	Reviews                                                 []ApprovalView
	Readiness                                               content.Readiness
	OpportunityID, RunID, StrategyRunID, ResearchRunID      string
	ExecutionID, ConfigHash, ContentConfigHash              string
	ScoringConfigHash, FixtureHash, CreatedAt, UpdatedAt    string
}

type ContentData struct {
	Title, BrandName, RunID, StrategyRunID, ResearchRunID, OpportunityID string
	Status, StatusLabel                                                  string
	Failed, Incomplete                                                   bool
	CreatedAt, FinishedAt, Generated                                     string
	ConfigHash, ContentConfigHash, FixtureHash, Provider                 string
	Steps                                                                []domain.StepSummary
	Warnings, Errors                                                     []string
	Counts                                                               domain.ContentCounts
	Opportunity                                                          *OppView
	NeedsReview, Draft, Approved, Rejected, Blocked                      []DraftView
	Execs                                                                []ExecView
	CostTotal, CostActual, CostUncertain                                 string
	CostMeasurable                                                       bool
	MonthlyCeiling                                                       string
	NextSteps                                                            []string
}

// ExportData wraps one draft for the manual-handoff export file.
type ExportData struct {
	Draft       DraftView
	Opportunity *OppView
	BrandName   string
	Generated   string
}

// DraftViewFor assembles the view of one stored draft, with evidence, versions, reviews and readiness.
func (r *Renderer) DraftViewFor(ctx context.Context, d *domain.ContentDraft) (*DraftView, error) {
	dt := content.DecodeDetails(d)
	v := &DraftView{ID: d.ID, ContentType: d.ContentType, TypeLabel: r.Cfg.Content.Types[d.ContentType].Label, Channel: d.Channel, Segment: d.TargetSegment, Objective: d.Objective,
		Variant: d.Variant, Version: d.Version, Hook: d.Hook, Headline: d.Headline, Body: d.Body, CTA: d.CallToAction, Status: d.Status, StatusLabel: content.StatusLabels[d.Status],
		ContentHash: d.ContentHash, ClaimsToVerify: dt.ClaimsToVerify(), Placeholders: dt.Placeholders, Assumptions: dt.Assumptions, Risks: dt.Risks, Checklist: dt.Checklist,
		Findings: dt.Findings, OpportunityID: d.OpportunityID, RunID: d.RunID, StrategyRunID: d.StrategyRunID, ResearchRunID: d.ResearchRunID, ExecutionID: d.ExecutionID,
		ConfigHash: d.ConfigHash, ContentConfigHash: d.ContentConfigHash, ScoringConfigHash: d.ScoringConfigHash, FixtureHash: d.FixtureHash, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
	if v.TypeLabel == "" {
		v.TypeLabel = d.ContentType
	}
	if v.StatusLabel == "" {
		v.StatusLabel = d.Status
	}
	for _, c := range dt.Claims {
		if c.Kind == "fact" && c.Supported {
			v.SupportedClaims = append(v.SupportedClaims, c)
		} else if c.Kind != "fact" {
			v.OtherClaims = append(v.OtherClaims, c)
		}
	}
	_, byRef, err := strategy.LoadEvidence(ctx, r.Store, d.ResearchRunID)
	if err != nil {
		return nil, err
	}
	byID := map[string]strategy.Evidence{}
	for _, e := range byRef {
		byID[e.ID] = e
	}
	links, err := r.Store.ContentEvidenceFor(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		e, ok := byID[l.ItemID]
		if !ok {
			v.Evidence = append(v.Evidence, ContentEvidenceView{ID: l.ItemID, Kind: l.Role, Statement: "butir ini tidak lagi tervalidasi di run riset induk"})
			continue
		}
		v.Evidence = append(v.Evidence, ContentEvidenceView{ID: e.ID, Ref: e.Ref, Kind: e.Kind, Quality: e.Quality, Statement: e.Statement, SourceName: e.SourceName})
	}
	facts := map[string]BrandFactView{}
	for _, f := range r.Cfg.Business.Facts {
		facts[f.ID] = BrandFactView{ID: f.ID, Status: f.Status, Text: f.Text}
	}
	for _, id := range dt.BrandFactIDs {
		if f, ok := facts[id]; ok {
			v.BrandFacts = append(v.BrandFacts, f)
		}
	}
	versions, err := r.Store.ContentDraftVersions(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	for _, x := range versions {
		v.Versions = append(v.Versions, VersionView{Version: x.Version, Author: x.Author, Note: x.Note, StatusAfter: x.StatusAfter, CreatedAt: x.CreatedAt, ContentHash: x.ContentHash})
	}
	reviews, err := r.Store.ApprovalsFor(ctx, domain.SubjectContentDraft, d.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range reviews {
		v.Reviews = append(v.Reviews, ApprovalView{Decision: a.Decision, By: a.DecidedBy, Note: a.Note, At: a.DecidedAt, Version: a.SubjectVersion})
	}
	v.Readiness = content.ReadinessOf(d, reviews)
	return v, nil
}

// BuildContent assembles the content report view model from the store.
func (r *Renderer) BuildContent(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (*ContentData, error) {
	drafts, err := r.Store.ContentDraftsForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	execs, err := r.Store.ExecutionsForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	d := &ContentData{Title: "Laporan Draf Konten MIA", BrandName: r.Cfg.Brand.Name, RunID: run.ID, StrategyRunID: run.ParentRunID, OpportunityID: sum.OpportunityID,
		Status: sum.Status, StatusLabel: statusLabels[sum.Status], Failed: sum.Status == domain.RunFailed,
		Incomplete: sum.Status == domain.RunFailed || sum.Status == domain.RunPartial || sum.Status == domain.RunRunning,
		CreatedAt:  run.CreatedAt, FinishedAt: run.FinishedAt, Generated: r.now().UTC().Format(time.RFC3339), ConfigHash: run.ConfigHash, ContentConfigHash: sum.ContentConfigHash,
		FixtureHash: sum.FixtureHash, Provider: run.ProviderName, Steps: sum.Steps, Warnings: sum.Warnings, Errors: sum.Errors,
		CostTotal: fmt.Sprintf("USD %.4f", sum.CostUSD), CostActual: fmt.Sprintf("USD %.4f", sum.CostActualUSD), CostUncertain: fmt.Sprintf("USD %.4f", sum.CostUncertainUSD),
		CostMeasurable: sum.CostMeasurable, MonthlyCeiling: fmt.Sprintf("USD %.2f", r.Cfg.Limits.Caps.MonthlyCeilingUSD)}
	if d.StatusLabel == "" {
		d.StatusLabel = sum.Status
	}
	if sum.Content != nil {
		d.Counts = *sum.Content
	}
	if d.OpportunityID != "" {
		if op, err := r.Store.GetOpportunity(ctx, d.OpportunityID); err == nil {
			ov, err := r.OppViewFor(ctx, op)
			if err != nil {
				return nil, err
			}
			d.Opportunity = ov
			d.ResearchRunID = op.ParentRunID
		}
	}
	for _, dr := range drafts {
		v, err := r.DraftViewFor(ctx, dr)
		if err != nil {
			return nil, err
		}
		switch dr.Status {
		case domain.DraftNeedsReview:
			d.NeedsReview = append(d.NeedsReview, *v)
		case domain.DraftApproved:
			d.Approved = append(d.Approved, *v)
		case domain.DraftRejected:
			d.Rejected = append(d.Rejected, *v)
		case domain.DraftBlocked:
			d.Blocked = append(d.Blocked, *v)
		default:
			d.Draft = append(d.Draft, *v)
		}
	}
	// Status, placeholder, claim and evidence counts always come from the stored drafts, so a re-rendered
	// report reflects reviews and revisions made after the run; only the agent-output counters
	// (proposed, structurally rejected, duplicates) come from the run summary.
	d.Counts.Stored, d.Counts.Draft, d.Counts.NeedsReview = len(drafts), len(d.Draft), len(d.NeedsReview)
	d.Counts.Approved, d.Counts.Rejected, d.Counts.Blocked = len(d.Approved), len(d.Rejected), len(d.Blocked)
	d.Counts.Placeholders, d.Counts.ClaimsToVerify, d.Counts.EvidenceLinks = 0, 0, 0
	for _, group := range [][]DraftView{d.NeedsReview, d.Draft, d.Approved, d.Rejected, d.Blocked} {
		for _, v := range group {
			d.Counts.Placeholders += len(v.Placeholders)
			d.Counts.ClaimsToVerify += len(v.ClaimsToVerify)
			d.Counts.EvidenceLinks += len(v.Evidence)
		}
	}
	for _, e := range execs {
		ev := ExecView{ID: e.ID, Agent: e.AgentName, Provider: e.ProviderName, Model: e.Model, Outcome: e.Outcome, InputTokens: e.InputTokens, OutputTokens: e.OutputTokens, Measurable: e.CostMeasurable, Error: e.ErrorText, Cost: "tidak terukur"}
		if e.CostUSD != nil {
			ev.Cost = fmt.Sprintf("USD %.4f", *e.CostUSD)
			if !e.CostMeasurable {
				ev.Cost += " (estimasi)"
			}
		}
		d.Execs = append(d.Execs, ev)
	}
	if d.Failed {
		d.NextSteps = append(d.NextSteps, "Perbaiki penyebab kegagalan lalu jalankan ulang dengan --resume "+run.ID+".")
	}
	if len(d.NeedsReview)+len(d.Draft) > 0 {
		d.NextSteps = append(d.NextSteps, "Tinjau setiap draf dan catat keputusan dengan: mia content review --id <id> --decision approved|rejected --note \"...\".")
	}
	if d.Counts.Placeholders > 0 || d.Counts.ClaimsToVerify > 0 {
		d.NextSteps = append(d.NextSteps, "Isi setiap [ISI: ...] dan verifikasi klaim yang ditandai melalui mia content revise; revisi mengembalikan draf ke tinjauan.")
	}
	if len(d.Blocked) > 0 {
		d.NextSteps = append(d.NextSteps, "Draf yang diblokir tidak dapat ditinjau; revisi teksnya atau buat ulang dengan --regenerate.")
	}
	d.NextSteps = append(d.NextSteps, "Ekspor draf untuk serah terima manual dengan mia content export --id <id>; MIA tidak menerbitkan atau mengirim apa pun.")
	return d, nil
}

func (r *Renderer) contentText(name string) (*texttpl.Template, error) {
	return texttpl.New(name).Funcs(mdFuncs).ParseFS(templateFS, "templates/"+name, "templates/content_draft.md.tmpl")
}

// RenderContent writes reports/<run-id>/content.md and content.html.
func (r *Renderer) RenderContent(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (string, string, error) {
	d, err := r.BuildContent(ctx, run, sum)
	if err != nil {
		return "", "", err
	}
	md, err := r.RenderContentMarkdown(d)
	if err != nil {
		return "", "", err
	}
	html, err := r.RenderContentHTML(d)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(r.Cfg.Paths.ReportsDir, run.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	mdPath, htmlPath := filepath.Join(dir, "content.md"), filepath.Join(dir, "content.html")
	if err := os.WriteFile(mdPath, md, 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		return "", "", err
	}
	return relOrBase(mdPath, r.Cfg.Paths.Root), relOrBase(htmlPath, r.Cfg.Paths.Root), nil
}

func (r *Renderer) RenderContentMarkdown(d *ContentData) ([]byte, error) {
	t, err := r.contentText("content.md.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RenderContentHTML(d *ContentData) ([]byte, error) {
	t, err := htmltpl.New("content.html.tmpl").ParseFS(templateFS, "templates/content.html.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderContentExport renders the Markdown export of one draft for manual handoff (always labelled DRAF).
func (r *Renderer) RenderContentExport(v *DraftView, opp *OppView) ([]byte, error) {
	t, err := r.contentText("content_export.md.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ExportData{Draft: *v, Opportunity: opp, BrandName: r.Cfg.Brand.Name, Generated: r.now().UTC().Format(time.RFC3339)}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
