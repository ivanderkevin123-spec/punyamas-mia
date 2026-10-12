package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func miniRoot(t *testing.T) string {
	return filepath.Join(testutil.RepoRoot(t), "testdata", "workspace-mini")
}

func TestParseBrandOracleMatchesConfig(t *testing.T) {
	root := testutil.RepoRoot(t)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(miniRoot(t), "tools/spec-oracle/brand-oracle.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	rules := ParseBrandOracle(b)
	if len(rules) != 9 {
		t.Fatalf("rules = %d, want 9", len(rules))
	}
	if rules[0].Category != "SUPERLATIF" || len(rules[0].Words) != 14 {
		t.Fatalf("first rule = %+v", rules[0])
	}
	if diffs := DiffRules(rules, cfg.Policy.Rules); len(diffs) != 0 {
		t.Fatalf("policy drift against workspace checker: %v", diffs)
	}
	cfg.Policy.Rules[0].Words = cfg.Policy.Rules[0].Words[:13]
	if diffs := DiffRules(rules, cfg.Policy.Rules); len(diffs) != 1 {
		t.Fatalf("expected one diff after removing a word, got %v", diffs)
	}
}

func TestParseFactsMatchesConfig(t *testing.T) {
	cfg, err := config.Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(miniRoot(t), "docs/02-brand/B4-fakta.json"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := ParseFacts(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Facts) != 14 || f.Version != "v0.1" {
		t.Fatalf("facts = %d version %s", len(f.Facts), f.Version)
	}
	if diffs := DiffFacts(f, cfg.Business.Facts); len(diffs) != 0 {
		t.Fatalf("facts drift: %v", diffs)
	}
	f.Facts[0].Text = "berubah"
	if diffs := DiffFacts(f, cfg.Business.Facts); len(diffs) != 1 {
		t.Fatalf("expected one diff, got %v", diffs)
	}
}

func TestParseFAQ(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(miniRoot(t), "docs/05-marketing/W1-naskah-situs-company-profile.md"))
	if err != nil {
		t.Fatal(err)
	}
	faqs := ParseFAQ(b)
	if len(faqs) != 5 {
		t.Fatalf("faqs = %d, want 5", len(faqs))
	}
	if faqs[2].Question != "Apakah saya perlu membawa identitas?" || faqs[2].Answer == "" {
		t.Fatalf("unexpected faq: %+v", faqs[2])
	}
}

func TestResolveUnderRejectsEscapes(t *testing.T) {
	root := miniRoot(t)
	for _, bad := range []string{"../x", "/etc/passwd", "docs/../../x"} {
		if _, err := ResolveUnder(root, bad); err == nil {
			t.Errorf("expected rejection for %q", bad)
		}
	}
	if _, err := ResolveUnder(root, "docs/02-brand/B4-fakta.json"); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintAndCompare(t *testing.T) {
	root := miniRoot(t)
	fp, err := FingerprintAsset(root, config.Asset{Key: "facts", Path: "docs/02-brand/B4-fakta.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !fp.Exists || len(fp.SHA256) != 64 || fp.Size == 0 {
		t.Fatalf("bad fingerprint %+v", fp)
	}
	if Compare(nil, fp) != "new" {
		t.Fatal("expected new")
	}
	prev := &domain.ImportedAsset{SHA256: fp.SHA256}
	if Compare(prev, fp) != "unchanged" {
		t.Fatal("expected unchanged")
	}
	prev.SHA256 = "other"
	if Compare(prev, fp) != "changed" {
		t.Fatal("expected changed")
	}
	missing, err := FingerprintAsset(root, config.Asset{Key: "x", Path: "does/not/exist.md"})
	if err != nil {
		t.Fatal(err)
	}
	if Compare(nil, missing) != "missing" {
		t.Fatal("expected missing")
	}
}

func TestHeadCommitWithoutGit(t *testing.T) {
	if got := HeadCommit(miniRoot(t)); got != "" {
		t.Fatalf("expected empty commit for non-git dir, got %q", got)
	}
}
