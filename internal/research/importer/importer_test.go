package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func TestImportSampleBatch(t *testing.T) {
	root := testutil.RepoRoot(t)
	res, err := ImportDir(filepath.Join(root, "testdata", "sample-batch"), 5_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	if len(res.Notes) != 3 {
		t.Fatalf("notes = %d, want 3", len(res.Notes))
	}
	items := 0
	for _, n := range res.Notes {
		items += len(n.Items)
		for _, it := range n.Items {
			if it.Ref == "" || it.Title == "" || it.Statement == "" || it.Quality == "" {
				t.Fatalf("incomplete item %+v", it)
			}
			if it.Kind == "fact" && it.Excerpt == "" {
				t.Fatalf("fact without excerpt: %s", it.Ref)
			}
		}
	}
	if items != 8 {
		t.Fatalf("items = %d, want 8", items)
	}
}

func TestImportFailures(t *testing.T) {
	root := testutil.RepoRoot(t)
	res, err := ImportDir(filepath.Join(root, "testdata", "failures"), 5_000_000)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range res.Errors {
		got[e.File] = e.Message
	}
	for file, want := range map[string]string{
		"malformed.csv":   "kolom wajib",
		"pii.csv":         "data pribadi",
		"pii-in-text.md":  "nomor telepon",
		"no-frontmatter.md": "front matter",
	} {
		if !strings.Contains(got[file], want) {
			t.Errorf("%s: error %q does not contain %q", file, got[file], want)
		}
	}
	if len(res.Notes) != 1 || res.Notes[0].File != "suspicious.md" || len(res.Notes[0].Items) != 2 {
		t.Fatalf("expected only suspicious.md to parse with 2 items, got %+v", res.Notes)
	}
}

func TestPII(t *testing.T) {
	cases := map[string]bool{
		"hubungi 081234567890":              true,
		"+6281234567890 adalah nomor":       true,
		"NIK 3578012345678901 tercatat":     true,
		"kirim ke orang@example.com":        true,
		"penawaran berlaku 10 menit":        false,
		"kadar 750 permil, berat 12,40 g":   false,
		"tahun 2026 bulan 10":               false,
	}
	for text, want := range cases {
		if got, _ := PII(text); got != want {
			t.Errorf("PII(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestMissingDirectory(t *testing.T) {
	if _, err := ImportDir(filepath.Join(t.TempDir(), "nope"), 1000); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestFileSizeLimit(t *testing.T) {
	root := testutil.RepoRoot(t)
	res, err := ImportDir(filepath.Join(root, "testdata", "sample-batch"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 3 || len(res.Notes) != 0 {
		t.Fatalf("expected every file rejected by size, got notes=%d errors=%d", len(res.Notes), len(res.Errors))
	}
}
