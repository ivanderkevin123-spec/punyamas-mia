# MIA — Data Model

> **Phase 4 update (2026-10-14):** migration `0003_content.sql` adds `content_drafts`, `content_draft_versions` and `content_draft_evidence`, adds `subject_version`/`subject_hash` to `approval_records` and makes it append-only; content runs are `research_runs` rows of kind `content`. The planned `campaign_briefs` entity (§4) was replaced by these tables. Field-level description in `content-agent.md` §6.

> **Phase 3 update (2026-10-13):** migration `0002_strategy.sql` adds `opportunities` and `opportunity_evidence`; `research_runs` rows of kind `strategy` carry `parent_run_id`; approval records now also cover subject type `opportunity`. Field-level description in `strategy-agent.md` §6.

**Status:** Phase 1 draft · 2026-10-12 · English. Storage decision and entity definitions for the MVP; later-phase entities are specified but not created until their phase.

## 0. Ringkasan (Bahasa Indonesia)

Data MIA disimpan dalam satu berkas **SQLite** lokal. Setiap temuan riset menyimpan **sumber, waktu pengambilan, tanggal terbit (bila ada), kutipan bukti**, dan **tautan antara bukti dan klaim** yang didukungnya. Fakta bersumber, interpretasi model, hipotesis, rekomendasi, dan keputusan manusia disimpan sebagai jenis rekaman yang berbeda. Tidak ada data pribadi pelanggan.

## 1. Storage decision

**SQLite** (`data/mia.sqlite`, WAL mode, single writer) via a pure-Go driver (candidate `modernc.org/sqlite`, already present in a local module cache per the audit; dependency approval at the Phase 2 gate).

Justification from the audit: one machine, one operator, no concurrent writers, workload of thousands (not millions) of rows, no service to run, backup = file copy, and the `sqlite3` CLI is available for inspection. Alternatives rejected: MySQL (installed but stopped; a service to operate for no benefit), PostgreSQL (not installed; Docker needed), vector database or warehouse (no requirement: similarity needs are met by hashing and normalized-title comparison; full-text search can use SQLite FTS5 if ever needed).

Conventions: `id TEXT PRIMARY KEY` = time-ordered 26-character identifier generated in code (no dependency); timestamps ISO-8601 UTC `TEXT`; JSON columns are `TEXT` validated on write; foreign keys enforced (`PRAGMA foreign_keys = ON`); schema versioned by embedded migrations.

## 2. Record kinds (separation of truth levels)

| `item_kind` | Meaning | Who produces it | Evidence rule |
|---|---|---|---|
| `fact` | Source-backed observation (what a page/document/export states) | Importers, fetcher, workspace importer (policy facts), validator | ≥ 1 `Evidence` with `source_id` and excerpt |
| `interpretation` | Model-generated reading of facts (e.g. "competitors emphasise speed") | Research/Strategy agents | ≥ 1 `Evidence` with `from_item_id` pointing to `fact` items |
| `hypothesis` | Unproven belief (internal FAQ assumptions, agent guesses, owner notes) | Any | May have none; never promoted without new `fact` evidence |
| `recommendation` | Proposed action (opportunity, experiment) | Strategy agent (proposal) + scorer (final) | Rationale + evidence refs required |
| `decision` | Human decision | User via `mia approve` | Stored in `approval_records`, never generated |

## 3. Entities selected for the MVP

### 3.1 `research_runs` (ResearchRun)
Purpose: one execution of a workflow. Fields: `id`, `kind` (`research`|`strategy`|`brief`|`metrics`), `status` (`created`|`running`|`completed`|`completed_with_warnings`|`partial`|`failed`|`stale`), `created_at`, `started_at`, `finished_at`, `config_hash`, `provider_name`, `inputs_json` (batch path, flags), `summary_json` (steps, warnings, errors, counts), `cost_usd_total` (nullable), `cost_measurable` (bool), `parent_run_id` (nullable, strategy ← research). Validation: status transitions enforced in code. Retention: keep indefinitely (small). Sensitivity: low. Index: `(kind, created_at)`.

### 3.2 `run_steps` (part of ResearchRun; needed for resume)
Fields: `id`, `run_id` FK, `name`, `ordinal`, `status` (`pending`|`running`|`completed`|`failed`|`timed_out`|`budget_exhausted`|`skipped`), `attempts`, `started_at`, `finished_at`, `error_json`. Unique `(run_id, name)`.

### 3.3 `sources` (Source)
Purpose: where information came from. Fields: `id`, `source_type` (`manual_note`|`imported_file`|`internal_workspace`|`web_page`|`trend_export`|`analytics_export`|`search_result`), `name`, `reference` (URL or file path relative to MIA root or workspace root), `canonical_url` (nullable), `publisher` (nullable), `retrieved_at`, `published_at` (nullable), `content_hash` (sha256 of stored snapshot), `snapshot_path` (nullable; under `data/snapshots/`), `status` (`ok`|`unavailable`|`blocked_by_robots`|`parse_error`), `status_reason`, `allowlist_entry` (nullable), `created_at`. Validation: `reference` required; web sources require `canonical_url` and an allowlist entry; `retrieved_at` required. Dedup: unique `(canonical_url, content_hash)` when both present. Retention: snapshots pruned after a configurable period (default 180 days) unless referenced by an approved opportunity. Sensitivity: public or internal content; no PII. Index: `canonical_url`, `content_hash`.

### 3.4 `research_items` (ResearchItem)
Purpose: a single claim/observation/finding. Fields: `id`, `run_id` FK, `origin` (`import`|`workspace`|`fetch`|`agent`), `item_kind` (§2), `topic_key` (normalized topic for conflict detection, e.g. `customer_question:valuation_process`), `title`, `statement` (the claim text), `language` (`id`|`en`), `evidence_quality` (`A`|`B`|`C`|`D`, computed: A primary dated source observed directly; B credible secondary; C anecdotal/forum/unverified; D no external evidence), `status` (`imported`|`validated`|`rejected`|`duplicate`|`conflict`), `duplicate_of` (nullable FK), `conflict_group` (nullable), `claim_check_json` (checker findings on the statement), `agent_execution_id` (nullable FK), `created_at`. Validation: `fact` requires source evidence; `interpretation` requires item evidence; checker runs on every statement; `statement` ≤ 2,000 chars. Retention: indefinite. Sensitivity: low. Index: `(run_id, item_kind)`, `topic_key`, `duplicate_of`.

### 3.5 `evidence` (Evidence)
Purpose: the link between a claim and what supports it. Fields: `id`, `item_id` FK (the claim supported), `source_id` (nullable FK), `from_item_id` (nullable FK; supporting item), `excerpt` (≤ 1,000 chars, verbatim), `locator` (page section, line range, CSV row, or selector), `retrieved_at`, `published_at` (nullable), `note`. Validation: exactly one of `source_id` / `from_item_id`; excerpt required when `source_id` is set; `from_item_id` must not create cycles. Retention: with the item. Sensitivity: excerpts are short quotations (copyright-conscious). Index: `item_id`, `source_id`.

### 3.6 `opportunities` (Opportunity)
Purpose: ranked marketing opportunity. Fields: `id`, `run_id` FK, `name`, `description`, `audience_ids_json` (AU1–AU5 refs), `score` (0–100), `score_breakdown_json` ({dimension: {value, weight, rationale, evidence_ids}}), `confidence` (0–1), `evidence_quality` (computed from linked items), `missing_evidence_json`, `status` (`proposed`|`approved`|`rejected`|`needs_more_evidence`), `scoring_config_hash`, `created_at`. Relationships: `opportunity_items(opportunity_id, item_id)` link table. Validation: every dimension has rationale and ≥ 1 evidence id; score recomputed by code equals stored score. Retention: indefinite. Index: `(run_id, score DESC)`.

### 3.7 `experiments` (Experiment)
Purpose: measurable test proposed for an opportunity. Fields: `id`, `opportunity_id` FK, `hypothesis`, `primary_metric` (KPI id), `baseline_requirement`, `design` (`pre_post`|`channel_split`|`holdout`|`observational`), `min_window_days`, `stop_criteria`, `data_source_required`, `data_source_status` (`available`|`pending_approval`|`unavailable`), `status` (`proposed`|`approved`|`running`|`completed`|`abandoned`), `result_json` (Phase 5), `created_at`. Retention: indefinite.

### 3.8 `imported_assets` (ImportedAsset)
Purpose: provenance and drift detection for read-only workspace assets and imported files. Fields: `id`, `asset_key` (e.g. `brand_oracle`, `facts`, `b1`, `m1`, `context`, `decisions`, `w1_faq`), `root` (`workspace`|`imports`), `relative_path`, `sha256`, `size_bytes`, `mtime`, `workspace_commit` (nullable), `imported_at`, `parsed_summary_json` (e.g. rule list, fact ids), `review_status` (`accepted`|`review_required`|`missing`), `accepted_at`, `accepted_by_note`. Validation: path must resolve under the configured root; never written back. Retention: keep history (one row per fingerprint). Index: `(asset_key, imported_at DESC)`.

### 3.9 `agent_executions` (AgentExecution)
Purpose: audit of every model call. Fields: `id`, `run_id` FK, `step_id` FK, `agent_name`, `provider_name`, `model` (nullable), `schema_version`, `request_hash`, `request_path` (file under `data/executions/`, contains the exact prompt with the untrusted block), `response_path`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cost_usd` (nullable), `cost_measurable`, `latency_ms`, `attempt`, `outcome` (`ok`|`invalid_output`|`repaired`|`timeout`|`error`|`budget_exhausted`), `error_text`, `created_at`. Sensitivity: contains untrusted excerpts; no secrets (redacted before write). Retention: 180 days default. Index: `(run_id)`, `(created_at)`.

### 3.10 `cost_records` (CostRecord)
Purpose: ledger for budget enforcement. Fields: `id`, `run_id`, `agent_execution_id` (nullable), `category` (`llm`|`search`|`fetch`|`other`), `provider_name`, `units_json` (tokens/requests), `unit_price_source` (`provider_reported`|`price_table:<version>`|`unmeasurable`), `cost_usd` (nullable), `estimated` (bool), `period_month` (`YYYY-MM`), `created_at`. Validation: monthly sum ≤ ceiling checked before each call. Index: `(period_month)`, `(run_id)`.

### 3.11 `approval_records` (ApprovalRecord)
Purpose: human decisions. Fields: `id`, `subject_type` (`opportunity`|`brief`|`imported_asset`|`experiment`|`run`), `subject_id`, `decision` (`approved`|`rejected`|`needs_more_evidence`|`accepted_drift`|`approved_for_handoff`), `decided_by` (free text; default from config), `note`, `decided_at`. Validation: subject must exist; decisions append-only (a new row supersedes). Retention: indefinite. Sensitivity: low.

## 4. Entities deferred to later phases

| Entity | Phase | Fields (summary) |
|---|---|---|
| `campaign_briefs` (CampaignBrief) | 4 | `id`, `opportunity_id`, `language = id`, `brief_json` (objective, audience, key message, channels M1–M5, outline, CTA, measurement plan), `placeholders_json` (`[ISI: …]` list), `checker_findings_json`, `facts_check_json`, `preconditions_unmet_json`, `status` (`draft`|`reviewed`|`approved_for_handoff`|`rejected`), `handoff_note`, `created_at` |
| `metric_records` (performance) | 5 | `id`, `experiment_id`/`brief_id` (nullable), `period_start`, `period_end`, `channel`, `campaign_code`, `metric` (KPI id), `value`, `unit`, `source_id` FK, `imported_at`; aggregates only; importer rejects PII-like columns |

## 5. Relationships (summary)

```
research_runs 1─* run_steps
research_runs 1─* research_items *─1 sources (via evidence.source_id)
research_items 1─* evidence (evidence.item_id); evidence.from_item_id → research_items
research_runs 1─* opportunities *─* research_items (opportunity_items)
opportunities 1─* experiments
research_runs 1─* agent_executions 1─* cost_records
imported_assets (independent; referenced by sources.reference for workspace sources)
approval_records → any subject by (subject_type, subject_id)
```

## 6. Validation rules enforced in code (summary)

1. No `fact` without source evidence; no `interpretation` without item evidence; cycles in `from_item_id` rejected.
2. Every statement passes through the claim checker; findings stored; banned categories in `recommendation` text block approval until edited.
3. PII guard on import: any column or value matching Indonesian phone patterns (`^(\+62|62|0)8\d{7,11}$`), 16-digit identity numbers, or email addresses in customer-like contexts → file rejected.
4. Paths: workspace and import paths must resolve under their configured roots (no `..`, no symlink escape).
5. Budget: monthly sum of `cost_records.cost_usd` (plus estimates for unmeasurable calls using the configured price table) must stay ≤ `agent-limits.yaml → monthly_ceiling_usd` (20) before any call.

## 7. Retention and backup

- Database and snapshots live under `data/` (git-ignored). Backup: `mia backup` copies the database with `VACUUM INTO` to `data/backups/mia-<date>.sqlite`; keep the last 7 by default. Restore = copy back while no run is active.
- Snapshots and execution files pruned by age (configurable); rows referenced by approved opportunities or briefs are kept.
- No customer data exists in the schema, so no personal-data retention obligations arise from MIA itself.
