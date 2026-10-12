// Package workspace reads the Punyamas workspace strictly read-only: it
// fingerprints approved assets, detects drift and parses the brand checker,
// facts file and FAQ. It never writes under the workspace root.
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

type Fingerprint struct {
	Key     string
	RelPath string
	AbsPath string
	SHA256  string
	Size    int64
	MTime   time.Time
	Exists  bool
}

// ResolveUnder returns the absolute path of rel under root, refusing paths that escape root.
func ResolveUnder(root, rel string) (string, error) {
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", fmt.Errorf("path %q must be relative and must not contain '..'", rel)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = r
	}
	p := filepath.Join(absRoot, rel)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	if p != absRoot && !strings.HasPrefix(p, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the workspace root", rel)
	}
	return p, nil
}

// FingerprintAsset hashes an asset file opened read-only.
func FingerprintAsset(root string, a config.Asset) (Fingerprint, error) {
	fp := Fingerprint{Key: a.Key, RelPath: a.Path}
	abs, err := ResolveUnder(root, a.Path)
	if err != nil {
		return fp, err
	}
	fp.AbsPath = abs
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fp, nil
		}
		return fp, err
	}
	if info.IsDir() {
		return fp, fmt.Errorf("asset %s is a directory", a.Key)
	}
	f, err := os.Open(abs) // read-only
	if err != nil {
		return fp, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fp, err
	}
	fp.Exists = true
	fp.SHA256 = hex.EncodeToString(h.Sum(nil))
	fp.Size = info.Size()
	fp.MTime = info.ModTime().UTC()
	return fp, nil
}

// ReadAsset returns the file content (read-only open).
func ReadAsset(fp Fingerprint) ([]byte, error) {
	return os.ReadFile(fp.AbsPath)
}

// HeadCommit reads .git/HEAD without executing git. Returns "" when unavailable.
func HeadCommit(root string) string {
	head, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	h := strings.TrimSpace(string(head))
	if !strings.HasPrefix(h, "ref: ") {
		return short(h)
	}
	ref := strings.TrimPrefix(h, "ref: ")
	if b, err := os.ReadFile(filepath.Join(root, ".git", ref)); err == nil {
		return short(strings.TrimSpace(string(b)))
	}
	if b, err := os.ReadFile(filepath.Join(root, ".git", "packed-refs")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			parts := strings.Fields(line)
			if len(parts) == 2 && parts[1] == ref {
				return short(parts[0])
			}
		}
	}
	return ""
}

func short(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// Compare classifies the current fingerprint against the last accepted one.
func Compare(prev *domain.ImportedAsset, cur Fingerprint) string {
	switch {
	case !cur.Exists:
		return "missing"
	case prev == nil:
		return "new"
	case prev.SHA256 == cur.SHA256:
		return "unchanged"
	default:
		return "changed"
	}
}

// ---- brand-oracle.mjs ----

type OracleRule struct {
	Category string   `json:"category"`
	Words    []string `json:"words"`
}

var (
	oracleEntry = regexp.MustCompile(`(?s)\{\s*kategori:\s*'([^']+)'\s*,\s*kata:\s*\[(.*?)\]`)
	quoted      = regexp.MustCompile(`'([^']*)'`)
)

// ParseBrandOracle extracts category and word lists from the POLA array.
func ParseBrandOracle(b []byte) []OracleRule {
	var out []OracleRule
	for _, m := range oracleEntry.FindAllStringSubmatch(string(b), -1) {
		r := OracleRule{Category: m[1]}
		for _, q := range quoted.FindAllStringSubmatch(m[2], -1) {
			r.Words = append(r.Words, q[1])
		}
		out = append(out, r)
	}
	return out
}

// DiffRules compares the workspace checker with MIA's policy configuration.
// Word lists are merged per category on both sides (the checker lists SUPERLATIF twice).
func DiffRules(oracle []OracleRule, cfg []config.Rule) []string {
	var diffs []string
	cfgByCat := map[string]map[string]bool{}
	var cfgOrder []string
	for _, r := range cfg {
		if _, ok := cfgByCat[r.Category]; !ok {
			cfgByCat[r.Category] = map[string]bool{}
			cfgOrder = append(cfgOrder, r.Category)
		}
		for _, w := range r.Words {
			cfgByCat[r.Category][strings.ToLower(w)] = true
		}
	}
	oracleByCat := map[string]map[string]bool{}
	var oracleOrder []string
	oracleWords := map[string][]string{}
	for _, o := range oracle {
		if _, ok := oracleByCat[o.Category]; !ok {
			oracleByCat[o.Category] = map[string]bool{}
			oracleOrder = append(oracleOrder, o.Category)
		}
		for _, w := range o.Words {
			oracleByCat[o.Category][strings.ToLower(w)] = true
			oracleWords[o.Category] = append(oracleWords[o.Category], w)
		}
	}
	for _, cat := range oracleOrder {
		cset, ok := cfgByCat[cat]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("kategori %s ada di brand-oracle.mjs tetapi tidak ada di marketing-policy.yaml", cat))
			continue
		}
		for _, w := range oracleWords[cat] {
			if !cset[strings.ToLower(w)] {
				diffs = append(diffs, fmt.Sprintf("kata %q (kategori %s) ada di brand-oracle.mjs tetapi tidak ada di marketing-policy.yaml", w, cat))
			}
		}
		for w := range cset {
			if !oracleByCat[cat][w] {
				diffs = append(diffs, fmt.Sprintf("kata %q (kategori %s) ada di marketing-policy.yaml tetapi tidak ada di brand-oracle.mjs", w, cat))
			}
		}
	}
	for _, cat := range cfgOrder {
		if _, ok := oracleByCat[cat]; !ok {
			diffs = append(diffs, fmt.Sprintf("kategori %s ada di marketing-policy.yaml tetapi tidak ada di brand-oracle.mjs", cat))
		}
	}
	sort.Strings(diffs)
	return diffs
}

// ---- B4-fakta.json ----

type FactEntry struct {
	ID     string `json:"id"`
	Text   string `json:"teks"`
	Status string `json:"status"`
	Source string `json:"sumber"`
}

type Facts struct {
	Version    string      `json:"versi"`
	Note       string      `json:"catatan"`
	Facts      []FactEntry `json:"fakta"`
	AngkaBoleh []string    `json:"angka_boleh"`
	IsianWajib []string    `json:"isian_wajib"`
}

func ParseFacts(b []byte) (Facts, error) {
	var f Facts
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	return f, nil
}

func DiffFacts(ws Facts, cfg []config.Fact) []string {
	var diffs []string
	c := map[string]config.Fact{}
	for _, f := range cfg {
		c[f.ID] = f
	}
	w := map[string]FactEntry{}
	for _, f := range ws.Facts {
		w[f.ID] = f
		cf, ok := c[f.ID]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("fakta %s ada di B4-fakta.json tetapi tidak ada di business.yaml", f.ID))
			continue
		}
		if strings.TrimSpace(cf.Text) != strings.TrimSpace(f.Text) {
			diffs = append(diffs, fmt.Sprintf("teks fakta %s berbeda antara B4-fakta.json dan business.yaml", f.ID))
		}
		if cf.Status != f.Status {
			diffs = append(diffs, fmt.Sprintf("status fakta %s berbeda (%s vs %s)", f.ID, f.Status, cf.Status))
		}
	}
	for id := range c {
		if _, ok := w[id]; !ok {
			diffs = append(diffs, fmt.Sprintf("fakta %s ada di business.yaml tetapi tidak ada di B4-fakta.json", id))
		}
	}
	return diffs
}

// ---- W1 FAQ ----

type FAQ struct {
	N        int
	Question string
	Answer   string
}

var boldQ = regexp.MustCompile(`^\*\*(.+\?)\*\*$`)

// ParseFAQ extracts bold questions and their answers from the "## Pertanyaan umum" section.
func ParseFAQ(b []byte) []FAQ {
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	in := false
	var out []FAQ
	var cur *FAQ
	var ans []string
	flush := func() {
		if cur != nil {
			cur.Answer = strings.TrimSpace(strings.Join(ans, " "))
			out = append(out, *cur)
			cur, ans = nil, nil
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			in = strings.HasPrefix(line, "## Pertanyaan umum")
			continue
		}
		if !in {
			continue
		}
		if m := boldQ.FindStringSubmatch(line); m != nil {
			flush()
			cur = &FAQ{N: len(out) + 1, Question: m[1]}
			continue
		}
		if cur == nil {
			continue
		}
		if line == "" || line == "---" {
			flush()
			continue
		}
		ans = append(ans, line)
	}
	flush()
	return out
}
