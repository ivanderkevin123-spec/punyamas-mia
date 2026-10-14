package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/provider"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/research/dedup"
)

// agentValidation is the verdict of an agent-specific parser on one response.
type agentValidation struct {
	parseErr error    // the output is not valid JSON for the schema
	problems []string // nothing usable remains after structural validation (triggers one repair)
	warnings []string // warnings recorded when the output is accepted
}

// agentCall describes one bounded agent step: at most maxPerRun calls per run, counted from
// persisted executions (errors and timeouts included), at most max_calls_per_step per step,
// budget-gated before every call, with every call recorded as an execution and a cost row.
type agentCall struct {
	agent       string // execution agent name
	label       string // business-facing agent name used in messages, e.g. "Agen Strategi"
	unit        string // what the agent proposes, e.g. "usulan" or "draf"
	schema      string
	step        string
	maxPerRun   int
	restartHint string
	request     provider.Request
	repair      func(req provider.Request, problems []string) provider.Request
	validate    func(content string) agentValidation
}

// invokeAgent runs the shared call loop used by the Strategy and Content agents.
func (o *Orchestrator) invokeAgent(ctx context.Context, rc *runCtx, ac agentCall) error {
	caps := o.d.Cfg.Limits.Caps
	already, err := o.d.Store.CountExecutions(ctx, rc.run.ID, ac.agent)
	if err != nil {
		return err
	}
	if already >= ac.maxPerRun {
		// The cap counts every call (errors and timeouts included); report the last outcome honestly.
		last, status := "tidak diketahui", domain.StepFailed
		if execs, err := o.d.Store.ExecutionsForRun(ctx, rc.run.ID); err == nil {
			for _, x := range execs {
				if x.AgentName == ac.agent {
					last = x.Outcome
				}
			}
		}
		if last == "timeout" {
			status = domain.StepTimedOut
		}
		return &stepErr{status: status, err: fmt.Errorf("batas %d panggilan %s per run sudah tercapai (%d tercatat; hasil terakhir: %s); %s", ac.maxPerRun, ac.label, already, last, ac.restartHint)}
	}
	req := ac.request
	req.Model = o.d.Cfg.Limits.Provider.Model
	req.MaxOutputTokens = caps.MaxOutputTokens
	st := rc.steps[ac.step]
	maxCalls := caps.MaxCallsPerStep
	if remaining := ac.maxPerRun - already; remaining < maxCalls {
		maxCalls = remaining
	}
	var problems []string
	for call := 1; call <= maxCalls; call++ {
		if call > 1 {
			req = ac.repair(req, problems)
		}
		estIn := provider.EstimateTokens(req.System + req.User)
		estOut := caps.MaxOutputTokens
		if estIn > caps.MaxInputTokens {
			return failf("masukan agen (%d token) melebihi batas %d token", estIn, caps.MaxInputTokens)
		}
		dec, err := o.d.Budget.Allow(ctx, rc.run.ID, ac.agent, req.Model, estIn, estOut)
		if err != nil {
			return failf("pemeriksaan anggaran gagal: %v", err)
		}
		if dec.Warn {
			rc.warn("anggaran bulanan mendekati plafon (terpakai USD %.4f dari USD %.2f)", dec.MonthSpent, caps.MonthlyCeilingUSD)
		}
		if !dec.Allowed {
			return &stepErr{status: domain.StepBudgetExhausted, err: errors.New(dec.Reason)}
		}
		start := time.Now()
		resp, usage, perr := o.d.Provider.Complete(ctx, req)
		now := o.ts()
		bctx := context.WithoutCancel(ctx)
		exec := &domain.AgentExecution{ID: o.d.IDs.New("exec"), RunID: rc.run.ID, StepID: st.ID, AgentName: ac.agent, ProviderName: o.d.Provider.Name(), Model: req.Model,
			SchemaVersion: ac.schema, RequestHash: dedup.FileHash([]byte(req.System + "\n" + req.User)), InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
			CacheReadTokens: usage.CacheReadTokens, CostMeasurable: usage.Measurable, LatencyMS: time.Since(start).Milliseconds(), Attempt: already + call, Outcome: "pending", CreatedAt: now}
		exec.RequestPath, exec.ResponsePath = o.saveExchange(exec.ID, req, resp.Content)
		if perr != nil {
			exec.Outcome = "error"
			if errors.Is(perr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				exec.Outcome = "timeout"
			}
			exec.ErrorText = perr.Error()
			exec.CostMeasurable = false
			if err := o.d.Store.InsertExecution(bctx, exec); err != nil {
				return err
			}
			if provider.Fatal(perr) {
				return &stepErr{status: domain.StepFailed, err: fmt.Errorf("penyedia menolak (%v); tidak diulang", perr)}
			}
			rec, err := o.d.Budget.RecordUncertain(bctx, rc.run.ID, exec.ID, ac.agent, o.d.Provider.Name(), req.Model, estIn, estOut, exec.Outcome)
			if err != nil {
				return err
			}
			exec.CostUSD = rec.CostUSD
			_ = o.d.Store.UpdateExecution(bctx, exec)
			return &stepErr{status: domain.StepFailed, err: perr, retryable: provider.Retryable(perr)}
		}
		if err := o.d.Store.InsertExecution(bctx, exec); err != nil {
			return err
		}
		cost, err := o.d.Budget.Record(bctx, rc.run.ID, exec.ID, ac.agent, o.d.Provider.Name(), req.Model, usage, estIn, estOut)
		if err != nil {
			return err
		}
		exec.CostUSD = cost.CostUSD
		exec.CostMeasurable = !cost.Estimated
		if resp.FixtureUsed != "" {
			if b, err := os.ReadFile(resp.FixtureUsed); err == nil {
				rc.sum.FixtureHash = dedup.FileHash(b)
			}
		}
		for _, w := range resp.Warnings {
			rc.warn("%s", w)
		}
		v := ac.validate(resp.Content)
		if v.parseErr != nil {
			exec.Outcome = "invalid_output"
			exec.ErrorText = v.parseErr.Error()
			_ = o.d.Store.UpdateExecution(bctx, exec)
			problems = []string{v.parseErr.Error()}
			if call < maxCalls {
				rc.warn("keluaran %s tidak valid (percobaan %d): %v; meminta perbaikan", ac.label, call, v.parseErr)
				continue
			}
			return &stepErr{status: domain.StepFailed, err: fmt.Errorf("keluaran %s tidak valid setelah %d percobaan: %w", ac.label, call, v.parseErr)}
		}
		if len(v.problems) > 0 {
			exec.Outcome = "invalid_output"
			problems = v.problems
			exec.ErrorText = joinProblems(problems)
			_ = o.d.Store.UpdateExecution(bctx, exec)
			if call < maxCalls {
				rc.warn("semua %s ditolak validasi struktural (percobaan %d); meminta perbaikan", ac.unit, call)
				continue
			}
			return &stepErr{status: domain.StepFailed, err: fmt.Errorf("semua %s ditolak validasi struktural setelah %d percobaan", ac.unit, call)}
		}
		exec.Outcome = "ok"
		if call > 1 {
			exec.Outcome = "repaired"
		}
		if err := o.d.Store.UpdateExecution(bctx, exec); err != nil {
			return err
		}
		for _, w := range v.warnings {
			rc.warn("%s", w)
		}
		return nil
	}
	return failf("%s tidak menghasilkan keluaran", ac.label)
}

func joinProblems(p []string) string {
	out := ""
	for i, s := range p {
		if i > 0 {
			out += " | "
		}
		out += s
	}
	return out
}
