package safepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDraftPathRefusesEscapes(t *testing.T) {
	tmp := t.TempDir()
	base := filepath.Join(tmp, "drafts", "content")
	ws := filepath.Join(tmp, "workspace")
	_ = os.MkdirAll(ws, 0o755)
	for _, name := range []string{"../x.md", "a/../../x.md", "/etc/x.md", filepath.Join(tmp, "x.md"), "", "~/x.md", "..", "a/.."} {
		if _, err := ResolveDraftPath(base, name, ws); err == nil {
			t.Errorf("%q must be refused", name)
		}
	}
	p, err := ResolveDraftPath(base, "sub/draf", ws)
	if err != nil || p != filepath.Join(base, "sub", "draf.md") {
		t.Fatalf("relative name: %q %v", p, err)
	}
	// A symlink inside the drafts directory that points elsewhere is an escape.
	_ = os.MkdirAll(base, 0o755)
	if err := os.Symlink(tmp, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDraftPath(base, "link/x.md", ws); err == nil {
		t.Fatal("symlink escape must be refused")
	}
	// A drafts directory configured inside the workspace is refused.
	if _, err := ResolveDraftPath(filepath.Join(ws, "docs"), "x.md", ws); err == nil || !strings.Contains(err.Error(), "ruang kerja") {
		t.Fatalf("drafts inside the workspace must be refused: %v", err)
	}
	if !Under(filepath.Join(ws, "a", "b.md"), ws) || Under(filepath.Join(tmp, "other"), ws) {
		t.Fatal("Under")
	}
}

func TestWriteAtomicLeavesNoPartialFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "out.md")
	if err := WriteAtomic(target, []byte("isi"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Renaming onto an existing directory fails; the temp file must be cleaned up.
	dirTarget := filepath.Join(tmp, "adir")
	_ = os.MkdirAll(filepath.Join(dirTarget, "child"), 0o755)
	if err := WriteAtomic(dirTarget, []byte("x"), 0o644); err == nil {
		t.Fatal("writing over a non-empty directory must fail")
	}
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".mia-export-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
	if b, _ := os.ReadFile(target); string(b) != "isi" {
		t.Fatal("successful write content")
	}
}
