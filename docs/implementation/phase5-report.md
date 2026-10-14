# Phase 5 report — Workflow orchestration and quality gate

**Date:** 2026-10-14 · **Status:** implemented and verified, **not committed** (awaiting approval) · **Base:** `fab8255` (Phase 4) on `main` = `origin/main` · **Provider:** mock only (USD 0) · **New dependencies:** none (`go.mod`/`go.sum` unchanged). Design: `docs/architecture/workflow.md`; plan: `docs/implementation/phase5-plan.md`.

## Ringkasan bisnis (Bahasa Indonesia)

MIA kini punya satu alur kerja yang menghubungkan riset, strategi, tinjauan peluang, draf konten, tinjauan draf, dan ekspor. Operator memulai workflow dari berkas riset, menjalankannya, lalu MIA berhenti setiap kali butuh keputusan manusia dan menyebut perintah persis yang perlu dijalankan. Status selalu dihitung dari basis data, sehingga laporan tidak pernah menampilkan keadaan lama. Workflow tidak pernah menyetujui peluang atau draf, tidak menerbitkan, dan tidak mengirim apa pun. Bila proses terhenti, `workflow resume` melanjutkan run yang sama tanpa membuat duplikat dan tanpa mengulang biaya yang sudah tercatat.

## 1. Delivered

1. `internal/workflow`: live assessment (stages, gates A–E, counts with "tidak tersedia", blockers, next actions, provenance) and `Advance` for `run`/`resume` with reservation-based idempotency, a compare-and-set lease, a per-command run limit and a monthly-budget precheck.
2. Migration `0004_workflow.sql`: `workflows`, `workflow_content_runs`, `workflow_events` with immutable run links and append-only history.
3. CLI `workflow start|run|resume|show|list|next|report|export`; `content export` and `workflow export` share one safe export helper.
4. Workflow report (Markdown + HTML) rendered from the database.
5. Fixes needed for honest reporting: evidence refs as code spans in the research, strategy and brief Markdown reports; content report counts derived from stored drafts; ID validation for every CLI ID argument; `Options.RunID` in the orchestrator (default behaviour unchanged); `config/workflow.yaml`.

## 2. Commands run with the built binary (isolated `$TMPDIR`, `MIA_WORKSPACE_ROOT=testdata/workspace-mini`)

| Command | Result |
|---|---|
| `workflow start --inputs testdata/sample-batch` | `wf_mv7ywooab498d466ae01` created, status `created` |
| `workflow run --id …` | research `run_mv7ywopjbd9e9de14f5e` and strategy `run_mv7ywoqf07812565269f` → `awaiting_opportunity_review` (4 proposals) |
| `workflow run --id …` again | "Tidak ada tahap otomatis yang aman"; still one research and one strategy run |
| `workflow next --id …` | four `mia approve opportunity --id … --decision …` commands |
| `approve opportunity` (1 approved: `opp_mv7ywoqx1e23cad3954f`; 3 rejected) then `workflow resume --id …` | content run `run_mv7ywovn557c893c3e95`, 10 drafts → `awaiting_content_review` |
| `content review` (5 approved, 5 rejected), `workflow show --id …` | `completed`; 5 ready, 0 exported, 0 open placeholders |
| `workflow export --id …` | 5 DRAF files under `drafts/content/wf_…/`; 5 not exported (not ready) |
| `workflow export --id … --dir ../keluar` | refused, exit 1 |
| `workflow next`, `workflow report`, `workflow list` | "Workflow selesai untuk tahap saat ini" note; report rendered from the database; live status `completed` |
| v0.4.0 database opened by the new binary | migration 0004 applied; 19 drafts and 2 approvals kept; `integrity_check` ok; `foreign_key_check` empty |

## 3. Tests

| Command | Result |
|---|---|
| `GOPROXY=off GOSUMDB=off make test` | all packages pass; module hashes identical before and after |
| `GOPROXY=off GOSUMDB=off make test-race` | pass (includes concurrent advances) |
| `make vet` | clean |
| `make modcheck` | `go.mod/go.sum unchanged vs HEAD` |

New tests cover the 25 required items (mapping in `acceptance-criteria.md` §11): `internal/workflow` (11 tests), `internal/store` (Phase 4 upgrade), `internal/report` (workflow injection, ref escaping), `internal/orchestrator` (content counts after review), `cmd/mia` (end-to-end CLI), and the no-network scan now covering `internal/workflow` and `internal/store`. Changed expectations: two migration counts (3 → 4) for the new migration; research and strategy goldens for the ref-escaping fix only (16 and 7 lines, each verified to be `[ref]` → `` `ref` ``); the content golden is unchanged.

## 4. Idempotency, resume, gates, export — evidence

- **Idempotency:** repeated `run` creates nothing (W-1); run IDs are reserved before runs start and are immutable; concurrent advances create one research and one strategy run (W-9).
- **Resume:** a failed research run is resumed by a fresh process and store handle (W-5); a content run is recovered without duplicate drafts or extra agent calls, including after a simulated crash during evaluation (W-7); after the call cap only an explicit `--new-content-run` creates attempt 2 (W-7); a failed transition transaction leaves no reserved run (W-8).
- **Gates:** rejected and undecided opportunities never reach the Content Agent (W-2); failed research and incomplete strategy stop the pipeline (W-5, W-6); drafts with placeholders are never ready, rejected drafts are never exported (W-3).
- **Export:** ready drafts only, labelled DRAF with provenance; `..`, absolute paths and workspace targets refused; the workspace fixture's tree hash is unchanged after the CLI test (W-13).

## 5. Audit findings

Fixed in Phase 5: unescaped evidence refs in Markdown reports (research, strategy, brief); stale content-report counts. Partially fixed: A12 (ID validation; `.env` permission check open). Still open: A8 (mock price 0 means the default budget gate cannot trigger), A9 (timeouts retried up to three step timeouts), A10 (title-only dedup), A11 (no PII scan on agent output), A13 (env overrides outside the config hash), A15 (binary not tied to a commit), A16 (`ResolveUnder` substring check). A14 has tests; formal closure is left to the next audit. Recommendations are unchanged from `acceptance-criteria.md` §9.

## 6. Files

26 tracked files modified and 15 new files (including this report): the `internal/workflow` package, migration 0004, workflow repository, CLI and report code and templates, tests, `config/workflow.yaml`, and documentation. A stray `mia` binary produced by a local `go build` during development was removed (never tracked).

## 7. Limitations

See `workflow.md` §11. In short: the status names the earliest pending stage; leases assume one machine; content regenerated outside the workflow is not linked automatically; the stored summary refreshes on `run`/`resume`/`report` while `show`/`list` are always live; mock fixtures say nothing about real model quality.

## 8. Confirmations

No commit, no push, no network access, no paid provider, no scheduler/daemon/server, and no writes to `~/Documents/bots/punyamas`. Phase 6 not started. Honest note on the workspace: during this task its HEAD moved from `295f129` to `4f8f04d` and its `dashboard/` folder gained uncommitted edits (commits titled "Dashboard D1 …"), made by another process; MIA's only write paths are its data, report and draft directories, and workspace targets are refused.
