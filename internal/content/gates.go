package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	agent "github.com/ivanderkevin123-spec/punyamas-mia/internal/agent/content"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/brand"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/claims"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
)

// Finding severities.
const (
	SeverityBlock  = "block"  // the draft is blocked and cannot be reviewed until revised
	SeverityReview = "review" // the draft needs human review before any approval
	SeverityInfo   = "info"   // recorded, no status effect
)

// Finding is one gate result on a draft.
type Finding struct {
	Gate     string `json:"gate"` // evidence | brand | factual_claims | language | placeholder | claims
	Category string `json:"category"`
	Match    string `json:"match,omitempty"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}

// ClaimView is a claim with its evidence resolved by code.
type ClaimView struct {
	Text         string   `json:"text"`
	Kind         string   `json:"kind"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	EvidenceIDs  []string `json:"evidence_ids,omitempty"`
	FactIDs      []string `json:"fact_ids,omitempty"`
	Supported    bool     `json:"supported"`
	Note         string   `json:"note,omitempty"`
}

// ChecklistItem is one human-review checklist line; Source tells whether code or the agent added it.
type ChecklistItem struct {
	Text   string `json:"text"`
	Source string `json:"source"` // code | agent
}

// Result is the code verdict on one proposed draft.
type Result struct {
	Index          int
	Draft          agent.Draft
	Status         string
	EvidenceIDs    []string
	Roles          map[string]string
	Claims         []ClaimView
	ClaimsToVerify []string
	Placeholders   []string
	Checklist      []ChecklistItem
	Findings       []Finding
	BrandFactIDs   []string
	ContentHash    string
	DuplicateOf    string // "usulan #N" (same run) or the id of an existing draft
}

type langRule struct {
	category, reason, action string
	re                       *regexp.Regexp
}

// Evaluator applies the content gates.
type Evaluator struct {
	cfg     *config.Config
	checker *brand.Checker
	claims  *claims.Scanner
	rules   []langRule
	facts   map[string]config.Fact
}

var (
	placeholderRe = regexp.MustCompile(`(?i)\[\s*ISI\s*:([^\[\]\n]*)\]`)
	listMarkerRe  = regexp.MustCompile(`(?m)^\s*\d{1,2}[.)]\s+`)
	digitRe       = regexp.MustCompile(`\d`)
)

func New(cfg *config.Config, checker *brand.Checker) (*Evaluator, error) {
	sc, err := claims.New(cfg.Scoring.UnsupportedClaims, cfg.Business.AllowedNumbers)
	if err != nil {
		return nil, err
	}
	e := &Evaluator{cfg: cfg, checker: checker, claims: sc, facts: map[string]config.Fact{}}
	for _, r := range cfg.Content.LanguageRules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("content language rule %s: %w", r.Category, err)
		}
		e.rules = append(e.rules, langRule{category: r.Category, reason: r.Reason, action: r.Action, re: re})
	}
	for _, f := range cfg.Business.Facts {
		e.facts[f.ID] = f
	}
	return e, nil
}

// PublicText joins the fields a reader would see.
func PublicText(d agent.Draft) string {
	return strings.Join([]string{d.Hook, d.Headline, d.Body, d.CallToAction}, "\n")
}

func normLabel(s string) string { return strings.Join(strings.Fields(s), " ") }

// Placeholders returns the normalised [ISI: ...] markers in text, unique and in order of appearance.
func Placeholders(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range placeholderRe.FindAllStringSubmatch(text, -1) {
		p := "[ISI: " + normLabel(m[1]) + "]"
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// StripPlaceholders replaces [ISI: ...] markers with a space so their labels are not scanned as claims.
func StripPlaceholders(text string) string { return placeholderRe.ReplaceAllString(text, " ") }

// Hash is the content hash of a draft version: type, channel and the whitespace-normalised public fields.
func Hash(contentType, channel, hook, headline, body, cta string) string {
	parts := []string{contentType, channel, normLabel(hook), normLabel(headline), normLabel(body), normLabel(cta)}
	h := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(h[:])
}

func uniq(lists ...[]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range lists {
		for _, s := range l {
			s = strings.TrimSpace(s)
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

func validQuality(q string) bool { return q == "A" || q == "B" || q == "C" }

// Evaluate applies every content gate to one draft of the given source opportunity.
func (e *Evaluator) Evaluate(d agent.Draft, src *Source) Result {
	r := Result{Draft: d, Roles: map[string]string{}}
	add := func(f Finding) { r.Findings = append(r.Findings, f) }
	text := PublicText(d)

	// Placeholders: the authoritative list is computed from the text; labels describe, they never carry values.
	for _, m := range placeholderRe.FindAllStringSubmatch(text, -1) {
		label := normLabel(m[1])
		switch {
		case utf8.RuneCountInString(label) < 3:
			add(Finding{Gate: "placeholder", Category: "PLACEHOLDER_TIDAK_BAKU", Match: m[0], Reason: "penanda [ISI: ...] tanpa keterangan", Severity: SeverityReview})
		case digitRe.MatchString(label):
			add(Finding{Gate: "placeholder", Category: "PLACEHOLDER_BERNILAI", Match: m[0], Reason: "penanda isian berisi nilai; isi keterangan saja, nilai diisi setelah diverifikasi", Severity: SeverityBlock})
		}
	}
	r.Placeholders = Placeholders(text)
	inText := map[string]bool{}
	for _, p := range r.Placeholders {
		inText[p] = true
	}
	for _, p := range d.Placeholders {
		if norm := Placeholders(p); len(norm) == 1 && !inText[norm[0]] {
			add(Finding{Gate: "placeholder", Category: "PLACEHOLDER_TIDAK_DIPAKAI", Match: p, Reason: "tercantum di metadata tetapi tidak ada di teks", Severity: SeverityInfo})
		}
	}
	scan := StripPlaceholders(text)
	scanNums := listMarkerRe.ReplaceAllString(scan, " ")

	// Gate A — evidence: refs must belong to this opportunity and still be validated.
	var support []string
	resolve := func(ref string) (string, bool) {
		if ev, ok := src.ByRef[ref]; ok {
			return ev.ID, true
		}
		switch status, exists := src.ResearchRefs[ref]; {
		case exists && status == domain.ItemValidated:
			add(Finding{Gate: "evidence", Category: "BUKTI_LINTAS_PELUANG", Match: ref, Reason: "butir ini bukan bukti peluang yang disetujui", Severity: SeverityBlock})
		case exists:
			add(Finding{Gate: "evidence", Category: "BUKTI_BELUM_VALID", Match: ref, Reason: "butir belum divalidasi (status " + status + ")", Severity: SeverityBlock})
		default:
			add(Finding{Gate: "evidence", Category: "BUKTI_TIDAK_ADA", Match: ref, Reason: "referensi tidak ada di run riset induk", Severity: SeverityBlock})
		}
		return "", false
	}
	seenRef := map[string]string{}
	resolveOnce := func(ref string) (string, bool) {
		if id, ok := seenRef[ref]; ok {
			return id, id != ""
		}
		id, ok := resolve(ref)
		seenRef[ref] = id
		return id, ok
	}
	var claimRefs []string
	for _, c := range d.Claims {
		claimRefs = append(claimRefs, c.EvidenceRefs...)
	}
	for _, ref := range uniq(d.SupportingEvidenceRefs, claimRefs) {
		if id, ok := resolveOnce(ref); ok {
			ev := src.ByID[id]
			r.EvidenceIDs = append(r.EvidenceIDs, id)
			r.Roles[id] = ev.Kind
			support = append(support, ev.Statement+" "+ev.Excerpt+" "+ev.Title)
		}
	}
	var claimFacts []string
	for _, c := range d.Claims {
		claimFacts = append(claimFacts, c.FactIDs...)
	}
	var rencana []string
	for _, id := range uniq(d.BrandFactIDs, claimFacts) {
		f, ok := e.facts[id]
		if !ok {
			add(Finding{Gate: "evidence", Category: "FAKTA_TIDAK_ADA", Match: id, Reason: "ID fakta usaha tidak ada di business.yaml", Severity: SeverityBlock})
			continue
		}
		r.BrandFactIDs = append(r.BrandFactIDs, id)
		support = append(support, f.Text)
		if f.Status == "RENCANA" {
			rencana = append(rencana, id)
		}
	}
	if len(d.SupportingEvidenceRefs) == 0 && len(r.BrandFactIDs) == 0 {
		add(Finding{Gate: "evidence", Category: "TANPA_BUKTI", Reason: "draf tidak mengutip bukti peluang maupun fakta usaha", Severity: SeverityReview})
	}

	// Claims: facts need a resolved validated fact (quality A–C) or an approved business fact.
	for _, c := range d.Claims {
		v := ClaimView{Text: strings.TrimSpace(c.Text), Kind: c.Kind, EvidenceRefs: c.EvidenceRefs, FactIDs: c.FactIDs}
		for _, ref := range c.EvidenceRefs {
			if id, ok := resolveOnce(ref); ok {
				v.EvidenceIDs = append(v.EvidenceIDs, id)
				if ev := src.ByID[id]; c.Kind == agent.ClaimFact && ev.Kind == domain.KindFact && validQuality(ev.Quality) {
					v.Supported = true
				}
			}
		}
		for _, id := range c.FactIDs {
			if _, ok := e.facts[id]; ok && c.Kind == agent.ClaimFact {
				v.Supported = true
			}
		}
		switch {
		case c.Kind != agent.ClaimFact:
			v.Note = "bukan fakta (" + c.Kind + ")"
		case !v.Supported:
			v.Note = "belum didukung bukti; perlu verifikasi"
			r.ClaimsToVerify = append(r.ClaimsToVerify, v.Text)
			add(Finding{Gate: "claims", Category: "KLAIM_PERLU_VERIFIKASI", Match: v.Text, Reason: "klaim fakta tanpa bukti tervalidasi atau fakta usaha", Severity: SeverityReview})
		}
		r.Claims = append(r.Claims, v)
	}

	// Gate B — brand checker, factual claims, marketing-language rules (public text, placeholders removed).
	if e.checker != nil {
		for _, f := range e.checker.Check(scan).Findings {
			add(Finding{Gate: "brand", Category: f.Category, Match: f.Match, Reason: f.Reason, Severity: SeverityBlock})
		}
	}
	for _, f := range e.claims.Categories(scan) {
		add(Finding{Gate: "factual_claims", Category: f.Category, Match: f.Match, Reason: f.Reason, Severity: SeverityBlock})
	}
	for _, n := range e.claims.UnsupportedNumbers(scanNums, support...) {
		add(Finding{Gate: "factual_claims", Category: claims.CategoryNumber, Match: n, Reason: "angka tidak muncul dalam bukti yang dikutip, fakta usaha, atau daftar angka yang disetujui", Severity: SeverityBlock})
	}
	for _, lr := range e.rules {
		if m := lr.re.FindString(scan); m != "" {
			add(Finding{Gate: "language", Category: lr.category, Match: strings.TrimSpace(m), Reason: lr.reason, Severity: lr.action})
		}
	}

	// Review checklist: code items first, then the agent's own items.
	var items []ChecklistItem
	seen := map[string]bool{}
	push := func(text, source string) {
		text = strings.TrimSpace(text)
		if text == "" || seen[text] || utf8.RuneCountInString(text) > 240 {
			return
		}
		seen[text] = true
		items = append(items, ChecklistItem{Text: text, Source: source})
	}
	for _, c := range e.cfg.Content.ReviewChecklist {
		push(c, "code")
	}
	for _, c := range e.cfg.Content.Types[d.ContentType].Checklist {
		push(c, "code")
	}
	for _, p := range r.Placeholders {
		push("Isi "+p+" dengan data terverifikasi sebelum digunakan.", "code")
	}
	for _, c := range r.ClaimsToVerify {
		push(fmt.Sprintf("Verifikasi atau hapus klaim: %q.", c), "code")
	}
	for _, id := range rencana {
		push("Fakta "+id+" berstatus RENCANA: pastikan ditulis dalam bahasa niat.", "code")
	}
	n := 0
	for _, c := range d.ReviewChecklist {
		if n < 10 {
			push(c, "agent")
			n++
		}
	}
	r.Checklist = items
	r.Status = statusFor(r.Findings, r.Placeholders, r.ClaimsToVerify)
	r.ContentHash = Hash(d.ContentType, d.Channel, d.Hook, d.Headline, d.Body, d.CallToAction)
	return r
}

func statusFor(findings []Finding, placeholders, toVerify []string) string {
	review := len(placeholders) > 0 || len(toVerify) > 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityBlock:
			return domain.DraftBlocked
		case SeverityReview:
			review = true
		}
	}
	if review {
		return domain.DraftNeedsReview
	}
	return domain.DraftDraft
}

func similarityText(hook, headline, body string) string { return hook + " " + headline + " " + body }

// EvaluateAll evaluates drafts in output order and marks near-duplicates (same type and channel, token
// Jaccard of hook+headline+body >= duplicate_similarity) of an earlier draft of this run or of an existing
// draft of the same opportunity. Duplicates are reported, never stored.
func (e *Evaluator) EvaluateAll(items []agent.Indexed, src *Source, existing []*domain.ContentDraft) []Result {
	thr := e.cfg.Content.DuplicateSimilarity
	var out, kept []Result
	for _, it := range items {
		r := e.Evaluate(it.Draft, src)
		r.Index = it.Index
		d := it.Draft
		for _, k := range kept {
			if k.Draft.ContentType == d.ContentType && k.Draft.Channel == d.Channel &&
				(k.ContentHash == r.ContentHash || dedup.Jaccard(similarityText(k.Draft.Hook, k.Draft.Headline, k.Draft.Body), similarityText(d.Hook, d.Headline, d.Body)) >= thr) {
				r.DuplicateOf = fmt.Sprintf("usulan #%d", k.Index)
				break
			}
		}
		if r.DuplicateOf == "" {
			for _, x := range existing {
				if x.ContentType == d.ContentType && x.Channel == d.Channel &&
					(x.ContentHash == r.ContentHash || dedup.Jaccard(similarityText(x.Hook, x.Headline, x.Body), similarityText(d.Hook, d.Headline, d.Body)) >= thr) {
					r.DuplicateOf = x.ID
					break
				}
			}
		}
		out = append(out, r)
		if r.DuplicateOf == "" {
			kept = append(kept, r)
		}
	}
	return out
}

// Provenance carries the run-level metadata stored on every draft.
type Provenance struct {
	RunID, ExecutionID, ConfigHash, ContentConfigHash, FixtureHash, Now string
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return "[]"
	}
	return string(b)
}

// Record builds the rows stored for a non-duplicate result: draft, first version, evidence links.
func (r Result) Record(id string, src *Source, p Provenance) (*domain.ContentDraft, []domain.ContentDraftEvidence, *domain.ContentDraftVersion) {
	d := r.Draft
	draft := &domain.ContentDraft{ID: id, RunID: p.RunID, OpportunityID: src.Opportunity.ID, StrategyRunID: src.Opportunity.RunID, ResearchRunID: src.ResearchRunID,
		ProposalIndex: r.Index, ContentType: d.ContentType, Variant: d.Variant, Channel: d.Channel, TargetSegment: d.TargetSegment, Objective: strings.TrimSpace(d.Objective),
		Hook: strings.TrimSpace(d.Hook), Headline: strings.TrimSpace(d.Headline), Body: strings.TrimSpace(d.Body), CallToAction: strings.TrimSpace(d.CallToAction),
		ClaimsJSON: mustJSON(r.Claims), PlaceholdersJSON: mustJSON(r.Placeholders), AssumptionsJSON: mustJSON(d.Assumptions), RisksJSON: mustJSON(d.Risks),
		ChecklistJSON: mustJSON(r.Checklist), FindingsJSON: mustJSON(r.Findings), BrandFactIDsJSON: mustJSON(r.BrandFactIDs),
		Status: r.Status, Version: 1, ContentHash: r.ContentHash, ExecutionID: p.ExecutionID, ConfigHash: p.ConfigHash, ContentConfigHash: p.ContentConfigHash,
		ScoringConfigHash: src.Opportunity.ScoringConfigHash, FixtureHash: p.FixtureHash, CreatedAt: p.Now, UpdatedAt: p.Now}
	var links []domain.ContentDraftEvidence
	for _, eid := range r.EvidenceIDs {
		links = append(links, domain.ContentDraftEvidence{DraftID: id, OpportunityID: src.Opportunity.ID, ItemID: eid, Role: r.Roles[eid]})
	}
	v := &domain.ContentDraftVersion{DraftID: id, Version: 1, Hook: draft.Hook, Headline: draft.Headline, Body: draft.Body, CallToAction: draft.CallToAction,
		ContentHash: r.ContentHash, Author: "agent", StatusAfter: r.Status, FindingsJSON: draft.FindingsJSON, PlaceholdersJSON: draft.PlaceholdersJSON, CreatedAt: p.Now}
	return draft, links, v
}

// Details is the decoded JSON metadata of a stored draft.
type Details struct {
	Claims       []ClaimView
	Placeholders []string
	Assumptions  []string
	Risks        []string
	Checklist    []ChecklistItem
	Findings     []Finding
	BrandFactIDs []string
}

func DecodeDetails(d *domain.ContentDraft) Details {
	var dt Details
	_ = json.Unmarshal([]byte(d.ClaimsJSON), &dt.Claims)
	_ = json.Unmarshal([]byte(d.PlaceholdersJSON), &dt.Placeholders)
	_ = json.Unmarshal([]byte(d.AssumptionsJSON), &dt.Assumptions)
	_ = json.Unmarshal([]byte(d.RisksJSON), &dt.Risks)
	_ = json.Unmarshal([]byte(d.ChecklistJSON), &dt.Checklist)
	_ = json.Unmarshal([]byte(d.FindingsJSON), &dt.Findings)
	_ = json.Unmarshal([]byte(d.BrandFactIDsJSON), &dt.BrandFactIDs)
	return dt
}

// ClaimsToVerify lists fact claims that no evidence supports.
func (dt Details) ClaimsToVerify() []string {
	var out []string
	for _, c := range dt.Claims {
		if c.Kind == agent.ClaimFact && !c.Supported {
			out = append(out, c.Text)
		}
	}
	return out
}

// OpenFindings counts block and review findings.
func (dt Details) OpenFindings() int {
	n := 0
	for _, f := range dt.Findings {
		if f.Severity == SeverityBlock || f.Severity == SeverityReview {
			n++
		}
	}
	return n
}
