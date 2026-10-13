// Package config loads and validates the YAML configuration files and the
// optional .env file. Secrets are read only from the environment.
package config

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

type SourceOfTruth struct {
	Path            string `yaml:"path"`
	WorkspaceRoot   string `yaml:"workspace_root,omitempty"`
	SHA256          string `yaml:"sha256"`
	WorkspaceCommit string `yaml:"workspace_commit"`
	ImportedAt      string `yaml:"imported_at"`
	Note            string `yaml:"note,omitempty"`
}

type Brand struct {
	SourceOfTruth      SourceOfTruth `yaml:"source_of_truth"`
	Name               string        `yaml:"name"`
	Status             string        `yaml:"status"`
	Positioning        string        `yaml:"positioning"`
	PreOpeningLanguage string        `yaml:"pre_opening_language"`
	Pillars            []Pillar      `yaml:"pillars"`
	Tone               []ToneAttr    `yaml:"tone"`
	LanguageRules      []string      `yaml:"language_rules"`
	Audiences          []Audience    `yaml:"audiences"`
	KeyMessages        []KeyMessage  `yaml:"key_messages"`
	TaglineCandidates  []Tagline     `yaml:"tagline_candidates"`
}

type Pillar struct {
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	Promise        string `yaml:"promise"`
	AllowedClaim   string `yaml:"allowed_claim"`
	ForbiddenClaim string `yaml:"forbidden_claim"`
}

type ToneAttr struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Means string `yaml:"means"`
	Not   string `yaml:"not"`
}

type Audience struct {
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name"`
	Wants     string   `yaml:"wants"`
	Fears     string   `yaml:"fears"`
	MustTrust string   `yaml:"must_trust"`
	Channels  []string `yaml:"channels"`
}

type KeyMessage struct {
	Audiences []string `yaml:"audiences"`
	Message   string   `yaml:"message"`
}

type Tagline struct {
	ID      string `yaml:"id"`
	Text    string `yaml:"text"`
	Checker string `yaml:"checker"`
}

type Business struct {
	SourceOfTruth   SourceOfTruth `yaml:"source_of_truth"`
	Version         string        `yaml:"version"`
	Note            string        `yaml:"note"`
	Facts           []Fact        `yaml:"facts"`
	AllowedNumbers  []string      `yaml:"angka_boleh"`
	RequiredFillIns []string      `yaml:"isian_wajib"`
}

type Fact struct {
	ID     string `yaml:"id"`
	Text   string `yaml:"text"`
	Status string `yaml:"status"`
	Source string `yaml:"source"`
}

type Policy struct {
	SourceOfTruth SourceOfTruth `yaml:"source_of_truth"`
	Version       string        `yaml:"version"`
	Note          string        `yaml:"note"`
	Rules         []Rule        `yaml:"rules"`
}

type Rule struct {
	Category string   `yaml:"category"`
	Words    []string `yaml:"words"`
	Pattern  string   `yaml:"pattern"`
	Reason   string   `yaml:"reason"`
}

type Sources struct {
	Workspace Workspace `yaml:"workspace"`
	Web       Web       `yaml:"web"`
	Imports   Imports   `yaml:"imports"`
}

type Workspace struct {
	Root   string  `yaml:"root"`
	Assets []Asset `yaml:"assets"`
}

type Asset struct {
	Key  string `yaml:"key"`
	Path string `yaml:"path"`
}

type Web struct {
	Enabled  bool         `yaml:"enabled"`
	Allow    []AllowEntry `yaml:"allow"`
	Block    []string     `yaml:"block"`
	Defaults WebDefaults  `yaml:"defaults"`
}

type AllowEntry struct {
	Host               string `yaml:"host"`
	PathPrefix         string `yaml:"path_prefix"`
	MaxRequestsPerHour int    `yaml:"max_requests_per_hour"`
	TermsNote          string `yaml:"terms_note"`
	TermsCheckedAt     string `yaml:"terms_checked_at"`
	ApprovalRecord     string `yaml:"approval_record"`
}

type WebDefaults struct {
	MaxRequestsPerHour int `yaml:"max_requests_per_hour"`
	MaxPagesPerRun     int `yaml:"max_pages_per_run"`
	RequestIntervalS   int `yaml:"request_interval_s"`
	MaxBytes           int `yaml:"max_bytes"`
}

type Imports struct {
	MaxFileBytes int64 `yaml:"max_file_bytes"`
}

type Limits struct {
	Provider   Provider   `yaml:"provider"`
	Caps       Caps       `yaml:"limits"`
	Drift      Drift      `yaml:"drift"`
	PriceTable PriceTable `yaml:"price_table"`
}

type Provider struct {
	Name  string       `yaml:"name"`
	Model string       `yaml:"model"`
	Mock  MockSettings `yaml:"mock"`
}

type MockSettings struct {
	FixturesDir string `yaml:"fixtures_dir"`
	FixtureKey  string `yaml:"fixture_key"`
}

type Caps struct {
	MonthlyCeilingUSD float64        `yaml:"monthly_ceiling_usd"`
	RunCeilingUSD     float64        `yaml:"run_ceiling_usd"`
	AgentCeilingUSD   float64        `yaml:"agent_ceiling_usd"`
	WarnAtFraction    float64        `yaml:"warn_at_fraction"`
	MaxRetries        int            `yaml:"max_retries"`
	StepTimeoutS      map[string]int `yaml:"step_timeout_s"`
	MaxCallsPerStep   int            `yaml:"max_calls_per_step"`
	MaxInputTokens    int            `yaml:"max_input_tokens"`
	MaxOutputTokens   int            `yaml:"max_output_tokens"`
	Concurrency       int            `yaml:"concurrency"`
	StaleAfterS       int            `yaml:"stale_after_s"`
}

type Drift struct {
	BlockRuns bool `yaml:"block_runs"`
}

type PriceTable struct {
	VerifiedAt string                `yaml:"verified_at"`
	Source     string                `yaml:"source"`
	Models     map[string]ModelPrice `yaml:"models"`
}

type ModelPrice struct {
	InputPerMTok     float64 `yaml:"input_per_mtok"`
	CacheReadPerMTok float64 `yaml:"cache_read_per_mtok"`
	OutputPerMTok    float64 `yaml:"output_per_mtok"`
}

type Paths struct {
	Root       string
	ConfigDir  string
	DataDir    string
	ReportsDir string
	DraftsDir  string
}

type Config struct {
	Paths    Paths
	Brand    Brand
	Business Business
	Policy   Policy
	Sources  Sources
	Limits   Limits
	Scoring  Scoring
	Hash     string
}

// Scoring holds the Phase 3 evaluation rules (config/scoring.yaml); Hash is the sha256 of the file.
type Scoring struct {
	Version              string        `yaml:"version"`
	Tiers                TierRules     `yaml:"tiers"`
	Dedup                DedupRules    `yaml:"dedup"`
	Conflict             ConflictRules `yaml:"conflict"`
	UnsupportedClaims    []Rule        `yaml:"unsupported_claims"`
	StoreDependencyTerms []string      `yaml:"store_dependency_terms"`
	KPIs                 []string      `yaml:"kpis"`
	KPIsAvailableNow     []string      `yaml:"kpis_available_now"`
	Segments             []string      `yaml:"segments"`
	Pillars              []string      `yaml:"pillars"`
	Efforts              []string      `yaml:"efforts"`
	Hash                 string        `yaml:"-"`
}

type TierRules struct {
	A TierThreshold `yaml:"a"`
	B TierThreshold `yaml:"b"`
}

type TierThreshold struct {
	EvidenceMin int `yaml:"evidence_min"`
	IntentMin   int `yaml:"intent_min"`
	FitMin      int `yaml:"fit_min"`
}

type DedupRules struct {
	SharedEvidenceFraction float64 `yaml:"shared_evidence_fraction"`
	TitleJaccard           float64 `yaml:"title_jaccard"`
}

type ConflictRules struct {
	NegationPattern string  `yaml:"negation_pattern"`
	MinSimilarity   float64 `yaml:"min_similarity"`
}

var files = []string{"brand.yaml", "business.yaml", "marketing-policy.yaml", "research-sources.yaml", "agent-limits.yaml", "scoring.yaml"}

var secretPattern = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{8,}|AKIA[0-9A-Z]{12,}|-----BEGIN [A-Z ]*PRIVATE KEY`)

// LoadEnvFile reads KEY=VALUE lines from path into the process environment
// without overriding variables that are already set. A missing file is not an error.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if k == "" || v == "" {
			continue
		}
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	return sc.Err()
}

// Load reads the configuration under <root>/config and applies environment overrides.
func Load(root string) (*Config, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := LoadEnvFile(filepath.Join(root, ".env")); err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}
	cfg := &Config{Paths: Paths{
		Root:       root,
		ConfigDir:  filepath.Join(root, "config"),
		DataDir:    filepath.Join(root, "data"),
		ReportsDir: filepath.Join(root, "reports"),
		DraftsDir:  filepath.Join(root, "drafts"),
	}}
	if v := os.Getenv("MIA_DATA_DIR"); v != "" {
		cfg.Paths.DataDir = expandHome(v)
	}
	if v := os.Getenv("MIA_REPORTS_DIR"); v != "" {
		cfg.Paths.ReportsDir = expandHome(v)
	}
	if v := os.Getenv("MIA_DRAFTS_DIR"); v != "" {
		cfg.Paths.DraftsDir = expandHome(v)
	}
	h := sha256.New()
	raw := map[string][]byte{}
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(cfg.Paths.ConfigDir, name))
		if err != nil {
			return nil, fmt.Errorf("config %s: %w", name, err)
		}
		if secretPattern.Match(b) {
			return nil, fmt.Errorf("config %s: contains what looks like a secret; secrets belong in .env only", name)
		}
		raw[name] = b
	}
	names := make([]string, 0, len(raw))
	for n := range raw {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h.Write([]byte(n))
		h.Write(raw[n])
	}
	cfg.Hash = hex.EncodeToString(h.Sum(nil))[:16]

	if err := decodeStrict(raw["brand.yaml"], &cfg.Brand); err != nil {
		return nil, fmt.Errorf("brand.yaml: %w", err)
	}
	if err := decodeStrict(raw["business.yaml"], &cfg.Business); err != nil {
		return nil, fmt.Errorf("business.yaml: %w", err)
	}
	if err := decodeStrict(raw["marketing-policy.yaml"], &cfg.Policy); err != nil {
		return nil, fmt.Errorf("marketing-policy.yaml: %w", err)
	}
	if err := decodeStrict(raw["research-sources.yaml"], &cfg.Sources); err != nil {
		return nil, fmt.Errorf("research-sources.yaml: %w", err)
	}
	if err := decodeStrict(raw["agent-limits.yaml"], &cfg.Limits); err != nil {
		return nil, fmt.Errorf("agent-limits.yaml: %w", err)
	}
	if err := decodeStrict(raw["scoring.yaml"], &cfg.Scoring); err != nil {
		return nil, fmt.Errorf("scoring.yaml: %w", err)
	}
	sh := sha256.Sum256(raw["scoring.yaml"])
	cfg.Scoring.Hash = hex.EncodeToString(sh[:])[:16]

	if v := os.Getenv("MIA_WORKSPACE_ROOT"); v != "" {
		cfg.Sources.Workspace.Root = v
	}
	cfg.Sources.Workspace.Root = expandHome(cfg.Sources.Workspace.Root)
	if v := os.Getenv("MIA_PROVIDER"); v != "" {
		cfg.Limits.Provider.Name = v
	}
	if v := os.Getenv("MIA_MODEL"); v != "" {
		cfg.Limits.Provider.Model = v
	}
	if v := os.Getenv("MIA_FIXTURE_KEY"); v != "" {
		cfg.Limits.Provider.Mock.FixtureKey = v
	}
	if cfg.Limits.Provider.Mock.FixturesDir != "" && !filepath.IsAbs(cfg.Limits.Provider.Mock.FixturesDir) {
		cfg.Limits.Provider.Mock.FixturesDir = filepath.Join(root, cfg.Limits.Provider.Mock.FixturesDir)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func decodeStrict(b []byte, v any) error {
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// Validate enforces ranges, Phase 2 availability rules and referential integrity.
func (c *Config) Validate() error {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	switch c.Limits.Provider.Name {
	case "mock":
	case "anthropic":
		add("provider 'anthropic' is not available in Phase 2 (no paid calls); set provider.name to 'mock'")
	case "":
		add("provider.name is required (use 'mock')")
	default:
		add("unknown provider %q; available: mock", c.Limits.Provider.Name)
	}
	if c.Limits.Provider.Model == "" {
		add("provider.model is required (use 'mock')")
	} else if _, ok := c.Limits.PriceTable.Models[c.Limits.Provider.Model]; !ok {
		add("price_table.models has no entry for provider.model %q", c.Limits.Provider.Model)
	}
	caps := c.Limits.Caps
	if caps.MonthlyCeilingUSD <= 0 {
		add("limits.monthly_ceiling_usd must be > 0")
	}
	if caps.RunCeilingUSD <= 0 || caps.AgentCeilingUSD <= 0 {
		add("limits.run_ceiling_usd and limits.agent_ceiling_usd must be > 0")
	}
	if caps.WarnAtFraction <= 0 || caps.WarnAtFraction > 1 {
		add("limits.warn_at_fraction must be in (0, 1]")
	}
	if caps.MaxRetries < 0 || caps.MaxRetries > 5 {
		add("limits.max_retries must be between 0 and 5")
	}
	if caps.MaxCallsPerStep < 1 || caps.MaxCallsPerStep > 5 {
		add("limits.max_calls_per_step must be between 1 and 5")
	}
	for name, s := range caps.StepTimeoutS {
		if s <= 0 {
			add("limits.step_timeout_s.%s must be > 0", name)
		}
	}
	if caps.Concurrency < 1 {
		add("limits.concurrency must be >= 1")
	}
	if c.Sources.Web.Enabled {
		add("web.enabled=true is not available in Phase 2 (live fetching is not implemented); set it to false")
	}
	if c.Sources.Imports.MaxFileBytes <= 0 {
		add("imports.max_file_bytes must be > 0")
	}
	if c.Sources.Workspace.Root == "" {
		add("workspace.root is required")
	}
	seen := map[string]bool{}
	for _, a := range c.Sources.Workspace.Assets {
		if a.Key == "" || a.Path == "" {
			add("workspace.assets entries need key and path")
			continue
		}
		if seen[a.Key] {
			add("workspace.assets duplicate key %q", a.Key)
		}
		seen[a.Key] = true
		if filepath.IsAbs(a.Path) || strings.Contains(a.Path, "..") {
			add("workspace.assets[%s].path must be relative and must not contain '..'", a.Key)
		}
	}
	for i, r := range c.Policy.Rules {
		if r.Category == "" || r.Pattern == "" {
			add("marketing-policy rules[%d] needs category and pattern", i)
			continue
		}
		if _, err := regexp.Compile(r.Pattern); err != nil {
			add("marketing-policy rule %s: invalid pattern: %v", r.Category, err)
		}
	}
	sc := c.Scoring
	for _, th := range []TierThreshold{sc.Tiers.A, sc.Tiers.B} {
		if th.EvidenceMin < 0 || th.EvidenceMin > 3 || th.IntentMin < 0 || th.IntentMin > 3 || th.FitMin < 0 || th.FitMin > 3 {
			add("scoring tiers thresholds must be between 0 and 3")
		}
	}
	if sc.Dedup.SharedEvidenceFraction <= 0 || sc.Dedup.SharedEvidenceFraction > 1 || sc.Dedup.TitleJaccard <= 0 || sc.Dedup.TitleJaccard > 1 {
		add("scoring dedup fractions must be in (0, 1]")
	}
	if sc.Conflict.NegationPattern == "" {
		add("scoring conflict.negation_pattern is required")
	} else if _, err := regexp.Compile(sc.Conflict.NegationPattern); err != nil {
		add("scoring conflict.negation_pattern: %v", err)
	}
	if sc.Conflict.MinSimilarity < 0 || sc.Conflict.MinSimilarity > 1 {
		add("scoring conflict.min_similarity must be between 0 and 1")
	}
	for i, r := range sc.UnsupportedClaims {
		if r.Category == "" || r.Pattern == "" {
			add("scoring unsupported_claims[%d] needs category and pattern", i)
		} else if _, err := regexp.Compile(r.Pattern); err != nil {
			add("scoring unsupported_claims %s: %v", r.Category, err)
		}
	}
	if len(sc.KPIs) == 0 || len(sc.Segments) == 0 || len(sc.Pillars) == 0 || len(sc.Efforts) == 0 {
		add("scoring kpis, segments, pillars and efforts must be non-empty")
	}
	if len(errs) > 0 {
		return errors.New("config validation failed:\n  - " + strings.Join(errs, "\n  - "))
	}
	return nil
}

// StepTimeout returns the configured timeout for a step name or the default.
func (c *Config) StepTimeout(step string) int {
	if v, ok := c.Limits.Caps.StepTimeoutS[step]; ok && v > 0 {
		return v
	}
	if v, ok := c.Limits.Caps.StepTimeoutS["default"]; ok && v > 0 {
		return v
	}
	return 60
}
