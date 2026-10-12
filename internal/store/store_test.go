package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data", "mia.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrateAndIntegrity(t *testing.T) {
	s := openTemp(t)
	res, err := s.Integrity(context.Background())
	if err != nil || res != "ok" {
		t.Fatalf("integrity = %q, err %v", res, err)
	}
	n, err := s.CountRows(context.Background(), "schema_migrations")
	if err != nil || n != 1 {
		t.Fatalf("migrations applied = %d, err %v", n, err)
	}
}

func TestRunRoundTrip(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	r := &domain.Run{ID: "run_1", Kind: "research", Status: domain.RunCreated, CreatedAt: "2026-10-12T00:00:00Z", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}", CostMeasurable: true}
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRun(ctx, "run_1")
	if err != nil || got.Status != domain.RunCreated || got.CostUSDTotal != nil {
		t.Fatalf("got %+v err %v", got, err)
	}
	cost := 0.5
	r.Status = domain.RunCompleted
	r.CostUSDTotal = &cost
	if err := s.UpdateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetRun(ctx, "run_1")
	if got.Status != domain.RunCompleted || got.CostUSDTotal == nil || *got.CostUSDTotal != 0.5 {
		t.Fatalf("update not persisted: %+v", got)
	}
	if _, err := s.GetRun(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEvidenceRequiresExactlyOneLink(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_ = s.CreateRun(ctx, &domain.Run{ID: "r", Kind: "research", Status: "created", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	it := &domain.Item{ID: "i", RunID: "r", Ref: "a#1", Origin: "import", Kind: "fact", Title: "t", Statement: "s", Language: "id", Quality: "A", Status: "imported", ContentHash: "c", CreatedAt: "t"}
	if err := s.InsertItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEvidence(ctx, &domain.Evidence{ID: "e1", ItemID: "i"}); err == nil {
		t.Fatal("evidence with neither source nor item link must be rejected")
	}
}

func TestItemIdentityIsUnique(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_ = s.CreateRun(ctx, &domain.Run{ID: "r", Kind: "research", Status: "created", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	it := &domain.Item{ID: "i1", RunID: "r", Ref: "a#1", Origin: "import", Kind: "fact", Title: "t", Statement: "s", Language: "id", Quality: "A", Status: "imported", ContentHash: "c", CreatedAt: "t"}
	if err := s.InsertItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	it.ID = "i2"
	if err := s.InsertItem(ctx, it); err == nil {
		t.Fatal("duplicate (origin, ref, hash) must be rejected")
	}
	found, err := s.FindItem(ctx, "import", "a#1", "c")
	if err != nil || found.ID != "i1" {
		t.Fatalf("find: %+v %v", found, err)
	}
}
