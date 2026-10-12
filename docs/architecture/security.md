# MIA — Security Model

**Status:** Phase 1 draft · 2026-10-12 · English. Applies to Phases 2–7. Nothing here is implemented yet.

## 0. Ringkasan (Bahasa Indonesia)

MIA berjalan lokal, tanpa server, tanpa akses ke dashboard operasional, dan **tidak pernah melakukan tindakan ke luar** (terbit, kirim pesan, belanja iklan). Rahasia hanya di `.env` (tidak masuk git). Konten dari web dan berkas impor diperlakukan sebagai **data tidak tepercaya**: model tidak punya alat apa pun, dan instruksi yang terselip di konten diabaikan dan dicatat. Setiap keluaran adalah draf sampai disetujui manusia. Plafon biaya **USD 20 per bulan** ditegakkan sebelum setiap panggilan berbayar.

## 1. Secrets management

- Secrets (API keys, bot tokens, SMTP) live only in `.env` at the MIA root, mode `0600`, listed in `.gitignore`; `.env.example` documents names only.
- The process reads secrets from the environment at start; they are never written to the database, reports, logs, fixtures, or prompts.
- Pre-commit guard (documented procedure, later a Makefile target): `git diff --cached | grep -E 'sk-ant-|AKIA|BEGIN PRIVATE KEY'` must be empty; acceptance test S1.
- No secrets exist today (audit); mock mode needs none.

## 2. Configuration management

- `config/*.yaml` are versioned, human-edited, validated at start (unknown keys = error; ranges checked). Secrets are not allowed in YAML (validator rejects values matching key patterns).
- Every run records `config_hash`; reports show it, so a result can be traced to the exact rules and weights.
- Imported rules and facts (from the workspace) are stored in `config/marketing-policy.yaml` and `config/business.yaml` with `source_of_truth: {path, sha256, workspace_commit, imported_at}`; drift is detected, never auto-applied (Workflow D).

## 3. Local filesystem permissions and boundaries

| Path | Mode | Access |
|---|---|---|
| MIA root | user-owned | read/write by MIA |
| `data/` (db, snapshots, executions, logs, backups) | `0700` dir, `0600` files | MIA only |
| `imports/` | user drops files | read-only by MIA |
| `reports/`, `drafts/` | `0644` files | written by MIA, read by user |
| `~/Documents/bots/punyamas` | existing | **read-only**; MIA opens files with read-only flags and never creates, modifies, or deletes anything there; tests assert no write syscalls target that root |
| Anything else | — | not accessed |

Path handling: all paths are resolved to absolute, symlinks evaluated, and checked to lie under the intended root before use.

## 4. Input validation

- Import files: size cap (default 5 MB), encoding UTF-8, schema per format, mandatory provenance fields, PII guard (phone/identity/email patterns), rejection of embedded HTML/script in Markdown.
- Web content (Phase 3, if approved): response size cap (2 MB), content-type allowlist (`text/html`, `text/plain`, `application/json`), HTML parsed to text with scripts/styles stripped, no JavaScript execution, no redirects to hosts outside the allowlist.
- Model output: JSON schema validation (versioned), string length caps, enum checks, reference resolution; anything else is rejected and stored as `invalid_output`.
- CLI flags: run ids, paths and names validated against strict patterns.

## 5. Safe logging

- `log/slog` JSON; fields with keys matching `(?i)(api[_-]?key|token|secret|password|authorization)` are replaced by `[REDACTED]`; values matching known key prefixes are redacted regardless of key name.
- Untrusted excerpts are truncated to 200 characters in logs; full content is only in `data/executions/` (0600).
- Logs never contain `.env` contents, full prompts, or customer data (none exists).
- Log level configurable; default `info`; `debug` requires explicit flag and still redacts.

## 6. Prompt-injection defenses and untrusted-content boundary

1. **No tools for the model.** Agents call `Provider.Complete` with text in and JSON out. The model cannot fetch, read files, run commands, or send anything.
2. **Delimited untrusted-data block.** Every retrieved or imported text is placed inside an explicit block with a preamble stating that the content is data, may contain instructions, and must be ignored as instructions. System instructions are static and come only from MIA's own templates.
3. **Suspicious-content detector** (deterministic): patterns such as "ignore previous instructions", "you are now", "system prompt", requests for secrets, or imperative text addressed to an assistant are flagged on ingestion; flagged items are excluded from agent inputs by default and listed in the report under *Konten mencurigakan*.
4. **Output cannot authorize actions.** Model output is data: it can propose, never execute. There is no code path from a model string to a shell, file write outside the store/reports, network request, or approval record.
5. **Rendering safety.** Reports escape all text (`html/template`); URLs are rendered as text and as links only when they match the stored canonical URL of an allowlisted or imported source.
6. **Workspace content is untrusted too.** Even documents from `bots/punyamas` are treated as data; their rules reach MIA only via parsed, validated configuration reviewed by a human.

## 7. Source allowlists and blocklists

- `config/research-sources.yaml`: `web.allow[]` (exact hosts, path prefixes, max requests per hour, notes on terms) — **empty until the owner approves the shortlist**; `web.block[]` (patterns for login pages, checkout, user-generated content requiring auth, government data behind forms); `workspace_assets[]` (the approved read-only paths).
- The fetcher is compiled in but disabled unless `web.enabled: true` **and** the allowlist is non-empty; both require an approval record (`mia approve source <host>`).
- robots.txt is fetched and honoured per host; disallowed paths are recorded as `blocked_by_robots`.
- No scraping of pages that require login; no CAPTCHA handling; no rate-limit evasion; one request per host per configured interval.

## 8. Human approval requirements

| Action | Requires |
|---|---|
| Treat an opportunity as approved (input to a brief) | `ApprovalRecord(opportunity, approved)` |
| Mark a brief ready for handoff | `ApprovalRecord(brief, approved_for_handoff)`; handoff itself is manual |
| Accept changed workspace rules/facts into config | `ApprovalRecord(imported_asset, accepted_drift)` |
| Enable a web source | `ApprovalRecord(source_host, approved)` + allowlist entry |
| Enable a live provider | `.env` secret present **and** `agent-limits.yaml → provider.name` changed **and** owner confirmation at the phase gate |
| Any external action (publish, message, ad spend, production change, dashboard access) | **Not implemented**; cannot be approved inside MIA in Phases 2–7 |

## 9. Budget exhaustion

- Before each paid call: `estimate = input_tokens_est × price_in + max_output × price_out` using the verified price table in `config/agent-limits.yaml` (with its verification date). If `monthly_spent + run_spent + estimate > min(ceiling_month = 20, ceiling_run, ceiling_agent)`, the call is refused, the step ends `budget_exhausted`, the run becomes `partial`/`failed`, and the report states what was skipped.
- After each call: actual usage recorded; if the provider cannot report usage, the estimate is recorded with `estimated = true` and the summary prints a warning that cost is unmeasurable.
- The monthly ceiling is checked against the ledger month in local time (WIB); a run cannot straddle the ceiling by splitting calls.
- Mock mode: the same gate runs with simulated usage so budget logic is tested without spending.

## 10. Tool permissions

- MIA itself: filesystem under its root, read-only workspace root, optional outbound HTTPS to allowlisted hosts and to the approved provider endpoint. No shell execution of external content, no package installation, no launchd changes at runtime.
- Agents: none (see §6).
- Scheduler (Phase 6): the launchd job runs the same binary with a fixed command line; no dynamic command construction.

## 11. Data retention

See `data-model.md` §7: database and approvals indefinite; snapshots and execution files 180 days (configurable) unless referenced by approved items; backups last 7; logs 90 days. No customer personal data is stored, by schema and by import guard.

## 12. Backup and recovery

- `mia backup` → `VACUUM INTO data/backups/mia-<date>.sqlite`; recommended before accepting drift or upgrading schema.
- Recovery: stop runs, copy backup over `data/mia.sqlite`, run `mia validate db` (integrity check + migration state).
- Reports and drafts are plain files; the git repository (once initialized) holds docs and config, never `data/`.

## 13. Safe failure behaviour

- A failed required step always produces a failed run and a report whose first line says so; no partial output is presented as complete.
- Interrupted runs are detectable (`stale`) and resumable from the last completed step; completed steps are not re-executed, and no provider call is repeated without a new budget check.
- Provider errors are classified; `auth` and `billing` errors stop the run immediately and never retry; `rate_limit` and `timeout` retry at most `max_retries` (default 2) with backoff.

## 14. Prohibited actions (design-level guarantee)

MIA contains no code to publish content, send messages (WhatsApp, email, Telegram, social), buy advertising, change production systems, or read operational/customer data. Notifications in Phase 6 are limited to sending MIA's own run summaries to the owner's approved channel and require their own approval gate.

## 15. Mapping to Phase 0 risks (`docs/audit/risks-and-constraints.md` §6)

| Risk | Mitigation in this design |
|---|---|
| R1 drift | §2 drift detection + approval (Workflow D) |
| R2 banned/invented claims | claim checker + facts check on all outputs; placeholders; DRAFT state (§8) |
| R3 fabricated research | evidence rules in `data-model.md` §6; reports rendered only from validated rows |
| R4 prompt injection | §6 |
| R5 unexpected spend | §9 |
| R6 subscription terms | CLI adapter not enabled; API-key path only after approval (`integrations.md` §3) |
| R7 scraping terms | §7 |
| R8 premature marketing activity | no external actions; M1 preconditions listed on every brief |
| R9 scope creep | out-of-scope list in `mvp-scope.md`; no customer entities |
| R10 toolchain confusion | `GOTOOLCHAIN=local`, Makefile `GO` variable (`system-design.md` §9) |
| R11 sandbox hides failures | tests avoid network/ports; smoke tests documented for the owner |
| R12 no performance data | honest "cannot compute"; tracking plan |
| R13 accidental workspace writes | §3 read-only boundary + tests + before/after `git status` check in acceptance |
| R14 Indonesian text quality | style rules, few-shot approved texts, human review |
