// Package logx configures structured JSON logging with secret redaction and
// truncation of untrusted excerpts.
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	secretKey   = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|authorization|credential)`)
	secretValue = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{8,}|AKIA[0-9A-Z]{12,}|-----BEGIN [A-Z ]*PRIVATE KEY[\s\S]*|Bearer [A-Za-z0-9._-]{16,}`)
	longKeys    = map[string]bool{"excerpt": true, "statement": true, "content": true, "prompt": true, "response": true, "text": true}
)

const maxUntrustedLen = 200

// Redact is a slog ReplaceAttr function.
func Redact(_ []string, a slog.Attr) slog.Attr {
	if secretKey.MatchString(a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}
	if a.Value.Kind() == slog.KindString {
		s := a.Value.String()
		if secretValue.MatchString(s) {
			s = secretValue.ReplaceAllString(s, "[REDACTED]")
		}
		if longKeys[strings.ToLower(a.Key)] && len(s) > maxUntrustedLen {
			s = s[:maxUntrustedLen] + "…[truncated]"
		}
		return slog.String(a.Key, s)
	}
	return a
}

// New builds a JSON logger writing to all writers.
func New(level slog.Level, writers ...io.Writer) *slog.Logger {
	var w io.Writer = os.Stderr
	if len(writers) > 0 {
		w = io.MultiWriter(writers...)
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: Redact})
	return slog.New(h)
}

// OpenLogFile opens (creating if needed) data/logs/mia-YYYY-MM-DD.jsonl with mode 0600.
func OpenLogFile(dataDir string, now time.Time) (*os.File, error) {
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	name := filepath.Join(dir, fmt.Sprintf("mia-%s.jsonl", now.Format("2006-01-02")))
	return os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
