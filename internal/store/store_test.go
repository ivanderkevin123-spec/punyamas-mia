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

// A2: item, run link and evidence are committed atomically; a failing evidence
// insert leaves no orphan item and a later import can insert the item again.
func TestInsertItemWithEvidenceIsAtomic(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_ = s.CreateRun(ctx, &domain.Run{ID: "r", Kind: "research", Status: "created", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	item := &domain.Item{ID: "i1", RunID: "r", Ref: "a#1", Origin: "import", Kind: "fact", Title: "t", Statement: "s", Language: "id", Quality: "A", Status: "imported", ContentHash: "c", CreatedAt: "t"}
	bad := &domain.Evidence{ID: "e1", SourceID: "src_missing", Excerpt: "x"} // foreign key violation injected deterministically
	if err := s.InsertItemWithEvidence(ctx, "r", item, []*domain.Evidence{bad}); err == nil {
		t.Fatal("expected the evidence insert to fail")
	}
	for _, table := range []string{"research_items", "run_items", "evidence"} {
		if n, _ := s.CountRows(ctx, table); n != 0 {
			t.Fatalf("%s has %d rows after a failed atomic insert; want 0", table, n)
		}
	}
	if _, err := s.FindItem(ctx, "import", "a#1", "c"); err != ErrNotFound {
		t.Fatalf("orphan item visible to re-import: %v", err)
	}
	src := &domain.Source{ID: "src_ok", SourceType: "manual_note", Name: "n", Reference: "ref", RetrievedAt: "t", ContentHash: "h", Status: "ok", CreatedAt: "t"}
	if err := s.InsertSource(ctx, src); err != nil {
		t.Fatal(err)
	}
	good := &domain.Evidence{ID: "e2", SourceID: "src_ok", Excerpt: "x"}
	if err := s.InsertItemWithEvidence(ctx, "r", item, []*domain.Evidence{good}); err != nil {
		t.Fatalf("recovery insert failed: %v", err)
	}
	for table, want := range map[string]int{"research_items": 1, "run_items": 1, "evidence": 1} {
		if n, _ := s.CountRows(ctx, table); n != want {
			t.Fatalf("%s has %d rows; want %d", table, n, want)
		}
	}
	evs, _ := s.EvidenceForItems(ctx, "r")
	if len(evs["i1"]) != 1 || evs["i1"][0].ItemID != "i1" {
		t.Fatalf("evidence not linked to item: %+v", evs)
	}
}

func TestCostUpsertByExecution(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_ = s.CreateRun(ctx, &domain.Run{ID: "r", Kind: "research", Status: "created", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	_ = s.InsertExecution(ctx, &domain.AgentExecution{ID: "x1", RunID: "r", AgentName: "research", ProviderName: "mock", SchemaVersion: "v1", RequestHash: "h", Outcome: "timeout", CreatedAt: "t"})
	v := 1.5
	c := &domain.CostRecord{ID: "c1", RunID: "r", AgentExecutionID: "x1", AgentName: "research", Category: "llm", ProviderName: "mock", UnitsJSON: "{}", UnitPriceSource: "uncertain:timeout", CostUSD: &v, Estimated: true, PeriodMonth: "2026-10", CreatedAt: "t"}
	if err := s.InsertCost(ctx, c); err != nil {
		t.Fatal(err)
	}
	found, err := s.FindCostByExecution(ctx, "x1")
	if err != nil || found.ID != "c1" || !found.Estimated {
		t.Fatalf("find: %+v %v", found, err)
	}
	w := 0.25
	c.CostUSD, c.Estimated, c.UnitPriceSource = &w, false, "provider_reported"
	if err := s.UpdateCost(ctx, c); err != nil {
		t.Fatal(err)
	}
	spent, _ := s.SpentRun(ctx, "r")
	if spent != 0.25 {
		t.Fatalf("spent = %v after reconciliation, want 0.25 (no double count)", spent)
	}
	if err := s.UpdateCost(ctx, &domain.CostRecord{ID: "nope", UnitsJSON: "{}", UnitPriceSource: "x", PeriodMonth: "p"}); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
