package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/report"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/safepath"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

const maxBodyFileBytes = 64 << 10

// termSafe drops control characters (ANSI escapes included) from untrusted text printed to a terminal.
func termSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(termSafe(s)), " ")
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = string(r[:max-1]) + "…"
	}
	return s
}

func cmdRunContent(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run content", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	opp := fs.String("opportunity", "", "id peluang berstatus approved")
	types := fs.String("types", "", "jenis konten dipisah koma (default: default_types di config/content.yaml)")
	fixture := fs.String("fixture", "", "kunci fixture mock (default: <kunci fixture strategi>-<topik peluang>)")
	regenerate := fs.Bool("regenerate", false, "buat ulang draf secara sengaja walaupun peluang sudah memiliki draf")
	resume := fs.String("resume", "", "lanjutkan run konten dengan id ini")
	force := fs.Bool("force", false, "jalankan ulang langkah yang sudah selesai (tanpa menggandakan draf)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *opp == "" && *resume == "" {
		fmt.Fprintln(stderr, "gunakan: mia run content --opportunity ID  (atau --resume RUN_ID)")
		return 1
	}
	var list []string
	for _, t := range strings.Split(*types, ",") {
		if t = strings.TrimSpace(t); t != "" {
			list = append(list, t)
		}
	}
	if !checkIDs(stderr, "peluang", *opp, "run", *resume) {
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	sum, err := a.orch.RunContent(ctx, orchestrator.Options{OpportunityID: *opp, ContentTypes: list, FixtureKey: *fixture, Regenerate: *regenerate, ResumeID: *resume, Force: *force})
	if err != nil {
		fmt.Fprintln(stderr, "ditolak:", err)
		return 1
	}
	printSummary(stdout, sum)
	if c := sum.Content; c != nil {
		fmt.Fprintf(stdout, "  draf: diusulkan=%d ditolak_struktural=%d duplikat=%d disimpan=%d perlu_tinjauan=%d lolos_gerbang=%d diblokir=%d isian=%d klaim_verifikasi=%d\n",
			c.Proposed, c.StructuralRejected, c.DuplicatesSkipped, c.Stored, c.NeedsReview, c.Draft, c.Blocked, c.Placeholders, c.ClaimsToVerify)
		fmt.Fprintln(stdout, "  semua draf berstatus DRAF dan belum dipublikasikan; lihat: mia content list --run", sum.RunID)
	}
	if sum.Status == domain.RunFailed {
		return 2
	}
	return 0
}

func cmdContent(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	use := "gunakan: mia content list|show|review|revise|export ..."
	if len(args) < 1 {
		fmt.Fprintln(stderr, use)
		return 1
	}
	switch args[0] {
	case "list":
		return cmdContentList(ctx, args[1:], stdout, stderr)
	case "show":
		return cmdContentShow(ctx, args[1:], stdout, stderr)
	case "review":
		return cmdContentReview(ctx, args[1:], stdout, stderr)
	case "revise":
		return cmdContentRevise(ctx, args[1:], stdout, stderr)
	case "export":
		return cmdContentExport(ctx, args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, use)
	return 1
}

func cmdContentList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	runID := fs.String("run", "", "id run konten")
	oppID := fs.String("opportunity", "", "id peluang")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if (*runID == "") == (*oppID == "") {
		fmt.Fprintln(stderr, "gunakan: mia content list --run RUN_ID  atau  --opportunity ID")
		return 1
	}
	if !checkIDs(stderr, "run", *runID, "peluang", *oppID) {
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	var drafts []*domain.ContentDraft
	if *runID != "" {
		drafts, err = a.store.ContentDraftsForRun(ctx, *runID)
	} else {
		drafts, err = a.store.ContentDraftsForOpportunity(ctx, *oppID)
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintln(stdout, "SEMUA DRAF BELUM DIPUBLIKASIKAN (DRAF).")
	fmt.Fprintf(stdout, "%-30s %-19s %-3s %-13s %-28s %-6s %-6s %-5s %s\n", "ID", "JENIS", "VAR", "STATUS", "PELUANG", "ISIAN", "KLAIM", "SIAP", "JUDUL/HOOK")
	for _, d := range drafts {
		dt := content.DecodeDetails(d)
		reviews, _ := a.store.ApprovalsFor(ctx, domain.SubjectContentDraft, d.ID)
		ready := "tidak"
		if content.ReadinessOf(d, reviews).Ready {
			ready = "ya"
		}
		title := d.Headline
		if title == "" {
			title = d.Hook
		}
		if title == "" {
			title = d.Body
		}
		fmt.Fprintf(stdout, "%-30s %-19s %-3d %-13s %-28s %-6d %-6d %-5s %s\n", d.ID, d.ContentType, d.Variant, d.Status, d.OpportunityID, len(dt.Placeholders), len(dt.ClaimsToVerify()), ready, oneLine(title, 60))
	}
	if len(drafts) == 0 {
		fmt.Fprintln(stdout, "(tidak ada draf)")
	}
	return 0
}

func loadDraftView(ctx context.Context, a *app, id string, stderr io.Writer) (*domain.ContentDraft, *report.DraftView, bool) {
	if !checkIDs(stderr, "draf", id) {
		return nil, nil, false
	}
	d, err := a.store.GetContentDraft(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "draf %q tidak ditemukan\n", id)
		return nil, nil, false
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return nil, nil, false
	}
	v, err := a.rend.DraftViewFor(ctx, d)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return nil, nil, false
	}
	return d, v, true
}

func cmdContentShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id draf")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		fmt.Fprintln(stderr, "gunakan: mia content show --id DRAFT_ID")
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	_, v, ok := loadDraftView(ctx, a, *id, stderr)
	if !ok {
		return 1
	}
	w := func(format string, x ...any) { fmt.Fprintf(stdout, format+"\n", x...) }
	w("DRAF — BELUM DIPUBLIKASIKAN. Persetujuan di MIA bukan izin publikasi.")
	w("Draf:       %s (%s, varian %d, versi %d)", v.ID, v.ContentType, v.Variant, v.Version)
	w("Status:     %s — %s", v.Status, v.StatusLabel)
	w("Peluang:    %s (run strategi %s, run riset %s)", v.OpportunityID, v.StrategyRunID, v.ResearchRunID)
	w("Kanal:      %s · segmen %s", v.Channel, v.Segment)
	w("Tujuan:     %s", oneLine(v.Objective, 300))
	if v.Readiness.Ready {
		w("Siap serah terima: YA (bukan izin publikasi)")
	} else {
		w("Siap serah terima: TIDAK")
		for _, r := range v.Readiness.Reasons {
			w("  - %s", termSafe(r))
		}
	}
	if v.Hook != "" {
		w("Hook:       %s", oneLine(v.Hook, 300))
	}
	if v.Headline != "" {
		w("Headline:   %s", oneLine(v.Headline, 300))
	}
	w("Isi:")
	for _, line := range strings.Split(termSafe(v.Body), "\n") {
		w("  | %s", line)
	}
	if v.CTA != "" {
		w("CTA:        %s", oneLine(v.CTA, 300))
	}
	for _, p := range v.Placeholders {
		w("Isian wajib: %s", termSafe(p))
	}
	for _, c := range v.ClaimsToVerify {
		w("Klaim perlu verifikasi: %s", oneLine(c, 300))
	}
	for _, e := range v.Evidence {
		w("Bukti:      %s [%s] %s %s — %s", e.ID, termSafe(e.Ref), e.Kind, e.Quality, oneLine(e.Statement, 200))
	}
	for _, f := range v.BrandFacts {
		w("Fakta usaha: %s (%s)", f.ID, f.Status)
	}
	for _, f := range v.Findings {
		w("Temuan:     %s/%s (%s) %s %s", f.Gate, f.Category, f.Severity, termSafe(f.Reason), oneLine(f.Match, 80))
	}
	for _, rv := range v.Reviews {
		w("Tinjauan:   %s %s v%d oleh %s — %s", rv.At, rv.Decision, rv.Version, oneLine(rv.By, 60), oneLine(rv.Note, 200))
	}
	w("Provenance: eksekusi %s · konfigurasi %s · aturan konten %s · fixture %s · hash isi %s", v.ExecutionID, v.ConfigHash, v.ContentConfigHash, v.FixtureHash, v.ContentHash)
	return 0
}

func cmdContentReview(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id draf")
	decision := fs.String("decision", "", "approved | rejected")
	note := fs.String("note", "", "catatan tinjauan")
	by := fs.String("by", "CEO", "nama peninjau")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" || *decision == "" || strings.TrimSpace(*note) == "" {
		fmt.Fprintln(stderr, "gunakan: mia content review --id DRAFT_ID --decision approved|rejected --note TEKS")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	if !checkIDs(stderr, "draf", *id) {
		return 1
	}
	d, err := content.Review(ctx, a.store, a.ids, a.now(), *id, *decision, *note, *by)
	if err != nil {
		fmt.Fprintln(stderr, "ditolak:", err)
		return 1
	}
	reviews, _ := a.store.ApprovalsFor(ctx, domain.SubjectContentDraft, d.ID)
	fmt.Fprintf(stdout, "draf %s versi %d: keputusan %q dicatat (riwayat: %d keputusan); status sekarang %s\n", d.ID, d.Version, *decision, len(reviews), d.Status)
	if r := content.ReadinessOf(d, reviews); !r.Ready {
		fmt.Fprintln(stdout, "belum siap serah terima:", strings.Join(r.Reasons, "; "))
	}
	fmt.Fprintln(stdout, "persetujuan di MIA bukan izin publikasi; MIA tidak menerbitkan atau mengirim apa pun")
	return 0
}

func cmdContentRevise(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content revise", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id draf")
	hook := fs.String("hook", "", "hook baru")
	headline := fs.String("headline", "", "headline baru")
	bodyFile := fs.String("body-file", "", "berkas teks berisi isi draf baru")
	cta := fs.String("cta", "", "ajakan (CTA) baru")
	note := fs.String("note", "", "catatan revisi")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if *id == "" || strings.TrimSpace(*note) == "" || !(set["hook"] || set["headline"] || set["body-file"] || set["cta"]) {
		fmt.Fprintln(stderr, "gunakan: mia content revise --id DRAFT_ID [--hook T] [--headline T] [--body-file PATH] [--cta T] --note TEKS")
		return 1
	}
	var edit content.Edit
	if set["hook"] {
		edit.Hook = hook
	}
	if set["headline"] {
		edit.Headline = headline
	}
	if set["cta"] {
		edit.CallToAction = cta
	}
	if set["body-file"] {
		info, err := os.Stat(*bodyFile)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxBodyFileBytes {
			fmt.Fprintf(stderr, "berkas isi %q tidak dapat dibaca atau melebihi %d byte\n", *bodyFile, maxBodyFileBytes)
			return 1
		}
		b, err := os.ReadFile(*bodyFile)
		if err != nil || !utf8.Valid(b) {
			fmt.Fprintf(stderr, "berkas isi %q harus teks UTF-8\n", *bodyFile)
			return 1
		}
		body := string(b)
		edit.Body = &body
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	if !checkIDs(stderr, "draf", *id) {
		return 1
	}
	ev, err := content.New(a.cfg, a.checker)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	d, r, err := content.Revise(ctx, a.store, ev, a.cfg, a.now(), *id, edit, *note)
	if err != nil {
		fmt.Fprintln(stderr, "ditolak:", err)
		return 1
	}
	fmt.Fprintf(stdout, "draf %s direvisi ke versi %d; status sekarang %s (%s)\n", d.ID, d.Version, d.Status, content.StatusLabels[d.Status])
	for _, f := range r.Findings {
		if f.Severity != content.SeverityInfo {
			fmt.Fprintf(stdout, "  temuan %s/%s (%s): %s\n", f.Gate, f.Category, f.Severity, termSafe(f.Reason))
		}
	}
	return 0
}

func cmdContentExport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("content export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id draf")
	out := fs.String("out", "", "nama berkas relatif di dalam drafts/content (default: <id>.md)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		fmt.Fprintln(stderr, "gunakan: mia content export --id DRAFT_ID [--out NAMA_RELATIF]")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	d, v, ok := loadDraftView(ctx, a, *id, stderr)
	if !ok {
		return 1
	}
	if d.Status == domain.DraftBlocked || d.Status == domain.DraftRejected {
		fmt.Fprintf(stderr, "draf berstatus %s tidak diekspor untuk serah terima; revisi atau buat ulang\n", d.Status)
		return 2
	}
	name := *out
	if name == "" {
		name = d.ID + ".md"
	}
	target, err := exportDraft(ctx, a, d, v, name)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "draf diekspor sebagai DRAF: %s (status %s; serah terima manual, bukan publikasi)\n", target, d.Status)
	return 0
}

// exportDraft renders the DRAF export of one draft and writes it atomically under drafts/content/<name>.
// It is the only export path for content drafts (used by content export and workflow export); it never
// writes the database and refuses targets outside the drafts directory or inside the Punyamas workspace.
func exportDraft(ctx context.Context, a *app, d *domain.ContentDraft, v *report.DraftView, name string) (string, error) {
	target, err := safepath.ResolveDraftPath(filepath.Join(a.cfg.Paths.DraftsDir, "content"), name, a.cfg.Sources.Workspace.Root)
	if err != nil {
		return "", fmt.Errorf("ditolak: %w", err)
	}
	var oppView *report.OppView
	if op, err := a.store.GetOpportunity(ctx, d.OpportunityID); err == nil {
		oppView, _ = a.rend.OppViewFor(ctx, op)
	}
	body, err := a.rend.RenderContentExport(v, oppView)
	if err != nil {
		return "", fmt.Errorf("kesalahan: %w", err)
	}
	if err := safepath.WriteAtomic(target, body, 0o644); err != nil {
		return "", fmt.Errorf("ekspor gagal (draf tersimpan tidak berubah): %w", err)
	}
	return target, nil
}
