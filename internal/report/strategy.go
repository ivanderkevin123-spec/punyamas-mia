package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"sort"
	texttpl "text/template"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
)

type DimView struct {
	Key, Label string
	Value      int
	Rationale  string
	Source     string
}

type ApprovalView struct {
	Decision, By, Note, At string
}

type OppView struct {
	ID, Title, Segment, TopicKey, Need, Interpretation string
	Status, StatusLabel, Tier                          string
	Hypothesis, RequiresStore                          bool
	Dims                                               []DimView
	OrdinalSum                                         int
	Caps                                               []strategy.Cap
	Gates                                              []strategy.GateResult
	Conflicts                                          []strategy.ConflictView
	Brief                                              strategy.Brief
	MergedFrom                                         []int
	RejectReasons                                      []string
	EvidenceIDs                                        []string
	Approvals                                          []ApprovalView
	LatestDecision                                     string
	ConfigHash, ScoringHash, FixtureHash               string
	RunID, ParentRunID, CreatedAt                      string
}

type StrategyData struct {
	Title, BrandName, RunID, ParentRunID, Status, StatusLabel string
	Failed, Incomplete                                       bool
	CreatedAt, FinishedAt, Generated                         string
	ConfigHash, ScoringHash, FixtureHash, Provider           string
	Steps                                                    []domain.StepSummary
	Warnings, Errors                                         []string
	Counts                                                   domain.StrategyCounts
	TierA, TierB, TierC, Blocked, Rejected                   []OppView
	Execs                                                    []ExecView
	CostTotal, CostActual, CostUncertain                     string
	CostMeasurable                                           bool
	MonthlyCeiling                                           string
	NextSteps                                                []string
}

var oppStatusLabels = map[string]string{
	domain.OppProposed:         "usulan — menunggu tinjauan manusia",
	domain.OppBlocked:          "DIBLOKIR — gagal pemeriksa teks merek; tidak dapat disetujui",
	domain.OppRejected:         "DITOLAK — gagal gerbang bukti atau klaim faktual",
	domain.OppApproved:         "DISETUJUI oleh peninjau",
	domain.OppNeedsEvidence:    "perlu bukti tambahan (keputusan peninjau)",
	domain.OppRejectedByReview: "ditolak peninjau",
}

var dimOrder = []struct{ Key, Label string }{
	{"evidence_strength", "Kekuatan bukti"},
	{"customer_intent", "Niat pelanggan"},
	{"strategic_fit", "Kesesuaian strategis"},
	{"ease_of_testing", "Kemudahan pengujian"},
	{"measurability", "Keterukuran"},
}

// OppViewFor assembles the view of one stored opportunity, including its append-only approval history.
func (r *Renderer) OppViewFor(ctx context.Context, op *domain.Opportunity) (*OppView, error) {
	v := &OppView{ID: op.ID, Title: op.Title, Segment: op.Segment, TopicKey: op.TopicKey, Need: op.Need, Interpretation: op.Interpretation, Status: op.Status,
		StatusLabel: oppStatusLabels[op.Status], Tier: op.Tier, Hypothesis: op.Hypothesis, RequiresStore: op.RequiresStore, OrdinalSum: op.OrdinalSum,
		ConfigHash: op.ConfigHash, ScoringHash: op.ScoringConfigHash, FixtureHash: op.FixtureHash, RunID: op.RunID, ParentRunID: op.ParentRunID, CreatedAt: op.CreatedAt}
	if v.StatusLabel == "" {
		v.StatusLabel = op.Status
	}
	dims := map[string]strategy.Dim{}
	_ = json.Unmarshal([]byte(op.DimensionsJSON), &dims)
	for _, d := range dimOrder {
		x := dims[d.Key]
		v.Dims = append(v.Dims, DimView{Key: d.Key, Label: d.Label, Value: x.Value, Rationale: x.Rationale, Source: x.Source})
	}
	_ = json.Unmarshal([]byte(op.CapsJSON), &v.Caps)
	_ = json.Unmarshal([]byte(op.GatesJSON), &v.Gates)
	_ = json.Unmarshal([]byte(op.ConflictsJSON), &v.Conflicts)
	_ = json.Unmarshal([]byte(op.BriefJSON), &v.Brief)
	_ = json.Unmarshal([]byte(op.MergedFromJSON), &v.MergedFrom)
	_ = json.Unmarshal([]byte(op.RejectReasonsJSON), &v.RejectReasons)
	links, err := r.Store.EvidenceForOpportunity(ctx, op.ID)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		v.EvidenceIDs = append(v.EvidenceIDs, l.ItemID)
	}
	sort.Strings(v.EvidenceIDs)
	apps, err := r.Store.ApprovalsFor(ctx, "opportunity", op.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range apps {
		v.Approvals = append(v.Approvals, ApprovalView{Decision: a.Decision, By: a.DecidedBy, Note: a.Note, At: a.DecidedAt})
		v.LatestDecision = a.Decision
	}
	return v, nil
}

// BuildStrategy assembles the strategy report view model from the store.
func (r *Renderer) BuildStrategy(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (*StrategyData, error) {
	opps, err := r.Store.OpportunitiesForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	execs, err := r.Store.ExecutionsForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	d := &StrategyData{Title: "Laporan Strategi Pemasaran MIA", BrandName: r.Cfg.Brand.Name, RunID: run.ID, ParentRunID: run.ParentRunID, Status: sum.Status,
		StatusLabel: statusLabels[sum.Status], Failed: sum.Status == domain.RunFailed,
		Incomplete: sum.Status == domain.RunFailed || sum.Status == domain.RunPartial || sum.Status == domain.RunRunning,
		CreatedAt: run.CreatedAt, FinishedAt: run.FinishedAt, Generated: r.now().UTC().Format(time.RFC3339), ConfigHash: run.ConfigHash, ScoringHash: sum.ScoringConfigHash,
		FixtureHash: sum.FixtureHash, Provider: run.ProviderName, Steps: sum.Steps, Warnings: sum.Warnings, Errors: sum.Errors,
		CostTotal: fmt.Sprintf("USD %.4f", sum.CostUSD), CostActual: fmt.Sprintf("USD %.4f", sum.CostActualUSD), CostUncertain: fmt.Sprintf("USD %.4f", sum.CostUncertainUSD),
		CostMeasurable: sum.CostMeasurable, MonthlyCeiling: fmt.Sprintf("USD %.2f", r.Cfg.Limits.Caps.MonthlyCeilingUSD)}
	if d.StatusLabel == "" {
		d.StatusLabel = sum.Status
	}
	if sum.Strategy != nil {
		d.Counts = *sum.Strategy
	}
	for _, op := range opps {
		v, err := r.OppViewFor(ctx, op)
		if err != nil {
			return nil, err
		}
		switch {
		case op.Status == domain.OppBlocked:
			d.Blocked = append(d.Blocked, *v)
		case op.Status == domain.OppRejected:
			d.Rejected = append(d.Rejected, *v)
		case op.Tier == domain.TierA:
			d.TierA = append(d.TierA, *v)
		case op.Tier == domain.TierB:
			d.TierB = append(d.TierB, *v)
		default:
			d.TierC = append(d.TierC, *v)
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
	if len(d.TierA)+len(d.TierB)+len(d.TierC) > 0 {
		d.NextSteps = append(d.NextSteps, "Tinjau setiap peluang dan catat keputusan dengan: mia approve opportunity --id <id> --decision approved|rejected|needs_more_evidence --note \"...\".")
		d.NextSteps = append(d.NextSteps, "Brief peluang yang disetujui dapat diekspor untuk serah terima manual: mia export brief --id <id>.")
	}
	if d.Counts.Conflicted > 0 {
		d.NextSteps = append(d.NextSteps, "Selesaikan konflik bukti yang ditampilkan sebelum menaikkan peluang terkait dari tingkat C.")
	}
	if d.Counts.StoreDependent > 0 {
		d.NextSteps = append(d.NextSteps, "Peluang yang memerlukan toko atau kapabilitas operasional menunggu konfirmasi operasional; jangan dianggap fakta.")
	}
	if !d.CostMeasurable {
		d.NextSteps = append(d.NextSteps, "Sebagian biaya dibukukan sebagai estimasi tidak pasti; rekonsiliasi dengan tagihan penyedia.")
	}
	return d, nil
}

// RenderStrategy writes reports/<run-id>/strategy.md and strategy.html.
func (r *Renderer) RenderStrategy(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (string, string, error) {
	d, err := r.BuildStrategy(ctx, run, sum)
	if err != nil {
		return "", "", err
	}
	md, err := r.RenderStrategyMarkdown(d)
	if err != nil {
		return "", "", err
	}
	html, err := r.RenderStrategyHTML(d)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(r.Cfg.Paths.ReportsDir, run.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	mdPath := filepath.Join(dir, "strategy.md")
	htmlPath := filepath.Join(dir, "strategy.html")
	if err := os.WriteFile(mdPath, md, 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		return "", "", err
	}
	return relOrBase(mdPath, r.Cfg.Paths.Root), relOrBase(htmlPath, r.Cfg.Paths.Root), nil
}

func (r *Renderer) RenderStrategyMarkdown(d *StrategyData) ([]byte, error) {
	t, err := texttpl.New("strategy.md.tmpl").Funcs(mdFuncs).ParseFS(templateFS, "templates/strategy.md.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RenderStrategyHTML(d *StrategyData) ([]byte, error) {
	t, err := htmltpl.New("strategy.html.tmpl").ParseFS(templateFS, "templates/strategy.html.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BriefData wraps one approved opportunity for manual handoff.
type BriefData struct {
	Opp       *OppView
	BrandName string
	Generated string
}

// RenderBrief renders the Markdown handoff brief of one opportunity (escaped like reports).
func (r *Renderer) RenderBrief(v *OppView) ([]byte, error) {
	t, err := texttpl.New("brief.md.tmpl").Funcs(mdFuncs).ParseFS(templateFS, "templates/brief.md.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, BriefData{Opp: v, BrandName: r.Cfg.Brand.Name, Generated: r.now().UTC().Format(time.RFC3339)}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
