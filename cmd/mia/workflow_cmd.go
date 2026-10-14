package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/report"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/workflow"
)

// checkIDs validates ID arguments (label, value pairs; empty values are skipped) before any lookup.
func checkIDs(stderr io.Writer, kv ...string) bool {
	for i := 0; i+1 < len(kv); i += 2 {
		if v := kv[i+1]; v != "" && !ids.Valid(v) {
			fmt.Fprintf(stderr, "ditolak: %s %q bukan ID MIA yang valid\n", kv[i], termSafe(v))
			return false
		}
	}
	return true
}

func cmdWorkflow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	use := "gunakan: mia workflow start|run|resume|show|list|next|report|export ..."
	if len(args) < 1 {
		fmt.Fprintln(stderr, use)
		return 1
	}
	switch args[0] {
	case "start":
		return cmdWorkflowStart(ctx, args[1:], stdout, stderr)
	case "run":
		return cmdWorkflowAdvance(ctx, args[1:], workflow.ModeRun, stdout, stderr)
	case "resume":
		return cmdWorkflowAdvance(ctx, args[1:], workflow.ModeResume, stdout, stderr)
	case "show":
		return cmdWorkflowShow(ctx, args[1:], false, stdout, stderr)
	case "next":
		return cmdWorkflowShow(ctx, args[1:], true, stdout, stderr)
	case "list":
		return cmdWorkflowList(ctx, args[1:], stdout, stderr)
	case "report":
		return cmdWorkflowReport(ctx, args[1:], stdout, stderr)
	case "export":
		return cmdWorkflowExport(ctx, args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, use)
	return 1
}

func cmdWorkflowStart(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow start", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	inputs := fs.String("inputs", "", "direktori berkas riset")
	fixture := fs.String("fixture", "", "kunci fixture mock (default: nama direktori input)")
	newFlag := fs.Bool("new", false, "mulai workflow baru walaupun workflow terbuka untuk input yang sama sudah ada")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *inputs == "" {
		fmt.Fprintln(stderr, "gunakan: mia workflow start --inputs DIR [--fixture KEY] [--new]")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	wf, err := a.wf.Start(ctx, *inputs, *fixture, *newFlag)
	if err != nil {
		fmt.Fprintln(stderr, "ditolak:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Workflow %s dibuat (status %s)\n  input: %s\n  fixture: %s\n  langkah berikutnya: mia workflow run --id %s\n", wf.ID, wf.Status, wf.InputsDir, wf.FixtureKey, wf.ID)
	return 0
}

func cmdWorkflowAdvance(ctx context.Context, args []string, mode workflow.Mode, stdout, stderr io.Writer) int {
	name := map[workflow.Mode]string{workflow.ModeRun: "workflow run", workflow.ModeResume: "workflow resume"}[mode]
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id workflow")
	var newContent *string
	if mode == workflow.ModeResume {
		newContent = fs.String("new-content-run", "", "buat run konten baru secara sengaja untuk peluang ini setelah run sebelumnya gagal")
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}
	opts := workflow.AdvanceOptions{}
	if newContent != nil {
		opts.NewContentRunFor = *newContent
	}
	if *id == "" {
		fmt.Fprintf(stderr, "gunakan: mia %s --id WORKFLOW_ID\n", name)
		return 1
	}
	if !checkIDs(stderr, "workflow", *id, "peluang", opts.NewContentRunFor) {
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	res, err := a.wf.Advance(ctx, *id, mode, opts)
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "workflow %q tidak ditemukan\n", *id)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "ditolak:", err)
		return 1
	}
	if len(res.Actions) == 0 {
		fmt.Fprintln(stdout, "Tidak ada tahap otomatis yang aman untuk dijalankan sekarang.")
	}
	for _, act := range res.Actions {
		fmt.Fprintln(stdout, "dijalankan:", termSafe(act))
	}
	if res.Stopped != "" {
		fmt.Fprintln(stdout, "berhenti:", termSafe(res.Stopped))
	}
	printAssessment(stdout, res.Assessment, false)
	if s := res.Assessment.Status; s == domain.WfFailed || s == domain.WfBlocked {
		return 2
	}
	return 0
}

func loadAssessment(ctx context.Context, a *app, id string, stderr io.Writer) (*workflow.Assessment, bool) {
	as, err := a.wf.Assess(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "workflow %q tidak ditemukan\n", id)
		return nil, false
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return nil, false
	}
	return as, true
}

func cmdWorkflowShow(ctx context.Context, args []string, nextOnly bool, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id workflow")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		fmt.Fprintln(stderr, "gunakan: mia workflow show|next --id WORKFLOW_ID")
		return 1
	}
	if !checkIDs(stderr, "workflow", *id) {
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	as, ok := loadAssessment(ctx, a, *id, stderr)
	if !ok {
		return 1
	}
	printAssessment(stdout, as, nextOnly)
	return 0
}

func label(status string) string {
	if l := report.WorkflowStatusLabels[status]; l != "" {
		return l
	}
	return status
}

func numOr(p *int) string {
	if p == nil {
		return "tidak tersedia"
	}
	return fmt.Sprint(*p)
}

func countsLine(total *int, m map[string]int) string {
	if total == nil {
		return "tidak tersedia"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	parts := []string{"total " + fmt.Sprint(*total)}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, " · ")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// printAssessment prints a live workflow assessment (all values from the database).
func printAssessment(w io.Writer, as *workflow.Assessment, nextOnly bool) {
	wf := as.Workflow
	p := func(format string, x ...any) { fmt.Fprintf(w, format+"\n", x...) }
	if !nextOnly {
		p("Workflow %s: %s — %s", wf.ID, as.Status, label(as.Status))
		p("  alasan: %s", termSafe(as.Reason))
		p("  dibuat %s · ringkasan tersimpan %s (status %s) · dinilai dari basis data %s", wf.CreatedAt, wf.UpdatedAt, wf.Status, as.AssessedAt)
		if as.LeaseHeld {
			p("  sedang dimajukan oleh proses lain sampai %s", wf.LeaseExpiresAt)
		}
		p("  tahap:")
		for _, st := range as.Stages {
			line := fmt.Sprintf("    %-24s %-15s %s", st.Label, st.Status, strings.Join(st.RunIDs, ","))
			if st.Detail != "" {
				line += "  " + termSafe(st.Detail)
			}
			p("%s", line)
		}
		p("  bukti tervalidasi: %s · peluang: %s", numOr(as.Counts.ValidatedEvidence), countsLine(as.Counts.OpportunityTotal, as.Counts.Opportunities))
		p("  draf: %s · siap serah terima %s · diekspor %s · isian belum diisi %s · klaim perlu verifikasi %s",
			countsLine(as.Counts.DraftTotal, as.Counts.Drafts), numOr(as.Counts.DraftsReady), numOr(as.Counts.DraftsExported), numOr(as.Counts.Placeholders), numOr(as.Counts.ClaimsToVerify))
		if as.Counts.CostUSD != nil {
			p("  biaya (ledger): USD %.4f (estimasi tidak pasti USD %.4f)", *as.Counts.CostUSD, *as.Counts.CostUncertainUSD)
		}
		if len(as.Blockers) > 0 {
			p("  blokir dan kegagalan:")
			for _, b := range as.Blockers {
				p("    - %s", termSafe(b))
			}
		}
	}
	p("  keputusan manusia diperlukan: %s", map[bool]string{true: "ya", false: "tidak"}[as.HumanDecisionNeeded])
	p("  tindakan berikutnya (rekomendasi, belum dilakukan):")
	if len(as.NextActions) == 0 {
		p("    (tidak ada)")
	}
	for _, act := range as.NextActions {
		who := "otomatis"
		switch {
		case act.Human:
			who = "manusia"
		case act.Command == "":
			who = "info"
		}
		p("    - [%s] %s", who, termSafe(act.Text))
		if act.Command != "" {
			p("      $ %s", termSafe(act.Command))
		}
	}
}

func cmdWorkflowList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	limit := fs.Int("limit", 20, "jumlah workflow")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	wfs, err := a.store.ListWorkflows(ctx, *limit)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%-28s %-28s %-8s %-24s %-24s %s\n", "WORKFLOW", "STATUS (LANGSUNG)", "MANUSIA", "RUN RISET", "RUN STRATEGI", "DIPERBARUI")
	for _, w := range wfs {
		as, err := a.wf.Assess(ctx, w.ID)
		if err != nil {
			fmt.Fprintln(stderr, "kesalahan:", err)
			return 1
		}
		human := "tidak"
		if as.HumanDecisionNeeded {
			human = "ya"
		}
		fmt.Fprintf(stdout, "%-28s %-28s %-8s %-24s %-24s %s\n", w.ID, as.Status, human, orDash(w.ResearchRunID), orDash(w.StrategyRunID), w.UpdatedAt)
	}
	if len(wfs) == 0 {
		fmt.Fprintln(stdout, "(belum ada workflow)")
	}
	return 0
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func cmdWorkflowReport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id workflow")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" || !checkIDs(stderr, "workflow", *id) {
		fmt.Fprintln(stderr, "gunakan: mia workflow report --id WORKFLOW_ID")
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	as, err := a.wf.Sync(ctx, *id, "")
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "workflow %q tidak ditemukan\n", *id)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	md, html, err := a.rend.RenderWorkflow(as)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintf(stdout, "laporan workflow %s (status %s, dirender dari basis data): %s | %s\n", as.Workflow.ID, as.Status, md, html)
	return 0
}

func cmdWorkflowExport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("workflow export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id workflow")
	dir := fs.String("dir", "", "subdirektori relatif di dalam drafts/content (default: id workflow)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" || !checkIDs(stderr, "workflow", *id) {
		fmt.Fprintln(stderr, "gunakan: mia workflow export --id WORKFLOW_ID [--dir NAMA_RELATIF]")
		return 1
	}
	sub := *dir
	if sub == "" {
		sub = *id
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	as, ok := loadAssessment(ctx, a, *id, stderr)
	if !ok {
		return 1
	}
	exported, skipped := 0, 0
	for _, row := range as.Drafts {
		if !row.Ready {
			skipped++
			continue
		}
		d, v, ok := loadDraftView(ctx, a, row.ID, stderr)
		if !ok {
			return 1
		}
		target, err := exportDraft(ctx, a, d, v, filepath.Join(sub, d.ID+".md"))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		rel, err := filepath.Rel(a.cfg.Paths.DraftsDir, target)
		if err != nil {
			rel = target
		}
		if err := a.wf.RecordExport(ctx, as.Workflow.ID, d.ID, d.Version, rel); err != nil {
			fmt.Fprintln(stderr, "kesalahan:", err)
			return 1
		}
		exported++
		fmt.Fprintf(stdout, "diekspor sebagai DRAF: %s (draf %s versi %d)\n", target, d.ID, d.Version)
	}
	if exported == 0 {
		fmt.Fprintf(stderr, "tidak ada draf yang siap serah terima (%d draf belum siap: belum ditinjau, ditolak, diblokir, atau masih berisi [ISI: ...])\n", skipped)
		return 2
	}
	fmt.Fprintf(stdout, "%d draf siap serah terima diekspor; %d draf lain tidak diekspor karena belum siap. Ini serah terima manual, bukan publikasi.\n", exported, skipped)
	return 0
}
