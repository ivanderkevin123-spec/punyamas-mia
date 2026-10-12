// Package validate holds deterministic checks on untrusted content.
package validate

import "regexp"

var suspiciousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore (all |any |the )?(previous|prior|above|earlier) (instructions?|prompts?|rules?)`),
	regexp.MustCompile(`(?i)disregard (all |any |the )?(previous|prior|above|system)`),
	regexp.MustCompile(`(?i)abaikan (semua |seluruh )?(instruksi|perintah|aturan)( sebelumnya| di atas)?`),
	regexp.MustCompile(`(?i)\byou are now\b`),
	regexp.MustCompile(`(?i)\bkamu (sekarang )?adalah\b.*\b(asisten|assistant|ai)\b`),
	regexp.MustCompile(`(?i)\bsystem prompt\b`),
	regexp.MustCompile(`(?i)\b(reveal|print|show|dump|tampilkan|bocorkan|kirimkan)\b.{0,40}\b(secret|api key|api_key|password|kata sandi|token|credential)`),
	regexp.MustCompile(`(?i)\bas an ai (model|assistant|language model)\b`),
	regexp.MustCompile(`(?i)^\s*(assistant|system)\s*:`),
	regexp.MustCompile(`(?i)\b(rm -rf|curl [^|]*\|\s*(ba)?sh|sudo )`),
}

// Suspicious reports whether text contains instruction-like content that
// attempts to steer the system. Flagged items are excluded from agent input.
func Suspicious(text string) (bool, string) {
	for _, re := range suspiciousPatterns {
		if m := re.FindString(text); m != "" {
			if len(m) > 60 {
				m = m[:60] + "…"
			}
			return true, m
		}
	}
	return false, ""
}
