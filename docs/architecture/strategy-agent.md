# MIA — Strategy Agent (Phase 3)

**Status:** implemented 2026-10-13, mock provider only, uncommitted pending approval. English for engineers; §0 is the business-facing summary.

## 0. Ringkasan untuk pemilik usaha (Bahasa Indonesia)

Agen Strategi membaca **hanya butir riset yang sudah tervalidasi** dari satu run riset, lalu mengusulkan peluang pemasaran dan brief kampanye dalam bahasa Indonesia. Model hanya **mengusulkan**; semua keputusan penting dibuat oleh kode: setiap kutipan bukti diperiksa, bukti yang saling bertentangan ditampilkan kedua sisinya, klaim harga/jam buka/alamat/jaminan/statistik ditolak, teks yang gagal pemeriksa merek diblokir, peluang yang memerlukan toko diberi label, usulan ganda digabung, dan tingkat (A/B/C) dihitung ulang dari bukti. Tidak ada peluang yang disetujui otomatis: CEO mencatat keputusan (`approved`, `rejected`, `needs_more_evidence`) dengan riwayat yang hanya bisa bertambah, dan brief yang disetujui diekspor ke `drafts/` untuk serah terima manual. Tidak ada panggilan berbayar, tidak ada akses web, dan ruang kerja Punyamas tidak pernah ditulis.

## 1. Position in the architecture

```
research run (Phase 2)                      strategy run (Phase 3)
import → workspace → dedup → research_agent  load_evidence → conflicts → strategy_agent → evaluate → report
          → validate → report                        ↑ validated items of the parent run only
                                                     ↓ opportunities + opportunity_evidence (SQLite)
                                                     ↓ reports/<run>/strategy.{md,html}
                                   approve opportunity (append-only approval_records) → export brief → drafts/<id>.md
```

A strategy run is a separate `research_runs` row (`kind = strategy`, `parent_run_id` = the research run). It reuses the Phase 2 orchestrator: steps with timeouts, bounded retries, budget gate, resume, report always re-rendered, counts rebuilt from the store.

## 2. Inputs and contracts

| Contract | Definition | Enforced by |
|---|---|---|
| Evidence input | validated items of the parent run (`status = validated`) with their first evidence excerpt and source name; agent-produced items carry `source = agent:<execution>` so independence counts stay honest | `Store.ValidatedItemsForRun`, `orchestrator.loadEvidence` |
| Prompt | static Indonesian instructions (rules 1–8, KPI/segment/pillar/effort enumerations, banned-claim categories) first; evidence inside a `DATA TIDAK TEPERCAYA` block, sorted by ref, last | `internal/agent/strategy.BuildRequest` |
| Output schema `strategy_proposals.v1` | `{"opportunities":[Proposal],"conflicts":[[ref,ref]],"notes":""}`; Proposal fields: `title, segment, topic_key, need, interpretation, fact_refs[], dimensions{evidence_strength,customer_intent,strategic_fit,ease_of_testing,measurability}, dimension_rationale{}, fit_pillars[], requires_store, message, offer_concept, channel, test_design, success_metric, success_criteria, resources, effort, risks[], assumptions[], missing_evidence[], dependencies[], next_action` | `ParseOutput` (strict JSON, unknown fields rejected), `Structural` (required fields, enumerations, 0–3 ranges, length limits, `topic_key` normalisation) |
| Repair | one bounded repair request listing the validation problems; at most `max_calls_per_step` (2) calls per step and **at most 2 Strategy Agent calls per run**, counted from persisted executions, errors and timeouts included | `stepStrategyAgent`, `Store.CountExecutions` |

The agent's `conflicts` and `dimensions.evidence_strength` are informational only; code recomputes both.

## 3. Evidence handling

**Separation of truth levels.** Cited items are grouped by kind: facts (sourced, validated), interpretations, hypotheses. Only facts contribute to evidence strength; interpretations and hypotheses are listed in the brief under their own headings and never presented as facts. `need` and `interpretation` are stored separately; the report labels the interpretation "bukan fakta".

**Conflict detection (step `conflicts`, deterministic heuristic).** Two validated facts are placed in one conflict group `cg:<topic_key>` when they share a `topic_key`, come from different sources, differ in negation polarity (`conflict.negation_pattern`), and are otherwise similar statements (token Jaccard ≥ `conflict.min_similarity`, default 0.5, after removing the negation words). The group is written to `research_items.conflict_group` of the parent run's items. Limitations: paraphrased contradictions without a negation word are missed; the agent's own `conflicts` list is not trusted.

**Evidence strength rubric (code).** 0 = no validated A–C fact (hypothesis) · 1 = one C-quality fact · 2 = one B-quality fact or two C-quality facts from different sources · 3 = one A-quality fact or two B-quality facts from different sources. D-quality facts count as no external evidence.

## 4. Gates (all in `internal/strategy`, applied in order)

| Gate | Rule | Outcome |
|---|---|---|
| 1 Evidence | every `fact_refs` entry must resolve to a validated item of the parent run; unknown or unvalidated refs are dropped and reported | no valid ref → `rejected`; no A–C fact → `hypothesis`, tier capped at C |
| 2 Conflicting evidence | a cited item in a conflict group | both sides rendered with the cited side marked; tier capped at C; brief dependency "selesaikan konflik" |
| 3 Brand and factual claims | ported brand checker (9 categories) on all generated text → `blocked`; `unsupported_claims` regexes (HARGA, JAM_BUKA, ALAMAT, STATISTIK, KLAIM_PELANGGAN) and any number not present in cited evidence or `business.yaml angka_boleh` → `rejected` (identifier tokens such as O4, PM1, FB5, Q61, §2, #5 are ignored) | `blocked` cannot be approved; `rejected` cannot be approved |
| 4 Operational dependency | `requires_store` flag or `store_dependency_terms` in the execution-oriented fields (message, offer, channel, test design, resources, dependencies, next action) | label `requires_store`, ease of testing capped at 1, operational note and dependency in the brief; never treated as fact |
| 5 Duplicates | same `topic_key` and (shared evidence IDs ≥ `dedup.shared_evidence_fraction` of the smaller set, or title Jaccard ≥ `dedup.title_jaccard`) | later proposal merged into the earlier one; its evidence IDs are added (corroboration kept); `merged_from` recorded |

## 5. Scoring and tiers

Five ordinal dimensions 0–3. `evidence_strength` is computed by code; the other four come from the agent and are capped by code: customer intent ≤ 1 without any A–C fact and ≤ 2 with fewer than two independent fact sources; strategic fit ≤ 1 without a valid pillar PM1–PM4; ease of testing ≤ 1 when a store is required; measurability = 0 for an unknown KPI and ≤ 1 for a KPI not available before opening (B-series). Each stored dimension carries its value, rationale and source (`code`, `agent`, `agent+cap (...)`). The **ordinal sum** (0–15) is labelled as an ordinal sum, never a probability; there are no confidence multipliers.

Tier rules in `config/scoring.yaml`, applied by code: **A** evidence ≥ 2, intent ≥ 2, fit ≥ 2 and no cap; **B** evidence ≥ 1, fit ≥ 2 and no cap forcing C; **C** otherwise, including every hypothesis and every conflict; blocked and rejected proposals have no tier.

## 6. Storage and provenance

Migration `0002_strategy.sql`: `opportunities` (status, tier, hypothesis, requires_store, dimensions/caps/gates/conflicts/brief/merged_from/reject_reasons JSON, `config_hash`, `scoring_config_hash`, `fixture_hash`, `agent_execution_id`) and `opportunity_evidence` (opportunity → item, role). The run summary records `parent_run_id`, `scoring_config_hash` (sha256 prefix of `config/scoring.yaml`) and `fixture_hash` (sha256 of the replayed mock fixture). Executions keep the full request and response under `data/executions/`. Re-evaluating a run (`--force`) never duplicates: proposal indexes already stored are skipped with a warning.

## 7. Approval workflow

Statuses: `proposed` → (`approved` | `rejected_by_review` | `needs_more_evidence`) by `mia approve opportunity` (since Phase 4 the decision row and the status mirror are written in one transaction, `Store.RecordOpportunityDecision`, and `approval_records` is append-only at the storage layer); `blocked` and `rejected` are terminal and refuse review. Every decision appends an `approval_records` row (subject `opportunity`); the opportunity's status mirrors the latest decision but the evaluation (tier, dimensions, evidence) is never modified. Reports and briefs show the full history. `mia export brief --id` renders the approved opportunity's brief (`templates/brief.md.tmpl`, DRAF header, evidence by kind, ordinal scores, dependencies, approval history, provenance) to `drafts/<id>.md` or `--out`; any path inside the Punyamas workspace root is refused. Handoff to `bots/punyamas` is manual.

## 8. Configuration and mock execution

- `config/scoring.yaml` (validated at start; hashed): tiers, dedup thresholds, conflict heuristic, unsupported-claim rules, store terms, KPI lists (`kpis`, `kpis_available_now` = O1–O11), segments, pillars, efforts.
- Fixtures `testdata/fixtures/strategy/`: `sample-batch.json` (five proposals: three Tier A, one duplicate merged, one store-dependent hypothesis), `gates.json` (one proposal per gate failure plus one valid store-dependent proposal), `conflict.json`, `default.json` (empty). The fixture key defaults to the parent research run's key.
- Cost: mock usage is estimated (runes/4), cost USD 0, measurable; the budget gate, uncertain-cost booking and ceilings apply exactly as in Phase 2.
- `MIA_DRAFTS_DIR` overrides `drafts/`.

## 9. Security and safety properties

No network, no tools for the model, evidence in an untrusted block, all untrusted strings escaped in Markdown (`text`/`cell`/`code`) and auto-escaped in HTML, prompts and outputs stored with mode 0600, no PII fields, workspace root never written, approvals append-only, and a run whose two calls are spent fails closed (`batas 2 panggilan`) instead of calling again.

## 10. Known limitations

1. Conflict detection is a lexical heuristic (negation + similarity); it does not understand meaning.
2. Factual-claim detection is regex-based; a paraphrased price or guarantee can pass the word filter, so human review stays mandatory (D-005).
3. Store-dependency terms are scanned in execution fields only; an operational dependency expressed elsewhere relies on the agent's `requires_store` flag.
4. The per-run cap counts failed and timed-out calls; a run that spent both calls must be restarted with `run strategy --from-run`, which creates a new run (the failed one keeps its report).
5. Dimension values other than evidence strength are the agent's opinion within code caps; they are not calibrated.
6. Only the mock provider exists; no live model has produced a strategy report yet, so fixture outputs are hand-written examples, not model behaviour.
