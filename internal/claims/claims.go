// Package claims scans generated marketing text for factual claims that need
// source support: configured regex categories (prices, hours, addresses,
// statistics, customer claims) and numbers that appear neither in the
// supporting evidence nor in the approved number list. It matches text, not
// meaning: paraphrased or implied claims can pass, so human review stays
// mandatory. Shared by the Strategy Agent (Phase 3) and the Content Agent (Phase 4).
package claims

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
)

// CategoryNumber is the finding category for unsupported numbers.
const CategoryNumber = "ANGKA"

// Finding is one unsupported factual claim.
type Finding struct {
	Category string `json:"category"`
	Match    string `json:"match"`
	Reason   string `json:"reason"`
}

type rule struct {
	category string
	re       *regexp.Regexp
	reason   string
}

// Scanner applies the configured claim rules and the number-support check.
type Scanner struct {
	rules   []rule
	allowed map[string]bool
}

var (
	numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
	// idTokens are internal identifiers (KPIs, segments, pillars, facts, decisions, sections) whose digits are not claims.
	idTokens = regexp.MustCompile(`(?:\b(?:[OB]\d{1,2}|AU\d|PM\d|TG\d|FB\d{1,2}|KP\d|CT\d{1,2}|M\d|L\d{1,2}|Q\d{1,3}|N\d{1,3}|D-\d{3}|T1-#\d)\b|#\d{1,2}\b|§\s?\d+)`)
)

// New compiles the rules; allowedNumbers are numbers approved in business.yaml (angka_boleh).
func New(rules []config.Rule, allowedNumbers []string) (*Scanner, error) {
	s := &Scanner{allowed: map[string]bool{}}
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("claim rule %s: %w", r.Category, err)
		}
		s.rules = append(s.rules, rule{category: r.Category, re: re, reason: r.Reason})
	}
	for _, n := range allowedNumbers {
		s.allowed[n] = true
	}
	return s, nil
}

// Categories returns the first match of every rule found in text, in rule order.
func (s *Scanner) Categories(text string) []Finding {
	var out []Finding
	for _, r := range s.rules {
		if m := r.re.FindString(text); m != "" {
			out = append(out, Finding{Category: r.category, Match: strings.TrimSpace(m), Reason: r.reason})
		}
	}
	return out
}

// UnsupportedNumbers returns every number in text (identifier tokens removed), in order and with
// repetitions, that is neither approved nor present in any of the support texts.
func (s *Scanner) UnsupportedNumbers(text string, support ...string) []string {
	ok := map[string]bool{}
	for k := range s.allowed {
		ok[k] = true
	}
	for _, sp := range support {
		for _, n := range numberRe.FindAllString(sp, -1) {
			ok[n] = true
		}
	}
	var out []string
	for _, n := range numberRe.FindAllString(StripIdentifiers(text), -1) {
		if !ok[n] {
			out = append(out, n)
		}
	}
	return out
}

// Scan returns rule findings followed by unsupported-number findings.
func (s *Scanner) Scan(text string, support ...string) []Finding {
	out := s.Categories(text)
	for _, n := range s.UnsupportedNumbers(text, support...) {
		out = append(out, Finding{Category: CategoryNumber, Match: n, Reason: "angka tidak muncul dalam bukti yang dikutip atau daftar angka yang disetujui"})
	}
	return out
}

// StripIdentifiers blanks internal identifier tokens so their digits are not read as claims.
func StripIdentifiers(text string) string { return idTokens.ReplaceAllString(text, " ") }

// Numbers returns the numbers in s.
func Numbers(s string) []string { return numberRe.FindAllString(s, -1) }
