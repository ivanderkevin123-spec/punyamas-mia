package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func TestCLIBasics(t *testing.T) {
	root := testutil.RepoRoot(t)
	var out, errb bytes.Buffer
	if code := run([]string{"version"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "mia") {
		t.Fatalf("version: code=%d out=%s", code, out.String())
	}
	out.Reset()
	if code := run([]string{"help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "run research") {
		t.Fatalf("help: code=%d", code)
	}
	out.Reset()
	if code := run([]string{"bogus"}, &out, &errb); code != 1 {
		t.Fatalf("unknown command: code=%d", code)
	}
	out.Reset()
	if code := run([]string{"validate", "--root", root, "--inputs", filepath.Join(root, "testdata", "sample-batch")}, &out, &errb); code != 0 || !strings.Contains(out.String(), "butir=8") {
		t.Fatalf("validate sample: code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	out.Reset()
	if code := run([]string{"validate", "--root", root, "--inputs", filepath.Join(root, "testdata", "failures")}, &out, &errb); code != 2 || !strings.Contains(out.String(), "GAGAL") {
		t.Fatalf("validate failures: code=%d out=%s", code, out.String())
	}
}

// Smoke test of run/status/report/config check with isolated data and report paths.
func TestCLISmokeIsolated(t *testing.T) {
	root := testutil.RepoRoot(t)
	tmp := t.TempDir()
	t.Setenv("MIA_DATA_DIR", filepath.Join(tmp, "data"))
	t.Setenv("MIA_REPORTS_DIR", filepath.Join(tmp, "reports"))
	t.Setenv("MIA_WORKSPACE_ROOT", filepath.Join(root, "testdata", "workspace-mini"))
	var out, errb bytes.Buffer
	code := run([]string{"run", "research", "--root", root, "--inputs", filepath.Join(root, "testdata", "sample-batch"), "--fixture", "clean"}, &out, &errb)
	if code != 0 {
		t.Fatalf("run: code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	line := strings.SplitN(out.String(), "\n", 2)[0] // "Run <id>: completed (...)"
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "Run" || !strings.Contains(line, "completed") {
		t.Fatalf("unexpected first line %q", line)
	}
	runID := strings.TrimSuffix(fields[1], ":")
	if _, err := os.Stat(filepath.Join(tmp, "reports", runID, "research.md")); err != nil {
		t.Fatalf("report not written under the isolated reports dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "reports", runID)); err == nil {
		t.Fatal("report leaked into the repository reports directory")
	}
	out.Reset()
	if code := run([]string{"status", "--root", root, runID}, &out, &errb); code != 0 || !strings.Contains(out.String(), "status=completed") {
		t.Fatalf("status: code=%d out=%s", code, out.String())
	}
	out.Reset()
	if code := run([]string{"report", "--root", root, runID}, &out, &errb); code != 0 {
		t.Fatalf("report: code=%d err=%s", code, errb.String())
	}
	out.Reset()
	// workspace-mini holds 3 of the 8 configured assets: the missing ones must be reported as review required (exit 2).
	if code := run([]string{"config", "check", "--root", root}, &out, &errb); code != 2 || !strings.Contains(out.String(), "brand_oracle   unchanged") || !strings.Contains(out.String(), "TINJAUAN DIPERLUKAN") {
		t.Fatalf("config check: code=%d out=%s", code, out.String())
	}
}
