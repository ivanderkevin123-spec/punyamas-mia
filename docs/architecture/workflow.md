# MIA — Workflow orchestration and quality gate (Phase 5)

**Status:** implemented 2026-10-14, mock provider only, uncommitted pending approval. Plan: `docs/implementation/phase5-plan.md`. Code: `internal/workflow`, migration `0004_workflow.sql`, CLI `cmd/mia/workflow_cmd.go`, report `internal/report/workflow.go`.

## 0. Ringkasan (Bahasa Indonesia)

Workflow menyatukan riset → strategi → tinjauan peluang → draf konten → tinjauan draf → ekspor menjadi satu alur yang dapat dijalankan berulang, dilanjutkan setelah terhenti, dan diaudit. Setiap kali dibaca, statusnya dihitung ulang dari basis data; laporan lama tidak pernah dipakai sebagai sumber status. Workflow hanya menjalankan langkah otomatis yang aman (riset mock, strategi mock, draf untuk peluang yang sudah disetujui). Workflow tidak pernah menyetujui peluang atau draf, tidak menerbitkan, dan tidak mengirim apa pun. Bila ada yang memblokir, workflow menyebut alasannya dan perintah konkret berikutnya.

## 1. Model

A workflow is one row in `workflows` that links artefacts that already exist in earlier phases:

```
workflows ──research_run_id──▶ research_runs (kind research)
          ──strategy_run_id──▶ research_runs (kind strategy, parent = research run) ──▶ opportunities ──▶ approval_records
          ──workflow_content_runs (per approved opportunity, per attempt)──▶ research_runs (kind content) ──▶ content_drafts ──▶ approval_records (content_draft)
          ──workflow_events (append-only audit trail)
```

The workflow status is an orchestration summary. Run statuses, opportunity statuses with their approval history, and draft statuses with their review history stay authoritative; the workflow never writes to them except through the existing orchestrator calls.

## 2. Status and precedence

`Assess` derives everything from the database. Precedence (first match wins):

| # | Condition | Status |
|---|---|---|
| 1 | a research, strategy or content run failed or was interrupted (stale heartbeat, partial) | `failed` |
| 2 | gate A or B blocks (no validated evidence, unvalidated opportunity evidence) or a run stopped on the budget | `blocked` |
| 3 | research not finished (no run, reserved, or running) | `created` |
| 4 | strategy not finished | `research_completed` |
| 5 | an approved, eligible opportunity has no content run yet (or one is running) | `strategy_completed` |
| 6 | content runs stopped on the budget | `blocked` |
| 7 | at least one reviewable opportunity is undecided (`proposed`) | `awaiting_opportunity_review` |
| 8 | drafts in `draft`/`needs_review`, or approved but not ready | `awaiting_content_review` |
| 9 | at least one draft ready for handoff and nothing above pending | `completed` |
| 10 | otherwise (no reviewable opportunity, none approved, approved ones ineligible, no ready draft) | `blocked` with the reason |

When both opportunity review and draft review are pending, the earlier stage wins (status 7); `workflow show` and `next` list every pending human decision either way. `completed` never requires an export; exporting ready drafts is listed as the next action and recorded when done.

Stage views: `pending`, `running`, `interrupted`, `completed`, `failed`, `blocked`, `awaiting_human`, `not_reached`. Counts are `nil` ("tidak tersedia") until the stage that produces them has data. Placeholder and claim counts cover open drafts only (`draft`, `needs_review`, `approved`).

## 3. Quality gates (reusing earlier validators)

| Gate | Check | Source |
|---|---|---|
| A — research | run kind `research`, status `completed`/`completed_with_warnings`, ≥ 1 validated item (`Orchestrator.CountsFromStore`) | Phase 2 |
| B — strategy | run kind `strategy`, completed, parent = workflow research run; every opportunity evidence link is a validated item of that run; zero reviewable (tier ≠ none) opportunities reported as blocked | Phase 3 |
| C — opportunity approval | opportunity status and latest `approval_records` decision read from the database; `proposed`, `blocked`, `rejected`, `rejected_by_review`, `needs_more_evidence` never reach the Content Agent | Phase 3 |
| D — content generation | `content.Load` eligibility (approved, history agrees, completed strategy run, validated evidence) before every content run; Content Agent and `content_drafts.v1` unchanged; drafts never auto-approved; Phase 4 duplicate handling | Phase 4 |
| E — review and handoff | `content.ReadinessOf`: only approved drafts for the current version and hash, with zero placeholders, claims to verify and findings, are ready; export only those, through the existing safe export | Phase 4 |

Gate reasons are concrete (which run, which opportunity, which item, which placeholder), never a generic "validation failed".

## 4. Commands and steps

`workflow run` executes, under the lease, the next safe automatic steps in a loop: research (mock), strategy (mock), content for each approved eligible opportunity. It never resumes failed runs. `workflow resume` additionally resumes failed or interrupted runs (each at most once per command) through the orchestrator's own resume (completed steps are skipped, call caps and the ledger are respected) and takes over expired leases. Both stop at `max_runs_per_command` runs and before any new run when the monthly ceiling is already used (`budget_stop` event). `show`, `next` and `list` are read-only live assessments; `report` re-renders the report from the database and stores the summary when no other process holds the lease.

## 5. Idempotency and resume

- **Reservation:** before a run starts, its ID is reserved in the workflow row (`research_run_id`, `strategy_run_id`) or in `workflow_content_runs`, together with an event, in one transaction. The orchestrator then creates the run with that ID (`Options.RunID`). After a crash: reserved and missing → started with the same ID; existing and unfinished → resumed (only by `workflow resume`); finished → never recreated. Run links are immutable (trigger), so artefact IDs never change.
- **Adoption:** if an approved opportunity already has drafts made outside the workflow, its latest content run is linked (`origin = adopted`) instead of generating again.
- **Explicit new attempt:** when a content run has used its call cap without finishing its agent step, the workflow does not retry; `next` shows `mia workflow resume --id WF --new-content-run OPP`, which reserves attempt n+1 and runs with `Regenerate` (Phase 4 dedup prevents duplicate drafts).
- **No false progress:** stage completion is read from run statuses; a failed reservation transaction leaves no reserved ID and no run; the summary and its `status_changed` event are written in one transaction.
- **Timeouts:** a timed-out call is never assumed not to have happened; the orchestrator books it as an execution and an uncertain cost, and resume re-reads both.

## 6. Concurrency

`workflows.lease_owner` / `lease_expires_at` form a lease acquired with a compare-and-set `UPDATE … WHERE lease free or expired`. A second process gets `ErrLeaseHeld` ("sedang dimajukan oleh proses lain") and changes nothing; `show` reports the foreign lease. Leases expire after `lease_seconds`; `workflow resume` then takes over. SQLite's single writer and the unique constraints on run links back this up.

## 7. Storage — migration `0004_workflow.sql`

`workflows` (status CHECK with the eight values, run links UNIQUE, CHECK strategy requires research, immutable links trigger, no delete), `workflow_content_runs` (PK workflow + opportunity + attempt, content run UNIQUE, origin `workflow|adopted`, append-only), `workflow_events` (PK workflow + seq, append-only). Reserved run IDs carry no foreign key because the run row is created after the reservation; the assessor reports a reserved run with the wrong kind or parent as an integrity failure. The migration upgrades Phase 4 databases in place (tested with Phase 4 data and with a real v0.4.0 database).

## 8. Configuration — `config/workflow.yaml`

| Key | Default | Meaning |
|---|---|---|
| `max_runs_per_command` | 4 | research, strategy and content runs started or resumed by one command (1–20) |
| `lease_seconds` | 900 | lease lifetime before another `resume` may take over (30–86400) |

The file is part of the configuration hash. Agent call caps, step timeouts and budget ceilings stay in `agent-limits.yaml` and `content.yaml`.

## 9. Reports

`reports/<workflow-id>/workflow.md|html`: status, reason, timestamps, stage table with run IDs, validated-evidence summary, opportunities with evidence IDs, latest decisions and content runs, drafts with status, readiness, open placeholders and claims, evidence IDs and export state, blockers, next actions (marked as recommendations), ledger cost, provenance (config hashes per run, scoring and content config hashes, fixture keys and hashes), and the event history. Markdown uses the shared escaping helpers; HTML uses `html/template`.

Also in Phase 5: evidence refs in the research, strategy and brief Markdown templates are rendered as code spans (they were in brackets, so characters from import file names could form links), and the content report derives its status counts from stored drafts, so `mia report <content-run>` is current after reviews.

## 10. Security

No network, no new provider, no server or daemon. Workflow, run, opportunity and draft IDs from the CLI are validated against `ids.Valid` before any lookup or path use. Reports are written under `reports/<validated-id>/`. Exports use `safepath.ResolveDraftPath` (no absolute paths, no `..`, no symlink escape, never inside the workspace root) and atomic writes; `workflow export` writes under `drafts/content/<workflow-id>/` or a validated relative `--dir`. Terminal output strips control characters.

## 11. Known limitations

1. The status is a summary; when opportunity and draft reviews are both pending only the earlier stage is named in the status (all actions are listed).
2. Leases rely on clocks of processes on one machine; there is no distributed locking.
3. A content run is linked per opportunity; regenerating content deliberately outside the workflow (`run content --regenerate`) is not linked automatically.
4. The stored summary in `workflows` is refreshed by `run`, `resume` and `report`; `show`/`list` always compute live and show both.
5. Mock fixtures only; nothing here measures the quality of a real model.
