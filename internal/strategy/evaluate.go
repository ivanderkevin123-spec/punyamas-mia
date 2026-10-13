// Package strategy applies the deterministic opportunity gates (evidence,
// conflicts, brand and factual claims, operational dependency, duplicates),
// recomputes scores and tiers from validated evidence, and assembles briefs.
// The model only proposes; this package decides.
package strategy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	agent "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/strategy"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
)

// Evidence is a validated research item as seen by the evaluator.
type Evidence struct {
	ID, Ref, Kind, Quality, TopicKey, Title, Statement, Excerpt, SourceID, SourceName, ConflictGroup string
}

type Dim struct {
	Value     int    `json:"value"`
	Rationale string `json:"rationale"`
	Source    string `json:"source"` // code | agent | agent+cap
}

type Cap struct {
	Gate   string `json:"gate"`
	Reason string `json:"reason"`
}

type GateResult struct {
	Gate    string   `json:"gate"`
	Passed  bool     `json:"passed"`
	Details []string `json:"details,omitempty"`
}

type ConflictSide struct {
	ID         string `json:"id"`
	Ref        string `json:"ref"`
	Statement  string `json:"statement"`
	SourceName string `json:"source_name"`
	Cited      bool   `json:"cited"`
}

type ConflictView struct {
	Group string         `json:"group"`
	Sides []ConflictSide `json:"sides"`
}

type BriefEvidence struct {
	ID         string `json:"id"`
	Ref        string `json:"ref"`
	Kind       string `json:"kind"`
	Quality    string `json:"quality"`
	Statement  string `json:"statement"`
	SourceName string `json:"source_name"`
}

// Brief is the campaign brief; business-facing text is Indonesian.
type Brief struct {
	Title           string          `json:"title"`
	Segment         string          `json:"segment"`
	Need            string          `json:"need"`
	Interpretation  string          `json:"interpretation"`
	Facts           []BriefEvidence `json:"facts"`
	Interpretations []BriefEvidence `json:"interpretations"`
	Hypotheses      []BriefEvidence `json:"hypotheses"`
	Message         string          `json:"message"`
	OfferConcept    string          `json:"offer_concept"`
	Channel         string          `json:"channel"`
	TestDesign      string          `json:"test_design"`
	SuccessMetric   string          `json:"success_metric"`
	SuccessCriteria string          `json:"success_criteria"`
	Resources       string          `json:"resources"`
	Effort          string          `json:"effort"`
	Risks           []string        `json:"risks"`
	Assumptions     []string        `json:"assumptions"`
	MissingEvidence []string        `json:"missing_evidence"`
	Dependencies    []string        `json:"dependencies"`
	NextAction      string          `json:"next_action"`
	OperationalNote string          `json:"operational_note"`
}

type Result struct {
	ProposalIndex  int
	Title          string
	Segment        string
	TopicKey       string
	Need           string
	Interpretation string
	Status         string
	Tier           string
	Hypothesis     bool
	RequiresStore  bool
	Dims           map[string]Dim
	OrdinalSum     int
	Caps           []Cap
	Gates          []GateResult
	Conflicts      []ConflictView
	EvidenceIDs    []string
	Roles          map[string]string // item id -> kind
	MergedFrom     []int
	RejectReasons  []string
	Brief          Brief
}

type claimRule struct {
	category string
	re       *regexp.Regexp
	reason   string
}

type Evaluator struct {
	cfg        *config.Config
	checker    *brand.Checker
	claims     []claimRule
	storeTerms []string
	kpis       map[string]bool
	kpisNow    map[string]bool
	pillars    map[string]bool
	allowedNum map[string]bool
}

var (
	numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
	idTokens = regexp.MustCompile(`(?:\b(?:[OB]\d{1,2}|AU\d|PM\d|TG\d|FB\d{1,2}|KP\d|CT\d{1,2}|M\d|L\d{1,2}|Q\d{1,3}|N\d{1,3}|D-\d{3}|T1-#\d)\b|#\d{1,2}\b|§\s?\d+)`)
)

func New(cfg *config.Config, checker *brand.Checker) (*Evaluator, error) {
	e := &Evaluator{cfg: cfg, checker: checker, kpis: map[string]bool{}, kpisNow: map[string]bool{}, pillars: map[string]bool{}, allowedNum: map[string]bool{}}
	for _, r := range cfg.Scoring.UnsupportedClaims {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("unsupported_claims %s: %w", r.Category, err)
		}
		e.claims = append(e.claims, claimRule{category: r.Category, re: re, reason: r.Reason})
	}
	for _, t := range cfg.Scoring.StoreDependencyTerms {
		e.storeTerms = append(e.storeTerms, strings.ToLower(t))
	}
	for _, k := range cfg.Scoring.KPIs {
		e.kpis[k] = true
	}
	for _, k := range cfg.Scoring.KPIsAvailableNow {
		e.kpisNow[k] = true
	}
	for _, p := range cfg.Scoring.Pillars {
		e.pillars[p] = true
	}
	for _, n := range cfg.Business.AllowedNumbers {
		e.allowedNum[n] = true
	}
	return e, nil
}

// DetectConflicts assigns a conflict group to validated facts that share a
// topic_key, come from different sources, differ in negation polarity, and are
// otherwise similar statements (token Jaccard >= minSimilarity once the negation
// words are removed). Deterministic and documented as a heuristic: statements
// about different subjects under one topic are not conflicts, and paraphrased
// contradictions can be missed.
func DetectConflicts(items []Evidence, negation *regexp.Regexp, minSimilarity float64) map[string]string {
	byTopic := map[string][]Evidence{}
	for _, it := range items {
		if it.Kind != domain.KindFact || it.TopicKey == "" {
			continue
		}
		byTopic[it.TopicKey] = append(byTopic[it.TopicKey], it)
	}
	groups := map[string]string{}
	topics := make([]string, 0, len(byTopic))
	for t := range byTopic {
		topics = append(topics, t)
	}
	sort.Strings(topics)
	for _, t := range topics {
		list := byTopic[t]
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				a, b := list[i], list[j]
				if a.SourceID == b.SourceID {
					continue
				}
				if negation.MatchString(a.Statement) != negation.MatchString(b.Statement) &&
					dedup.Jaccard(negation.ReplaceAllString(a.Statement, " "), negation.ReplaceAllString(b.Statement, " ")) >= minSimilarity {
					g := "cg:" + t
					groups[a.ID] = g
					groups[b.ID] = g
				}
			}
		}
	}
	return groups
}

type candidate struct {
	idx      int
	p        agent.Proposal
	ids      []string // valid cited item ids, in citation order
	invalid  []string
	merged   []int
	mergedIn bool
}

func (e *Evaluator) resolve(p agent.Proposal, ev map[string]Evidence) ([]string, []string) {
	var ids, invalid []string
	seen := map[string]bool{}
	for _, r := range p.FactRefs {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		it, ok := ev[r]
		if !ok {
			invalid = append(invalid, fmt.Sprintf("referensi %q tidak ada atau belum divalidasi; diabaikan", r))
			continue
		}
		ids = append(ids, it.ID)
	}
	return ids, invalid
}

func sharedFraction(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	n := 0
	for _, y := range b {
		if set[y] {
			n++
		}
	}
	m := len(a)
	if len(b) < m {
		m = len(b)
	}
	return float64(n) / float64(m)
}

// Evaluate applies dedup and all gates to the proposals, in proposal order.
func (e *Evaluator) Evaluate(proposals []agent.Proposal, ev map[string]Evidence) []Result {
	byID := map[string]Evidence{}
	for _, it := range ev {
		byID[it.ID] = it
	}
	var cands []*candidate
	for i, p := range proposals {
		ids, invalid := e.resolve(p, ev)
		cands = append(cands, &candidate{idx: i + 1, p: p, ids: ids, invalid: invalid})
	}
	// Gate 5: deduplicate within topic_key; the earlier proposal survives and absorbs corroborating evidence.
	for i, c := range cands {
		if c.mergedIn {
			continue
		}
		for j := 0; j < i; j++ {
			k := cands[j]
			if k.mergedIn || k.p.TopicKey == "" || k.p.TopicKey != c.p.TopicKey {
				continue
			}
			if sharedFraction(k.ids, c.ids) >= e.cfg.Scoring.Dedup.SharedEvidenceFraction || dedup.Jaccard(k.p.Title, c.p.Title) >= e.cfg.Scoring.Dedup.TitleJaccard {
				c.mergedIn = true
				k.merged = append(k.merged, c.idx)
				have := map[string]bool{}
				for _, id := range k.ids {
					have[id] = true
				}
				for _, id := range c.ids {
					if !have[id] {
						k.ids = append(k.ids, id)
						have[id] = true
					}
				}
				k.invalid = append(k.invalid, c.invalid...)
				break
			}
		}
	}
	var out []Result
	for _, c := range cands {
		if c.mergedIn {
			continue
		}
		out = append(out, e.evaluateOne(c, byID))
	}
	return out
}

func (e *Evaluator) texts(p agent.Proposal) string {
	parts := []string{p.Title, p.Need, p.Interpretation, p.Message, p.OfferConcept, p.Channel, p.TestDesign, p.SuccessCriteria, p.Resources, p.NextAction}
	parts = append(parts, p.Risks...)
	parts = append(parts, p.Assumptions...)
	parts = append(parts, p.MissingEvidence...)
	parts = append(parts, p.Dependencies...)
	for _, r := range p.DimensionRationale {
		parts = append(parts, r)
	}
	return strings.Join(parts, "\n")
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (e *Evaluator) evaluateOne(c *candidate, byID map[string]Evidence) Result {
	p := c.p
	r := Result{ProposalIndex: c.idx, Title: p.Title, Segment: p.Segment, TopicKey: p.TopicKey, Need: p.Need, Interpretation: p.Interpretation,
		Dims: map[string]Dim{}, Roles: map[string]string{}, MergedFrom: c.merged, Status: domain.OppProposed, Tier: domain.TierNone}
	text := e.texts(p)

	// Gate 1 — evidence: every cited id must exist and be validated (resolve already dropped the rest).
	g1 := GateResult{Gate: "evidence", Passed: true, Details: append([]string(nil), c.invalid...)}
	var facts, interps, hyps []Evidence
	for _, id := range c.ids {
		it := byID[id]
		r.EvidenceIDs = append(r.EvidenceIDs, id)
		r.Roles[id] = it.Kind
		switch it.Kind {
		case domain.KindFact:
			facts = append(facts, it)
		case domain.KindInterpretation:
			interps = append(interps, it)
		default:
			hyps = append(hyps, it)
		}
	}
	strength, strengthWhy := evidenceStrength(facts)
	if len(c.ids) == 0 {
		g1.Passed = false
		g1.Details = append(g1.Details, "tidak ada bukti tervalidasi yang dikutip")
		r.Status = domain.OppRejected
		r.RejectReasons = append(r.RejectReasons, "tidak ada bukti tervalidasi yang dikutip (Gerbang 1)")
	}
	r.Hypothesis = strength == 0 && r.Status != domain.OppRejected
	if r.Hypothesis && r.Status != domain.OppRejected {
		g1.Details = append(g1.Details, "tidak ada fakta tervalidasi berkualitas A/B yang dikutip: dicatat sebagai hipotesis, tingkat maksimal C")
		r.Caps = append(r.Caps, Cap{Gate: "evidence", Reason: "hipotesis tanpa fakta A/B"})
	}
	r.Gates = append(r.Gates, g1)

	// Gate 2 — conflicts: any cited item in a conflict group shows both sides and caps the tier at C.
	g2 := GateResult{Gate: "conflict", Passed: true}
	groups := map[string]bool{}
	for _, id := range c.ids {
		if g := byID[id].ConflictGroup; g != "" {
			groups[g] = true
		}
	}
	if len(groups) > 0 {
		g2.Passed = false
		cited := map[string]bool{}
		for _, id := range c.ids {
			cited[id] = true
		}
		var names []string
		for g := range groups {
			names = append(names, g)
		}
		sort.Strings(names)
		for _, g := range names {
			view := ConflictView{Group: g}
			var members []Evidence
			for _, it := range byID {
				if it.ConflictGroup == g {
					members = append(members, it)
				}
			}
			sort.Slice(members, func(i, j int) bool { return members[i].Ref < members[j].Ref })
			for _, m := range members {
				view.Sides = append(view.Sides, ConflictSide{ID: m.ID, Ref: m.Ref, Statement: m.Statement, SourceName: m.SourceName, Cited: cited[m.ID]})
			}
			r.Conflicts = append(r.Conflicts, view)
			g2.Details = append(g2.Details, fmt.Sprintf("bukti yang dikutip berada dalam kelompok konflik %s (%d sisi ditampilkan); tingkat maksimal C sampai konflik diselesaikan", g, len(view.Sides)))
		}
		r.Caps = append(r.Caps, Cap{Gate: "conflict", Reason: "bukti saling bertentangan belum diselesaikan"})
	}
	r.Gates = append(r.Gates, g2)

	// Gate 3 — brand and factual claims.
	g3 := GateResult{Gate: "brand", Passed: true}
	if e.checker != nil {
		res := e.checker.Check(text)
		if !res.Passes {
			g3.Passed = false
			for _, f := range res.Findings {
				g3.Details = append(g3.Details, fmt.Sprintf("%s: %q", f.Category, f.Match))
			}
			r.Status = domain.OppBlocked
			r.RejectReasons = append(r.RejectReasons, "gagal pemeriksa teks merek: "+strings.Join(g3.Details, "; "))
		}
	}
	r.Gates = append(r.Gates, g3)
	g3b := GateResult{Gate: "factual_claims", Passed: true}
	for _, cr := range e.claims {
		if m := cr.re.FindString(text); m != "" {
			g3b.Passed = false
			g3b.Details = append(g3b.Details, fmt.Sprintf("%s (%q): %s", cr.category, strings.TrimSpace(m), cr.reason))
		}
	}
	// Numbers in generated text must appear in cited evidence or in the allowed list.
	allowed := map[string]bool{}
	for k := range e.allowedNum {
		allowed[k] = true
	}
	for _, id := range c.ids {
		it := byID[id]
		for _, n := range numberRe.FindAllString(it.Statement+" "+it.Excerpt+" "+it.Title, -1) {
			allowed[n] = true
		}
	}
	for _, n := range numberRe.FindAllString(idTokens.ReplaceAllString(text, " "), -1) {
		if !allowed[n] {
			g3b.Passed = false
			g3b.Details = append(g3b.Details, fmt.Sprintf("angka %q tidak muncul dalam bukti yang dikutip", n))
		}
	}
	if !g3b.Passed && r.Status != domain.OppBlocked {
		r.Status = domain.OppRejected
		r.RejectReasons = append(r.RejectReasons, "klaim faktual tanpa dukungan: "+strings.Join(g3b.Details, "; "))
	}
	r.Gates = append(r.Gates, g3b)

	// Gate 4 — operational dependency: scan the execution-oriented fields only (message, offer, channel,
	// test design, resources, dependencies, next action); need, interpretation and rationales describe evidence
	// and may legitimately mention other stores.
	g4 := GateResult{Gate: "operational", Passed: true}
	lower := strings.ToLower(strings.Join(append([]string{p.Message, p.OfferConcept, p.Channel, p.TestDesign, p.Resources, p.NextAction}, p.Dependencies...), "\n"))
	var hits []string
	for _, t := range e.storeTerms {
		if strings.Contains(lower, t) {
			hits = append(hits, t)
		}
	}
	r.RequiresStore = p.RequiresStore || len(hits) > 0
	if r.RequiresStore {
		g4.Passed = false // not a failure: a label
		g4.Details = append(g4.Details, "memerlukan toko, staf, atau kapabilitas operasional yang belum terverifikasi (usaha belum beroperasi)")
		if len(hits) > 0 {
			g4.Details = append(g4.Details, "istilah operasional: "+strings.Join(hits, ", "))
		}
	}
	r.Gates = append(r.Gates, g4)

	// Scores: evidence strength computed by code; agent values for the rest, capped by code rules.
	r.Dims["evidence_strength"] = Dim{Value: strength, Rationale: strengthWhy, Source: "code"}
	intent := clamp(p.Dimensions.CustomerIntent, 0, 3)
	intentSrc := "agent"
	srcs := distinctSources(facts)
	if strength == 0 && intent > 1 {
		intent, intentSrc = 1, "agent+cap (tanpa fakta tervalidasi berkualitas A–C)"
	} else if srcs < 2 && intent > 2 {
		intent, intentSrc = 2, "agent+cap (kurang dari dua sumber independen)"
	}
	r.Dims["customer_intent"] = Dim{Value: intent, Rationale: p.DimensionRationale["customer_intent"], Source: intentSrc}
	fit := clamp(p.Dimensions.StrategicFit, 0, 3)
	fitSrc := "agent"
	validPillars := 0
	for _, pl := range p.FitPillars {
		if e.pillars[pl] {
			validPillars++
		}
	}
	if validPillars == 0 && fit > 1 {
		fit, fitSrc = 1, "agent+cap (tidak merujuk pilar PM1–PM4)"
	}
	r.Dims["strategic_fit"] = Dim{Value: fit, Rationale: p.DimensionRationale["strategic_fit"], Source: fitSrc}
	ease := clamp(p.Dimensions.EaseOfTesting, 0, 3)
	easeSrc := "agent"
	if r.RequiresStore && ease > 1 {
		ease, easeSrc = 1, "agent+cap (memerlukan toko/operasional)"
	}
	r.Dims["ease_of_testing"] = Dim{Value: ease, Rationale: p.DimensionRationale["ease_of_testing"], Source: easeSrc}
	meas := clamp(p.Dimensions.Measurability, 0, 3)
	measSrc := "agent"
	switch {
	case !e.kpis[p.SuccessMetric]:
		meas, measSrc = 0, "code (success_metric bukan KPI yang dikenal)"
	case !e.kpisNow[p.SuccessMetric] && meas > 1:
		meas, measSrc = 1, "agent+cap (data KPI belum tersedia sebelum toko buka)"
	}
	r.Dims["measurability"] = Dim{Value: meas, Rationale: p.DimensionRationale["measurability"], Source: measSrc}
	r.OrdinalSum = strength + intent + fit + ease + meas

	// Tier rules (code), with caps.
	capC := r.Hypothesis || len(groups) > 0
	t := e.cfg.Scoring.Tiers
	switch {
	case r.Status == domain.OppRejected || r.Status == domain.OppBlocked:
		r.Tier = domain.TierNone
	case !capC && strength >= t.A.EvidenceMin && intent >= t.A.IntentMin && fit >= t.A.FitMin:
		r.Tier = domain.TierA
	case !capC && strength >= t.B.EvidenceMin && fit >= t.B.FitMin:
		r.Tier = domain.TierB
	default:
		r.Tier = domain.TierC
	}

	// Brief.
	b := Brief{Title: p.Title, Segment: p.Segment, Need: p.Need, Interpretation: p.Interpretation, Message: p.Message, OfferConcept: p.OfferConcept, Channel: p.Channel,
		TestDesign: p.TestDesign, SuccessMetric: p.SuccessMetric, SuccessCriteria: p.SuccessCriteria, Resources: p.Resources, Effort: p.Effort,
		Risks: p.Risks, Assumptions: p.Assumptions, MissingEvidence: p.MissingEvidence, Dependencies: p.Dependencies, NextAction: p.NextAction}
	for _, f := range facts {
		b.Facts = append(b.Facts, briefEv(f))
	}
	for _, f := range interps {
		b.Interpretations = append(b.Interpretations, briefEv(f))
	}
	for _, f := range hyps {
		b.Hypotheses = append(b.Hypotheses, briefEv(f))
	}
	if r.Hypothesis {
		b.MissingEvidence = append(b.MissingEvidence, "Belum ada fakta tervalidasi berkualitas A/B yang mendukung; kumpulkan bukti sebelum dijalankan.")
	}
	for _, cv := range r.Conflicts {
		b.Dependencies = append(b.Dependencies, "Selesaikan konflik bukti "+cv.Group+" sebelum peluang ini dinaikkan dari tingkat C.")
	}
	if r.RequiresStore {
		b.OperationalNote = "Memerlukan ketersediaan toko atau konfirmasi operasional yang belum terverifikasi; kapabilitas ini belum menjadi fakta."
		b.Dependencies = append(b.Dependencies, "Konfirmasi operasional (toko, staf, atau kapabilitas terkait) sebelum dijalankan.")
	}
	if p.SuccessMetric != "" && e.kpis[p.SuccessMetric] && !e.kpisNow[p.SuccessMetric] {
		b.Dependencies = append(b.Dependencies, "Data KPI "+p.SuccessMetric+" belum tersedia sebelum toko beroperasi.")
	}
	r.Brief = b
	return r
}

func briefEv(f Evidence) BriefEvidence {
	return BriefEvidence{ID: f.ID, Ref: f.Ref, Kind: f.Kind, Quality: f.Quality, Statement: f.Statement, SourceName: f.SourceName}
}

func distinctSources(facts []Evidence) int {
	s := map[string]bool{}
	for _, f := range facts {
		s[f.SourceID] = true
	}
	return len(s)
}

// evidenceStrength implements the documented rubric from the cited validated facts.
func evidenceStrength(facts []Evidence) (int, string) {
	if len(facts) == 0 {
		return 0, "tidak ada fakta tervalidasi yang dikutip"
	}
	bySrc := map[string]map[string]bool{} // quality -> sources
	for _, f := range facts {
		if bySrc[f.Quality] == nil {
			bySrc[f.Quality] = map[string]bool{}
		}
		bySrc[f.Quality][f.SourceID] = true
	}
	switch {
	case len(bySrc["A"]) >= 1:
		return 3, "minimal satu fakta kualitas A dikutip"
	case len(bySrc["B"]) >= 2:
		return 3, "dua fakta kualitas B dari sumber berbeda"
	case len(bySrc["B"]) == 1:
		return 2, "satu fakta kualitas B"
	case len(bySrc["C"]) >= 2:
		return 2, "dua fakta kualitas C dari sumber berbeda"
	case len(bySrc["C"]) == 1:
		return 1, "satu fakta kualitas C"
	default:
		return 0, "fakta yang dikutip berkualitas D (tanpa bukti eksternal)"
	}
}
