package strategy

import (
	"context"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/store"
)

// LoadEvidence returns the validated items of a research run as evaluator evidence (with the first
// source excerpt and source name), as a list and keyed by ref. Agent-produced items get the producing
// execution as their source so independence counts stay honest. Used by the Strategy and Content agents.
func LoadEvidence(ctx context.Context, st *store.Store, researchRunID string) ([]Evidence, map[string]Evidence, error) {
	items, err := st.ValidatedItemsForRun(ctx, researchRunID)
	if err != nil {
		return nil, nil, err
	}
	evid, err := st.EvidenceForItems(ctx, researchRunID)
	if err != nil {
		return nil, nil, err
	}
	srcNames := map[string]string{}
	var list []Evidence
	byRef := map[string]Evidence{}
	for _, it := range items {
		e := Evidence{ID: it.ID, Ref: it.Ref, Kind: it.Kind, Quality: it.Quality, TopicKey: it.TopicKey, Title: it.Title, Statement: it.Statement, ConflictGroup: it.ConflictGroup}
		for _, ev := range evid[it.ID] {
			if ev.SourceID != "" {
				e.SourceID = ev.SourceID
				e.Excerpt = ev.Excerpt
				name, ok := srcNames[ev.SourceID]
				if !ok {
					if s, err := st.GetSource(ctx, ev.SourceID); err == nil {
						name = s.Name
					}
					srcNames[ev.SourceID] = name
				}
				e.SourceName = name
				break
			}
		}
		if e.SourceID == "" {
			e.SourceID = "agent:" + it.AgentExecutionID
			e.SourceName = "keluaran agen riset"
		}
		list = append(list, e)
		byRef[it.Ref] = e
	}
	return list, byRef, nil
}
