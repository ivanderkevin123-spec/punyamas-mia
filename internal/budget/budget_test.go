package budget

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

var fixedNow = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func newGate(t *testing.T) (*Gate, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "mia.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for _, id := range []string{"run_0", "run_1"} {
		_ = st.CreateRun(context.Background(), &domain.Run{ID: id, Kind: "research", Status: "running", CreatedAt: "t", ConfigHash: "h", ProviderName: "mock", InputsJSON: "{}", SummaryJSON: "{}"})
	}
	g := &Gate{Store: st, Caps: config.Caps{MonthlyCeilingUSD: 20, RunCeilingUSD: 2, AgentCeilingUSD: 1, WarnAtFraction: 0.8}, Prices: config.PriceTable{VerifiedAt: "2026-10-12", Models: map[string]config.ModelPrice{"claude-opus-5-5": {InputPerMTok: 4, CacheReadPerMTok: 0.2, OutputPerMTok: 20}, "mock": {}}}, Now: func() time.Time { return fixedNow }, IDs: &ids.Sequential{}, Loc: time.UTC}
	return g, st
}

func seed(t *testing.T, g *Gate, run, agent string, usd float64) {
	t.Helper()
	c := usd
	if err := g.Store.InsertCost(context.Background(), &domain.CostRecord{ID: g.IDs.New("cost"), RunID: run, AgentName: agent, Category: "llm", ProviderName: "x", UnitsJSON: "{}", UnitPriceSource: "test", CostUSD: &c, PeriodMonth: "2026-10", CreatedAt: "t"}); err != nil {
		t.Fatal(err)
	}
}

func TestEstimateFormula(t *testing.T) {
	g, _ := newGate(t)
	usd, ok := Estimate(g.Prices, "claude-opus-5-5", 1_000_000, 0, 1_000_000)
	if !ok || usd != 24 {
		t.Fatalf("estimate = %v ok=%v, want 24", usd, ok)
	}
	if _, ok := Estimate(g.Prices, "unknown", 1, 0, 1); ok {
		t.Fatal("unknown model must not estimate")
	}
}

func TestAllowUnderCeilings(t *testing.T) {
	g, _ := newGate(t)
	d, err := g.Allow(context.Background(), "run_1", "research", "claude-opus-5-5", 10_000, 2_000)
	if err != nil || !d.Allowed || d.Warn {
		t.Fatalf("decision %+v err %v", d, err)
	}
}

func TestMonthlyCeilingBlocks(t *testing.T) {
	g, _ := newGate(t)
	seed(t, g, "run_0", "research", 19.99)
	d, err := g.Allow(context.Background(), "run_1", "research", "claude-opus-5-5", 10_000, 2_000)
	if err != nil || d.Allowed || !strings.Contains(d.Reason, "plafon bulanan") {
		t.Fatalf("decision %+v err %v", d, err)
	}
}

func TestRunAndAgentCeilingsBlock(t *testing.T) {
	g, _ := newGate(t)
	seed(t, g, "run_1", "other", 1.99)
	d, _ := g.Allow(context.Background(), "run_1", "research", "claude-opus-5-5", 10_000, 2_000)
	if d.Allowed || !strings.Contains(d.Reason, "per run") {
		t.Fatalf("expected run ceiling block, got %+v", d)
	}
	g2, _ := newGate(t)
	seed(t, g2, "run_1", "research", 0.99)
	d, _ = g2.Allow(context.Background(), "run_1", "research", "claude-opus-5-5", 10_000, 2_000)
	if d.Allowed || !strings.Contains(d.Reason, "per agen") {
		t.Fatalf("expected agent ceiling block, got %+v", d)
	}
}

func TestWarnFraction(t *testing.T) {
	g, _ := newGate(t)
	seed(t, g, "run_0", "research", 16.5)
	d, _ := g.Allow(context.Background(), "run_1", "research", "mock", 10, 10)
	if !d.Allowed || !d.Warn {
		t.Fatalf("expected allowed with warning, got %+v", d)
	}
}

func TestRecordMeasurableAndEstimated(t *testing.T) {
	g, _ := newGate(t)
	ctx := context.Background()
	rec, err := g.Record(ctx, "run_1", "", "research", "x", "claude-opus-5-5", provider.Usage{InputTokens: 1_000_000, OutputTokens: 0, Measurable: true}, 0, 0)
	if err != nil || rec.Estimated || rec.CostUSD == nil || *rec.CostUSD != 4 || !strings.HasPrefix(rec.UnitPriceSource, "price_table:") {
		t.Fatalf("record %+v err %v", rec, err)
	}
	rec, err = g.Record(ctx, "run_1", "", "research", "x", "claude-opus-5-5", provider.Usage{Measurable: false}, 500_000, 0)
	if err != nil || !rec.Estimated || rec.CostUSD == nil || *rec.CostUSD != 2 || rec.UnitPriceSource != "unmeasurable:estimate" {
		t.Fatalf("estimated record %+v err %v", rec, err)
	}
	spent, _ := g.Store.SpentRun(ctx, "run_1")
	if spent != 6 {
		t.Fatalf("spent = %v, want 6", spent)
	}
}

func TestPeriodUsesLedgerTimezone(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	// 2026-10-31 20:00 UTC is 2026-11-01 03:00 WIB.
	if p := Period(time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC), loc); p != "2026-11" {
		t.Fatalf("period = %s, want 2026-11", p)
	}
}

// A7: failed or timed-out calls are booked from the pre-call estimate as
// uncertain cost; they count against the ceilings; a reconciliation for the
// same execution replaces the row instead of adding to it.
func TestRecordUncertainCountsAgainstCeilings(t *testing.T) {
	g, st := newGate(t)
	ctx := context.Background()
	_ = st.InsertExecution(ctx, &domain.AgentExecution{ID: "x1", RunID: "run_1", AgentName: "research", ProviderName: "mock", SchemaVersion: "v1", RequestHash: "h", Outcome: "timeout", CreatedAt: "t"})
	rec, err := g.RecordUncertain(ctx, "run_1", "x1", "research", "mock", "claude-opus-5-5", 100_000, 10_000, "timeout")
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Estimated || rec.UnitPriceSource != "uncertain:timeout" || rec.CostUSD == nil || *rec.CostUSD != 0.6 {
		t.Fatalf("uncertain record %+v", rec)
	}
	d, _ := g.Allow(ctx, "run_1", "research", "claude-opus-5-5", 100_000, 10_000)
	if d.Allowed || !strings.Contains(d.Reason, "per agen") {
		t.Fatalf("uncertain spend must count against the agent ceiling: %+v", d)
	}
	// Reconciliation with actual usage replaces the uncertain row (no double counting).
	actual, err := g.Record(ctx, "run_1", "x1", "research", "mock", "claude-opus-5-5", provider.Usage{InputTokens: 10_000, OutputTokens: 1_000, Measurable: true}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if actual.ID != rec.ID || actual.Estimated || *actual.CostUSD != 0.06 {
		t.Fatalf("reconciled record %+v", actual)
	}
	n, _ := st.CountRows(ctx, "cost_records")
	spent, _ := st.SpentRun(ctx, "run_1")
	if n != 1 || spent != 0.06 {
		t.Fatalf("rows=%d spent=%v; want 1 row and 0.06", n, spent)
	}
	if _, err := g.RecordUncertain(ctx, "run_1", "", "research", "mock", "unknown-model", 1, 1, "error"); err == nil {
		t.Fatal("unknown model must be rejected")
	}
}
