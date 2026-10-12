package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func TestLoadRepoConfig(t *testing.T) {
	cfg, err := Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Limits.Provider.Name != "mock" {
		t.Fatalf("provider = %q, want mock", cfg.Limits.Provider.Name)
	}
	if cfg.Limits.Caps.MonthlyCeilingUSD != 20 {
		t.Fatalf("monthly ceiling = %v, want 20", cfg.Limits.Caps.MonthlyCeilingUSD)
	}
	if len(cfg.Policy.Rules) != 9 {
		t.Fatalf("rules = %d, want 9", len(cfg.Policy.Rules))
	}
	if len(cfg.Business.Facts) != 14 {
		t.Fatalf("facts = %d, want 14", len(cfg.Business.Facts))
	}
	if cfg.Sources.Web.Enabled {
		t.Fatal("web fetching must be disabled in Phase 2")
	}
	if len(cfg.Hash) != 16 {
		t.Fatalf("hash = %q", cfg.Hash)
	}
	if strings.HasPrefix(cfg.Sources.Workspace.Root, "~") {
		t.Fatalf("workspace root not expanded: %s", cfg.Sources.Workspace.Root)
	}
}

func withModifiedConfig(t *testing.T, file, old, new string) string {
	t.Helper()
	root := testutil.RepoRoot(t)
	tmp := t.TempDir()
	testutil.CopyDir(t, filepath.Join(root, "config"), filepath.Join(tmp, "config"))
	p := filepath.Join(tmp, "config", file)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(b), old, new, 1)
	if s == string(b) {
		t.Fatalf("replacement %q not found in %s", old, file)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func TestRejectsAnthropicProviderInPhase2(t *testing.T) {
	tmp := withModifiedConfig(t, "agent-limits.yaml", "name: mock ", "name: anthropic ")
	_, err := Load(tmp)
	if err == nil || !strings.Contains(err.Error(), "anthropic") {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestRejectsWebFetchingInPhase2(t *testing.T) {
	tmp := withModifiedConfig(t, "research-sources.yaml", "enabled: false", "enabled: true")
	_, err := Load(tmp)
	if err == nil || !strings.Contains(err.Error(), "web.enabled") {
		t.Fatalf("expected web error, got %v", err)
	}
}

func TestRejectsSecretsInYAML(t *testing.T) {
	tmp := withModifiedConfig(t, "agent-limits.yaml", "fixture_key: \"\"", "fixture_key: \"sk-ant-abcdefghijklmnop\"")
	_, err := Load(tmp)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected secret error, got %v", err)
	}
}

func TestRejectsUnknownKeys(t *testing.T) {
	tmp := withModifiedConfig(t, "agent-limits.yaml", "block_runs: false", "block_runs: false\n  typo_key: 1")
	_, err := Load(tmp)
	if err == nil {
		t.Fatal("expected unknown-field error")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("MIA_WORKSPACE_ROOT", "/tmp/ws-override")
	t.Setenv("MIA_DATA_DIR", "/tmp/data-override")
	cfg, err := Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sources.Workspace.Root != "/tmp/ws-override" || cfg.Paths.DataDir != "/tmp/data-override" {
		t.Fatalf("overrides not applied: %+v", cfg.Paths)
	}
}

func TestStepTimeoutDefault(t *testing.T) {
	cfg := &Config{}
	if cfg.StepTimeout("x") != 60 {
		t.Fatal("default timeout should be 60")
	}
}
