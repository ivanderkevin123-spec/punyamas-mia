package main

import (
	"bytes"
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
