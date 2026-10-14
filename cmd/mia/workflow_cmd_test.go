package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

// treeHash fingerprints every file under dir (paths and contents).
func treeHash(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		h.Write([]byte(p))
		h.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// (20, 25) Definition of done through the CLI, isolated from the real Punyamas workspace.
func TestCLIWorkflowEndToEnd(t *testing.T) {
	root := testutil.RepoRoot(t)
	tmp := t.TempDir()
	drafts := filepath.Join(tmp, "drafts")
	t.Setenv("MIA_DATA_DIR", filepath.Join(tmp, "data"))
	t.Setenv("MIA_REPORTS_DIR", filepath.Join(tmp, "reports"))
	t.Setenv("MIA_DRAFTS_DIR", drafts)
	workspace := filepath.Join(root, "testdata", "workspace-mini")
	t.Setenv("MIA_WORKSPACE_ROOT", workspace)
	before := treeHash(t, workspace)
	var out, errb bytes.Buffer
	mustRun := func(code int, args ...string) string {
		t.Helper()
		out.Reset()
		errb.Reset()
		if got := run(args, &out, &errb); got != code {
			t.Fatalf("%v: exit %d, want %d\nout=%s\nerr=%s", args, got, code, out.String(), errb.String())
		}
		return out.String() + errb.String()
	}

	// Invalid inputs leave no workflow behind; invalid IDs are refused before any lookup.
	if s := mustRun(1, "workflow", "start", "--root", root, "--inputs", filepath.Join(root, "testdata", "failures")); !strings.Contains(s, "input riset tidak valid") {
		t.Fatalf("invalid inputs: %s", s)
	}
	if s := mustRun(0, "workflow", "list", "--root", root); !strings.Contains(s, "belum ada workflow") {
		t.Fatalf("no half-made workflow: %s", s)
	}
	for _, bad := range []string{"../x", "wf_x;rm", "WF_0001"} {
		if s := mustRun(1, "workflow", "show", "--root", root, "--id", bad); !strings.Contains(s, "bukan ID MIA yang valid") {
			t.Fatalf("id %q: %s", bad, s)
		}
	}
	mustRun(1, "content", "show", "--root", root, "--id", "draft_../../x")

	// 1. Create the workflow from the fixture batch.
	start := mustRun(0, "workflow", "start", "--root", root, "--inputs", filepath.Join(root, "testdata", "sample-batch"))
	wf := regexp.MustCompile(`Workflow (wf_[0-9a-z]+)`).FindStringSubmatch(start)[1]
	mustRun(1, "workflow", "start", "--root", root, "--inputs", filepath.Join(root, "testdata", "sample-batch"))
	// 2. Research and strategy run once; repeating creates nothing.
	if s := mustRun(0, "workflow", "run", "--root", root, "--id", wf); !strings.Contains(s, "awaiting_opportunity_review") || !strings.Contains(s, "menjalankan riset") {
		t.Fatalf("run: %s", s)
	}
	if s := mustRun(0, "workflow", "run", "--root", root, "--id", wf); !strings.Contains(s, "Tidak ada tahap otomatis") {
		t.Fatalf("repeat: %s", s)
	}
	status := mustRun(0, "status", "--root", root)
	if strings.Count(status, " research ") != 1 || strings.Count(status, " strategy ") != 1 {
		t.Fatalf("duplicate runs:\n%s", status)
	}
	// 3. Manual opportunity review; the workflow lists the exact commands.
	next := mustRun(0, "workflow", "next", "--root", root, "--id", wf)
	oppIDs := regexp.MustCompile(`approve opportunity --id (opp_[0-9a-z]+)`).FindAllStringSubmatch(next, -1)
	if len(oppIDs) != 4 || !strings.Contains(next, "keputusan manusia diperlukan: ya") {
		t.Fatalf("next:\n%s", next)
	}
	show := mustRun(0, "workflow", "show", "--root", root, "--id", wf)
	target := regexp.MustCompile(`Tinjau peluang (opp_[0-9a-z]+) \("Tunjukkan penimbangan`).FindStringSubmatch(show)[1]
	for _, m := range oppIDs {
		decision := "rejected"
		if m[1] == target {
			decision = "approved"
		}
		mustRun(0, "approve", "opportunity", "--root", root, "--id", m[1], "--decision", decision, "--note", "keputusan uji")
	}
	// 4–5. Resume after the approval: drafts are generated, never approved automatically.
	if s := mustRun(0, "workflow", "resume", "--root", root, "--id", wf); !strings.Contains(s, "awaiting_content_review") || !strings.Contains(s, "membuat draf konten") {
		t.Fatalf("after approval: %s", s)
	}
	list := mustRun(0, "content", "list", "--root", root, "--opportunity", target)
	rows := draftRows(list)
	if len(rows) != 10 {
		t.Fatalf("drafts: %d", len(rows))
	}
	var rejected string
	for id, r := range rows {
		if r[2] == "approved" || r[2] == "rejected" {
			t.Fatal("no draft may start with a decision")
		}
		if r[2] == "draft" {
			mustRun(0, "content", "review", "--root", root, "--id", id, "--decision", "approved", "--note", "siap")
		} else {
			mustRun(0, "content", "review", "--root", root, "--id", id, "--decision", "rejected", "--note", "tidak dipakai")
			rejected = id
		}
	}
	// 6–7. The status and report reflect the current database state.
	if s := mustRun(0, "workflow", "show", "--root", root, "--id", wf); !strings.Contains(s, ": completed") || !strings.Contains(s, "siap serah terima 5") {
		t.Fatalf("show after review:\n%s", s)
	}
	mustRun(0, "workflow", "report", "--root", root, "--id", wf)
	rep, err := os.ReadFile(filepath.Join(tmp, "reports", wf, "workflow.md"))
	if err != nil || !strings.Contains(string(rep), "(`completed`)") || !strings.Contains(string(rep), "approved 5") {
		t.Fatalf("report: %v\n%s", err, rep)
	}
	// 9. Safe export of ready drafts only; traversal and the workspace are refused; rejected drafts are not exported.
	exp := mustRun(0, "workflow", "export", "--root", root, "--id", wf)
	files, _ := filepath.Glob(filepath.Join(drafts, "content", wf, "*.md"))
	if len(files) != 5 || !strings.Contains(exp, "bukan publikasi") {
		t.Fatalf("export: %d files\n%s", len(files), exp)
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if !strings.Contains(string(b), "DRAF — BELUM DIPUBLIKASIKAN DAN BUKAN IZIN PUBLIKASI") || !strings.Contains(string(b), "Provenance") || strings.Contains(f, rejected) {
			t.Fatalf("export file %s", f)
		}
	}
	mustRun(2, "content", "export", "--root", root, "--id", rejected)
	for _, bad := range []string{"../keluar", "/tmp/x"} {
		if s := mustRun(1, "workflow", "export", "--root", root, "--id", wf, "--dir", bad); !strings.Contains(s, "ditolak") {
			t.Fatalf("export dir %q: %s", bad, s)
		}
	}
	t.Setenv("MIA_DRAFTS_DIR", filepath.Join(workspace, "docs"))
	if s := mustRun(1, "workflow", "export", "--root", root, "--id", wf); !strings.Contains(s, "ruang kerja") {
		t.Fatalf("workspace export: %s", s)
	}
	t.Setenv("MIA_DRAFTS_DIR", drafts)
	if s := mustRun(0, "workflow", "show", "--root", root, "--id", wf); !strings.Contains(s, "diekspor 5") || !strings.Contains(s, "Workflow selesai untuk tahap saat ini") {
		t.Fatalf("after export:\n%s", s)
	}
	if s := mustRun(0, "workflow", "list", "--root", root); !strings.Contains(s, wf) || !strings.Contains(s, "completed") {
		t.Fatalf("list: %s", s)
	}
	if treeHash(t, workspace) != before {
		t.Fatal("the workspace fixture changed")
	}
}
