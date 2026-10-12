# MIA — Implementation Roadmap

**Status:** Phase 1 · 2026-10-12. Phases 2–7 are **not started** and each requires its own approval. Costs are external USD costs; the monthly ceiling is USD 20.

## 0. Ringkasan (Bahasa Indonesia)

Pekerjaan dibagi menjadi tahap-tahap kecil dengan gerbang persetujuan: Fase 2 membangun fondasi dan alur mock (biaya USD 0), Fase 3 menambahkan agen Riset dan Strategi dengan data impor dan, bila disetujui, sumber web terbatas, Fase 4 agen Konten, Fase 5 analisis kinerja dari data impor, Fase 6 penjadwalan dan notifikasi, Fase 7 evaluasi dan pengerasan. Setiap fase berakhir dengan laporan dan berhenti menunggu persetujuan. Sebelum Fase 2 dimulai, pemilik perlu: membuat repositori git `mia` (inisialisasi diblokir di sesi ini), menetapkan identitas commit, dan menyetujui daftar dependensi.

## Phase 0 — Audit (completed 2026-10-12)

Artifacts: `docs/audit/environment.md`, `existing-projects.md`, `integrations.md`, `risks-and-constraints.md`, `phase0-report.md`. Verified unchanged in Phase 1 (sha256 recorded in the Phase 1 report).

## Phase 1 — Specification (this phase, completed with this document set)

Artifacts: 11 documents under `docs/product`, `docs/architecture`, `docs/implementation` plus `docs/phase1-decision-memo.md`; `.gitignore`. Not done, by constraint: `git init` (blocked in this session), commits (no identity configured in `mia`).

**Follow-ups before Phase 2 starts (owner):**
1. Create the repository: `git init -b main` in `~/Documents/punyamas/mia` (or allow the session to do it).
2. Set the local commit identity (proposal: the same name/email as in `kerjalancar`): `git config user.name "Kevin Ivander"` and `git config user.email <account email>`.
3. Approve the Phase 2 dependency list (§ Phase 2 → Dependencies).
4. Confirm the workspace asset paths listed in `docs/architecture/integrations.md` §6.

## Phase 2 — Minimal foundation and mock workflow (completed 2026-10-12; see `phase2-report.md`)

| Field | Content |
|---|---|
| Objective | A working end-to-end mock run: import → workspace import + drift check → dedup → research step (mock) → validation → Markdown/HTML report, with run status, budget gate, and tests |
| Scope | Project skeleton, Makefile with `GO ?= $(HOME)/sdk/go1.27.2/bin/go` and `GOTOOLCHAIN=local`, config loading/validation, `log/slog` with redaction, SQLite store + migrations, importers (md/csv/json) with PII guard, workspace importer + fingerprint/drift, dedup, mock provider, budget ledger, orchestrator (steps, timeouts, retries, resume), research-step plumbing using a mock fixture, report templates, CLI commands `run`, `status`, `import`, `config check`, `report`, `validate`. README and `docs/implementation/operations.md` |
| Prerequisites | Phase 1 approval; repository initialized; commit identity set; dependencies approved |
| Files / modules | `cmd/mia`, `internal/{config,orchestrator,research/importer,research/workspace,research/dedup,provider/mock,budget,store,report,log,validate,brand}` (brand: rule loading only), `schemas/research_findings.v1.json`, `config/*.yaml` (initial values imported from the workspace with provenance), `testdata/sample-batch`, `Makefile`, `.env.example`, `go.mod` |
| Dependencies (approval requested) | `modernc.org/sqlite` (pure-Go SQLite; present in a local cache), `go.yaml.in/yaml/v3` (YAML; present in a local cache). Everything else from the standard library (`log/slog`, `encoding/json`, `html/template`, `text/template`, `crypto/sha256`, `database/sql`). JSON schema validation implemented with typed structs + hand-written checks first; a schema library is deferred until justified |
| Test plan | Unit: config validation, redaction, PII guard, dedup, fingerprint/drift, budget gate, resume logic. Integration: store migrations + run lifecycle in a temp dir. Workflow: `TestWorkflow_MockEndToEnd` with golden report. All under `make test` with the native toolchain |
| Acceptance criteria | F1, F3, F6, F7, F8, F9, F10, R1–R7, S1, S4, S6, Q1 (partial), Q2, Q5, Q6, Q7 (see `acceptance-criteria.md`) |
| Risks | Toolchain confusion (mitigated by Makefile + `GOTOOLCHAIN=local`); sandbox limits on tests (no ports/network needed); scope creep (no agents beyond plumbing) |
| Cost | USD 0 (mock only) |
| Approval gate | Phase 2 report with test output, `make go-version` output, workspace-unchanged check, dependency list as installed; owner approves Phase 3 |

## Phase 3 — Research and Strategy agents with approved sources

| Field | Content |
|---|---|
| Objective | Useful research and strategy reports from imported data (and allowlisted sources if approved), with evidence preservation, claim checking, scoring with rationale, approvals |
| Scope | Research Agent and Strategy Agent (prompts, schemas v1, parsers), claim checker port with equivalence fixture from B1 (BT1–12, LK1–10, CT1–10), facts check against `B4-fakta.json`, scorer + `config/scoring.yaml`, experiments, conflict detection, `mia approve`, strategy report. Optional, only if approved: fetcher with robots/allowlist/snapshots for the approved shortlist entries; Google Trends CSV importer; Anthropic API adapter with usage ledger |
| Prerequisites | Phase 2 accepted; owner decisions on: live fetching (allowlist from `integrations.md` §5 after the §4 checklist), AI provider (Option A key or stay mock), model choice, `ceiling_run`/`ceiling_agent` values |
| Files / modules | `internal/agent/{research,strategy}`, `internal/scoring`, `internal/brand` (checker + equivalence test), `internal/validate` (semantic rules), `schemas/*.v1.json`, `config/scoring.yaml`, optional `internal/research/fetch`, optional `internal/provider/anthropic` |
| Dependencies | Optional `golang.org/x/net/html` (HTML text extraction) only if fetching is approved; optional `github.com/anthropics/anthropic-sdk-go` only if Option A is approved (raw `net/http` is the fallback if the SDK is not approved) |
| Test plan | Unit: scorer, checker equivalence, facts check, conflict detection, fetch allow/robots logic (with local test server replaced by in-memory round-tripper to avoid port binding). Workflow: full mock run golden report. If a live provider is approved: one smoke test with `ceiling_run = 0.10` and usage comparison |
| Acceptance criteria | F2, F4, F5, F11, F12, S2, S3, S7, Q1, Q3, Q4 |
| Risks | Fabricated citations (mitigated by evidence rules); Indonesian output quality; terms of approved sources; cost if live |
| Cost | USD 0 in mock; ≤ USD 5 for live smoke tests and first runs if Option A is approved (within the USD 20 ceiling) |
| Approval gate | Phase 3 report including an inspected sample report (Q4 checklist), cost ledger, allowlist decisions recorded; owner approves Phase 4 |

## Phase 4 — Content and campaign capabilities

| Field | Content |
|---|---|
| Objective | Indonesian draft briefs from approved opportunities, brand-safe by construction, with manual handoff |
| Scope | Content Agent, brief schema and templates (objective, audience AU1–AU5, key message in intent language, channels M1–M5, outline, CTA, measurement plan), `[ISI: …]` placeholders, checker + facts check on all fields, M1 precondition list, `drafts/` output with DRAFT header, `mia approve brief` |
| Prerequisites | Phase 3 accepted; at least one approved opportunity; owner confirms brief template and tone examples |
| Files / modules | `internal/agent/content`, `schemas/brief.v1.json`, `internal/report/brief_*.tmpl`, migration adding `campaign_briefs` |
| Dependencies | None new |
| Test plan | Unit: placeholder detection, forbidden-topic detection (prices, addresses, hours, guarantees, reviews, statistics), precondition listing. Workflow: mock brief golden file. Manual: owner reviews two briefs for tone |
| Acceptance criteria | P4-1, P4-2, O5 measurable, F12 (approval before brief) |
| Risks | Model Indonesian register; subtle unsupported claims passing the word filter (human review mandatory) |
| Cost | USD 0 mock; ≈ USD 0.03–0.5 per 3 briefs live depending on model [A] |
| Approval gate | Phase 4 report with sample briefs and checker results; owner approves Phase 5 |

## Phase 5 — Performance analysis

| Field | Content |
|---|---|
| Objective | Compute KPIs from approved, aggregated campaign data and report honestly when data is missing |
| Scope | Metrics CSV importer (aggregates only, PII guard), `metric_records`, KPI calculations in code (O-series automatically; B-series only from imported aggregates), Performance Agent interpretation (hypotheses, confounders, follow-up experiments), tracking-plan generator, campaign review report |
| Prerequisites | Phase 4 accepted; sample dataset approved by the owner; if real data: approved exports (analytics/GSC/ads CSV) and, only if explicitly approved, a written aggregated export contract from the dashboard owner |
| Files / modules | `internal/research/importer/metrics`, `internal/kpi`, `internal/agent/performance`, `schemas/performance.v1.json`, migration adding `metric_records` |
| Dependencies | None new |
| Test plan | Unit: KPI formulas, missing-data reporting, PII rejection. Workflow: sample dataset golden report |
| Acceptance criteria | P5-1; KPI definitions from `kpi-framework.md` implemented with limitations printed |
| Risks | Causal overclaiming (mitigated by wording rules and design labels); unavailable data for months |
| Cost | USD 0 mock; negligible live |
| Approval gate | Phase 5 report; owner approves Phase 6 |

## Phase 6 — Scheduling and notifications

| Field | Content |
|---|---|
| Objective | Optional unattended runs with visible status and easy disable |
| Scope | `mia schedule render` (launchd plist, absolute paths, logs under `data/logs`), documented manual install/uninstall, run priorities under the monthly ceiling, notification adapter (Telegram bot or email) sending run summaries only, `mia notify test` |
| Prerequisites | Manual runs stable for at least 4 weeks; owner approval of schedule and channel; bot token or SMTP provided by the owner (external account creation is the owner's action) |
| Files / modules | `internal/schedule`, `internal/notify/{telegram,email}`, `docs/implementation/operations.md` updates |
| Dependencies | None new (Telegram via `net/http`; email via `net/smtp`) |
| Test plan | Unit: plist rendering, message formatting, redaction in notifications. Manual: owner installs the plist and observes one scheduled run |
| Acceptance criteria | P6-1; R3 under scheduled load; S4 for notifications |
| Risks | Silent failures (mitigated by failure notifications); subscription/terms if any provider other than Option A is used (not planned) |
| Cost | Scenario C bounds (`cost-model.md` §7), ≤ USD 20 |
| Approval gate | Phase 6 report with one observed scheduled run; owner approves Phase 7 |

## Phase 7 — Evaluation and hardening

| Field | Content |
|---|---|
| Objective | Prove the acceptance criteria, measure cost and KPIs, document limitations, fix critical defects |
| Scope | Full acceptance run (`acceptance-criteria.md` all sections), cost review against the ledger, KPI baseline report (O1–O11), security review (secrets scan, path guards, redaction), backup/restore drill, operations and troubleshooting docs, limitations list |
| Prerequisites | Phases 2–6 accepted (or 2–5 if scheduling is deferred) |
| Files / modules | Fixes as needed; `docs/implementation/evaluation-report.md` |
| Dependencies | None new |
| Test plan | Everything in `make test`; manual drills recorded |
| Acceptance criteria | P7-1 and all pending criteria resolved to passed/failed/N-A with evidence |
| Risks | Discovering design gaps late (mitigated by phase gates) |
| Cost | Within the ledger; reported |
| Approval gate | Final report; owner decides on steady-state operation |

## Dependency summary (nothing installed yet)

| Dependency | Phase | Why | Status |
|---|---|---|---|
| `modernc.org/sqlite` | 2 | Pure-Go SQLite, no cgo, single binary | Approval requested |
| `go.yaml.in/yaml/v3` | 2 | YAML configuration | Approval requested |
| `golang.org/x/net/html` | 3 (optional) | HTML → text for approved fetching | Only if fetching approved |
| `github.com/anthropics/anthropic-sdk-go` | 3 (optional) | Official SDK for Option A | Only if Option A approved; raw HTTP fallback |

## Gate checklist used at every phase end

1. Files changed, features completed, tests executed with output.
2. `make go-version` output.
3. `git -C ~/Documents/bots/punyamas status --porcelain` + `rev-parse HEAD` before/after.
4. Cost ledger for the phase.
5. Dependencies added (exact versions).
6. Known limitations and risks.
7. Explicit request for approval of the next phase.
