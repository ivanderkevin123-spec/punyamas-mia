# Phase 5 plan — Marketing workflow orchestration and quality gate

**Status:** plan written 2026-10-14 before implementation (this file did not exist when Phase 5 was requested). English for engineers; the business summary is in Indonesian.

## Ringkasan (Bahasa Indonesia)

Fase 5 menyatukan langkah yang sudah ada — riset, strategi, tinjauan peluang, draf konten, tinjauan konten, dan ekspor — menjadi satu workflow yang dapat dijalankan ulang, dilanjutkan setelah terhenti, dan diaudit. Status workflow dihitung dari basis data setiap kali dibaca. Workflow tidak pernah menyetujui peluang atau draf, tidak menerbitkan, dan tidak mengirim apa pun. Tidak ada AI baru, tidak ada provider berbayar, dan tidak ada akses jaringan.

## Deviation from the roadmap

`roadmap.md` (Phase 1) planned Phase 5 as *Performance analysis* (metrics import, KPIs, Performance Agent). The Phase 5 request redefines it as *workflow orchestration and quality gate*. Following the request, existing contracts and architecture are kept and the smallest scope that delivers the workflow is implemented. Performance analysis is deferred; its phase number is for the owner to assign.

## Verified starting point (git, not earlier reports)

| Commit | Content | Files |
|---|---|---|
| `fad5e36` | Phases 0–2 foundation | 80 |
| `4d510e0` | Phase 2 remediation A2–A7 | 20 |
| `4ccd4fa` | Phase 3 Strategy Agent | 35 |
| `fab8255` | Phase 4 Content Agent (`main` = `origin/main`) | 51 |

## Scope

1. `internal/workflow`: one workflow record per inputs batch that links the research run, the strategy run and one content run per approved opportunity; drives the existing orchestrator; derives status, gates, blockers and next actions from the database.
2. Migration `0004_workflow.sql`: `workflows`, `workflow_content_runs`, `workflow_events`, with immutability and append-only triggers.
3. CLI `workflow start|run|resume|show|list|next|report|export`.
4. Quality gates A–E reusing the Phase 2–4 validators (run statuses, validated evidence, opportunity eligibility, `content.Load`, `content.ReadinessOf`).
5. Idempotency (reserved run IDs, `Options.RunID`), resume (re-read DB, orchestrator resume semantics), concurrency (lease with compare-and-set), bounded automatic runs per command, budget precheck.
6. Workflow report (Markdown + HTML) rendered from current database state.
7. Fixes needed for honest reporting: escaped evidence refs in the research, strategy and brief Markdown templates (golden change documented); content report counts derived from stored drafts instead of the run-time snapshot; ID validation for CLI arguments (part of A12).

## Out of scope

New agents or AI, paid providers, network access, publication or messaging, scheduler/daemon/server, writes to `~/Documents/bots/punyamas`, Phase 6, commits and pushes.
