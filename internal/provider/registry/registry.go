// Package registry builds the configured provider by name. Only "mock" exists in Phase 2.
package registry

import (
	"fmt"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider/mock"
)

func Build(cfg *config.Config) (provider.Provider, error) {
	switch cfg.Limits.Provider.Name {
	case "mock":
		return mock.New(cfg.Limits.Provider.Mock.FixturesDir), nil
	case "anthropic":
		return nil, fmt.Errorf("penyedia 'anthropic' belum tersedia (Fase 3, memerlukan persetujuan dan API key); gunakan provider.name: mock di config/agent-limits.yaml")
	default:
		return nil, fmt.Errorf("penyedia %q tidak dikenal; yang tersedia: mock", cfg.Limits.Provider.Name)
	}
}
