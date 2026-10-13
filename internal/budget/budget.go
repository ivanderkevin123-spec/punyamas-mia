// Package budget enforces per-run, per-agent and monthly cost ceilings before
// every provider call and records usage afterwards. Failed calls are booked
// conservatively as uncertain estimates so repeated failures cannot bypass a ceiling.
package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/ids"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

type Gate struct {
	Store  *store.Store
	Caps   config.Caps
	Prices config.PriceTable
	Now    func() time.Time
	IDs    ids.Generator
	Loc    *time.Location
}

type Decision struct {
	Allowed     bool
	Reason      string
	EstimateUSD float64
	MonthSpent  float64
	RunSpent    float64
	AgentSpent  float64
	Warn        bool
}

// Unit price sources recorded on cost rows.
const (
	SourceProviderReported = "provider_reported"
	SourceUnmeasurable     = "unmeasurable:estimate"
	SourceUncertainPrefix  = "uncertain:" // + reason (timeout, error); booked from the pre-call estimate
)

// Location returns the ledger timezone (WIB).
func Location() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}

// Period returns the ledger month (YYYY-MM) in the ledger timezone.
func Period(now time.Time, loc *time.Location) string {
	return now.In(loc).Format("2006-01")
}

// Estimate computes USD for token counts from the price table.
func Estimate(prices config.PriceTable, model string, in, cacheRead, out int) (float64, bool) {
	p, ok := prices.Models[model]
	if !ok {
		return 0, false
	}
	return float64(in)/1e6*p.InputPerMTok + float64(cacheRead)/1e6*p.CacheReadPerMTok + float64(out)/1e6*p.OutputPerMTok, true
}

func (g *Gate) loc() *time.Location {
	if g.Loc != nil {
		return g.Loc
	}
	return Location()
}

// Allow checks the ceilings against the ledger (actual plus uncertain spend) plus the estimate for the next call.
func (g *Gate) Allow(ctx context.Context, runID, agent, model string, estIn, estOut int) (Decision, error) {
	d := Decision{}
	est, known := Estimate(g.Prices, model, estIn, 0, estOut)
	if !known {
		return d, fmt.Errorf("price table has no entry for model %q", model)
	}
	d.EstimateUSD = est
	period := Period(g.Now(), g.loc())
	var err error
	if d.MonthSpent, err = g.Store.SpentMonth(ctx, period); err != nil {
		return d, err
	}
	if d.RunSpent, err = g.Store.SpentRun(ctx, runID); err != nil {
		return d, err
	}
	if d.AgentSpent, err = g.Store.SpentAgent(ctx, runID, agent); err != nil {
		return d, err
	}
	switch {
	case d.MonthSpent+est > g.Caps.MonthlyCeilingUSD:
		d.Reason = fmt.Sprintf("plafon bulanan USD %.2f akan terlampaui (terpakai %.4f + estimasi %.4f)", g.Caps.MonthlyCeilingUSD, d.MonthSpent, est)
	case d.RunSpent+est > g.Caps.RunCeilingUSD:
		d.Reason = fmt.Sprintf("plafon per run USD %.2f akan terlampaui (terpakai %.4f + estimasi %.4f)", g.Caps.RunCeilingUSD, d.RunSpent, est)
	case d.AgentSpent+est > g.Caps.AgentCeilingUSD:
		d.Reason = fmt.Sprintf("plafon per agen USD %.2f akan terlampaui (terpakai %.4f + estimasi %.4f)", g.Caps.AgentCeilingUSD, d.AgentSpent, est)
	default:
		d.Allowed = true
	}
	if d.Allowed && g.Caps.WarnAtFraction > 0 && d.MonthSpent+est >= g.Caps.WarnAtFraction*g.Caps.MonthlyCeilingUSD {
		d.Warn = true
	}
	return d, nil
}

// Record books the cost of a completed call from provider usage. Unmeasurable
// usage is booked as an estimate and flagged. If a row already exists for the
// execution (for example an uncertain booking being reconciled), it is replaced.
func (g *Gate) Record(ctx context.Context, runID, execID, agent, providerName, model string, u provider.Usage, estIn, estOut int) (*domain.CostRecord, error) {
	in, out, cache := u.InputTokens, u.OutputTokens, u.CacheReadTokens
	estimated := !u.Measurable
	if estimated {
		in, out = estIn, estOut
	}
	var cost *float64
	source := SourceUnmeasurable
	if u.CostUSD != nil && u.Measurable {
		v := *u.CostUSD
		cost = &v
		source = SourceProviderReported
	} else if v, ok := Estimate(g.Prices, model, in, cache, out); ok {
		cost = &v
		if !estimated {
			source = "price_table:" + g.Prices.VerifiedAt
		}
	}
	units, _ := json.Marshal(map[string]int{"input_tokens": in, "output_tokens": out, "cache_read_tokens": cache})
	rec := g.newRecord(runID, execID, agent, providerName, string(units), source, cost, estimated)
	if err := g.upsert(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// RecordUncertain books the pre-call estimate for a call whose outcome is unknown
// (timeout or provider error after the request may have been processed). The row
// is flagged estimated and its source is "uncertain:<reason>". No actual charge is claimed.
func (g *Gate) RecordUncertain(ctx context.Context, runID, execID, agent, providerName, model string, estIn, estOut int, reason string) (*domain.CostRecord, error) {
	est, ok := Estimate(g.Prices, model, estIn, 0, estOut)
	if !ok {
		return nil, fmt.Errorf("price table has no entry for model %q", model)
	}
	units, _ := json.Marshal(map[string]int{"input_tokens_est": estIn, "output_tokens_est": estOut})
	rec := g.newRecord(runID, execID, agent, providerName, string(units), SourceUncertainPrefix+reason, &est, true)
	if err := g.upsert(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (g *Gate) newRecord(runID, execID, agent, providerName, units, source string, cost *float64, estimated bool) *domain.CostRecord {
	now := g.Now()
	return &domain.CostRecord{
		ID: g.IDs.New("cost"), RunID: runID, AgentExecutionID: execID, AgentName: agent, Category: "llm", ProviderName: providerName,
		UnitsJSON: units, UnitPriceSource: source, CostUSD: cost, Estimated: estimated,
		PeriodMonth: Period(now, g.loc()), CreatedAt: now.UTC().Format(time.RFC3339),
	}
}

// upsert keeps at most one cost row per execution to avoid double counting on reconciliation.
func (g *Gate) upsert(ctx context.Context, rec *domain.CostRecord) error {
	if rec.AgentExecutionID != "" {
		existing, err := g.Store.FindCostByExecution(ctx, rec.AgentExecutionID)
		switch {
		case err == nil:
			rec.ID = existing.ID
			rec.CreatedAt = existing.CreatedAt
			return g.Store.UpdateCost(ctx, rec)
		case !errors.Is(err, store.ErrNotFound):
			return err
		}
	}
	return g.Store.InsertCost(ctx, rec)
}
