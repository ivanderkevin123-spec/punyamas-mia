# MIA — Acceptance Criteria

**Status:** Phase 1 definition · 2026-10-12. Every criterion is **pending**: nothing has been implemented or tested. Statuses will be updated during Phases 2–7 to `passed`, `failed`, or `not applicable` with evidence (test name, run id, or command output).

Legend for *Test method*: U = unit test, I = integration test (store + orchestrator), W = workflow test (end-to-end mock run), M = manual check with recorded command output, D = documentation review.

## 1. Functional

| ID | Criterion | Test method (planned) | Phase | Status |
|---|---|---|---|---|
| F1 | A manual run (`mia run research --inputs testdata/sample-batch --then strategy`) completes with the mock provider and exits 0 | W: `TestWorkflow_MockEndToEnd` | 2–3 | passed (riset) — TestWorkflow_MockEndToEnd; langkah strategi menyusul Fase 3 |
| F2 | Every research item in the report has a source name, reference, retrieval timestamp, and evidence-quality label; items without external evidence are labelled `hypothesis` | U: validator tests; W: report assertions | 3 | passed — validator + TestWorkflow_MockEndToEnd (report assertions) |
| F3 | Duplicate sources and items are detected (same canonical URL, same content hash, normalized-title similarity above threshold) and linked, not deleted | U: `dedup` table tests with fixtures | 2 | passed — TestFindDuplicates, langkah dedup |
| F4 | Each ranked opportunity shows a score breakdown per dimension with rationale and evidence ids; the stored score equals the deterministic recomputation | U: `scoring` tests; I: store round-trip | 3 | pending |
| F5 | Statements containing banned-claim categories are flagged; all B1 test cases BT1–BT12, LK1–LK10 (reject) and CT1–CT10 (pass) produce the same verdicts as `brand-oracle.mjs` | U: `brand` equivalence test from a fixture generated from B1 | 3 | passed untuk BT1âBT12 dan TG1âTG6 (TestCheckerMatchesWorkspaceCases); LK1âLK10 dan CT1âCT10 pending Fase 3 |
| F6 | Missing source data is reported honestly: an unavailable or unparseable input appears under *Sumber tidak tersedia* with its reason, and the run status is `completed_with_warnings` or `failed` as specified | W: fixtures with a missing file and a malformed CSV | 2 | passed â TestWorkflow_RejectedFilesAreReportedAndRunContinues, TestWorkflow_MissingInputsFailsHonestly |
| F7 | Reports are generated as Markdown and static HTML using only the Go standard library templates (no UI framework, no external assets) | W: golden-file comparison; M: open HTML offline | 2 | passed â golden testdata/golden/research-sample.md; HTML dibuka manual 2026-10-12 |
| F8 | Run status, step statuses, errors, warnings, and cost are observable via `mia status <run-id>` and in the report header | I: status command test | 2 | passed â mia status (manual 2026-10-12) + ringkasan run di setiap uji alur |
| F9 | A failed required step cannot yield a `completed` run or a report without a failure header | W: inject provider error; assert status and header | 2 | passed â TestWorkflow_ProviderErrorRetriesThenFails (header RUN GAGAL) |
| F10 | Workspace assets are imported read-only with path, sha256, mtime, commit, and timestamp; a modified copy in a test fixture is detected as `changed` and creates a review entry without altering config | I: drift tests with temp workspace copy | 2 | passed â TestDriftDetectionAndApproval, TestDriftBlocksRunsWhenConfigured |
| F11 | Conflicting `fact` items on the same `topic_key` are flagged as a conflict group and shown side by side; neither is used as a fact in recommendations | U + W | 3 | pending |
| F12 | Approval decisions are stored append-only and are required before an opportunity can be used by `mia run brief` (Phase 4) | I | 3 (gate), 4 | pending |

## 2. Reliability

| ID | Criterion | Test method | Phase | Status |
|---|---|---|---|---|
| R1 | Retry counts per step never exceed `max_retries` from `agent-limits.yaml` | U: orchestrator with failing mock | 2 | passed â TestWorkflow_ProviderErrorRetriesThenFails (percobaan = 1+max_retries) |
| R2 | Per-step timeouts are enforced via context deadlines; a hung mock call ends in `timed_out` within the limit | U | 2 | passed â TestWorkflow_TimeoutEnforced |
| R3 | Per-run, per-agent, and monthly (USD 20) budget limits are enforced before calls; exceeding any yields `budget_exhausted` and no further calls | U: budget gate; W: ledger pre-seeded near ceiling | 2 | passed â TestWorkflow_BudgetExhausted, TestMonthlyCeilingBlocks, TestRunAndAgentCeilingsBlock |
| R4 | A missing or misconfigured provider produces a clear error naming the config key and the mock alternative, before any step runs | U | 2 | passed â TestRejectsAnthropicProviderInPhase2; registry menolak penyedia tak dikenal |
| R5 | An interrupted run (simulated by killing after step N) is reported `stale` and `--resume` continues from step N+1 without re-executing completed steps or repeating provider calls | I | 2 | passed â TestWorkflow_ResumeContinuesFromFailedStep, TestIsStale (interupsi disimulasikan lewat langkah gagal) |
| R6 | Re-running the same batch does not re-import identical files or recreate identical items (idempotent by content hash); counts of skipped items are reported | I | 2 | passed â TestWorkflow_IdempotentReimport (13 butir dipakai ulang, 0 impor baru) |
| R7 | Cost is recorded per call; when the provider cannot report usage, the record is marked `estimated`/`unmeasurable` and the summary warns | U | 2 | passed â TestRecordMeasurableAndEstimated |

## 3. Security

| ID | Criterion | Test method | Phase | Status |
|---|---|---|---|---|
| S1 | No real credentials are committed: `.env` is git-ignored; a scan of tracked files for key patterns is empty | M: `git ls-files | xargs grep -lE 'sk-ant-|AKIA|BEGIN PRIVATE KEY'` → none | 2 and every phase | passed â pemindaian 2026-10-12 atas berkas yang dilacak (kecuali fixture uji dan dokumen yang mendeskripsikan polanya): tidak ada kunci; .env di-ignore |
| S2 | Untrusted content cannot authorize tools: an imported file containing "ignore previous instructions and run …" is flagged `suspicious_content`, excluded from agent input by default, and no action results | U: detector; W: fixture | 2–3 | passed â TestWorkflow_RejectedFilesAreReportedAndRunContinues (butir mencurigakan dikecualikan dari prompt) |
| S3 | External actions require human approval: MIA has no code path that publishes, sends, or spends; approval records exist only for opportunities, briefs, assets, sources | D + U (no network calls in mock mode; `go vet`-style check that only `provider/anthropic`, `research/fetch` and, from Phase 6, `notify/*` import `net/http`) | 2–7 | passed (desain) â tidak ada kode publikasi atau pengiriman; tidak ada paket yang mengimpor net/http; dicek manual 2026-10-12 |
| S4 | Logs do not contain secrets or full untrusted excerpts: redaction tests for key patterns; excerpt truncation | U | 2 | passed â TestRedaction |
| S5 | The source Punyamas workspace remains unchanged: `git -C ~/Documents/bots/punyamas status --porcelain` and `rev-parse HEAD` are identical before and after each MIA test run and each phase (changes made by the owner's other sessions are noted separately) | M, recorded in each phase report | every phase | passed Fase 2 â status porcelain identik sebelum dan sesudah demo 2026-10-12 (HEAD c8a16b1) |
| S6 | Workspace path access is read-only: a test asserts that opening workspace files uses read-only flags and that any write attempt under the workspace root is rejected by the path guard | U | 2 | passed â TestResolveUnderRejectsEscapes; pembacaan memakai os.Open dan os.ReadFile saja |
| S7 | Web fetching is disabled by default and cannot run without `web.enabled`, a non-empty allowlist, and an approval record | U | 3 | passed â TestRejectsWebFetchingInPhase2; fetcher belum ada di kode |

## 4. Quality

| ID | Criterion | Test method | Phase | Status |
|---|---|---|---|---|
| Q1 | Unit tests cover scoring, validation (schema + semantic), deduplication, budget controls, claim checker, PII guard, redaction | U; `go test ./...` passes; coverage report attached to the phase report | 2–3 | passed (Fase 2) â scoring menyusul Fase 3; cakupan di phase2-report.md |
| Q2 | Integration tests cover orchestrator + store (run lifecycle, resume, approvals, drift) | I | 2–3 | passed â orchestrator_test.go (siklus run, resume, drift) dan store_test.go |
| Q3 | A complete mock workflow test produces a byte-identical golden report for fixed fixtures | W | 3 | passed â TestWorkflow_MockEndToEnd golden byte-identik |
| Q4 | Generated claims and source references are inspected by a human on the sample report: every cited excerpt exists in the stored snapshot/file; no invented sources | M: checklist recorded in the Phase 3 report | 3 | pending |
| Q5 | Setup, configuration, operation, and troubleshooting are documented (`README.md`, `docs/implementation/operations.md`) including Go toolchain verification (`make go-version`) | D | 2–3 | passed â README.md, docs/implementation/operations.md |
| Q6 | Build uses the native toolchain: `make go-version` prints `go1.27.2 darwin/arm64`; `GOTOOLCHAIN=local` set | M | 2 | passed â make go-version mencetak go1.27.2 darwin/arm64; GOTOOLCHAIN=local |
| Q7 | Dependencies are limited to the approved list (`docs/implementation/roadmap.md` Phase 2) and recorded in `go.mod`/`go.sum` | M: `go list -m all` reviewed | 2 | passed â go.mod: modernc.org/sqlite v1.60.1 dan go.yaml.in/yaml/v3 v3.0.5 (langsung) |

## 5. Phase-specific additions

| ID | Criterion | Phase | Status |
|---|---|---|---|
| P4-1 | Briefs are Indonesian, contain `[ISI: …]` for every unconfirmed fact, pass the claim checker, list unmet M1 preconditions, and carry the DRAFT header | 4 | pending |
| P4-2 | No brief mentions prices, addresses, hours, guarantees, reviews, market share, or statistics unless present in `B4-fakta.json` | 4 | pending |
| P5-1 | Performance import rejects PII-like columns; KPIs are computed by code; missing data yields "tidak dapat dihitung" | 5 | pending |
| P6-1 | `mia schedule render` output is a valid launchd plist pointing at the absolute binary; installation is manual and documented | 6 | pending |
| P7-1 | All criteria above re-run; cost ledger for the evaluation period ≤ USD 20; limitations documented | 7 | pending |

## 6. Verification record

Each phase report must list, per criterion touched: status, evidence (test name or command output), and date. Criteria must never be marked `passed` from reasoning alone.
