// Package dedup provides URL canonicalization, content hashing and
// near-duplicate detection for sources and research items.
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

// CanonicalURL lowercases scheme and host, drops fragments, utm_* and tracking
// parameters, sorts the remaining query and removes a trailing slash.
func CanonicalURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "fbclid" || lk == "gclid" || lk == "ref" {
			q.Del(k)
		}
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	u.RawQuery = strings.Join(parts, "&")
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u.String()
}

// Normalize lowercases, strips punctuation and collapses whitespace.
func Normalize(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		default:
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// ContentHash is the sha256 (first 32 hex chars) of the normalized text.
func ContentHash(s string) string {
	sum := sha256.Sum256([]byte(Normalize(s)))
	return hex.EncodeToString(sum[:])[:32]
}

// FileHash is the full sha256 of raw bytes.
func FileHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func tokens(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range strings.Fields(Normalize(s)) {
		if len(t) > 1 {
			out[t] = struct{}{}
		}
	}
	return out
}

// Jaccard similarity of token sets.
func Jaccard(a, b string) float64 {
	ta, tb := tokens(a), tokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	inter := 0
	for t := range ta {
		if _, ok := tb[t]; ok {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	return float64(inter) / float64(union)
}

type Candidate struct {
	ID          string
	Title       string
	ContentHash string
}

type Match struct {
	ID          string
	DuplicateOf string
	Reason      string
}

// FindDuplicates keeps the first occurrence as canonical; later candidates with
// the same content hash or a title similarity >= threshold are reported.
func FindDuplicates(cands []Candidate, threshold float64) []Match {
	var out []Match
	byHash := map[string]string{}
	var canon []Candidate
	for _, c := range cands {
		if first, ok := byHash[c.ContentHash]; ok {
			out = append(out, Match{ID: c.ID, DuplicateOf: first, Reason: "isi identik (hash sama)"})
			continue
		}
		dup := false
		for _, k := range canon {
			if sim := Jaccard(c.Title, k.Title); sim >= threshold {
				out = append(out, Match{ID: c.ID, DuplicateOf: k.ID, Reason: "judul sangat mirip"})
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		byHash[c.ContentHash] = c.ID
		canon = append(canon, c)
	}
	return out
}
