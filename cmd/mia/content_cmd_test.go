package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

// draftRows parses `mia content list` output into id -> {type, variant, status}.
func draftRows(out string) map[string][3]string {
	rows := map[string][3]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 && strings.HasPrefix(f[0], "draft_") {
			rows[f[0]] = [3]string{f[1], f[2], f[3]}
		}
	}
	return rows
}

func findDraft(t *testing.T, rows map[string][3]string, typ, variant string) string {
	t.Helper()
	for id, r := range rows {
		if r[0] == typ && r[1] == variant {
			return id
		}
	}
	t.Fatalf("no %s variant %s in %v", typ, variant, rows)
	return ""
}

func TestCLIContentWorkflowIsolated(t *testing.T) {
	root := testutil.RepoRoot(t)
	tmp := t.TempDir()
	data, drafts := filepath.Join(tmp, "data"), filepath.Join(tmp, "drafts")
	t.Setenv("MIA_DATA_DIR", data)
	t.Setenv("MIA_REPORTS_DIR", filepath.Join(tmp, "reports"))
	t.Setenv("MIA_DRAFTS_DIR", drafts)
	workspace := filepath.Join(root, "testdata", "workspace-mini")
	t.Setenv("MIA_WORKSPACE_ROOT", workspace)
	var out, errb bytes.Buffer
	reset := func() { out.Reset(); errb.Reset() }
	mustRun := func(code int, args ...string) string {
		t.Helper()
		reset()
		if got := run(append(args[:1:1], append([]string{}, args[1:]...)...), &out, &errb); got != code {
			t.Fatalf("%v: exit %d, want %d\nout=%s\nerr=%s", args, got, code, out.String(), errb.String())
		}
		return out.String() + errb.String()
	}

	research := firstRunID(t, mustRun(0, "run", "research", "--root", root, "--inputs", filepath.Join(root, "testdata", "sample-batch")))
	strategyID := firstRunID(t, mustRun(0, "run", "strategy", "--root", root, "--from-run", research))
	listing := mustRun(0, "opportunities", "--root", root, "--run", strategyID)
	var oppID, pendingID string
	for _, line := range strings.Split(listing, "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && strings.Contains(line, "Tunjukkan penimbangan dan pengujian") {
			oppID = f[0]
		} else if len(f) > 0 && strings.Contains(line, "Jawab pertanyaan tentang KTP") {
			pendingID = f[0]
		}
	}
	if oppID == "" || pendingID == "" {
		t.Fatalf("opportunities not found in:\n%s", listing)
	}
	mustRun(1, "run", "content", "--root", root)
	if s := mustRun(1, "run", "content", "--root", root, "--opportunity", oppID); !strings.Contains(s, "status peluang proposed") {
		t.Fatalf("an unapproved opportunity must be refused: %s", s)
	}
	mustRun(0, "approve", "opportunity", "--root", root, "--id", oppID, "--decision", "approved", "--note", "Siap dibuatkan konten")
	if s := mustRun(1, "run", "content", "--root", root, "--opportunity", pendingID); !strings.Contains(s, "tidak memenuhi syarat") {
		t.Fatalf("pending opportunity: %s", s)
	}
	runOut := mustRun(0, "run", "content", "--root", root, "--opportunity", oppID)
	contentRun := firstRunID(t, runOut)
	if !strings.Contains(runOut, "diusulkan=10") || !strings.Contains(runOut, "belum dipublikasikan") {
		t.Fatalf("run content output: %s", runOut)
	}
	if s := mustRun(1, "run", "content", "--root", root, "--opportunity", oppID); !strings.Contains(s, "--regenerate") {
		t.Fatalf("second generation without --regenerate must be refused: %s", s)
	}
	list := mustRun(0, "content", "list", "--root", root, "--run", contentRun)
	if !strings.Contains(list, "SEMUA DRAF BELUM DIPUBLIKASIKAN") || !strings.Contains(list, oppID) {
		t.Fatalf("list: %s", list)
	}
	rows := draftRows(list)
	if len(rows) != 10 {
		t.Fatalf("rows = %d", len(rows))
	}
	clean := findDraft(t, rows, "instagram_caption", "1")
	withPH := findDraft(t, rows, "instagram_caption", "2")
	if rows[clean][2] != "draft" || rows[withPH][2] != "needs_review" {
		t.Fatalf("statuses %v %v", rows[clean], rows[withPH])
	}
	show := mustRun(0, "content", "show", "--root", root, "--id", withPH)
	for _, want := range []string{"DRAF — BELUM DIPUBLIKASIKAN", "Siap serah terima: TIDAK", "Isian wajib: [ISI: kontak resmi Punyamas]", "Peluang:    " + oppID} {
		if !strings.Contains(show, want) {
			t.Errorf("show lacks %q", want)
		}
	}
	mustRun(1, "content", "review", "--root", root, "--id", clean, "--decision", "published", "--note", "x")
	if s := mustRun(0, "content", "review", "--root", root, "--id", clean, "--decision", "approved", "--note", "Draf diperiksa"); !strings.Contains(s, "bukan izin publikasi") || !strings.Contains(s, "riwayat: 1") {
		t.Fatalf("review: %s", s)
	}
	if s := mustRun(0, "content", "review", "--root", root, "--id", withPH, "--decision", "approved", "--note", "Isian menyusul"); !strings.Contains(s, "belum siap serah terima") {
		t.Fatalf("approved draft with placeholders must report it is not ready: %s", s)
	}

	// Export: DRAF label, review metadata, placeholders; default location under drafts/content.
	mustRun(0, "content", "export", "--root", root, "--id", clean)
	exp, err := os.ReadFile(filepath.Join(drafts, "content", clean+".md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DRAF — BELUM DIPUBLIKASIKAN DAN BUKAN IZIN PUBLIKASI", "Draf diperiksa", "Siap serah terima ke proses persetujuan publikasi:** YA", "Riwayat tinjauan (hanya-tambah)", "Provenance"} {
		if !strings.Contains(string(exp), want) {
			t.Errorf("export lacks %q", want)
		}
	}
	mustRun(0, "content", "export", "--root", root, "--id", withPH, "--out", "tinjauan/caption-2")
	exp2, _ := os.ReadFile(filepath.Join(drafts, "content", "tinjauan", "caption-2.md"))
	if !strings.Contains(string(exp2), "Isian wajib diisi:** `[ISI: kontak resmi Punyamas]`") || !strings.Contains(string(exp2), "| TIDAK |") {
		t.Fatalf("export with placeholders must list them and say not ready:\n%s", exp2)
	}
	for _, bad := range []string{"../keluar", "../../x.md", "/tmp/x.md", "a/../../x"} {
		if s := mustRun(1, "content", "export", "--root", root, "--id", clean, "--out", bad); !strings.Contains(s, "ditolak") {
			t.Fatalf("%q must be refused: %s", bad, s)
		}
	}
	// Exports into the Punyamas workspace are refused even when the drafts directory is configured there.
	t.Setenv("MIA_DRAFTS_DIR", filepath.Join(workspace, "docs"))
	if s := mustRun(1, "content", "export", "--root", root, "--id", clean); !strings.Contains(s, "ruang kerja") {
		t.Fatalf("workspace export must be refused: %s", s)
	}
	if _, err := os.Stat(filepath.Join(workspace, "docs", "content")); err == nil {
		t.Fatal("nothing may be written into the workspace")
	}
	// A failed export leaves the stored draft unchanged and no partial file.
	broken := filepath.Join(tmp, "broken")
	_ = os.MkdirAll(broken, 0o755)
	if err := os.WriteFile(filepath.Join(broken, "content"), []byte("bukan direktori"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(data, "mia.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := st.GetContentDraft(context.Background(), clean)
	st.Close()
	t.Setenv("MIA_DRAFTS_DIR", broken)
	if s := mustRun(1, "content", "export", "--root", root, "--id", clean); !strings.Contains(s, "draf tersimpan tidak berubah") {
		t.Fatalf("export failure: %s", s)
	}
	st, _ = store.Open(filepath.Join(data, "mia.sqlite"))
	after, _ := st.GetContentDraft(context.Background(), clean)
	versions, _ := st.ContentDraftVersions(context.Background(), clean)
	st.Close()
	if after.Status != before.Status || after.Version != before.Version || after.ContentHash != before.ContentHash || after.UpdatedAt != before.UpdatedAt || len(versions) != 1 {
		t.Fatalf("draft changed by a failed export: %+v vs %+v", before, after)
	}
	entries, _ := os.ReadDir(broken)
	if len(entries) != 1 {
		t.Fatalf("failed export left files behind: %v", entries)
	}
	t.Setenv("MIA_DRAFTS_DIR", drafts)

	// Revision after approval returns the draft to review; the export reflects it.
	bodyFile := filepath.Join(tmp, "isi.txt")
	_ = os.WriteFile(bodyFile, []byte("Kami akan menimbang dan menguji barang di depan Bapak atau Ibu. Dasar hitung akan ditampilkan."), 0o644)
	mustRun(1, "content", "revise", "--root", root, "--id", clean, "--note", "tanpa perubahan")
	if s := mustRun(0, "content", "revise", "--root", root, "--id", clean, "--body-file", bodyFile, "--note", "Menyederhanakan isi"); !strings.Contains(s, "versi 2") || !strings.Contains(s, "needs_review") {
		t.Fatalf("revise: %s", s)
	}
	mustRun(0, "content", "export", "--root", root, "--id", clean)
	exp, _ = os.ReadFile(filepath.Join(drafts, "content", clean+".md"))
	if !strings.Contains(string(exp), "| needs\\_review | 2 |") || !strings.Contains(string(exp), "Menyederhanakan isi") {
		t.Fatalf("export after revision:\n%s", exp)
	}

	// Blocked drafts can be neither reviewed nor exported.
	gatesRun := firstRunID(t, mustRun(0, "run", "content", "--root", root, "--opportunity", oppID, "--fixture", "gates", "--regenerate"))
	grows := draftRows(mustRun(0, "content", "list", "--root", root, "--run", gatesRun))
	blocked := findDraft(t, grows, "instagram_caption", "1")
	if grows[blocked][2] != "blocked" {
		t.Fatalf("gates draft status %v", grows[blocked])
	}
	if s := mustRun(1, "content", "review", "--root", root, "--id", blocked, "--decision", "approved", "--note", "coba"); !strings.Contains(s, "diblokir") {
		t.Fatalf("blocked review: %s", s)
	}
	mustRun(2, "content", "export", "--root", root, "--id", blocked)
	if s := mustRun(0, "content", "list", "--root", root, "--opportunity", oppID); len(draftRows(s)) != 19 {
		t.Fatalf("list by opportunity: %d rows", len(draftRows(s)))
	}
	if s := mustRun(0, "report", "--root", root, contentRun); !strings.Contains(s, "content.md") {
		t.Fatalf("report re-render: %s", s)
	}
	if _, err := os.Stat(filepath.Join(root, "drafts", "content")); err == nil {
		t.Fatal("exports leaked into the repository drafts directory")
	}
}
