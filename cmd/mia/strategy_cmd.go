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
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/orchestrator"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

func cmdRunStrategy(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run strategy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	from := fs.String("from-run", "", "id run riset yang menyediakan bukti tervalidasi")
	fixture := fs.String("fixture", "", "kunci fixture mock (default: kunci fixture run riset)")
	resume := fs.String("resume", "", "lanjutkan run strategi dengan id ini")
	force := fs.Bool("force", false, "jalankan ulang langkah yang sudah selesai")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *from == "" && *resume == "" {
		fmt.Fprintln(stderr, "gunakan: mia run strategy --from-run RUN_ID  (atau --resume RUN_ID)")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	sum, err := a.orch.RunStrategy(ctx, orchestrator.Options{FromRun: *from, FixtureKey: *fixture, ResumeID: *resume, Force: *force})
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	printSummary(stdout, sum)
	if sum.Strategy != nil {
		s := sum.Strategy
		fmt.Fprintf(stdout, "  bukti=%d usulan=%d digabung=%d dievaluasi=%d tingkatA=%d tingkatB=%d tingkatC=%d diblokir=%d ditolak=%d hipotesis=%d konflik=%d perlu_toko=%d\n",
			s.EvidenceItems, s.Proposed, s.Merged, s.Evaluated, s.TierA, s.TierB, s.TierC, s.Blocked, s.Rejected, s.Hypotheses, s.Conflicted, s.StoreDependent)
	}
	if sum.Status == domain.RunFailed {
		return 2
	}
	return 0
}

func cmdOpportunities(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("opportunities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	runID := fs.String("run", "", "id run strategi")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *runID == "" {
		fmt.Fprintln(stderr, "gunakan: mia opportunities --run RUN_ID")
		return 1
	}
	a, err := newApp(*root, io.Discard)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	opps, err := a.store.OpportunitiesForRun(ctx, *runID)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%-28s %-5s %-22s %-7s %-5s %s\n", "ID", "TIER", "STATUS", "SEGMEN", "ORD", "JUDUL")
	for _, o := range opps {
		flags := ""
		if o.Hypothesis {
			flags += " [hipotesis]"
		}
		if o.RequiresStore {
			flags += " [perlu toko]"
		}
		fmt.Fprintf(stdout, "%-28s %-5s %-22s %-7s %2d/15 %s%s\n", o.ID, o.Tier, o.Status, o.Segment, o.OrdinalSum, o.Title, flags)
	}
	if len(opps) == 0 {
		fmt.Fprintln(stdout, "(tidak ada peluang untuk run ini)")
	}
	return 0
}

var decisionStatus = map[string]string{
	"approved":            domain.OppApproved,
	"rejected":            domain.OppRejectedByReview,
	"needs_more_evidence": domain.OppNeedsEvidence,
}

func cmdApproveOpportunity(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("approve opportunity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id peluang")
	decision := fs.String("decision", "", "approved | rejected | needs_more_evidence")
	note := fs.String("note", "", "catatan tinjauan")
	by := fs.String("by", "CEO", "nama peninjau")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	status, ok := decisionStatus[*decision]
	if *id == "" || !ok || *note == "" {
		fmt.Fprintln(stderr, "gunakan: mia approve opportunity --id ID --decision approved|rejected|needs_more_evidence --note TEKS")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	opp, err := a.store.GetOpportunity(ctx, *id)
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(stderr, "peluang %q tidak ditemukan\n", *id)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	if opp.Status == domain.OppBlocked || opp.Status == domain.OppRejected {
		fmt.Fprintf(stderr, "peluang %s berstatus %s dan tidak dapat ditinjau; perbaiki usulan dan jalankan ulang strategi\n", opp.ID, opp.Status)
		return 1
	}
	now := a.now().UTC().Format(time.RFC3339)
	rec := &domain.ApprovalRecord{ID: a.ids.New("appr"), SubjectType: "opportunity", SubjectID: opp.ID, Decision: *decision, DecidedBy: *by, Note: *note, DecidedAt: now}
	if err := a.store.InsertApproval(ctx, rec); err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	if err := a.store.UpdateOpportunityStatus(ctx, opp.ID, status); err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	history, _ := a.store.ApprovalsFor(ctx, "opportunity", opp.ID)
	fmt.Fprintf(stdout, "peluang %s: keputusan %q dicatat (riwayat: %d keputusan); status sekarang %s\n", opp.ID, *decision, len(history), status)
	if status == domain.OppApproved {
		fmt.Fprintf(stdout, "ekspor brief untuk serah terima manual: mia export brief --id %s\n", opp.ID)
	}
	return 0
}

func cmdExportBrief(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export brief", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "direktori proyek MIA")
	id := fs.String("id", "", "id peluang yang sudah disetujui")
	out := fs.String("out", "", "berkas tujuan (default: drafts/<id>.md)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		fmt.Fprintln(stderr, "gunakan: mia export brief --id ID [--out PATH]")
		return 1
	}
	a, err := newApp(*root, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	defer a.close()
	opp, err := a.store.GetOpportunity(ctx, *id)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	if opp.Status != domain.OppApproved {
		fmt.Fprintf(stderr, "hanya peluang berstatus %s yang dapat diekspor; status sekarang %s\n", domain.OppApproved, opp.Status)
		return 2
	}
	view, err := a.rend.OppViewFor(ctx, opp)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	body, err := a.rend.RenderBrief(view)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	target := *out
	if target == "" {
		target = filepath.Join(a.cfg.Paths.DraftsDir, opp.ID+".md")
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	// Never write into the read-only Punyamas workspace: handoff is manual.
	if under(abs, a.cfg.Sources.Workspace.Root) {
		fmt.Fprintf(stderr, "tujuan %s berada di dalam ruang kerja Punyamas (baca-saja); ekspor ke luar ruang kerja lalu serahkan manual\n", abs)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	if err := os.WriteFile(abs, body, 0o644); err != nil {
		fmt.Fprintln(stderr, "kesalahan:", err)
		return 1
	}
	fmt.Fprintf(stdout, "brief diekspor: %s (serah terima ke ruang kerja Punyamas dilakukan manual)\n", abs)
	return 0
}

// under reports whether path lies inside root (symlinks resolved).
func under(path, root string) bool {
	if root == "" {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if rr, err := filepath.EvalSymlinks(r); err == nil {
		r = rr
	}
	p := path
	if pp, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		p = filepath.Join(pp, filepath.Base(p))
	}
	return p == r || strings.HasPrefix(p, r+string(filepath.Separator))
}
