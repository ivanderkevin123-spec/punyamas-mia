// Package report renders Markdown and static HTML reports from validated
// store rows only. Nothing is rendered from raw model text.
package report

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	texttpl "text/template"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

type Renderer struct {
	Store *store.Store
	Cfg   *config.Config
	Now   func() time.Time
}

type SourceView struct {
	Name, Reference, Type, RetrievedAt, PublishedAt, Publisher string
}

type ItemView struct {
	Ref, Title, Statement, Kind, Quality, TopicKey, Status, Rationale string
	Source                                                            *SourceView
	Excerpt, Locator                                                  string
	Cites                                                             []string
	Claims                                                            []string
	Errors                                                            []string
	Suspicious                                                        bool
	DuplicateOf                                                       string
}

type ExecView struct {
	ID, Agent, Provider, Model, Outcome string
	InputTokens, OutputTokens            int
	Cost                                 string
	Measurable                           bool
	Error                                string
}

type Data struct {
	Title          string
	BrandName      string
	RunID          string
	Kind           string
	Status         string
	StatusLabel    string
	Failed         bool
	CreatedAt      string
	FinishedAt     string
	Generated      string
	ConfigHash     string
	Provider       string
	Inputs         string
	Steps          []domain.StepSummary
	Counts         domain.Counts
	Warnings       []string
	Errors         []string
	Drift          []domain.DriftEntry
	Facts          []ItemView
	Interps        []ItemView
	Hyps           []ItemView
	Rejected       []ItemView
	Dups           []ItemView
	Susp           []ItemView
	Sources        []SourceView
	Execs          []ExecView
	CostTotal      string
	CostMeasurable bool
	MonthlyCeiling string
	NextSteps      []string
}

var statusLabels = map[string]string{
	domain.RunCompleted:             "Selesai",
	domain.RunCompletedWithWarnings: "Selesai dengan peringatan",
	domain.RunPartial:               "Sebagian (dihentikan sesuai permintaan)",
	domain.RunFailed:                "GAGAL / TIDAK LENGKAP",
	domain.RunRunning:               "Masih berjalan",
}

// Build assembles the view model from the store.
func (r *Renderer) Build(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (*Data, error) {
	items, err := r.Store.ItemsForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	evid, err := r.Store.EvidenceForItems(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	sources, err := r.Store.SourcesForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	execs, err := r.Store.ExecutionsForRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	srcByID := map[string]*domain.Source{}
	for _, s := range sources {
		srcByID[s.ID] = s
	}
	itemByID := map[string]*domain.Item{}
	for _, it := range items {
		itemByID[it.ID] = it
	}
	var in struct {
		InputsDir string `json:"inputs_dir"`
	}
	_ = json.Unmarshal([]byte(run.InputsJSON), &in)
	d := &Data{
		Title: "Laporan Riset Pasar MIA", BrandName: r.Cfg.Brand.Name, RunID: run.ID, Kind: run.Kind, Status: sum.Status,
		StatusLabel: statusLabels[sum.Status], Failed: sum.Status == domain.RunFailed, CreatedAt: run.CreatedAt, FinishedAt: run.FinishedAt,
		Generated: r.now().UTC().Format(time.RFC3339), ConfigHash: run.ConfigHash, Provider: run.ProviderName, Inputs: relOrBase(in.InputsDir, r.Cfg.Paths.Root),
		Steps: sum.Steps, Counts: sum.Counts, Warnings: sum.Warnings, Errors: sum.Errors, Drift: sum.Drift,
		CostTotal: fmt.Sprintf("USD %.4f", sum.CostUSD), CostMeasurable: sum.CostMeasurable, MonthlyCeiling: fmt.Sprintf("USD %.2f", r.Cfg.Limits.Caps.MonthlyCeilingUSD),
	}
	if d.StatusLabel == "" {
		d.StatusLabel = sum.Status
	}
	for _, it := range items {
		v := ItemView{Ref: it.Ref, Title: it.Title, Statement: it.Statement, Kind: it.Kind, Quality: it.Quality, TopicKey: it.TopicKey, Status: it.Status, Rationale: it.Rationale, Suspicious: it.Suspicious}
		if it.DuplicateOf != "" {
			if t := itemByID[it.DuplicateOf]; t != nil {
				v.DuplicateOf = t.Ref
			}
		}
		for _, e := range evid[it.ID] {
			if e.SourceID != "" && v.Source == nil {
				if s := srcByID[e.SourceID]; s != nil {
					v.Source = &SourceView{Name: s.Name, Reference: s.Reference, Type: s.SourceType, RetrievedAt: s.RetrievedAt, PublishedAt: s.PublishedAt, Publisher: s.Publisher}
				}
				v.Excerpt, v.Locator = e.Excerpt, e.Locator
			}
			if e.FromItemID != "" {
				if t := itemByID[e.FromItemID]; t != nil {
					v.Cites = append(v.Cites, t.Ref)
				}
			}
		}
		sort.Strings(v.Cites)
		if it.ClaimCheckJSON != "" {
			var cc struct {
				Claims []struct {
					Category, Match string
				} `json:"claims"`
				ValidationErrors []string `json:"validation_errors"`
			}
			if json.Unmarshal([]byte(it.ClaimCheckJSON), &cc) == nil {
				for _, c := range cc.Claims {
					v.Claims = append(v.Claims, fmt.Sprintf("%s (%q)", c.Category, c.Match))
				}
				v.Errors = cc.ValidationErrors
			}
		}
		switch {
		case it.Status == domain.ItemDuplicate:
			d.Dups = append(d.Dups, v)
		case it.Suspicious:
			d.Susp = append(d.Susp, v)
		case it.Status == domain.ItemRejected:
			d.Rejected = append(d.Rejected, v)
		case it.Kind == domain.KindFact:
			d.Facts = append(d.Facts, v)
		case it.Kind == domain.KindInterpretation:
			d.Interps = append(d.Interps, v)
		default:
			d.Hyps = append(d.Hyps, v)
		}
	}
	for _, s := range sources {
		d.Sources = append(d.Sources, SourceView{Name: s.Name, Reference: s.Reference, Type: s.SourceType, RetrievedAt: s.RetrievedAt, PublishedAt: s.PublishedAt, Publisher: s.Publisher})
	}
	for _, e := range execs {
		ev := ExecView{ID: e.ID, Agent: e.AgentName, Provider: e.ProviderName, Model: e.Model, Outcome: e.Outcome, InputTokens: e.InputTokens, OutputTokens: e.OutputTokens, Measurable: e.CostMeasurable, Error: e.ErrorText, Cost: "tidak terukur"}
		if e.CostUSD != nil {
			ev.Cost = fmt.Sprintf("USD %.4f", *e.CostUSD)
		}
		d.Execs = append(d.Execs, ev)
	}
	d.NextSteps = nextSteps(d)
	return d, nil
}

func nextSteps(d *Data) []string {
	var out []string
	if d.Failed {
		out = append(out, "Perbaiki penyebab kegagalan pada langkah yang gagal, lalu jalankan ulang dengan --resume "+d.RunID+".")
	}
	if d.Counts.FilesRejected > 0 {
		out = append(out, "Perbaiki berkas impor yang ditolak (lihat peringatan) dan jalankan ulang.")
	}
	for _, dr := range d.Drift {
		if dr.Status == "changed" || dr.Status == "review_required" {
			out = append(out, fmt.Sprintf("Tinjau perubahan aset ruang kerja %q dan selaraskan konfigurasi (mia approve asset --key %s).", dr.Key, dr.Key))
		}
	}
	if len(d.Hyps) > 0 {
		out = append(out, "Hipotesis belum berbukti eksternal; kumpulkan bukti (catatan lapangan, sumber publik yang disetujui) sebelum dipakai dalam keputusan.")
	}
	if len(d.Interps) > 0 {
		out = append(out, "Tinjau interpretasi model terhadap butir yang dikutip; interpretasi bukan fakta.")
	}
	if len(out) == 0 {
		out = append(out, "Lanjutkan ke pemeringkatan peluang (Agen Strategi, Fase 3) setelah temuan ditinjau manusia.")
	}
	return out
}

func relOrBase(p, root string) string {
	if p == "" {
		return ""
	}
	if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filepath.Base(p)
}

func (r *Renderer) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Render writes reports/<run-id>/research.md and research.html and returns their paths relative to the project root.
func (r *Renderer) Render(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (string, string, error) {
	d, err := r.Build(ctx, run, sum)
	if err != nil {
		return "", "", err
	}
	md, err := r.RenderMarkdown(d)
	if err != nil {
		return "", "", err
	}
	html, err := r.RenderHTML(d)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(r.Cfg.Paths.ReportsDir, run.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	mdPath := filepath.Join(dir, "research.md")
	htmlPath := filepath.Join(dir, "research.html")
	if err := os.WriteFile(mdPath, md, 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		return "", "", err
	}
	return relOrBase(mdPath, r.Cfg.Paths.Root), relOrBase(htmlPath, r.Cfg.Paths.Root), nil
}

func (r *Renderer) RenderMarkdown(d *Data) ([]byte, error) {
	t, err := texttpl.New("research.md.tmpl").ParseFS(templateFS, "templates/research.md.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RenderHTML(d *Data) ([]byte, error) {
	t, err := htmltpl.New("research.html.tmpl").ParseFS(templateFS, "templates/research.html.tmpl")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
