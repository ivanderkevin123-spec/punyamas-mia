// Package brand ports the Punyamas brand text checker (tools/spec-oracle/brand-oracle.mjs)
// as data-driven rules loaded from config/marketing-policy.yaml. It filters words
// and phrases only; it does not judge meaning, so human review stays mandatory.
package brand

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
)

type Finding struct {
	Category string `json:"category"`
	Match    string `json:"match"`
	Reason   string `json:"reason,omitempty"`
}

type Result struct {
	Passes   bool      `json:"passes"`
	Findings []Finding `json:"findings"`
}

type rule struct {
	category string
	re       *regexp.Regexp
	reason   string
}

type Checker struct {
	rules []rule
}

func New(rules []config.Rule) (*Checker, error) {
	c := &Checker{}
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", r.Category, err)
		}
		c.rules = append(c.rules, rule{category: r.Category, re: re, reason: r.Reason})
	}
	return c, nil
}

// Check returns the first match per rule, mirroring periksaTeks() in the workspace checker.
func (c *Checker) Check(text string) Result {
	res := Result{Passes: true}
	for _, r := range c.rules {
		loc := r.re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		m := strings.TrimLeftFunc(text[loc[0]:loc[1]], func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		res.Findings = append(res.Findings, Finding{Category: r.category, Match: m, Reason: r.reason})
	}
	res.Passes = len(res.Findings) == 0
	return res
}

// Categories returns the distinct categories found, in rule order.
func (r Result) Categories() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range r.Findings {
		if !seen[f.Category] {
			seen[f.Category] = true
			out = append(out, f.Category)
		}
	}
	return out
}
