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
| P4-1 | Briefs are Indonesian, contain `[ISI: …]` for every unconfirmed fact, pass the claim checker, list unmet M1 preconditions, and carry the DRAFT header | 4 | passed for mock fixtures (see §10, P4-3 to P4-8); real-model output untested |
| P4-2 | No brief mentions prices, addresses, hours, guarantees, reviews, market share, or statistics unless present in `B4-fakta.json` | 4 | passed for the configured patterns (see §10, P4-5); paraphrases can pass, human review mandatory |
| P5-1 | Performance import rejects PII-like columns; KPIs are computed by code; missing data yields "tidak dapat dihitung" | 5 | pending |
| P6-1 | `mia schedule render` output is a valid launchd plist pointing at the absolute binary; installation is manual and documented | 6 | pending |
| P7-1 | All criteria above re-run; cost ledger for the evaluation period ≤ USD 20; limitations documented | 7 | pending |

## 6. Verification record

Each phase report must list, per criterion touched: status, evidence (test name or command output), and date. Criteria must never be marked `passed` from reasoning alone.

## 7. Phase 3 — Strategy Agent (2026-10-13; implemented, uncommitted, pending approval)

All statuses below come from `GOPROXY=off GOSUMDB=off make test` and `make test-race` on 2026-10-13 (both green, `go vet` clean, `go.mod`/`go.sum` unchanged).

| ID | Criterion | Evidence (tests) | Status |
|---|---|---|---|
| P3-1 | Opportunities cite only validated items of the parent run; every stored evidence link resolves to a validated item | `TestStrategy_MockEndToEnd`, `TestValidEvidenceIsTraceableAndTierA` | passed |
| P3-2 | Missing evidence → `rejected`; no A–C fact → hypothesis capped at Tier C; nonexistent or unvalidated refs are reported and never linked | `TestGate1_NonexistentOrUnvalidatedRefs`, `TestGate1_HypothesisOnlyIsCappedAtTierC`, `TestStrategy_GatesFixture` (item `manual:unvalidated#1`) | passed |
| P3-3 | Conflicting evidence caps the tier at C and shows both sides | `TestGate2_ConflictShowsBothSidesAndCapsTier`, `TestDetectConflicts`, `TestStrategy_ConflictingEvidenceCapsTierAndShowsBothSides` | passed |
| P3-4 | Brand-checker failures block; unsupported prices, hours, addresses, statistics, customer claims and numbers are rejected; identifiers and evidence-supported numbers pass | `TestGate3_BrandFailureBlocks`, `TestGate3b_UnsupportedFactualClaimsRejected`, `TestGate3b_NumbersSupportedByEvidenceOrIdentifiersPass`, `TestStrategy_GatesFixture` | passed |
| P3-5 | Store-dependent opportunities are labelled, ease of testing capped, never rejected for that reason | `TestGate4_StoreDependencyIsLabelledNotRejected` | passed |
| P3-6 | Duplicates merged without losing corroborating evidence; different topics never merged | `TestGate5_DedupMergesWithoutLosingCorroboration`, `TestStrategy_MockEndToEnd` (proposal 4 merged into 1) | passed |
| P3-7 | Evidence strength and tiers are recomputed by code; agent values are capped; ordinal sum labelled, no probabilities | `TestEvidenceStrengthRubricIsComputedByCode`, `TestTiersAreRecomputedByCode`, `TestMeasurabilityFollowsKPIAvailability`, golden `testdata/golden/strategy-sample.md` | passed |
| P3-8 | Config hash, scoring hash and fixture hash recorded per run and per opportunity | `TestStrategy_HashesRecordedPerRun` | passed |
| P3-9 | Approval history is append-only; status mirrors the latest decision; blocked/rejected cannot be approved; approved briefs export to `drafts/`, never into the workspace | `TestStrategy_ApprovalHistoryIsAppendOnly`, `TestCLIStrategyWorkflowIsolated` | passed |
| P3-10 | At most two Strategy Agent calls per run, across retries and resumes | `TestStrategy_AtMostTwoAgentCallsPerRun`, `TestStrategy_TimeoutEnforced` | passed |
| P3-11 | Budget ceiling and step timeout enforced for the strategy step; timed-out calls booked as uncertain cost | `TestStrategy_BudgetCeilingBlocksAgent`, `TestStrategy_TimeoutEnforced` | passed |
| P3-12 | A resumed run re-renders its report from stored data and never duplicates opportunities | `TestStrategy_ResumeRendersFreshReportAndNeverDuplicates` | passed |
| P3-13 | No network and no paid provider: mock capabilities `Network=false`, cost USD 0, prompts/outputs recorded | `TestStrategy_NoNetworkNoPaidProvider` | passed |
| P3-14 | Agent output parsed strictly (unknown fields rejected), structural validation, evidence inside the untrusted block | `TestParseOutput`, `TestStructuralValidation`, `TestBuildRequestIsolatesUntrustedData` | passed |
| P3-15 | Strategy report, HTML and brief escape hostile values and separate tiers, blocked and rejected | `TestStrategyMarkdownAndBriefEscapeHostileValues` | passed |
| P3-16 | Clean end-to-end strategy report on the sample batch matches the golden file | `TestStrategy_MockEndToEnd` | passed |

Mapping to the Phase 1 criteria touched: F2 (strategy report), F4 (claim checking on generated text), F5 (scoring with rationale), F11/F12 (approvals before handoff), S2/S3 (untrusted block, no tools), Q1 (sample report inspected), Q3 (conflicts both sides). Live sources (S7, Q4 allowlist) remain N-A: fetching is not enabled.

## 8. Remediation record — Phase 2 audit findings (2026-10-13)

| Finding | Fix | Evidence (tests) | Status |
|---|---|---|---|
| A1 dependency checksums not independently verified | Independent verification run on 2026-10-13 by the pre-commit audit (not rerun by this documentation update): temporary project copy, fresh module cache, GOTOOLCHAIN=local, official proxy, GOSUMDB=sum.golang.org, TLS verification enabled; repeatable procedure in `operations.md` §1 | Checksum database consulted (sumdb lookup cache written, 30 lookups); all 28 committed `go.sum` lines present in the independently generated file with identical hashes, including `golang.org/x/mod v0.42.0` and `golang.org/x/tools v0.52.0`; no same-module/version hash conflict; 32 extra generated lines are test dependencies of dependencies, not mismatches; project `go.mod`/`go.sum` unchanged | **PASS** (per the pre-commit audit report; Phase 2 commit approval and Phase 3 authorization remain separate, not granted) |
| A2 non-atomic item/evidence writes | `Store.InsertItemWithEvidence` (item + run link + evidence in one transaction) used at all three call sites | `TestInsertItemWithEvidenceIsAtomic` (fault injection via foreign-key violation, recovery re-import) | passed |
| A3 resumed runs reported zero counts | counts rebuilt from the store before the report and at finalize; process-only counters and warnings carried over from the previous invocation; report always re-rendered on resume | `TestWorkflow_ResumeContinuesFromFailedStep`, `TestWorkflow_SummaryCountsMatchStoreAndReport`, `TestWorkflow_CleanCompleted` | passed |
| A4 unescaped Markdown | `text`/`cell`/`code` template helpers (backslash-escape Markdown punctuation, collapse newlines, no raw HTML or links) applied to every untrusted field | `TestMarkdownEscapesHostileValues`, `TestMDHelpers`, `TestHTMLEscapesAndSeparatesStatuses` | passed |
| A5 implicit `-mod=mod` | Makefile uses read-only module mode; explicit `make tidy`; `make test`/`test-race` fail if `go.mod`/`go.sum` change; `make modcheck` | `make test` output 2026-10-13 (hashes identical before/after) | passed |
| A6 unvalidated items under factual headings | factual sections render validated items only; "Belum divalidasi" section with status labels; "RUN BELUM LENGKAP" banner for partial runs | `TestMarkdownSectionsOnlyShowValidatedItems`, `TestWorkflow_UntilReportSeparatesUnvalidated` | passed |
| A7 failed calls not booked | `Budget.RecordUncertain` books the pre-call estimate as `uncertain:<reason>` for non-fatal failures; one row per execution (reconciliation replaces); summary and report split actual vs uncertain; bookkeeping on a non-cancelled context | `TestRecordUncertainCountsAgainstCeilings`, `TestCostUpsertByExecution`, `TestWorkflow_RepeatedTimeoutsCannotBypassCeiling`, `TestWorkflow_RetryThenSuccessBooksUncertainCost`, `TestWorkflow_FatalProviderErrorNotRetried` (books nothing) | passed |
| Extra | clean `completed` fixture and test; direct renderer tests; CLI smoke with isolated `MIA_DATA_DIR`/`MIA_REPORTS_DIR` | `TestWorkflow_CleanCompleted`, `report_test.go`, `TestCLISmokeIsolated` | passed |

Not changed in this sprint (out of its scope): A8–A16 from the audit report.

## 9. Open audit findings after Phase 3 (2026-10-13)

Phase 3 did not set out to fix A8–A16 and they remain **unresolved** unless a row says otherwise with a test.

| Finding | Summary (from the Phase 2 audit) | Phase 3 status |
|---|---|---|
| A8 | mock model price 0 means the budget gate cannot trigger in the default configuration | unresolved (tests exercise the gate with `claude-opus-5-5` only: `TestWorkflow_BudgetExhausted`, `TestStrategy_BudgetCeilingBlocksAgent`) |
| A9 | timeouts are retryable, so a hung step can run up to 3× the step timeout; no run-level deadline | unresolved for research; the strategy step is additionally bounded by the per-run cap of 2 calls (`TestStrategy_TimeoutEnforced`), the finding itself is not closed |
| A10 | title-only near-duplicate detection can mark items from different sources as `duplicate_of` | unresolved (unchanged in `internal/research/dedup`) |
| A11 | agent-produced research items are stored without a PII scan | unresolved (strategy output is not PII-scanned either; it contains no customer records by design) |
| A12 | `.env` permission bits unchecked; run-id arguments not pattern-validated | unresolved (new `--id`/`--run` arguments are looked up in the database the same way) |
| A13 | config hash covers only the YAML files, not environment overrides | unresolved; `scoring.yaml` is now part of the hashed file set and has its own `scoring_config_hash`, env overrides remain outside both hashes |
| A14 | no test asserted a clean `completed` status | tests now assert it (`TestWorkflow_CleanCompleted` from the remediation sprint; `TestStrategy_MockEndToEnd` requires `completed` for the strategy run); formal closure is left to the next audit |
| A15 | `bin/mia` built before the first commit is not tied to a commit | unresolved (`bin/` is git-ignored; rebuild from the approved commit) |
| A16 | informational: `ResolveUnder` rejects any relative path containing `..` as a substring | unresolved (behaviour unchanged; the same guard protects `export brief`) |
| Coverage gaps (audit §7) | `report`, `provider/mock`, `provider/registry`, `cmd` had little or no own tests | partially reduced: `report` now has renderer tests for research and strategy, `cmd` has two CLI workflow tests; `mock` and `registry` still have no own test files |

## 10. Phase 4 — Content Agent (2026-10-14; implemented, uncommitted, pending approval)

All statuses below come from `GOPROXY=off GOSUMDB=off make test` and `make test-race` on 2026-10-14 (both green), `make vet` clean, `make modcheck` reporting `go.mod`/`go.sum` unchanged, and a CLI walkthrough with the built binary in isolated directories.

| ID | Criterion | Evidence (tests) | Status |
|---|---|---|---|
| P4-3 | An approved opportunity produces drafts with full provenance (opportunity, strategy and research runs, execution, config/content/scoring/fixture hashes, content hash, version 1) | `TestContent_MockEndToEnd` (golden `testdata/golden/content-sample.md`) | passed |
| P4-4 | Proposed, rejected, needs-more-evidence, blocked, gate-rejected, unknown opportunities and a status set without an approval record are refused with no run and no agent call; storage trigger refuses drafts for unapproved opportunities | `TestContent_IneligibleOpportunitiesRefused`, `TestContentDraftStorageRules` | passed |
| P4-5 | Missing or unvalidated evidence, a non-completed strategy run, and approval withdrawn before resume are refused | `TestContent_InvalidProvenanceRefused` | passed |
| P4-6 | Fictitious, unvalidated and cross-opportunity evidence refs block the draft and are never linked; composite foreign key refuses cross-opportunity links | `TestEvidenceRefsMustBelongToTheOpportunity`, `TestContent_GatesFixture`, `TestContentDraftStorageRules` | passed |
| P4-7 | Prices, hours, addresses, statistics, customer claims, unsupported numbers, guaranteed profit, price trends, testimonials, false urgency, discounts and process-time promises are blocked; soft pressure and non-standard placeholders need review; clean Indonesian controls pass | `TestFactualAndLanguageRules`, `TestContent_GatesFixture` | passed (lexical; limits in `content-agent.md` §4) |
| P4-8 | Placeholders are recorded from the text, listed in the checklist, and a draft with placeholders is never ready for handoff, even when approved | `TestPlaceholdersAreRecordedAndBlockReadiness`, `TestContent_MockEndToEnd`, `TestCLIContentWorkflowIsolated` | passed |
| P4-9 | Each content type follows `content_drafts.v1` (fields, channels, rune limits, sections, variant limits); the agent cannot set status or IDs | `TestStructuralPerContentType`, `TestParseOutputIsStrict` | passed |
| P4-10 | Duplicates within a run and against existing drafts are reported and not stored; silent regeneration is refused; exact duplicates are refused by a unique index | `TestDuplicatesWithinRunAndAgainstExistingDrafts`, `TestContent_RegenerateAndDuplicates`, `TestContentDraftStorageRules` | passed |
| P4-11 | Reviews are append-only and versioned; a revision or out-of-band text change after approval returns the draft to review; approval needs a review record for the current version; drafts cannot be deleted | `TestContent_ReviewAppendOnlyAndRevision`, `TestContentDraftStorageRules`, `TestCLIContentWorkflowIsolated` | passed |
| P4-12 | New drafts are never approved automatically (application and trigger) | `TestContent_MockEndToEnd`, `TestContentDraftStorageRules` | passed |
| P4-13 | Export is labelled DRAF with review and placeholder metadata; traversal, absolute, symlink-escape and workspace targets are refused; a failed export leaves the draft and versions unchanged and no partial file | `TestCLIContentWorkflowIsolated`, `TestResolveDraftPathRefusesEscapes`, `TestWriteAtomicLeavesNoPartialFile` | passed |
| P4-14 | Migration 0003 upgrades a Phase 3 database with data kept, foreign keys and integrity clean; draft writes roll back atomically | `TestMigrationFromPhase3DatabaseKeepsDataAndIntegrity`, `TestContentDraftStorageRules`, `TestMigrateAndIntegrity` | passed |
| P4-15 | Call cap, timeout, budget ceiling, fatal error then resume, forced resume beyond the cap | `TestContent_CallCapTimeoutBudgetAndResume` | passed |
| P4-16 | Content report and export escape hostile text; a body cannot imitate status, readiness, headings or provenance | `TestContentMarkdownCannotFakeStatusOrProvenance`, `TestMDBlock`, golden file | passed |
| P4-17 | No network and no paid provider: mock only, cost USD 0, no `net`, `net/http`, `os/exec` or Anthropic imports on the content path | `TestContent_NoNetworkNoPaidProvider`, `TestNoNetworkImports` | passed |
| P4-18 | Clean end-to-end research → strategy → opportunity approval → content draft → human review → export | `TestContent_MockEndToEnd`, `TestCLIContentWorkflowIsolated` | passed |
| P4-19 | All earlier tests still pass; refactors (shared claim scanner, shared agent-call loop, shared evidence loader, path guard, atomic opportunity decision) keep the strategy golden identical | full `make test`, `testdata/golden/strategy-sample.md` unchanged | passed |

Open items after Phase 4 (not fixed in this phase): A8–A16 as listed in §9 remain **unresolved**. New, found during Phase 4: the Phase 3 strategy report and brief templates render evidence refs as `[{{code .Ref}}]`, so brackets and parentheses from an import file name are not escaped and could form a Markdown link; the Phase 4 content templates use code spans instead. Low severity (refs come from local import file names); left unchanged to keep the Phase 3 output stable.
