package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func firstRunID(t *testing.T, out string) string {
	t.Helper()
	line := strings.SplitN(out, "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "Run" {
		t.Fatalf("unexpected first line %q", line)
	}
	return strings.TrimSuffix(fields[1], ":")
}

// pickOpportunity returns the first opportunity row matching tier and status from `mia opportunities` output.
func pickOpportunity(t *testing.T, out, tier, status string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 3 && (tier == "" || f[1] == tier) && f[2] == status {
			return f[0]
		}
	}
	t.Fatalf("no opportunity with tier=%q status=%q in:\n%s", tier, status, out)
	return ""
}

func TestCLIStrategyWorkflowIsolated(t *testing.T) {
	root := testutil.RepoRoot(t)
	tmp := t.TempDir()
	t.Setenv("MIA_DATA_DIR", filepath.Join(tmp, "data"))
	t.Setenv("MIA_REPORTS_DIR", filepath.Join(tmp, "reports"))
	t.Setenv("MIA_DRAFTS_DIR", filepath.Join(tmp, "drafts"))
	workspace := filepath.Join(root, "testdata", "workspace-mini")
	t.Setenv("MIA_WORKSPACE_ROOT", workspace)
	var out, errb bytes.Buffer
	reset := func() { out.Reset(); errb.Reset() }

	if code := run([]string{"run", "research", "--root", root, "--inputs", filepath.Join(root, "testdata", "sample-batch")}, &out, &errb); code != 0 {
		t.Fatalf("research: code=%d err=%s", code, errb.String())
	}
	researchID := firstRunID(t, out.String())
	reset()
	if code := run([]string{"run", "strategy", "--root", root}, &out, &errb); code != 1 {
		t.Fatalf("run strategy without --from-run must fail: code=%d", code)
	}
	reset()
	if code := run([]string{"run", "strategy", "--root", root, "--from-run", researchID}, &out, &errb); code != 0 || !strings.Contains(out.String(), "tingkatA=3") {
		t.Fatalf("strategy: code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	strategyID := firstRunID(t, out.String())
	if _, err := os.Stat(filepath.Join(tmp, "reports", strategyID, "strategy.md")); err != nil {
		t.Fatalf("strategy report not written under the isolated reports dir: %v", err)
	}
	reset()
	if code := run([]string{"status", "--root", root, strategyID}, &out, &errb); code != 0 || !strings.Contains(out.String(), "status=completed") {
		t.Fatalf("status: code=%d out=%s", code, out.String())
	}
	reset()
	if code := run([]string{"report", "--root", root, strategyID}, &out, &errb); code != 0 || !strings.Contains(out.String(), "strategy.md") {
		t.Fatalf("report re-render: code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	reset()
	if code := run([]string{"opportunities", "--root", root, "--run", strategyID}, &out, &errb); code != 0 {
		t.Fatalf("opportunities: code=%d err=%s", code, errb.String())
	}
	listing := out.String()
	oppID := pickOpportunity(t, listing, "A", "proposed")
	if !strings.Contains(listing, "[hipotesis]") || !strings.Contains(listing, "[perlu toko]") {
		t.Fatalf("listing lacks labels:\n%s", listing)
	}
	reset()
	if code := run([]string{"export", "brief", "--root", root, "--id", oppID}, &out, &errb); code != 2 {
		t.Fatalf("export before approval must be refused: code=%d", code)
	}
	reset()
	if code := run([]string{"approve", "opportunity", "--root", root, "--id", oppID, "--decision", "maybe", "--note", "x"}, &out, &errb); code != 1 {
		t.Fatalf("unknown decision: code=%d", code)
	}
	reset()
	if code := run([]string{"approve", "opportunity", "--root", root, "--id", oppID, "--decision", "approved", "--note", "Disetujui untuk uji konten"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "riwayat: 1") {
		t.Fatalf("approve: code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	reset()
	if code := run([]string{"export", "brief", "--root", root, "--id", oppID}, &out, &errb); code != 0 {
		t.Fatalf("export: code=%d err=%s", code, errb.String())
	}
	brief, err := os.ReadFile(filepath.Join(tmp, "drafts", oppID+".md"))
	if err != nil {
		t.Fatalf("brief not written to the drafts dir: %v", err)
	}
	for _, want := range []string{"Brief Kampanye", "DRAF UNTUK SERAH TERIMA MANUAL", "Disetujui untuk uji konten", "Fakta pendukung", "jumlah ordinal, bukan probabilitas", "Riwayat keputusan (hanya-tambah)"} {
		if !strings.Contains(string(brief), want) {
			t.Errorf("brief lacks %q", want)
		}
	}
	reset()
	inWorkspace := filepath.Join(workspace, "docs", "05-marketing", "brief-uji.md")
	if code := run([]string{"export", "brief", "--root", root, "--id", oppID, "--out", inWorkspace}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "baca-saja") {
		t.Fatalf("export into the workspace must be refused: code=%d err=%s", code, errb.String())
	}
	if _, err := os.Stat(inWorkspace); err == nil {
		t.Fatal("brief was written into the read-only workspace")
	}
	reset()
	if code := run([]string{"approve", "opportunity", "--root", root, "--id", oppID, "--decision", "needs_more_evidence", "--note", "Butuh data nyata"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "riwayat: 2") {
		t.Fatalf("second decision must append: code=%d out=%s", code, out.String())
	}
	reset()
	if code := run([]string{"export", "brief", "--root", root, "--id", oppID}, &out, &errb); code != 2 {
		t.Fatalf("export after needs_more_evidence must be refused: code=%d", code)
	}
	reset()
	if code := run([]string{"approve", "opportunity", "--root", root, "--id", "opp_missing", "--decision", "approved", "--note", "x"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "tidak ditemukan") {
		t.Fatalf("missing opportunity: code=%d err=%s", code, errb.String())
	}

	// Blocked and rejected opportunities cannot be approved at all.
	reset()
	if code := run([]string{"run", "strategy", "--root", root, "--from-run", researchID, "--fixture", "gates"}, &out, &errb); code != 0 {
		t.Fatalf("gates strategy: code=%d err=%s", code, errb.String())
	}
	gatesID := firstRunID(t, out.String())
	reset()
	if code := run([]string{"opportunities", "--root", root, "--run", gatesID}, &out, &errb); code != 0 {
		t.Fatalf("opportunities: code=%d", code)
	}
	blocked := pickOpportunity(t, out.String(), "", "blocked")
	rejected := pickOpportunity(t, out.String(), "", "rejected")
	for _, id := range []string{blocked, rejected} {
		reset()
		if code := run([]string{"approve", "opportunity", "--root", root, "--id", id, "--decision", "approved", "--note", "coba"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "tidak dapat ditinjau") {
			t.Fatalf("approve %s: code=%d err=%s", id, code, errb.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, "drafts", oppID+".md")); err == nil {
		t.Fatal("brief leaked into the repository drafts directory")
	}
}
