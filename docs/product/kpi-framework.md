# MIA — KPI Framework

**Status:** Phase 1 draft · 2026-10-12. No baseline exists yet; nothing below is a measured result.

## 0. Ringkasan untuk pemilik usaha (Bahasa Indonesia)

KPI dibagi dua: **KPI operasional** (seberapa cepat, murah, dan bersumber hasil kerja MIA) yang dapat diukur sejak Fase 3 dari data MIA sendiri, dan **KPI bisnis** (pertanyaan masuk, biaya per pertanyaan, konversi) yang **belum dapat diukur** sampai toko beroperasi dan sumber datanya disetujui. MIA tidak akan mengklaim menaikkan penjualan tanpa bukti; yang dilaporkan adalah indikator awal dan hasil eksperimen yang terukur.

## 1. Principles

1. Every KPI names its baseline, measurement method, data source, frequency, and limitations.
2. KPIs that need data MIA does not have are marked **Unavailable** with the approval or integration that would unlock them.
3. Leading indicators (operational) are reported separately from business outcomes; no causal claim is made from correlation.
4. Baselines are measured, not assumed: the first three manual runs in Phase 3 establish operational baselines; the owner records the manual-process baseline for time metrics before relying on MIA.

## 2. Operational KPIs (measured by MIA from its own records)

| ID | KPI | Definition | Baseline | Measurement | Data source | Frequency | Limitations | Availability |
|---|---|---|---|---|---|---|---|---|
| O1 | Time to prepare a research report | Wall-clock from `ResearchRun.created_at` to report written, plus owner-recorded manual preparation time for comparison | Manual baseline recorded by owner once; MIA baseline = median of first 3 runs | `RunSummary.duration_s` | SQLite `research_runs` | Per run; monthly median | Mock runs are faster than live runs; compare like with like | Phase 3 |
| O2 | % research findings with valid source references | Findings whose `evidence_refs` resolve to items with source name + reference + retrieval time ÷ all findings | Target 100 % by construction | Validator counts | `research_items`, `findings` | Per run | A valid reference is not a true statement; evidence quality is separate (O10) | Phase 3 |
| O3 | % reports passing validation | Runs whose required steps all passed schema + semantic validation ÷ runs | n/a | `RunSummary.status` | `research_runs` | Monthly | Includes mock runs | Phase 2 |
| O4 | Recommendation acceptance rate | Opportunities with `ApprovalRecord.decision = approved` ÷ opportunities reviewed | n/a | Approval records | `approval_records` | Monthly | Depends on the owner reviewing; unreviewed items are excluded, not counted as rejected | Phase 3 |
| O5 | Time from opportunity selection to brief readiness | `ApprovalRecord(opportunity).at` → `CampaignBrief.status = approved_for_handoff` | n/a | Timestamps | `approval_records`, `campaign_briefs` | Per brief | Phase 4 only; human editing time outside MIA is not captured | Phase 4 |
| O6 | Execution failure rate | Runs ending `failed` ÷ all runs, by failure class | n/a | `RunSummary` | `research_runs`, `agent_executions` | Monthly | — | Phase 2 |
| O7 | Cost per successful run | Σ `CostRecord.cost_usd` for runs with status completed ÷ number of such runs; separately report runs where cost is `unmeasurable` | USD 0 in mock mode | Cost ledger | `cost_records` | Per run; monthly total vs USD 20 ceiling | Cost is provider-reported or estimated from token counts with the verified price table; estimates are labelled | Phase 2 |
| O8 | Duplicate research rate | Items flagged `duplicate_of` ÷ items imported | n/a | Dedup step | `research_items` | Per run | Near-duplicate threshold is configurable; false positives possible | Phase 2 |
| O9 | % drafts passing the claim checker on first attempt | Briefs with zero blocking checker findings before human edit ÷ briefs generated | n/a | Checker results | `campaign_briefs` | Per brief | Checker filters words, not meaning | Phase 4 |
| O10 | Evidence quality mix | Share of findings by evidence level A/B/C/D | n/a | Validator | `research_items` | Per run | Levels are rule-based labels, not truth scores | Phase 3 |
| O11 | Drift events reviewed within N days | Drift entries resolved ÷ raised, and median time to review | n/a | Drift queue | `imported_assets`, `approval_records` | Monthly | — | Phase 2 |

## 3. Business KPIs (require external or operational data)

| ID | KPI | Definition | Baseline required | Measurement method | Data source (must be approved) | Frequency | Limitations | Availability |
|---|---|---|---|---|---|---|---|---|
| B1 | Qualified inquiries attributable to a campaign | Count of inquiries meeting the owner's qualification rule, by campaign/channel/period, from aggregated exports | A pre-campaign period of the same length with the same channels | Aggregated counts imported via Workflow F; attribution by channel code / UTM / stated source | WhatsApp Business counts, site analytics export, dashboard aggregated export | Weekly after launch | Attribution is self-reported or last-touch; no customer-level data in MIA | **Unavailable** until store opens and exports are approved |
| B2 | Cost per qualified inquiry | Campaign cost (approved spend records) ÷ B1 | Same as B1 | Imported spend + B1 | Spend records entered by owner or ad exports | Monthly | No paid media is planned yet; organic campaigns have cost = owner time | **Unavailable** |
| B3 | Customer acquisition cost | Total marketing cost ÷ new transacting customers in period | Same as B1 plus dashboard aggregate | Imported aggregates only | Dashboard aggregated export (count only) | Monthly | Requires dashboard export contract; MIA never sees customer records | **Unavailable** |
| B4 | Lead-to-transaction conversion | Transacting customers who came via a campaign ÷ qualified inquiries | B1 + dashboard aggregate | Aggregates | Dashboard aggregated export | Monthly | Linking inquiry to transaction requires a channel/source field in the dashboard; privacy review needed | **Unavailable** |
| B5 | Incremental campaign contribution to gross margin | Margin from campaign-attributed transactions minus campaign cost, compared against baseline period | Baseline period and margin data | Aggregates | Dashboard aggregated export (margin totals only) | Quarterly | Causality cannot be claimed from this; requires controlled experiments (holdout periods/channels) | **Unavailable** |

## 4. Experiment-level measurement (how MIA avoids unsupported impact claims)

Each `Experiment` proposed by the Strategy Agent must carry: hypothesis, primary metric (from O or B lists), baseline requirement, minimum observation window, stop criteria, and the data source needed. Experiments without an available data source are labelled *"menunggu sumber data"* and are not reported as results. Observed changes are reported as *observed*, with the confounders the agent lists, and never as *caused by* a campaign unless a comparison design (holdout, A/B by channel, pre/post with control) is in place.

## 5. Reporting cadence

| Report | Content | When |
|---|---|---|
| Run summary | O1, O3, O6, O7, O8 for that run | Every run |
| Monthly MIA operations note (Indonesian summary) | O1–O11 trends, budget use vs USD 20, drift events | Monthly, manual in MVP; scheduled in Phase 6 |
| Campaign review | B1–B5 where available; otherwise explicit "tidak tersedia" list and tracking plan | Only after Phase 5 and data approval |
