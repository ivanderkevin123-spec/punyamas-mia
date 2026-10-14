// Package content applies the Phase 4 Content Agent rules in code: eligibility
// of the source opportunity, evidence and claim gates, marketing-language
// rules, [ISI: ...] placeholders, duplicates, draft status, readiness for
// manual handoff, and the human review and revision services. The agent only
// proposes drafts; nothing here publishes, sends or schedules anything.
package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/domain"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/strategy"
)

// Source is an eligible approved opportunity with everything the gates may rely on, all read from the database.
type Source struct {
	Opportunity        *domain.Opportunity
	StrategyRun        *domain.Run
	ResearchRunID      string
	Brief              strategy.Brief
	Evidence           []strategy.Evidence          // validated items linked to the opportunity, sorted by ref
	ByRef              map[string]strategy.Evidence // opportunity evidence by ref
	ByID               map[string]strategy.Evidence // opportunity evidence by item id
	ResearchRefs       map[string]string            // every item ref linked to the research run -> item status
	Approvals          []*domain.ApprovalRecord
	StrategyFixtureKey string
}

// IneligibleError lists why an opportunity cannot be used by the Content Agent.
type IneligibleError struct {
	OpportunityID string
	Reasons       []string
}

func (e *IneligibleError) Error() string {
	return fmt.Sprintf("peluang %s tidak memenuhi syarat untuk Agen Konten: %s", e.OpportunityID, strings.Join(e.Reasons, "; "))
}

// IsIneligible reports whether err is an eligibility refusal.
func IsIneligible(err error) bool {
	var ie *IneligibleError
	return errors.As(err, &ie)
}

// Load checks eligibility from the database only and returns the source. Rules: the opportunity
// exists and is approved; its latest approval record is an approval; it has a tier; its strategy run is a
// completed strategy run over the same research run; its agent execution belongs to that run and
// succeeded; it has evidence links and every linked item is still validated in the research run.
func Load(ctx context.Context, st *store.Store, opportunityID string) (*Source, error) {
	opp, err := st.GetOpportunity(ctx, opportunityID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, &IneligibleError{OpportunityID: opportunityID, Reasons: []string{"peluang tidak ditemukan"}}
	}
	if err != nil {
		return nil, err
	}
	var reasons []string
	if opp.Status != domain.OppApproved {
		reasons = append(reasons, fmt.Sprintf("status peluang %s; hanya peluang berstatus approved yang dapat dibuatkan konten", opp.Status))
	}
	apps, err := st.ApprovalsFor(ctx, "opportunity", opp.ID)
	if err != nil {
		return nil, err
	}
	last := "tidak ada"
	if len(apps) > 0 {
		last = apps[len(apps)-1].Decision
	}
	if last != domain.OppApproved {
		reasons = append(reasons, fmt.Sprintf("riwayat keputusan tidak mendukung persetujuan (keputusan terakhir: %s)", last))
	}
	if opp.Tier == "" || opp.Tier == domain.TierNone {
		reasons = append(reasons, "peluang tidak memiliki tingkat (diblokir atau ditolak gerbang strategi)")
	}
	src := &Source{Opportunity: opp, ResearchRunID: opp.ParentRunID, Approvals: apps}
	run, err := st.GetRun(ctx, opp.RunID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		reasons = append(reasons, "run strategi asal tidak ditemukan")
	case err != nil:
		return nil, err
	default:
		src.StrategyRun = run
		if run.Kind != "strategy" {
			reasons = append(reasons, fmt.Sprintf("run asal %s bukan run strategi", run.ID))
		}
		if run.Status != domain.RunCompleted && run.Status != domain.RunCompletedWithWarnings {
			reasons = append(reasons, fmt.Sprintf("run strategi %s berstatus %s", run.ID, run.Status))
		}
		if run.ParentRunID == "" || run.ParentRunID != opp.ParentRunID {
			reasons = append(reasons, "provenance tidak cocok: run riset peluang berbeda dari run riset run strategi")
		}
		var in struct {
			FixtureKey string `json:"fixture_key"`
		}
		_ = json.Unmarshal([]byte(run.InputsJSON), &in)
		src.StrategyFixtureKey = in.FixtureKey
		exec, err := st.ExecutionByID(ctx, run.ID, opp.AgentExecutionID)
		if err != nil || (exec.Outcome != "ok" && exec.Outcome != "repaired") {
			reasons = append(reasons, "eksekusi Agen Strategi asal tidak ditemukan atau tidak berhasil")
		}
	}
	links, err := st.EvidenceForOpportunity(ctx, opp.ID)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		reasons = append(reasons, "peluang tidak memiliki tautan bukti")
	}
	_, byRef, err := strategy.LoadEvidence(ctx, st, opp.ParentRunID)
	if err != nil {
		return nil, err
	}
	validatedByID := map[string]strategy.Evidence{}
	for _, e := range byRef {
		validatedByID[e.ID] = e
	}
	src.ByRef, src.ByID = map[string]strategy.Evidence{}, map[string]strategy.Evidence{}
	for _, l := range links {
		e, ok := validatedByID[l.ItemID]
		if !ok {
			reasons = append(reasons, fmt.Sprintf("butir bukti %s tidak lagi tervalidasi atau tidak terhubung ke run riset induk", l.ItemID))
			continue
		}
		src.Evidence = append(src.Evidence, e)
		src.ByRef[e.Ref], src.ByID[e.ID] = e, e
	}
	sort.Slice(src.Evidence, func(i, j int) bool { return src.Evidence[i].Ref < src.Evidence[j].Ref })
	items, err := st.ItemsForRun(ctx, opp.ParentRunID)
	if err != nil {
		return nil, err
	}
	src.ResearchRefs = map[string]string{}
	for _, it := range items {
		if cur, ok := src.ResearchRefs[it.Ref]; !ok || cur != domain.ItemValidated {
			src.ResearchRefs[it.Ref] = it.Status
		}
	}
	if len(reasons) > 0 {
		return nil, &IneligibleError{OpportunityID: opp.ID, Reasons: reasons}
	}
	_ = json.Unmarshal([]byte(opp.BriefJSON), &src.Brief)
	return src, nil
}
