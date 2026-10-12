package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	var buf bytes.Buffer
	log := New(slog.LevelInfo, &buf)
	long := strings.Repeat("x", 400)
	log.Info("test", "api_key", "sk-ant-secretsecretsecret", "note", "token sk-ant-abcdefghijklmnopqrstuvwxyz here", "excerpt", long, "Authorization", "Bearer abcdefghijklmnopqrstuvwxyz")
	out := buf.String()
	if strings.Contains(out, "secretsecret") || strings.Contains(out, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected redaction marker: %s", out)
	}
	if strings.Contains(out, long) {
		t.Fatal("long excerpt not truncated")
	}
	if !strings.Contains(out, "[truncated]") {
		t.Fatal("expected truncation marker")
	}
}
