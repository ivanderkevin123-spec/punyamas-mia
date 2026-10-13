// Package orchestrator runs the research workflow as an ordered list of
// deterministic steps with timeouts, bounded retries, budget gating,
// persistence, resume support and a run summary. Agents never call each other.
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/research"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/budget"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/importer"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/workspace"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/validate"
)

// Step names in execution order.
const (
	StepImport    = "import"
	StepWorkspace = "workspace"
	StepDedup     = "dedup"
	StepAgent     = "research_agent"
	StepValidate  = "validate"
	StepReport    = "report"
)

const dedupThreshold = 0.8

// Renderer is implemented by the report package.
type Renderer interface {
	Render(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (mdPath, htmlPath string, err error)
	RenderStrategy(ctx context.Context, run *domain.Run, sum *domain.RunSummary) (mdPath, htmlPath string, err error)
}

type Deps struct {
	Cfg      *config.Config
	Store    *store.Store
	Provider provider.Provider
	Budget   *budget.Gate
	Checker  *brand.Checker
	Renderer Renderer
	Log      *slog.Logger
	IDs      ids.Generator
	Now      func() time.Time
}

type Options struct {
	InputsDir  string
	FromRun    string // strategy runs: the research run that supplies validated evidence
	FixtureKey string
	ResumeID   string
	Until      string
	Force      bool
}

type Orchestrator struct {
	d Deps
}

func New(d Deps) *Orchestrator {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.IDs == nil {
		d.IDs = ids.TimeRandom{}
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Orchestrator{d: d}
}

type stepErr struct {
	status    string
	err       error
	retryable bool
}

func (e *stepErr) Error() string { return e.err.Error() }
func (e *stepErr) Unwrap() error { return e.err }

func failf(format string, a ...any) *stepErr {
	return &stepErr{status: domain.StepFailed, err: fmt.Errorf(format, a...)}
}

type stepDef struct {
	name     string
	required bool
	retry    bool
	fn       func(ctx context.Context, rc *runCtx) error
}

type runCtx struct {
	run        *domain.Run
	steps      map[string]*domain.Step
	opts       Options
	sum        *domain.RunSummary
	prev       *domain.RunSummary // summary of the previous invocation when resuming
	skipped    map[string]bool    // steps already completed and skipped in this invocation
	failed      bool
	stopped     bool
	costWarned  bool
	parentRunID string // strategy runs: research run id whose items are the evidence base
}

func (rc *runCtx) warn(format string, a ...any) {
	rc.sum.Warnings = append(rc.sum.Warnings, fmt.Sprintf(format, a...))
}

type inputsJSON struct {
	InputsDir  string `json:"inputs_dir"`
	FixtureKey string `json:"fixture_key"`
}

func (o *Orchestrator) ts() string { return o.d.Now().UTC().Format(time.RFC3339) }

func (o *Orchestrator) defs() []stepDef {
	return []stepDef{
		{name: StepImport, required: true, fn: o.stepImport},
		{name: StepWorkspace, required: o.d.Cfg.Limits.Drift.BlockRuns, fn: o.stepWorkspace},
		{name: StepDedup, required: true, fn: o.stepDedup},
		{name: StepAgent, required: true, retry: true, fn: o.stepAgent},
		{name: StepValidate, required: true, fn: o.stepValidate},
		{name: StepReport, required: true, fn: o.stepReport},
	}
}

// StepNames lists the research workflow steps in order.
func StepNames() []string {
	return []string{StepImport, StepWorkspace, StepDedup, StepAgent, StepValidate, StepReport}
}

// RunResearch executes (or resumes) a research run and returns its summary.
// The returned error is non-nil only for infrastructure failures; a failed run
// is reported through the summary status.
func (o *Orchestrator) RunResearch(ctx context.Context, opts Options) (*domain.RunSummary, error) {
	now := o.ts()
	var run *domain.Run
	var steps []*domain.Step
	var prev *domain.RunSummary
	if opts.ResumeID != "" {
		r, err := o.d.Store.GetRun(ctx, opts.ResumeID)
		if err != nil {
			return nil, fmt.Errorf("run %s: %w", opts.ResumeID, err)
		}
		if r.Kind != "research" {
			return nil, fmt.Errorf("run %s bukan run riset", r.ID)
		}
		var in inputsJSON
		_ = json.Unmarshal([]byte(r.InputsJSON), &in)
		if opts.InputsDir == "" {
			opts.InputsDir = in.InputsDir
		}
		if opts.FixtureKey == "" {
			opts.FixtureKey = in.FixtureKey
		}
		var ps domain.RunSummary
		if json.Unmarshal([]byte(r.SummaryJSON), &ps) == nil && ps.RunID != "" {
			prev = &ps
		}
		steps, err = o.d.Store.StepsForRun(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		run = r
		run.Status = domain.RunRunning
		run.HeartbeatAt = now
		run.FinishedAt = ""
		if err := o.d.Store.UpdateRun(ctx, run); err != nil {
			return nil, err
		}
	} else {
		if opts.InputsDir == "" {
			return nil, errors.New("direktori impor (--inputs) wajib diisi")
		}
		abs, err := filepath.Abs(opts.InputsDir)
		if err != nil {
			return nil, err
		}
		opts.InputsDir = abs
		if opts.FixtureKey == "" {
			opts.FixtureKey = o.d.Cfg.Limits.Provider.Mock.FixtureKey
		}
		if opts.FixtureKey == "" {
			opts.FixtureKey = filepath.Base(abs)
		}
		inJ, _ := json.Marshal(inputsJSON{InputsDir: opts.InputsDir, FixtureKey: opts.FixtureKey})
		run = &domain.Run{ID: o.d.IDs.New("run"), Kind: "research", Status: domain.RunRunning, CreatedAt: now, StartedAt: now, HeartbeatAt: now,
			ConfigHash: o.d.Cfg.Hash, ProviderName: o.d.Provider.Name(), InputsJSON: string(inJ), SummaryJSON: "{}", CostMeasurable: true}
		if err := o.d.Store.CreateRun(ctx, run); err != nil {
			return nil, err
		}
		for i, def := range o.defs() {
			steps = append(steps, &domain.Step{ID: o.d.IDs.New("step"), RunID: run.ID, Name: def.name, Ordinal: i + 1, Status: domain.StepPending})
		}
		if err := o.d.Store.CreateSteps(ctx, steps); err != nil {
			return nil, err
		}
	}
	rc := &runCtx{run: run, steps: map[string]*domain.Step{}, opts: opts, prev: prev, skipped: map[string]bool{},
		sum: &domain.RunSummary{RunID: run.ID, Kind: run.Kind, CostMeasurable: true}}
	for _, st := range steps {
		rc.steps[st.Name] = st
	}
	if prev != nil {
		for _, w := range prev.Warnings {
			if !strings.HasPrefix(w, "[sebelumnya] ") {
				w = "[sebelumnya] " + w
			}
			rc.sum.Warnings = append(rc.sum.Warnings, w)
		}
	}
	o.d.Log.Info("run started", "run_id", run.ID, "resume", opts.ResumeID != "", "inputs", opts.InputsDir, "provider", run.ProviderName)

	return o.execute(ctx, rc, o.defs())
}

func (o *Orchestrator) execStep(ctx context.Context, rc *runCtx, def stepDef) {
	st := rc.steps[def.name]
	st.Status = domain.StepRunning
	st.StartedAt = o.ts()
	_ = o.d.Store.UpdateStep(ctx, st)
	maxAttempts := 1
	if def.retry {
		maxAttempts = 1 + o.d.Cfg.Limits.Caps.MaxRetries
	}
	var last *stepErr
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		st.Attempts++
		_ = o.d.Store.UpdateStep(ctx, st)
		timeout := time.Duration(o.d.Cfg.StepTimeout(def.name)) * time.Second
		sctx, cancel := context.WithTimeout(ctx, timeout)
		err := def.fn(sctx, rc)
		timedOut := errors.Is(sctx.Err(), context.DeadlineExceeded)
		cancel()
		if err == nil {
			last = nil
			break
		}
		last = classify(err, timedOut)
		o.d.Log.Warn("step failed", "run_id", rc.run.ID, "step", def.name, "attempt", attempt, "status", last.status, "error", last.err.Error())
		if def.retry && last.retryable && attempt < maxAttempts {
			rc.warn("langkah %s gagal pada percobaan %d (%v); mencoba lagi", def.name, attempt, last.err)
			continue
		}
		break
	}
	if last == nil {
		st.Status = domain.StepCompleted
		st.ErrorJSON = ""
	} else {
		st.Status = last.status
		ej, _ := json.Marshal(map[string]string{"status": last.status, "message": last.err.Error()})
		st.ErrorJSON = string(ej)
	}
	st.FinishedAt = o.ts()
	_ = o.d.Store.UpdateStep(ctx, st)
	_ = o.d.Store.Heartbeat(ctx, rc.run.ID, st.FinishedAt)
	entry := domain.StepSummary{Name: st.Name, Status: st.Status, Attempts: st.Attempts}
	if last != nil {
		entry.Error = last.err.Error()
		if def.required {
			rc.failed = true
			rc.sum.Errors = append(rc.sum.Errors, fmt.Sprintf("langkah %s: %v", def.name, last.err))
		} else {
			rc.warn("langkah %s (opsional) gagal: %v", def.name, last.err)
		}
	}
	rc.sum.Steps = append(rc.sum.Steps, entry)
}

func classify(err error, timedOut bool) *stepErr {
	var se *stepErr
	if errors.As(err, &se) {
		if timedOut && se.status == domain.StepFailed {
			return &stepErr{status: domain.StepTimedOut, err: fmt.Errorf("melebihi batas waktu: %w", se.err), retryable: true}
		}
		return se
	}
	if timedOut || errors.Is(err, context.DeadlineExceeded) {
		return &stepErr{status: domain.StepTimedOut, err: fmt.Errorf("melebihi batas waktu: %w", err), retryable: true}
	}
	return &stepErr{status: domain.StepFailed, err: err, retryable: provider.Retryable(err)}
}

func (o *Orchestrator) provisionalStatus(rc *runCtx) string {
	switch {
	case rc.failed:
		return domain.RunFailed
	case rc.stopped:
		return domain.RunPartial
	case len(rc.sum.Warnings) > 0:
		return domain.RunCompletedWithWarnings
	default:
		return domain.RunCompleted
	}
}

// CountsFromStore derives every item- and source-based count from persisted
// rows linked to the run, so summaries do not depend on which steps executed in
// the current process (A3). Process-only counters (files, agent rejections) are
// not derivable from the store and are carried separately.
func (o *Orchestrator) CountsFromStore(ctx context.Context, runID string) (domain.Counts, error) {
	var c domain.Counts
	items, err := o.d.Store.ItemsForRun(ctx, runID)
	if err != nil {
		return c, err
	}
	for _, it := range items {
		if it.Origin == domain.OriginAgent {
			if it.RunID == runID {
				c.AgentFindings++
			}
		} else if it.RunID == runID {
			c.ItemsImported++
		} else {
			c.ItemsReused++
		}
		if it.Suspicious {
			c.Suspicious++
		}
		switch it.Status {
		case domain.ItemDuplicate:
			c.Duplicates++
		case domain.ItemRejected:
			c.Rejected++
		case domain.ItemValidated:
			switch it.Kind {
			case domain.KindFact:
				c.Facts++
			case domain.KindInterpretation:
				c.Interpretations++
			case domain.KindHypothesis:
				c.Hypotheses++
			}
		}
		if it.ClaimCheckJSON != "" {
			var cc claimCheckJSON
			if json.Unmarshal([]byte(it.ClaimCheckJSON), &cc) == nil && len(cc.Claims) > 0 {
				c.ClaimFlags++
			}
		}
	}
	srcs, err := o.d.Store.SourcesForRun(ctx, runID)
	if err != nil {
		return c, err
	}
	c.Sources = len(srcs)
	return c, nil
}

// refreshCounts replaces the summary counts with store-derived values, keeping
// process-only counters from this invocation or, for skipped steps, from the previous one.
func (o *Orchestrator) refreshCounts(ctx context.Context, rc *runCtx) error {
	c, err := o.CountsFromStore(ctx, rc.evidenceRunID())
	if err != nil {
		return err
	}
	c.FilesImported, c.FilesRejected, c.AgentRejected = rc.sum.Counts.FilesImported, rc.sum.Counts.FilesRejected, rc.sum.Counts.AgentRejected
	if rc.prev != nil {
		if rc.skipped[StepImport] {
			c.FilesImported, c.FilesRejected = rc.prev.Counts.FilesImported, rc.prev.Counts.FilesRejected
		}
		if rc.skipped[StepAgent] {
			c.AgentRejected = rc.prev.Counts.AgentRejected
		}
	}
	rc.sum.Counts = c
	if rc.run.Kind == "strategy" {
		sc, err := o.strategyCounts(ctx, rc)
		if err != nil {
			return err
		}
		rc.sum.Strategy = sc
	}
	return nil
}

// refreshCosts splits the ledger for the run into actual and uncertain (estimated) spend (A7).
func (o *Orchestrator) refreshCosts(ctx context.Context, rc *runCtx) error {
	costs, err := o.d.Store.CostsForRun(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	actual, uncertain := 0.0, 0.0
	for _, c := range costs {
		v := 0.0
		if c.CostUSD != nil {
			v = *c.CostUSD
		}
		if c.Estimated {
			uncertain += v
		} else {
			actual += v
		}
	}
	rc.sum.CostActualUSD = actual
	rc.sum.CostUncertainUSD = uncertain
	rc.sum.CostUSD = actual + uncertain
	measurable := true
	for _, c := range costs {
		if c.Estimated {
			measurable = false
		}
	}
	rc.sum.CostMeasurable = measurable
	if !measurable && !rc.costWarned {
		rc.costWarned = true
		rc.warn("biaya tidak dapat diukur sepenuhnya: USD %.4f dibukukan sebagai estimasi tidak pasti (panggilan gagal, habis waktu, atau penggunaan tak terukur)", uncertain)
	}
	return nil
}

func (o *Orchestrator) finalize(ctx context.Context, rc *runCtx) (*domain.RunSummary, error) {
	if err := o.refreshCounts(ctx, rc); err != nil {
		return nil, err
	}
	if err := o.refreshCosts(ctx, rc); err != nil {
		return nil, err
	}
	rc.sum.Status = o.provisionalStatus(rc)
	rc.run.FinishedAt = o.ts()
	rc.run.Status = rc.sum.Status
	total := rc.sum.CostUSD
	rc.run.CostUSDTotal = &total
	rc.run.CostMeasurable = rc.sum.CostMeasurable
	if created, err := time.Parse(time.RFC3339, rc.run.CreatedAt); err == nil {
		rc.sum.DurationS = o.d.Now().UTC().Sub(created).Seconds()
	}
	sj, _ := json.Marshal(rc.sum)
	rc.run.SummaryJSON = string(sj)
	if err := o.d.Store.UpdateRun(ctx, rc.run); err != nil {
		return nil, err
	}
	o.d.Log.Info("run finished", "run_id", rc.run.ID, "status", rc.sum.Status, "cost_usd", total, "cost_uncertain_usd", rc.sum.CostUncertainUSD, "warnings", len(rc.sum.Warnings), "errors", len(rc.sum.Errors))
	return rc.sum, nil
}

// ---- steps ----

func canonicalIfURL(ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return dedup.CanonicalURL(ref)
	}
	return ""
}

func (o *Orchestrator) stepImport(ctx context.Context, rc *runCtx) error {
	cfg := o.d.Cfg
	res, err := importer.ImportDir(rc.opts.InputsDir, cfg.Sources.Imports.MaxFileBytes)
	if err != nil {
		return failf("%v", err)
	}
	for _, s := range res.Skipped {
		rc.warn("berkas dilewati (format tidak didukung): %s", s)
	}
	for _, fe := range res.Errors {
		rc.sum.Counts.FilesRejected++
		rc.warn("impor gagal: %s: %s", fe.File, fe.Message)
	}
	now := o.ts()
	files := map[string]bool{}
	for _, n := range res.Notes {
		files[n.File] = true
		var sb strings.Builder
		sb.WriteString(n.Reference)
		for _, it := range n.Items {
			sb.WriteString("\n" + it.Statement + "\n" + it.Excerpt)
		}
		noteHash := dedup.ContentHash(sb.String())
		src, err := o.d.Store.FindSource(ctx, n.Reference, noteHash)
		if errors.Is(err, store.ErrNotFound) {
			src = &domain.Source{ID: o.d.IDs.New("src"), SourceType: n.SourceType, Name: n.SourceName, Reference: n.Reference, CanonicalURL: canonicalIfURL(n.Reference),
				Publisher: n.Publisher, RetrievedAt: n.RetrievedAt, PublishedAt: n.PublishedAt, ContentHash: noteHash, Status: "ok", CreatedAt: now}
			if err := o.d.Store.InsertSource(ctx, src); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := o.d.Store.LinkRunSource(ctx, rc.run.ID, src.ID); err != nil {
			return err
		}
		for _, it := range n.Items {
			h := dedup.ContentHash(it.Statement)
			existing, err := o.d.Store.FindItem(ctx, domain.OriginImport, it.Ref, h)
			if err == nil {
				rc.sum.Counts.ItemsReused++
				if err := o.d.Store.LinkRunItem(ctx, rc.run.ID, existing.ID); err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, store.ErrNotFound) {
				return err
			}
			item := &domain.Item{ID: o.d.IDs.New("item"), RunID: rc.run.ID, Ref: it.Ref, Origin: domain.OriginImport, Kind: it.Kind, TopicKey: it.TopicKey,
				Title: it.Title, Statement: it.Statement, Language: it.Language, Quality: it.Quality, Status: domain.ItemImported, ContentHash: h, CreatedAt: now}
			if ok, m := validate.Suspicious(it.Title + " " + it.Statement + " " + it.Excerpt); ok {
				item.Suspicious = true
				rc.warn("konten mencurigakan pada %s (%q); dikecualikan dari masukan agen", it.Ref, m)
			}
			ev := &domain.Evidence{ID: o.d.IDs.New("ev"), SourceID: src.ID, Excerpt: it.Excerpt, Locator: fmt.Sprintf("%s bagian %d", n.File, it.Row), RetrievedAt: n.RetrievedAt, PublishedAt: n.PublishedAt}
			if ev.Excerpt == "" {
				ev.Excerpt = it.Statement
				ev.Note = "hipotesis: kutipan sama dengan pernyataan"
			}
			// Item, run link and evidence are committed atomically (A2).
			if err := o.d.Store.InsertItemWithEvidence(ctx, rc.run.ID, item, []*domain.Evidence{ev}); err != nil {
				return err
			}
			rc.sum.Counts.ItemsImported++
		}
	}
	rc.sum.Counts.FilesImported = len(files)
	if len(res.Notes) == 0 {
		return failf("tidak ada berkas riset yang valid di %s", rc.opts.InputsDir)
	}
	return nil
}

func (o *Orchestrator) stepWorkspace(ctx context.Context, rc *runCtx) error {
	drifts, warnings, err := o.CheckWorkspace(ctx, rc.run.ID, rc.sum)
	for _, w := range warnings {
		rc.warn("%s", w)
	}
	rc.sum.Drift = drifts
	if err != nil {
		return failf("%v", err)
	}
	review := false
	for _, d := range drifts {
		if d.Status == "changed" || d.Status == "review_required" {
			review = true
		}
	}
	if review && o.d.Cfg.Limits.Drift.BlockRuns {
		return failf("aset ruang kerja berubah dan memerlukan tinjauan (drift.block_runs = true)")
	}
	return nil
}

// CheckWorkspace fingerprints every configured workspace asset, records
// baselines, flags drift for review and (when runID is set) derives research
// items from the FAQ. It never writes under the workspace root.
func (o *Orchestrator) CheckWorkspace(ctx context.Context, runID string, sum *domain.RunSummary) ([]domain.DriftEntry, []string, error) {
	cfg := o.d.Cfg
	root := cfg.Sources.Workspace.Root
	var warnings []string
	var drifts []domain.DriftEntry
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		warnings = append(warnings, fmt.Sprintf("ruang kerja Punyamas tidak ditemukan di %s; impor aset dilewati", root))
		return nil, warnings, nil
	}
	commit := workspace.HeadCommit(root)
	now := o.ts()
	for _, a := range cfg.Sources.Workspace.Assets {
		fp, err := workspace.FingerprintAsset(root, a)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("aset %s: %v", a.Key, err))
			continue
		}
		prev, err := o.d.Store.LatestAcceptedAsset(ctx, a.Key)
		if errors.Is(err, store.ErrNotFound) {
			prev = nil
		} else if err != nil {
			return nil, warnings, err
		}
		status := workspace.Compare(prev, fp)
		d := domain.DriftEntry{Key: a.Key, Status: status, Path: a.Path}
		if status == "missing" {
			warnings = append(warnings, fmt.Sprintf("aset %s tidak ditemukan di ruang kerja (%s)", a.Key, a.Path))
			drifts = append(drifts, d)
			continue
		}
		content, err := workspace.ReadAsset(fp)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("aset %s tidak dapat dibaca: %v", a.Key, err))
			continue
		}
		summary := map[string]any{"size_bytes": fp.Size}
		switch a.Key {
		case "brand_oracle":
			rules := workspace.ParseBrandOracle(content)
			summary["rules"] = rules
			if len(rules) == 0 {
				d.Details = append(d.Details, "tidak ada aturan POLA yang dapat dibaca dari brand-oracle.mjs")
			}
			d.Details = append(d.Details, workspace.DiffRules(rules, cfg.Policy.Rules)...)
		case "facts":
			facts, err := workspace.ParseFacts(content)
			if err != nil {
				d.Details = append(d.Details, "B4-fakta.json tidak dapat diurai: "+err.Error())
			} else {
				ids := make([]string, 0, len(facts.Facts))
				for _, f := range facts.Facts {
					ids = append(ids, f.ID)
				}
				summary["fact_ids"] = ids
				summary["versi"] = facts.Version
				d.Details = append(d.Details, workspace.DiffFacts(facts, cfg.Business.Facts)...)
			}
		case "w1":
			faqs := workspace.ParseFAQ(content)
			summary["faq_count"] = len(faqs)
			if runID != "" {
				if err := o.deriveFAQItems(ctx, runID, a, fp, faqs, now, sum); err != nil {
					return nil, warnings, err
				}
			}
		}
		sj, _ := json.Marshal(summary)
		asset := &domain.ImportedAsset{ID: o.d.IDs.New("asset"), AssetKey: a.Key, Root: root, RelativePath: a.Path, SHA256: fp.SHA256, SizeBytes: fp.Size,
			MTime: fp.MTime.Format(time.RFC3339), WorkspaceCommit: commit, ImportedAt: now, ParsedSummaryJSON: string(sj)}
		switch status {
		case "new":
			asset.ReviewStatus = domain.AssetAccepted
			asset.AcceptedAt = now
			asset.AcceptedByNote = "baseline: impor pertama"
			if err := o.d.Store.InsertAsset(ctx, asset); err != nil {
				return nil, warnings, err
			}
		case "changed":
			latest, _ := o.d.Store.LatestAsset(ctx, a.Key)
			if latest == nil || latest.ReviewStatus != domain.AssetReviewRequired || latest.SHA256 != fp.SHA256 {
				asset.ReviewStatus = domain.AssetReviewRequired
				if err := o.d.Store.InsertAsset(ctx, asset); err != nil {
					return nil, warnings, err
				}
			}
			warnings = append(warnings, fmt.Sprintf("aset %s berubah sejak terakhir diterima; tinjau lalu jalankan: mia approve asset --key %s --note \"...\"", a.Key, a.Key))
		}
		if len(d.Details) > 0 {
			if d.Status != "changed" {
				d.Status = "review_required"
			}
			warnings = append(warnings, fmt.Sprintf("aset %s: %d perbedaan dengan konfigurasi MIA; tinjau dan selaraskan konfigurasi", a.Key, len(d.Details)))
		}
		drifts = append(drifts, d)
	}
	return drifts, warnings, nil
}

func (o *Orchestrator) deriveFAQItems(ctx context.Context, runID string, a config.Asset, fp workspace.Fingerprint, faqs []workspace.FAQ, now string, sum *domain.RunSummary) error {
	if len(faqs) == 0 {
		return nil
	}
	src, err := o.d.Store.FindSource(ctx, a.Path, fp.SHA256)
	if errors.Is(err, store.ErrNotFound) {
		src = &domain.Source{ID: o.d.IDs.New("src"), SourceType: domain.SourceInternalWS, Name: "Ruang kerja Punyamas: " + a.Path, Reference: a.Path,
			RetrievedAt: now, ContentHash: fp.SHA256, Status: "ok", CreatedAt: now}
		if err := o.d.Store.InsertSource(ctx, src); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := o.d.Store.LinkRunSource(ctx, runID, src.ID); err != nil {
		return err
	}
	for _, f := range faqs {
		ref := fmt.Sprintf("workspace:%s#%d", a.Key, f.N)
		statement := "Calon pelanggan mungkin bertanya: " + f.Question
		h := dedup.ContentHash(statement)
		existing, err := o.d.Store.FindItem(ctx, domain.OriginWorkspace, ref, h)
		if err == nil {
			if sum != nil {
				sum.Counts.ItemsReused++
			}
			if err := o.d.Store.LinkRunItem(ctx, runID, existing.ID); err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		item := &domain.Item{ID: o.d.IDs.New("item"), RunID: runID, Ref: ref, Origin: domain.OriginWorkspace, Kind: domain.KindHypothesis, TopicKey: "pertanyaan_pelanggan",
			Title: f.Question, Statement: statement, Language: "id", Quality: "D", Status: domain.ItemImported, ContentHash: h, CreatedAt: now}
		answer := f.Answer
		if len(answer) > 1000 {
			answer = answer[:1000]
		}
		ev := &domain.Evidence{ID: o.d.IDs.New("ev"), SourceID: src.ID, Excerpt: answer, Locator: fmt.Sprintf("Pertanyaan umum #%d", f.N), RetrievedAt: now,
			Note: "hipotesis internal dari naskah situs; bukan bukti eksternal"}
		if err := o.d.Store.InsertItemWithEvidence(ctx, runID, item, []*domain.Evidence{ev}); err != nil {
			return err
		}
		if sum != nil {
			sum.Counts.ItemsImported++
		}
	}
	return nil
}

func (o *Orchestrator) stepDedup(ctx context.Context, rc *runCtx) error {
	items, err := o.d.Store.ItemsForRun(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	byID := map[string]*domain.Item{}
	var cands []dedup.Candidate
	for _, it := range items {
		byID[it.ID] = it
		if it.Origin == domain.OriginAgent || it.Status == domain.ItemDuplicate {
			continue
		}
		cands = append(cands, dedup.Candidate{ID: it.ID, Title: it.Title, ContentHash: it.ContentHash})
	}
	for _, m := range dedup.FindDuplicates(cands, dedupThreshold) {
		it := byID[m.ID]
		it.Status = domain.ItemDuplicate
		it.DuplicateOf = m.DuplicateOf
		if err := o.d.Store.UpdateItem(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func worstQuality(qs []string) string {
	worst := ""
	for _, q := range qs {
		if q > worst {
			worst = q
		}
	}
	if worst == "" {
		return "D"
	}
	return worst
}

func (o *Orchestrator) stepAgent(ctx context.Context, rc *runCtx) error {
	cfg := o.d.Cfg
	caps := cfg.Limits.Caps
	items, err := o.d.Store.ItemsForRun(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	var pitems []research.ItemForPrompt
	refs := map[string]research.RefInfo{}
	evid, err := o.d.Store.EvidenceForItems(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	srcNames := map[string]string{}
	for _, it := range items {
		if it.Origin == domain.OriginAgent || it.Status == domain.ItemDuplicate || it.Suspicious {
			continue
		}
		excerpt, srcName := "", ""
		for _, e := range evid[it.ID] {
			if e.SourceID != "" {
				excerpt = e.Excerpt
				name, ok := srcNames[e.SourceID]
				if !ok {
					if s, err := o.d.Store.GetSource(ctx, e.SourceID); err == nil {
						name = s.Name
					}
					srcNames[e.SourceID] = name
				}
				srcName = name
				break
			}
		}
		pitems = append(pitems, research.ItemForPrompt{Ref: it.Ref, Kind: it.Kind, Quality: it.Quality, Title: it.Title, Statement: it.Statement, Excerpt: excerpt, SourceName: srcName})
		refs[it.Ref] = research.RefInfo{ID: it.ID, Kind: it.Kind, Origin: it.Origin, Quality: it.Quality}
	}
	if len(pitems) == 0 {
		rc.warn("tidak ada butir riset yang dapat dianalisis; agen riset dilewati")
		return nil
	}
	req := research.BuildRequest(cfg, pitems, rc.opts.FixtureKey)
	req.Model = cfg.Limits.Provider.Model
	req.MaxOutputTokens = caps.MaxOutputTokens
	st := rc.steps[StepAgent]
	var problems []string
	for call := 1; call <= caps.MaxCallsPerStep; call++ {
		if call > 1 {
			req = research.RepairRequest(req, problems)
		}
		estIn := provider.EstimateTokens(req.System + req.User)
		estOut := caps.MaxOutputTokens
		if estIn > caps.MaxInputTokens {
			return failf("masukan agen (%d token) melebihi batas %d token", estIn, caps.MaxInputTokens)
		}
		dec, err := o.d.Budget.Allow(ctx, rc.run.ID, research.Name, req.Model, estIn, estOut)
		if err != nil {
			return failf("pemeriksaan anggaran gagal: %v", err)
		}
		if dec.Warn {
			rc.warn("anggaran bulanan mendekati plafon (terpakai USD %.4f dari USD %.2f)", dec.MonthSpent, caps.MonthlyCeilingUSD)
		}
		if !dec.Allowed {
			return &stepErr{status: domain.StepBudgetExhausted, err: errors.New(dec.Reason)}
		}
		start := time.Now()
		resp, usage, perr := o.d.Provider.Complete(ctx, req)
		now := o.ts()
		// Bookkeeping must succeed even when the step deadline has just expired (A7).
		bctx := context.WithoutCancel(ctx)
		exec := &domain.AgentExecution{ID: o.d.IDs.New("exec"), RunID: rc.run.ID, StepID: st.ID, AgentName: research.Name, ProviderName: o.d.Provider.Name(), Model: req.Model,
			SchemaVersion: research.SchemaVersion, RequestHash: dedup.FileHash([]byte(req.System + "\n" + req.User)), InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
			CacheReadTokens: usage.CacheReadTokens, CostMeasurable: usage.Measurable, LatencyMS: time.Since(start).Milliseconds(), Attempt: call, Outcome: "pending", CreatedAt: now}
		exec.RequestPath, exec.ResponsePath = o.saveExchange(exec.ID, req, resp.Content)
		if perr != nil {
			exec.Outcome = "error"
			if errors.Is(perr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				exec.Outcome = "timeout"
			}
			exec.ErrorText = perr.Error()
			exec.CostMeasurable = false
			if err := o.d.Store.InsertExecution(bctx, exec); err != nil {
				return err
			}
			if provider.Fatal(perr) {
				// Authentication/billing rejections process no tokens: nothing is booked.
				return &stepErr{status: domain.StepFailed, err: fmt.Errorf("penyedia menolak (%v); tidak diulang", perr)}
			}
			// Outcome unknown: book the pre-call estimate as uncertain cost so repeated failures count against the ceilings (A7).
			rec, err := o.d.Budget.RecordUncertain(bctx, rc.run.ID, exec.ID, research.Name, o.d.Provider.Name(), req.Model, estIn, estOut, exec.Outcome)
			if err != nil {
				return err
			}
			exec.CostUSD = rec.CostUSD
			_ = o.d.Store.UpdateExecution(bctx, exec)
			return &stepErr{status: domain.StepFailed, err: perr, retryable: provider.Retryable(perr)}
		}
		if err := o.d.Store.InsertExecution(bctx, exec); err != nil {
			return err
		}
		cost, err := o.d.Budget.Record(bctx, rc.run.ID, exec.ID, research.Name, o.d.Provider.Name(), req.Model, usage, estIn, estOut)
		if err != nil {
			return err
		}
		exec.CostUSD = cost.CostUSD
		exec.CostMeasurable = !cost.Estimated
		for _, w := range resp.Warnings {
			rc.warn("%s", w)
		}
		out, perr := research.ParseOutput(resp.Content)
		if perr != nil {
			exec.Outcome = "invalid_output"
			exec.ErrorText = perr.Error()
			_ = o.d.Store.UpdateExecution(ctx, exec)
			problems = []string{perr.Error()}
			if call < caps.MaxCallsPerStep {
				rc.warn("keluaran agen tidak valid (percobaan %d): %v; meminta perbaikan", call, perr)
				continue
			}
			return &stepErr{status: domain.StepFailed, err: fmt.Errorf("keluaran agen tidak valid setelah %d percobaan: %w", call, perr)}
		}
		valid, rejected := research.Validate(out, refs)
		if len(valid) == 0 && len(rejected) > 0 {
			exec.Outcome = "invalid_output"
			problems = problems[:0]
			for _, r := range rejected {
				problems = append(problems, fmt.Sprintf("temuan %d (%s): %s", r.Index, r.Title, strings.Join(r.Reasons, "; ")))
			}
			exec.ErrorText = strings.Join(problems, " | ")
			_ = o.d.Store.UpdateExecution(ctx, exec)
			if call < caps.MaxCallsPerStep {
				rc.warn("semua temuan agen ditolak validasi (percobaan %d); meminta perbaikan", call)
				continue
			}
			return &stepErr{status: domain.StepFailed, err: fmt.Errorf("semua temuan agen ditolak validasi setelah %d percobaan", call)}
		}
		exec.Outcome = "ok"
		if call > 1 {
			exec.Outcome = "repaired"
		}
		if err := o.d.Store.UpdateExecution(ctx, exec); err != nil {
			return err
		}
		for i, f := range valid {
			var qs []string
			for _, r := range f.EvidenceRefs {
				qs = append(qs, refs[r].Quality)
			}
			quality := worstQuality(qs)
			if f.Kind == domain.KindHypothesis {
				quality = "D"
			}
			item := &domain.Item{ID: o.d.IDs.New("item"), RunID: rc.run.ID, Ref: fmt.Sprintf("agent:%s#%d", exec.ID, i+1), Origin: domain.OriginAgent, Kind: f.Kind, TopicKey: f.TopicKey,
				Title: f.Title, Statement: f.Statement, Rationale: f.Rationale, Language: "id", Quality: quality, Status: domain.ItemImported, ContentHash: dedup.ContentHash(f.Statement),
				AgentExecutionID: exec.ID, CreatedAt: now}
			var evs []*domain.Evidence
			for _, r := range f.EvidenceRefs {
				evs = append(evs, &domain.Evidence{ID: o.d.IDs.New("ev"), FromItemID: refs[r].ID, Note: "dikutip oleh agen riset", RetrievedAt: now})
			}
			if err := o.d.Store.InsertItemWithEvidence(ctx, rc.run.ID, item, evs); err != nil {
				return err
			}
		}
		rc.sum.Counts.AgentRejected += len(rejected)
		for _, r := range rejected {
			rc.warn("temuan agen ditolak: %q: %s", r.Title, strings.Join(r.Reasons, "; "))
		}
		if strings.TrimSpace(out.Notes) != "" {
			rc.warn("catatan agen: %s", strings.TrimSpace(out.Notes))
		}
		return nil
	}
	return failf("agen riset tidak menghasilkan keluaran")
}

func (o *Orchestrator) saveExchange(execID string, req provider.Request, response string) (string, string) {
	dir := filepath.Join(o.d.Cfg.Paths.DataDir, "executions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", ""
	}
	reqPath := filepath.Join(dir, execID+".request.txt")
	respPath := filepath.Join(dir, execID+".response.txt")
	_ = os.WriteFile(reqPath, []byte("### SYSTEM\n"+req.System+"\n\n### USER\n"+req.User+"\n"), 0o600)
	_ = os.WriteFile(respPath, []byte(response), 0o600)
	return reqPath, respPath
}

type claimCheckJSON struct {
	Claims           []brand.Finding `json:"claims"`
	ValidationErrors []string        `json:"validation_errors"`
}

func (o *Orchestrator) stepValidate(ctx context.Context, rc *runCtx) error {
	items, err := o.d.Store.ItemsForRun(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	evid, err := o.d.Store.EvidenceForItems(ctx, rc.run.ID)
	if err != nil {
		return err
	}
	byID := map[string]*domain.Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	for _, it := range items {
		if it.Status == domain.ItemDuplicate {
			continue
		}
		var verrs []string
		var claims []brand.Finding
		if o.d.Checker != nil {
			res := o.d.Checker.Check(it.Title + " " + it.Statement)
			if !res.Passes {
				claims = res.Findings
			}
		}
		if ok, _ := validate.Suspicious(it.Title + " " + it.Statement); ok && !it.Suspicious {
			it.Suspicious = true
		}
		evs := evid[it.ID]
		switch it.Kind {
		case domain.KindFact:
			ok := false
			for _, e := range evs {
				if e.SourceID != "" {
					ok = true
				}
			}
			if !ok {
				verrs = append(verrs, "fakta tanpa bukti bersumber")
			}
		case domain.KindInterpretation:
			ok := false
			for _, e := range evs {
				if e.FromItemID != "" {
					if t := byID[e.FromItemID]; t != nil && t.Status != domain.ItemRejected && t.Status != domain.ItemDuplicate {
						ok = true
					}
				}
			}
			if !ok {
				verrs = append(verrs, "interpretasi tanpa butir pendukung yang valid")
			}
		}
		payload, _ := json.Marshal(claimCheckJSON{Claims: claims, ValidationErrors: verrs})
		it.ClaimCheckJSON = string(payload)
		if len(verrs) > 0 {
			it.Status = domain.ItemRejected
		} else {
			it.Status = domain.ItemValidated
		}
		if err := o.d.Store.UpdateItem(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) stepReport(ctx context.Context, rc *runCtx) error {
	if o.d.Renderer == nil {
		return failf("penyaji laporan tidak dikonfigurasi")
	}
	var md, html string
	var err error
	if rc.run.Kind == "strategy" {
		md, html, err = o.d.Renderer.RenderStrategy(ctx, rc.run, rc.sum)
	} else {
		md, html, err = o.d.Renderer.Render(ctx, rc.run, rc.sum)
	}
	if err != nil {
		return failf("laporan gagal dibuat: %v", err)
	}
	rc.sum.ReportMarkdown = md
	rc.sum.ReportHTML = html
	return nil
}

// IsStale reports whether a running run has not sent a heartbeat within staleAfter.
func IsStale(run *domain.Run, now time.Time, staleAfter time.Duration) bool {
	if run.Status != domain.RunRunning {
		return false
	}
	hb, err := time.Parse(time.RFC3339, run.HeartbeatAt)
	if err != nil {
		return true
	}
	return now.Sub(hb) > staleAfter
}
