# MIA — User Workflows

**Status:** Phase 1 draft · 2026-10-12 · English with Indonesian user-facing labels where they will appear in reports.

Conventions: every workflow is a sequence of **deterministic steps** executed by the orchestrator; AI is called only in the steps marked *(AI)*, and only through the `Provider` interface (mock in the MVP). Every run writes a `RunSummary` (status, steps, errors, cost, warnings) that the user can read with `mia status <run-id>`.

## Workflow A — Manual research run (MVP)

**Trigger:** `mia run research --inputs imports/2026-10/ --sources approved` (sources flag has no effect until live fetching is approved; in Phases 2–3 only imports and workspace documents are read).

| Step | Actor | Action | Output / state |
|---|---|---|---|
| A1 | User | Places research notes/exports in `imports/<batch>/` with the required front-matter or CSV columns (source name, URL or file reference, date, excerpt) | Files on disk |
| A2 | Orchestrator | Creates `ResearchRun` (status `created`), loads config, checks budget headroom and provider availability | Run id |
| A3 | Importer | Parses each file; validates provenance; rejects PII-like columns; records `Source` and `ResearchItem` rows with `item_kind = fact|hypothesis` as declared by the file, `evidence_quality` from the import rules | Items `status = imported`; parse errors listed per file |
| A4 | Workspace importer | Reads approved `bots/punyamas` files read-only, stores `ImportedAsset` (path, sha256, mtime, commit) and derived research items tagged `source_type = internal_workspace`, `item_kind = hypothesis` | Drift check runs first (Workflow D); a drift flag pauses the run for review if configured |
| A5 | Dedup | Canonicalizes URLs, hashes normalized content, flags near-duplicate titles; duplicates linked, not deleted | `duplicate_of` set; duplicate rate recorded |
| A6 | Research Agent *(AI)* | Receives validated items in a delimited untrusted-data block; returns structured findings: customer needs, competitor positioning summaries, topics, candidate opportunities, each with `evidence_refs[]` pointing to item ids and a `kind` of `fact`, `interpretation` or `hypothesis` | `AgentExecution` with usage/cost; findings validated against JSON schema |
| A7 | Validator | Rejects findings whose `evidence_refs` do not resolve or whose `kind = fact` lacks a source-backed item; applies claim checker to free text; flags banned categories | Invalid findings dropped and counted; step fails if the failure ratio exceeds the configured threshold |
| A8 | Report | Renders Markdown + HTML: *Ringkasan eksekutif*, *Temuan bersumber*, *Interpretasi*, *Hipotesis*, *Bukti dan tautan*, *Asumsi dan ketidakpastian*, *Biaya*, *Langkah berikutnya* | `reports/<run-id>/research.md|.html` |
| A9 | Orchestrator | Status `completed` or `completed_with_warnings`; never `completed` if any required step failed | `RunSummary` |

**Resume:** rerunning with the same batch reuses imported items (idempotent by content hash) and skips completed steps unless `--force`.

## Workflow B — Opportunity prioritization (MVP)

**Trigger:** `mia run strategy --from-run <research-run-id>` (or chained automatically after A with `--then strategy`).

| Step | Actor | Action | Output |
|---|---|---|---|
| B1 | Orchestrator | Loads validated findings and existing opportunities; loads `config/scoring.yaml` (weights, scales, thresholds) | — |
| B2 | Grouping *(AI)* | Clusters related findings into candidate opportunities with names and one-paragraph descriptions, each citing finding ids | Candidates (schema-validated) |
| B3 | Scoring proposal *(AI)* | For each candidate proposes a value 0–5 per dimension (relevance, evidence quality, customer intent, business impact, implementation cost, time to test) plus confidence 0–1 and a **written rationale per dimension** citing evidence ids | Proposal (schema-validated) |
| B4 | Scorer (deterministic) | Recomputes `score = Σ weight_d × value_d` (cost and time inverted), applies confidence multiplier, clamps to 0–100, overrides `evidence_quality` with the value computed from linked evidence levels, rejects proposals whose rationale is missing or whose cited ids do not exist | `Opportunity` rows with `score_breakdown` JSON |
| B5 | Gap analysis *(AI, optional)* | Lists missing evidence per opportunity and proposes 1–3 measurable experiments with metric, baseline requirement, and stop criteria | `Experiment` rows (status `proposed`) |
| B6 | Report | *Peluang prioritas* table with score, breakdown, rationale, evidence links, missing evidence, proposed experiments; explicit note "Skor adalah heuristik yang didokumentasikan, bukan prediksi statistik" | `reports/<run-id>/strategy.md|.html` |
| B7 | User | Marks opportunities `approved`, `rejected`, or `needs_more_evidence` via `mia approve opportunity <id> --decision ... --note ...` | `ApprovalRecord` |

## Workflow C — Content brief preparation (Phase 4)

| Step | Actor | Action | Output |
|---|---|---|---|
| C1 | User | `mia run brief --opportunity <id>` for an **approved** opportunity only | Run |
| C2 | Orchestrator | Loads brand config (tone KP1–KP4, audiences AU1–AU5, messages, approved facts FB1–FB14, `isian_wajib`), M1 preconditions, channel constraints | — |
| C3 | Content Agent *(AI)* | Drafts an Indonesian brief: objective, audience, key message (intent language before opening), channel ideas mapped to M1–M5, content outline, CTA limited to approved capabilities, measurement plan; uses `[ISI: …]` for every unknown fact | Draft (schema-validated) |
| C4 | Checker (deterministic) | Runs the claim checker on every text field; checks facts against `B4-fakta.json`; verifies no prices, addresses, hours, guarantees, reviews, statistics appear; lists M1 preconditions not yet met | Findings list; brief `status = draft` with `blocking_findings` |
| C5 | Report | Brief rendered to `drafts/<brief-id>.md` with a header "DRAF — belum disetujui; tidak untuk publikasi" | File |
| C6 | User | Reviews, edits outside MIA if desired, records decision (`approved_for_handoff` / `rejected`) | `ApprovalRecord` |
| C7 | User | **Manual handoff**: copies the approved brief into `bots/punyamas/docs/05-marketing/` through the workspace's own process. MIA never writes there | — |

## Workflow D — Source drift detection (MVP)

**Trigger:** automatically at the start of every run, and on demand with `mia config check`.

| Step | Action | Result |
|---|---|---|
| D1 | For each asset listed in `config/research-sources.yaml → workspace_assets` (path relative to the workspace root): stat the file, compute sha256, read the workspace `HEAD` commit | Current fingerprint |
| D2 | Compare with the last `ImportedAsset` record | `unchanged` / `changed` / `missing` / `new` (a path listed but never imported) |
| D3 | For `brand-oracle.mjs`: parse the `POLA` array (category + regex) and diff against `config/marketing-policy.yaml` rules | Rule-level diff |
| D4 | For `B4-fakta.json`: diff fact ids, text, and status against `config/business.yaml → facts` | Fact-level diff |
| D5 | Any difference creates a `review_required` entry in the run summary and in `reports/drift/<date>.md`, with the diff. **MIA does not update its configuration automatically** | Human review queue |
| D6 | User runs `mia config sync --asset <name> --accept` after review; the new fingerprint and the accepted values are stored with the reviewer's note | `ImportedAsset` updated; `ApprovalRecord` |
| D7 | If `agent-limits.yaml → drift.block_runs = true`, runs stop at A4 while a drift is unreviewed; otherwise they continue with a prominent warning | Configurable |

## Workflow E — Failure handling (MVP)

| Situation | Detection | Behaviour | Run status |
|---|---|---|---|
| Source unavailable (file missing, fetch denied, HTTP error, robots disallow) | Importer/fetcher error | Record `Source.status = unavailable` with reason; continue with other sources; report lists it under *Sumber tidak tersedia* | `completed_with_warnings` (or `failed` if zero sources succeed) |
| Document cannot be parsed | Parser error | File skipped, error with line/column stored; never partially imported | `completed_with_warnings` |
| Provider missing or misconfigured (no key, unknown provider name) | Provider factory at A2 | Run refuses to start with a clear message naming the config key and the mock alternative | `failed` (before any step) |
| Agent timeout | Context deadline per agent (`agent-limits.yaml`) | Step marked `timed_out`; one bounded retry if allowed; otherwise step fails | `failed` for the step; run `failed` if the step is required |
| Budget limit reached (per-run, per-agent, monthly) | Pre-call estimate and post-call ledger | No further provider calls; step `budget_exhausted`; report states what was not done | `failed` or `partial` as configured; never `completed` |
| Output fails schema validation | JSON schema + semantic checks | One bounded repair attempt with the validation errors; then step fails. Invalid content is stored for debugging but never rendered as results | `failed` for the step |
| Conflicting information between sources | Validator detects contradictory `fact` items on the same topic (same `topic_key`, incompatible values) | Both items kept, flagged `conflict_group`; report shows both with sources and asks for human resolution; neither is promoted to fact in recommendations | `completed_with_warnings` |
| Interrupted process (SIGINT/SIGTERM, power loss) | Run left in `running` with a heartbeat older than the limit | `mia status` shows `stale`; `mia run --resume <id>` continues from the last completed step; completed steps are never redone | resumed |

A failed required step **always** blocks the final report from claiming success; the report header states "Run gagal / tidak lengkap" with the failed steps.

## Workflow F — Future performance analysis (Phase 5)

| Step | Action | Safeguard |
|---|---|---|
| F1 | User exports **aggregated** metrics from an approved source (WhatsApp Business counts, site analytics, ad exports) to CSV with columns: period, channel, campaign, metric, value, source | Importer rejects any column resembling phone numbers, names, identity numbers; rows are aggregates only |
| F2 | `mia import metrics file.csv --campaign <id>` stores `MetricRecord`s linked to `Experiment`/`CampaignBrief` | No customer-level data; no link to dashboard records |
| F3 | Performance Agent *(AI, optional)* compares outcomes with documented goals, lists plausible explanations labelled as hypotheses, distinguishes correlation from causation, recommends follow-up experiments | Deterministic code computes the KPIs; the model only interprets |
| F4 | If data is missing for a KPI, the report states "Tidak dapat dihitung: data X belum tersedia" and proposes the tracking plan | Honest reporting |
| F5 | Future dashboard export (aggregated inquiry counts) is imported through the same CSV contract once the dashboard owner approves the format | No direct database or API access |

## Cross-cutting rules

- Every AI step receives inputs inside a delimited untrusted-data block and has **no tools**; the application decides what is read or fetched.
- Every model output is validated before persistence; raw outputs are stored with the `AgentExecution` for audit.
- Every report separates *Fakta bersumber*, *Interpretasi model*, *Hipotesis*, *Rekomendasi*, and *Keputusan manusia*.
- Approval is a stored human action; nothing leaves the machine.
