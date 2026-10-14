// Command mia is the Punyamas Marketing Intelligence Agent CLI (Phase 5: mock-only).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/budget"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/logx"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider/registry"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/report"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/importer"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/workflow"
)

const version = "0.5.0-phase5"

const usage = `mia — Punyamas Marketing Intelligence Agent (Fase 5, mode mock)

Perintah:
  run research --inputs DIR [--fixture KEY] [--until STEP] [--force]   jalankan alur riset
  run research --resume RUN_ID                                         lanjutkan run yang terhenti
  import --inputs DIR                                                  impor saja (berhenti setelah dedup)
  status [RUN_ID] [--limit N]                                          daftar run atau rincian satu run
  report RUN_ID                                                        render ulang laporan
  validate --inputs DIR                                                periksa berkas impor tanpa menyimpan
  config check [--json]                                                periksa konfigurasi dan perubahan aset ruang kerja
  approve asset --key KEY --note TEKS                                  terima perubahan aset setelah ditinjau
  run strategy --from-run RUN_ID [--fixture KEY] [--resume ID]         Agen Strategi: peluang dari bukti tervalidasi (Fase 3)
  opportunities --run RUN_ID                                           daftar peluang sebuah run strategi
  approve opportunity --id ID --decision approved|rejected|needs_more_evidence --note TEKS
  export brief --id ID [--out PATH]                                    ekspor brief peluang yang disetujui untuk serah terima manual
  run content --opportunity ID [--types a,b] [--fixture KEY] [--regenerate]   Agen Konten: draf dari peluang approved (Fase 4)
  run content --resume RUN_ID [--force]                                lanjutkan run konten
  content list --run RUN_ID | --opportunity ID                         daftar draf dengan status, isian, dan kesiapan
  content show --id DRAFT_ID                                           rincian draf, bukti, temuan, dan provenance
  content review --id DRAFT_ID --decision approved|rejected --note TEKS   keputusan peninjau (bukan izin publikasi)
  content revise --id DRAFT_ID [--hook T] [--headline T] [--body-file PATH] [--cta T] --note TEKS
  content export --id DRAFT_ID [--out NAMA_RELATIF]                    ekspor DRAF ke drafts/content untuk serah terima manual
  workflow start --inputs DIR [--fixture KEY] [--new]                  Fase 5: workflow riset → strategi → tinjauan → draf → tinjauan → ekspor
  workflow run --id WF | workflow resume --id WF [--new-content-run OPP]   tahap otomatis aman berikutnya (tidak pernah menyetujui atau menerbitkan)
  workflow show --id WF | workflow next --id WF | workflow list        status langsung dari basis data dan tindakan berikutnya
  workflow report --id WF | workflow export --id WF [--dir NAMA]       laporan workflow; ekspor draf yang siap serah terima
  version

Opsi umum: --root DIR (default: direktori kerja). Kode keluar: 0 ok, 1 kesalahan, 2 run gagal atau perlu tinjauan.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type app struct {
	cfg     *config.Config
	store   *store.Store
	log     *slog.Logger
	orch    *orchestrator.Orchestrator
	rend    *report.Renderer
	ids     ids.Generator
	now     func() time.Time
	logF    *os.File
	checker *brand.Checker
	wf      *workflow.Service
}

func newApp(root string, stderr io.Writer) (*app, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	now := time.Now
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("MIA_LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	writers := []io.Writer{stderr}
	logF, err := logx.OpenLogFile(cfg.Paths.DataDir, now())
	if err == nil {
		writers = append(writers, logF)
	}
	logger := logx.New(level, writers...)
	st, err := store.Open(filepath.Join(cfg.Paths.DataDir, "mia.sqlite"))
	if err != nil {
		return nil, fmt.Errorf("basis data: %w", err)
	}
	prov, err := registry.Build(cfg)
	if err != nil {
		st.Close()
		return nil, err
	}
	checker, err := brand.New(cfg.Policy.Rules)
	if err != nil {
		st.Close()
		return nil, err
	}
	gen := ids.TimeRandom{}
	gate := &budget.Gate{Store: st, Caps: cfg.Limits.Caps, Prices: cfg.Limits.PriceTable, Now: now, IDs: gen}
	rend := &report.Renderer{Store: st, Cfg: cfg, Now: now}
	orch := orchestrator.New(orchestrator.Deps{Cfg: cfg, Store: st, Provider: prov, Budget: gate, Checker: checker, Renderer: rend, Log: logger, IDs: gen, Now: now})
	wfs := &workflow.Service{Store: st, Orch: orch, Cfg: cfg, IDs: gen, Now: now, Log: logger}
	return &app{cfg: cfg, store: st, log: logger, orch: orch, rend: rend, ids: gen, now: now, logF: logF, checker: checker, wf: wfs}, nil
}

func (a *app) close() {
	if a.store != nil {
		a.store.Close()
	}
	if a.logF != nil {
		a.logF.Close()
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	ctx := context.Background()
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, "mia", version)
		return 0
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "gunakan: mia run research --inputs DIR  |  mia run strategy --from-run RUN_ID  |  mia run content --opportunity ID")
			return 1
		}
		switch args[1] {
		case "research":
			return cmdRun(ctx, args[2:], "", stdout, stderr)
		case "strategy":
			return cmdRunStrategy(ctx, args[2:], stdout, stderr)
		case "content":
			return cmdRunContent(ctx, args[2:], stdout, stderr)
		default:
			fmt.Fprintln(stderr, "gunakan: mia run research --inputs DIR  |  mia run strategy --from-run RUN_ID  |  mia run content --opportunity ID")
			return 1
		}
	case "import":
		return cmdRun(ctx, args[1:], orchestrator.StepDedup, stdout, stderr)
	case "status":
		return cmdStatus(ctx, args[1:], stdout, stderr)
	case "report":
		return cmdReport(ctx, args[1:], stdout, stderr)
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "config":
		if len(args) < 2 || args[1] != "check" {
			fmt.Fprintln(stderr, "gunakan: mia config check")
			return 1
		}
		return cmdConfigCheck(ctx, args[2:], stdout, stderr)
	case "approve":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "gunakan: mia approve asset --key KEY --note TEKS  |  mia approve opportunity --id ID --decision ... --note TEKS")
			return 1
		}
		switch args[1] {
		case "asset":
			return cmdApproveAsset(ctx, args[2:], stdout, stderr)
		case "opportunity":
			return cmdApproveOpportunity(ctx, args[2:], stdout, stderr)
		default:
			fmt.Fprintln(stderr, "gunakan: mia approve asset ... | mia approve opportunity ...")
			return 1
		}
	case "workflow":
		return cmdWorkflow(ctx, args[1:], stdout, stderr)
	case "opportunities":
		return cmdOpportunities(ctx, args[1:], stdout, stderr)
	case "content":
		return cmdContent(ctx, args[1:], stdout, stderr)
	case "export":
		if len(args) < 2 || args[1] != "brief" {
			fmt.Fprintln(stderr, "gunakan: mia export brief --id ID [--out PATH]")
			return 1
		}
		return cmdExportBrief(ctx, args[2:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "perintah tidak dikenal: %s\n\n%s", args[0], usage)
		return 1
	}
}

func cmdRun(ctx context.Context, args []string, until string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	inputs := fs.String("inputs", "", "direktori berkas impor")
	fixture := fs.String("fixture", "", "kunci fixture mock (default: nama direktori impor)")
	resume := fs.String("resume", "", "lanjutkan run dengan id ini")
	force := fs.Bool("force", false, "jalankan ulang langkah yang sudah selesai")
	untilFlag := fs.String("until", until, "berhenti setelah langkah ini")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	if !checkIDs(stderr, "run", *resume) {
		return 1
	}
	sum, err := a.orch.RunResearch(ctx, orchestrator.Options{InputsDir: *inputs, FixtureKey: *fixture, ResumeID: *resume, Until: *untilFlag, Force: *force})
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	printSummary(stdout, sum)
	if sum.Status == domain.RunFailed {
		return 2
	}
	return 0
}

func printSummary(w io.Writer, sum *domain.RunSummary) {
	fmt.Fprintf(w, "Run %s: %s (%.1fs)\n", sum.RunID, sum.Status, sum.DurationS)
	for _, s := range sum.Steps {
		line := fmt.Sprintf("  %-15s %-18s percobaan=%d", s.Name, s.Status, s.Attempts)
		if s.Error != "" {
			line += "  " + s.Error
		}
		fmt.Fprintln(w, line)
	}
	if c := sum.Counts; sum.Kind != "content" {
		fmt.Fprintf(w, "  sumber=%d butir=%d (reuse %d) fakta=%d interpretasi=%d hipotesis=%d duplikat=%d ditolak=%d mencurigakan=%d klaim=%d\n",
			c.Sources, c.ItemsImported, c.ItemsReused, c.Facts, c.Interpretations, c.Hypotheses, c.Duplicates, c.Rejected, c.Suspicious, c.ClaimFlags)
	}
	cost := fmt.Sprintf("USD %.4f", sum.CostUSD)
	if !sum.CostMeasurable {
		cost += " (tidak dapat diukur sepenuhnya; estimasi)"
	}
	fmt.Fprintf(w, "  biaya=%s\n", cost)
	for _, wmsg := range sum.Warnings {
		fmt.Fprintln(w, "  peringatan:", wmsg)
	}
	for _, e := range sum.Errors {
		fmt.Fprintln(w, "  KESALAHAN:", e)
	}
	if sum.ReportMarkdown != "" {
		fmt.Fprintf(w, "  laporan: %s | %s\n", sum.ReportMarkdown, sum.ReportHTML)
	}
}

func cmdStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	limit := fs.Int("limit", 20, "jumlah run yang ditampilkan")
	asJSON := fs.Bool("json", false, "keluaran JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	stale := time.Duration(a.cfg.Limits.Caps.StaleAfterS) * time.Second
	if stale <= 0 {
		stale = 10 * time.Minute
	}
	if fs.NArg() == 0 {
		runs, err := a.store.ListRuns(ctx, *limit)
		if err != nil {
			fmt.Fprintln(stderr, "kesalahan:", err)
			return 1
		}
		if *asJSON {
			return writeJSON(stdout, runs)
		}
		fmt.Fprintf(stdout, "%-28s %-10s %-24s %-22s %s\n", "RUN", "JENIS", "STATUS", "DIBUAT (UTC)", "BIAYA")
		for _, r := range runs {
			status := r.Status
			if orchestrator.IsStale(r, a.now(), stale) {
				status += " (STALE)"
			}
			cost := "—"
			if r.CostUSDTotal != nil {
				cost = fmt.Sprintf("USD %.4f", *r.CostUSDTotal)
			}
			fmt.Fprintf(stdout, "%-28s %-10s %-24s %-22s %s\n", r.ID, r.Kind, status, r.CreatedAt, cost)
		}
		return 0
	}
	if !checkIDs(stderr, "run", fs.Arg(0)) {
		return 1
	}
	r, err := a.store.GetRun(ctx, fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	steps, _ := a.store.StepsForRun(ctx, r.ID)
	var sum domain.RunSummary
	_ = json.Unmarshal([]byte(r.SummaryJSON), &sum)
	if *asJSON {
		return writeJSON(stdout, map[string]any{"run": r, "steps": steps, "summary": sum, "stale": orchestrator.IsStale(r, a.now(), stale)})
	}
	status := r.Status
	if orchestrator.IsStale(r, a.now(), stale) {
		status += " (STALE: tidak ada detak sejak " + r.HeartbeatAt + "; jalankan ulang dengan --resume)"
	}
	fmt.Fprintf(stdout, "Run %s\n  jenis=%s status=%s\n  dibuat=%s mulai=%s selesai=%s\n  penyedia=%s konfigurasi=%s\n", r.ID, r.Kind, status, r.CreatedAt, r.StartedAt, r.FinishedAt, r.ProviderName, r.ConfigHash)
	for _, s := range steps {
		fmt.Fprintf(stdout, "  %-15s %-18s percobaan=%d %s\n", s.Name, s.Status, s.Attempts, s.ErrorJSON)
	}
	if sum.RunID != "" {
		sum.Steps = nil
		printSummary(stdout, &sum)
	}
	if r.Status == domain.RunFailed {
		return 2
	}
	return 0
}

func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

func cmdReport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "gunakan: mia report RUN_ID")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	if !checkIDs(stderr, "run", fs.Arg(0)) {
		return 1
	}
	r, err := a.store.GetRun(ctx, fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	var sum domain.RunSummary
	_ = json.Unmarshal([]byte(r.SummaryJSON), &sum)
	if sum.Status == "" {
		sum.Status = r.Status
	}
	var md, html string
	switch r.Kind {
	case "strategy":
		md, html, err = a.rend.RenderStrategy(ctx, r, &sum)
	case "content":
		md, html, err = a.rend.RenderContent(ctx, r, &sum)
	default:
		md, html, err = a.rend.Render(ctx, r, &sum)
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintf(stdout, "laporan: %s | %s\n", md, html)
	return 0
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	inputs := fs.String("inputs", "", "direktori berkas impor")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *inputs == "" {
		fmt.Fprintln(stderr, "gunakan: mia validate --inputs DIR")
		return 1
	}
	cfg, err := config.Load(*root)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	res, err := importer.ImportDir(*inputs, cfg.Sources.Imports.MaxFileBytes)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	items := 0
	for _, n := range res.Notes {
		items += len(n.Items)
		fmt.Fprintf(stdout, "OK   %-40s sumber=%q butir=%d\n", n.File, n.SourceName, len(n.Items))
	}
	for _, e := range res.Errors {
		fmt.Fprintf(stdout, "GAGAL %-40s %s\n", e.File, e.Message)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(stdout, "LEWAT %-40s format tidak didukung\n", s)
	}
	fmt.Fprintf(stdout, "catatan=%d butir=%d gagal=%d\n", len(res.Notes), items, len(res.Errors))
	if len(res.Errors) > 0 {
		return 2
	}
	return 0
}

func cmdConfigCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	asJSON := fs.Bool("json", false, "keluaran JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	drifts, warnings, err := a.orch.CheckWorkspace(ctx, "", nil)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	review := false
	for _, d := range drifts {
		if d.Status == "changed" || d.Status == "review_required" || d.Status == "missing" {
			review = true
		}
	}
	if *asJSON {
		return writeJSON(stdout, map[string]any{"config_hash": a.cfg.Hash, "provider": a.cfg.Limits.Provider.Name, "workspace_root": a.cfg.Sources.Workspace.Root, "drift": drifts, "warnings": warnings, "review_required": review})
	}
	fmt.Fprintf(stdout, "konfigurasi valid: hash=%s penyedia=%s model=%s plafon_bulanan=USD %.2f\n", a.cfg.Hash, a.cfg.Limits.Provider.Name, a.cfg.Limits.Provider.Model, a.cfg.Limits.Caps.MonthlyCeilingUSD)
	fmt.Fprintf(stdout, "ruang kerja (baca-saja): %s\n", a.cfg.Sources.Workspace.Root)
	for _, d := range drifts {
		fmt.Fprintf(stdout, "  %-14s %-16s %s\n", d.Key, d.Status, d.Path)
		for _, det := range d.Details {
			fmt.Fprintf(stdout, "      - %s\n", det)
		}
	}
	for _, w := range warnings {
		fmt.Fprintln(stdout, "  peringatan:", w)
	}
	if review {
		fmt.Fprintln(stdout, "TINJAUAN DIPERLUKAN: konfigurasi MIA tidak diubah otomatis.")
		return 2
	}
	fmt.Fprintln(stdout, "tidak ada perubahan yang memerlukan tinjauan")
	return 0
}

func cmdApproveAsset(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("approve asset", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	key := fs.String("key", "", "kunci aset")
	note := fs.String("note", "", "catatan tinjauan")
	by := fs.String("by", "CEO", "nama peninjau")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *key == "" || *note == "" {
		fmt.Fprintln(stderr, "gunakan: mia approve asset --key KEY --note TEKS")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	latest, err := a.store.LatestAsset(ctx, *key)
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "aset %q belum pernah diimpor; jalankan mia config check dulu\n", *key)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	if latest.ReviewStatus != domain.AssetReviewRequired {
		fmt.Fprintf(stdout, "aset %q tidak memerlukan tinjauan (status %s)\n", *key, latest.ReviewStatus)
		return 0
	}
	now := a.now().UTC().Format(time.RFC3339)
	if err := a.store.AcceptAsset(ctx, latest.ID, now, *note); err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	rec := &domain.ApprovalRecord{ID: a.ids.New("appr"), SubjectType: "imported_asset", SubjectID: latest.ID, Decision: "accepted_drift", DecidedBy: *by, Note: *note, DecidedAt: now}
	if err := a.store.InsertApproval(ctx, rec); err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintf(stdout, "aset %q diterima (sha256 %s…); perbarui config/*.yaml secara manual bila rincian perbedaan menuntutnya\n", *key, latest.SHA256[:12])
	return 0
}
