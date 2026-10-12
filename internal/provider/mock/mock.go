// Package mock is the deterministic, zero-cost provider used in Phase 2. It
// replays fixture files and never opens a network connection.
package mock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
)

type Mock struct {
	FixturesDir string
	// Test hooks: fail the first FailAttempts calls with Fail; Hang blocks until ctx is done.
	Fail         error
	FailAttempts int
	Hang         bool

	mu    sync.Mutex
	calls int
}

func New(fixturesDir string) *Mock { return &Mock{FixturesDir: fixturesDir} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Capabilities() provider.Capabilities {
	return provider.Capabilities{MeasurableCost: true, Network: false}
}

func (m *Mock) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

var keyRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (m *Mock) Complete(ctx interface{ Done() <-chan struct{}; Err() error }, req provider.Request) (provider.Response, provider.Usage, error) {
	m.mu.Lock()
	m.calls++
	n := m.calls
	m.mu.Unlock()
	if m.Hang {
		<-ctx.Done()
		return provider.Response{}, provider.Usage{}, &provider.Error{Kind: provider.KindTimeout, Err: ctx.Err()}
	}
	if m.Fail != nil && n <= m.FailAttempts {
		return provider.Response{}, provider.Usage{}, m.Fail
	}
	content, used, warn := m.lookup(req)
	zero := 0.0
	u := provider.Usage{
		InputTokens:  provider.EstimateTokens(req.System + req.User),
		OutputTokens: provider.EstimateTokens(content),
		CostUSD:      &zero,
		Measurable:   true,
	}
	resp := provider.Response{Content: content, FixtureUsed: used}
	if warn != "" {
		resp.Warnings = append(resp.Warnings, warn)
	}
	return resp, u, nil
}

func (m *Mock) lookup(req provider.Request) (content, used, warn string) {
	key := keyRe.ReplaceAllString(req.FixtureKey, "_")
	candidates := []string{}
	if key != "" {
		candidates = append(candidates, filepath.Join(m.FixturesDir, req.Agent, key+".json"))
	}
	candidates = append(candidates, filepath.Join(m.FixturesDir, req.Agent, "default.json"))
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return string(b), p, ""
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Sprintf("mock: fixture %s tidak dapat dibaca: %v", p, err)
		}
	}
	return `{"findings":[],"notes":"mock: tidak ada fixture; tidak ada temuan yang dihasilkan"}`, "", fmt.Sprintf("mock: fixture tidak ditemukan untuk agent=%s key=%q; keluaran kosong", req.Agent, req.FixtureKey)
}
