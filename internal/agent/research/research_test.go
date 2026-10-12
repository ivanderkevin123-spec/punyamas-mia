package research

import (
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func TestBuildRequestPlacesUntrustedBlockLast(t *testing.T) {
	cfg, err := config.Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	req := BuildRequest(cfg, []ItemForPrompt{{Ref: "b.md#1", Kind: "fact", Quality: "A", Title: "T", Statement: "S", Excerpt: "E", SourceName: "src"}, {Ref: "a.md#1", Kind: "hypothesis", Quality: "D", Title: "T2", Statement: "S2"}}, "key")
	if req.Agent != Name || req.SchemaVersion != SchemaVersion || req.FixtureKey != "key" {
		t.Fatalf("bad request meta %+v", req)
	}
	if !strings.Contains(req.System, "SUPERLATIF") {
		t.Fatal("system prompt should list policy categories")
	}
	start := strings.Index(req.User, "DATA TIDAK TEPERCAYA — MULAI")
	end := strings.Index(req.User, "DATA TIDAK TEPERCAYA — SELESAI")
	if start < 0 || end < start {
		t.Fatal("untrusted block markers missing or misordered")
	}
	if strings.Index(req.User, "[a.md#1]") > strings.Index(req.User, "[b.md#1]") {
		t.Fatal("items should be sorted by ref for determinism")
	}
}

func TestParseOutputToleratesFences(t *testing.T) {
	out, err := ParseOutput("```json\n{\"findings\":[],\"notes\":\"x\"}\n```")
	if err != nil || out.Notes != "x" {
		t.Fatalf("parse: %+v %v", out, err)
	}
	if _, err := ParseOutput("{\"findings\":[{\"kind\":\"fact\",\"extra\":1}]}"); err == nil {
		t.Fatal("unknown fields must be rejected")
	}
	if _, err := ParseOutput("not json"); err == nil {
		t.Fatal("invalid json must be rejected")
	}
	if _, err := ParseOutput("{\"notes\":\"only\"}"); err == nil {
		t.Fatal("missing findings must be rejected")
	}
}

func TestValidateEvidenceRules(t *testing.T) {
	refs := map[string]RefInfo{
		"f#1": {ID: "i1", Kind: domain.KindFact, Quality: "A"},
		"h#1": {ID: "i2", Kind: domain.KindHypothesis, Quality: "D"},
	}
	out := Output{Findings: []Finding{
		{Kind: "fact", Title: "ok fact", Statement: "s", EvidenceRefs: []string{"f#1", "f#1"}},
		{Kind: "fact", Title: "fact citing hypothesis", Statement: "s", EvidenceRefs: []string{"h#1"}},
		{Kind: "interpretation", Title: "no refs", Statement: "s"},
		{Kind: "interpretation", Title: "bad ref", Statement: "s", EvidenceRefs: []string{"nope#9"}},
		{Kind: "hypothesis", Title: "free hypothesis", Statement: "s", TopicKey: "Topik Baru"},
		{Kind: "opinion", Title: "bad kind", Statement: "s"},
		{Kind: "hypothesis", Title: "", Statement: ""},
	}}
	valid, rejected := Validate(out, refs)
	if len(valid) != 2 || len(rejected) != 5 {
		t.Fatalf("valid=%d rejected=%d: %+v", len(valid), len(rejected), rejected)
	}
	if len(valid[0].EvidenceRefs) != 1 {
		t.Fatal("duplicate refs should be collapsed")
	}
	if valid[1].TopicKey != "topik_baru" {
		t.Fatalf("topic not normalized: %q", valid[1].TopicKey)
	}
}
