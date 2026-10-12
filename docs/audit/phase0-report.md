# Phase 0 Audit Report (decision record)

**Status:** Phase 0 complete, approved for artifact creation on 2026-10-12. Phase 1 (specification) has NOT started and waits for separate approval.

This is the report presented for approval; the four companion files in this folder hold the detail: `environment.md`, `existing-projects.md`, `integrations.md`, `risks-and-constraints.md`.

---

## Context

The master prompt asks for an internal marketing-intelligence platform for Punyamas (an Indonesian gold-buying business in pre-launch) that is separate from the operational dashboard, economical, evidence-driven, and usable with no paid API in mock mode. Phase 0 is inspection only: audit the machine and workspace, then stop for approval before specification (Phase 1).

The key discovery of this audit: a substantial Punyamas workspace already exists on this machine at `~/Documents/bots/punyamas` (Indonesian-language docs, brand rules, a brand-claim text checker, a pre-launch marketing plan, a mock API server and Flutter apps). The marketing intelligence agent must **consume** that workspace as its source of truth and **feed** it, not duplicate it.

---

## 1. Executive summary

- **Machine is capable; toolchain needs one deliberate choice.** Apple Silicon MacBook Air, macOS 26.6.2, 217 GiB free. The Go on `PATH` is an obsolete Intel build (1.17.5), but a native **Go 1.27.2 (arm64)** already lives at `~/sdk/go1.27.2` and is used by your `kerjalancar` project via `GO=~/sdk/go1.27.2/bin/go`. Node 22 and Python 3.12 exist but are not the defaults (`node` → v10, `python3` → 3.9 x86_64).
- **No application-level AI credentials exist.** No `ANTHROPIC_API_KEY`/`OPENAI_API_KEY`; `~/.config/anthropic` is empty. Only the Claude Code CLI (2.1.226, subscription) is present. The MVP must run in mock mode first; live AI needs a decision (API billing vs. headless `claude -p` under subscription, terms to verify).
- **Punyamas already has a marketing foundation** in `~/Documents/bots/punyamas`: business context, decision log (D-001..D-032), open questions (Q1..Q91), brand strategy with audiences AU1–AU5, tone KP1–KP4, 9-category banned-claim list implemented in `tools/spec-oracle/brand-oracle.mjs`, a facts file `B4-fakta.json`, a 17-page static company-profile site, and `M1-rencana-pra-peluncuran.md` (channels M1–M5, success measures). These are reusable and must be the single source of truth for `config/brand.yaml`, `config/business.yaml`, `config/marketing-policy.yaml`.
- **The operational dashboard is not on this machine** (CONTEXT.md: built by the CEO on a separate PC). Only the mobile apps' mock server and OpenAPI contract exist here, with no marketing-relevant endpoints. Integration must be file/export-based for now; no conflict risk as long as `bots/punyamas` is treated as read-only input.
- **Hard business constraints already decided:** no public activity until the name is cleared by a trademark agent (D-019), no buying-price claims (CT7), promo contact needs separate consent, all public text must pass the brand text checker and CEO approval (D-005), Surabaya first store (D-016), pre-launch messaging must use "bahasa niat" (intent language), Indonesian for customer-facing text.
- **Recommendation:** Go 1.27 CLI + SQLite (pure Go) + YAML config + Markdown/HTML reports + provider abstraction with mock mode, launchd scheduling later. Smallest architecture that satisfies the prompt and matches your current Go practice.

---

## 2. Verified environment findings

| Item | Finding | Status |
|---|---|---|
| OS / arch | macOS 26.6.2 (25G83), Darwin 25.6.0, arm64 (Apple M1 family per kernel tag T8103), host `Kevins-MacBook-Air` | [V] |
| CPU / RAM | `sysctl` blocked by sandbox; not measured | [?] |
| Disk | 460 GiB volume, **217 GiB free** | [V] |
| Timezone / locale | WIB (UTC+7); `LANG` unset in sandbox shell | [V] |
| Go on PATH | `/usr/local/go` = **go1.17.5 darwin/amd64** (Intel binary under Rosetta; too old for current libraries) | [V] |
| Go (native) | `~/sdk/go1.27.2/bin/go` = **go1.27.2 darwin/arm64**, not on PATH; `GOTOOLCHAIN=auto`; used by `kerjalancar` via `GO=` Makefile variable | [V] |
| Go module caches | `~/go/pkg/mod` 587 MB of 2022-era modules (chi, gorm, mysql driver, testify, yaml.v2, godotenv). `kerjalancar/.toolchain/modcache` holds **modernc.org/sqlite v1.60.1**, go.yaml.in/yaml/v3, chi v5, pgx, testify | [V] |
| Python | `python3` = 3.9.10 x86_64 (Homebrew Intel; EOL); `/usr/bin/python3` = 3.9.6; `/usr/local/opt/python@3.12/bin/python3.12` = 3.12.14 x86_64. pip has only pytest, PyYAML, jsonschema, click | [V] |
| Node | nvm default **v10.24.1**; v20.20.2 and v22.23.2 installed; `npm` under v22 works only after `nvm use 22`; pnpm/yarn present; `node:sqlite` loads on v22 with an ExperimentalWarning | [V] |
| Ruby / Java / Rust | Ruby 2.6.5 (rvm, x86_64); Java runtime missing; Rust absent | [V] |
| Compiler | Xcode CLT, Apple clang 21 (cgo possible) | [V] |
| Package managers | Homebrew 6.0.5 at `/usr/local` (Intel prefix, Rosetta); no `/opt/homebrew`; nvm; rvm | [V] |
| Databases | **sqlite3 3.51.0** CLI; MySQL 8.0.28 installed, **not running**; Redis 6.2.6 **running** on 127.0.0.1:6379 (brew service); RabbitMQ installed, stopped; PostgreSQL not installed (kerjalancar uses Docker Compose for it) | [V] |
| Docker | CLI 20.10.11 present; no `dockerd` process; daemon socket unreachable from sandbox | [V] daemon state [?] |
| Scheduling | launchd user agents exist, including your own `com.kevinivander.xauea.{detect,flask,review}.plist` (StartInterval 900 s, logs to project folder) → proven pattern. `crontab` blocked by sandbox | [V] |
| Ports in use | 5000 (Python Flask, xau-ea), 6379 (Redis), **8791 (`server-tiruan` is running now)**, 5037/5554/5555 (Android), several Claude/VS Code ports. `bots/.claude/launch.json` reserves 8765, 8767, 8781–8784, 8791 | [V] |
| Git | git 2.54.0; `gh` missing; global config has no user.name/email (commits in kerjalancar are by "Kevin Ivander", per-repo config); `mia` is **not** a git repo | [V] |
| Network (from sandbox) | proxy.golang.org 200, pypi.org 200, registry.npmjs.org 200, api.anthropic.com reachable (404 on HEAD /), trends.google.com 301, google.com 200 — outbound HTTP works for declared hosts | [V] |
| Web research tooling | curl, jq present; wget absent; no Playwright browsers; `tradingview-mcp` uses `chrome-remote-interface` (CDP) | [V] |
| AI provider config | Env var names present: `ANTHROPIC_BASE_URL`, `API_TIMEOUT_MS`, `CLAUDE_CODE_MESSAGING_TOKEN` (set by this Claude Code session; values not read). **No** `ANTHROPIC_API_KEY`. `~/.config/anthropic/` empty. Claude Code CLI 2.1.226 at `~/.local/bin/claude` supports `-p --output-format json`. No Ollama/LM Studio | [V] |
| Session-only tools (not app-level) | This Claude session has WebSearch/WebFetch and MCP connectors (Google Drive, Canva, Vercel, Alpha Vantage, FMP). They are not available to an application process | [V] |
| Test tools | `go test` (stdlib), pytest 8.4.2, vitest (in bots/punyamas); golangci-lint not installed | [V] |

---

## 3. Existing project and tool inventory

### 3.1 `~/Documents/bots/punyamas` — the Punyamas workspace (read-only for this project)

Git repo on branch `docs/rename-punyamas` (main untouched), 16 active Claw-Empire worktrees in `.climpire-worktrees/`, Node ≥22 workspace with vitest. All content Indonesian. Relevant assets:

| Asset | Path | Reuse for MIA |
|---|---|---|
| Business context, AI working rules, glossary | `CONTEXT.md` | Source for `config/business.yaml`; rules 1–7 become agent guardrails |
| Decision log D-001..D-032 | `DECISIONS.md` | Constraints (name unverified D-019, Surabaya D-016, no price claims, logo palette D-020) |
| Open questions Q1..Q91 | `OPEN-QUESTIONS.md` | Q55 name, Q56 tagline, Q61 digital channels, budget unknown → "requires confirmation" fields |
| Brand strategy: audiences AU1–AU5, pillars PM1–PM4, tone KP1–KP4, messages, taglines TG1–TG4, claim rules (9 categories), approved texts CT1–CT10, banned examples LK1–LK10, checker test cases BT1–BT12 | `docs/02-brand/B1-arah-dan-strategi-merek.md` | Direct source for `config/brand.yaml` and `config/marketing-policy.yaml`; BT/LK/CT rows become the equivalence test fixture |
| Brand text checker (25 lines, 9 regex categories, `periksaTeks()`) | `tools/spec-oracle/brand-oracle.mjs` | Port rules to data + Go checker; prove equivalence; drift check against this file |
| Structured facts FB1–FB14 with status RENCANA/KEBIJAKAN and mandatory fill-ins (`isian_wajib`) | `docs/02-brand/B4-fakta.json` | Direct source for approved claims and "missing information" list |
| Brand guide and design tokens | `docs/02-brand/B3-*.md` | Report styling, number/date formatting rules (section 8) |
| Pre-launch marketing plan: channels M1–M5, sequence, process/outcome measures, unknowns | `docs/05-marketing/M1-rencana-pra-peluncuran.md` | KPI framework input; MIA outputs should map to M1–M5 |
| Website copy incl. 13 FAQ questions, 4 education articles | `docs/05-marketing/W1-naskah-situs-company-profile.md` + `situs/` | Seed "customer questions" research items (status: internal hypothesis, not external evidence) |
| Prior brand search method and research-limit disclosure style | `docs/02-brand/penelusuran-merek-2026-10-06.md`, B1 §15 | Template for evidence-quality labelling |
| API contract (OpenAPI 0.1.0-ASUMSI, zod) and mock server | `packages/kontrak`, `server-tiruan` | Only to document the integration boundary; no marketing endpoints exist |

### 3.2 `~/Documents/kerjalancar` — your Go web app (modified today)

Go 1.27 + chi + pgx + goose + HTMX + Tailwind, `cmd/{server,migrate}`, `internal/{auth,config,db,web,workspace}`, Makefile with `GO ?= go`, `.toolchain/` for local caches, `docs/{ARCHITECTURE,DECISIONS,RISK_REGISTER,ASSUMPTION_REGISTER,COMPETITOR_RESEARCH}.md`. **Reusable patterns:** project layout, Makefile, toolchain selection, `.env.example` discipline, docs set. Not reusable as code (Postgres, auth, web UI are out of MIA scope).

### 3.3 Other projects (context only)

- `bots/claw-empire` — third-party local AI agent orchestrator (Node, SQLite) that drives Claude Code CLI etc. This is how Punyamas agent tasks ("Tata (Marketing)") have been run; D-028 notes quota is shared with Claude sessions and cost is confirmed per batch.
- `bots/tradingview-mcp` (Node MCP server, CDP browser automation), `dev/xau-ea-bot` (Python Flask + launchd scripts), `forstok_project` (2022 Go microservices; origin of the old module cache), trading bots. None overlap with MIA.

---

## 4. Potential conflicts with the Punyamas dashboard and workspace

| Conflict / overlap | Assessment | Proposed handling |
|---|---|---|
| Operational dashboard (system of record) | **Not on this machine** (CEO's separate PC) [V from CONTEXT.md]. No DB to touch. | No integration in MVP. Define a file-based import contract (CSV/JSON of inquiry counts per channel/period) to be agreed later with the dashboard team. |
| Mobile apps + `server-tiruan` + OpenAPI contract | Endpoints are queue/attendance/transaction flows; nothing marketing-related. `server-tiruan` is running on 8791 now. | MIA never calls it. MIA has no server in MVP; if a local report viewer is added later, bind 127.0.0.1 on a port outside 5000, 6379, 8765–8791. |
| Brand/business source of truth | Lives in `bots/punyamas` docs and `brand-oracle.mjs`. Copying into MIA config risks **divergence**. | `config/*.yaml` carry a `source_of_truth` reference (path + commit hash + date). A `mia config check` command diffs the 9 claim categories against `brand-oracle.mjs` and the facts against `B4-fakta.json`, and warns on drift. Never write into `bots/punyamas`. |
| Marketing deliverables (M1 plan, W1 copy, site) | Owned by the Punyamas workspace / Claw-Empire "Tata". | MIA produces briefs and reports into `mia/reports/` and `mia/drafts/`; handoff to `bots/punyamas/docs/05-marketing/` is a manual, approved step. |
| Claw-Empire worktrees in `bots/punyamas` | 16 active worktrees; any write there could collide. | Treat `bots/punyamas` as read-only. |
| Future CRM / lead management (dashboard roadmap) | MIA must not become a CRM. | MIA stores only aggregated campaign metrics imported from exports; no customer records. |
| Customer personal data | Workspace rule 5 and `.gitignore` forbid it. | MIA schema has no PII fields; imports are validated to reject rows with phone/ID-like columns. |

---

## 5. Recommended MVP scope

**In (Phases 2–5):**
1. Research record store with full provenance (source, URL/file, retrieval timestamp, publication date, claim, excerpt, source type, evidence quality, status) and deduplication.
2. Importers: Markdown/CSV/JSON research notes; read-only import of selected Punyamas workspace docs (FAQ, M1 unknowns) tagged as *internal hypotheses*.
3. Optional allowlisted HTTP fetch of public pages (robots.txt respected, text extraction, untrusted-content handling) — gated behind config, off by default.
4. Market Research Agent and Strategy Agent (configurable scoring with written rationale, hypotheses, missing-evidence list).
5. Content and Campaign Agent producing Indonesian briefs that pass the ported brand checker and carry "[ISI: …]" placeholders for unconfirmed facts, never claims from the banned list; everything marked DRAFT pending CEO approval.
6. Performance Analyst on an approved sample dataset; honest "cannot compute" when data is absent; tracking-plan proposal.
7. Orchestrator with bounded retries, timeouts, budgets, explicit run states, cost record.
8. Reports: Markdown + static HTML, no web framework.
9. Mock provider with recorded fixtures; provider abstraction for Anthropic Messages API (and optionally `claude -p`).

**Out (per prompt §4):** dashboard replacement, CRM, POS, customer app, price/trading engine, autonomous publishing/WhatsApp/ads, multi-agent framework, data warehouse, SaaS.

**Deferred to Phase 6+:** launchd scheduling, Telegram/email notifications, live analytics connectors (no properties exist yet: no domain, no GBP, no ad accounts — Q61 open).

---

## 6. Three viable architecture options

| | Option A — Go 1.27 CLI + SQLite (pure Go) | Option B — Node 22 TypeScript (tsx/vitest/zod) | Option C — Python 3.12 + sqlite3 stdlib |
|---|---|---|---|
| Fit with environment | Native arm64 toolchain exists at `~/sdk/go1.27.2`; proven today in kerjalancar; modernc sqlite + yaml already cached | Same stack as `bots/punyamas` workspace; could import `brand-oracle.mjs` directly | 3.12 exists but is an x86_64 Homebrew build; needs venv + new deps (httpx, pydantic, jinja2) |
| Deterministic core (scoring, dedup, validation) | Strong typing, single static binary, stdlib testing | zod schemas good; runtime deps grow | pydantic good; dynamic typing |
| LLM integration | Official `anthropic-sdk-go` or raw HTTP; mock via fixtures | Official `@anthropic-ai/sdk` | Official `anthropic` SDK |
| Web fetch / HTML text | net/http + `golang.org/x/net/html` | fetch + cheerio (new dep) | httpx + bs4 (new deps) |
| Storage | modernc.org/sqlite (no cgo) | `node:sqlite` is experimental on v22; better-sqlite3 needs native build | sqlite3 stdlib |
| Scheduling | launchd runs the binary | launchd must source nvm (default node is v10 — footgun) | launchd with explicit interpreter path |
| Main risks | Go not on PATH → Makefile `GO=`; porting 25-line brand checker (with equivalence test) | Node default version trap; experimental SQLite; more dependencies | EOL default python; x86_64 build; more deps; weaker single-artifact deployment |
| Dependencies to approve | modernc.org/sqlite, go.yaml.in/yaml/v3, golang.org/x/net; later anthropic-sdk-go | tsx, typescript, vitest, zod, @anthropic-ai/sdk, better-sqlite3 or node:sqlite | pydantic, httpx, jinja2, pyyaml, pytest, anthropic |

---

## 7. Recommended architecture and reasoning

**Option A.** Reasons: matches the prompt's default and your active practice (kerjalancar, today); one static binary runs from launchd without environment juggling; the deterministic parts (scoring, dedup, schema validation, budgets) benefit from static typing; SQLite needs no service; required modules are reachable (proxy.golang.org verified) and partly cached. The only real cost versus Option B is porting the 25-line brand checker, which is mitigated by a data-driven rule file plus an equivalence test built from B1's BT1–BT12, LK1–LK10, CT1–CT10 rows and a drift check against the original file.

Shape (to be finalized in Phase 1):

```
mia/
  cmd/mia/                 CLI: run, import, report, status, validate, config check, (later) schedule
  internal/config          YAML loading + validation, source_of_truth references
  internal/store           SQLite schema + migrations (embedded SQL), repositories
  internal/research        importers (md/csv/json), allowlisted fetcher, extractor, dedup (URL canon + content hash + fuzzy title)
  internal/agents/{research,strategy,content,performance}
  internal/provider        Provider interface; mock (fixtures), anthropic (Messages API), optional claudecli
  internal/brand           claim checker (rules from config), equivalence + drift tests
  internal/orchestrator    run state machine, budgets, retries, timeouts, cost ledger
  internal/report          Markdown + HTML templates
  config/{brand,business,marketing-policy,research-sources,agent-limits}.yaml
  imports/  reports/  drafts/  testdata/fixtures/
  docs/{audit,product,architecture,implementation}
  Makefile (GO ?= ~/sdk/go1.27.2/bin/go), .env.example, .gitignore
```

Trade-offs accepted: no web UI; AI used only for synthesis/prioritization/drafting; scoring is a documented heuristic, not a prediction.

---

## 8. Data-source and integration feasibility matrix

| Source / integration | Available now? | Auth | Cost | Limits / terms | MVP approach | Status |
|---|---|---|---|---|---|---|
| Manual notes, CSV/JSON/Markdown imports | Yes | None | Free | — | Primary input | [V] feasible |
| Punyamas workspace docs (FAQ, M1, B1, B4-fakta) | Yes, local | None | Free | Read-only; tag as internal hypothesis | Importer with provenance | [V] feasible |
| Public competitor websites (gold buyers in Surabaya/Indonesia) | Network reachable for declared hosts | None | Free | robots.txt, ToS, JS-rendered pages, blocking; untrusted content | Allowlist in `research-sources.yaml`, off by default; snapshot + hash | [A] untested per site; list of competitors [?] |
| Google Trends | Site reachable; no official API | None | Free | Unofficial libraries break; rate limits | Manual CSV export → importer | [A] import only |
| Google Search Console / GA4 | No property (site unpublished, no domain, Q61) | OAuth | Free | N/A until site exists | CSV export importer, adapter later | N/A now |
| Google Business Profile | Not created (Q61) | OAuth | Free | N/A | Later | N/A now |
| Meta / TikTok ads exports | No accounts | — | Paid media | — | CSV importer only | N/A now |
| Web search provider (Tavily, Brave, SerpAPI, Anthropic server-side `web_search`) | No keys | API key | Paid/freemium; **pricing not verified** | Rate limits | Behind provider interface; mock first | [?] needs decision + pricing verification |
| LLM — Anthropic Messages API | Reachable; **no key** | API key, separate billing | Paid (see §9) | Rate limits per tier | Provider adapter, mock first | [?] needs account/billing decision |
| LLM — Claude Code CLI `claude -p` (subscription) | Installed 2.1.226 | Existing login | Consumes subscription quota | **Terms for programmatic/automated use must be verified**; headless JSON output supported | Optional adapter, disabled by default | [?] needs your decision + terms check |
| Local LLM (Ollama etc.) | Not installed | — | Free | Install = approval | Not planned | N/A |
| Telegram / email notifications | No bot token, no SMTP config | Bot token / SMTP | Free / varies | Creating a bot = external account | Phase 6, approval required | [?] |
| Operational dashboard | Not on this machine | — | — | Owned by CEO/Development | File-based import contract later | N/A now |

---

## 9. Initial operating-cost model and unresolved pricing questions

Pricing source: bundled `claude-api` skill reference (cached 2026-10-06), USD per million tokens — **verify at anthropic.com/pricing before Phase 2**: Haiku 5.5 $0.10 in / $0.50 out; Sonnet 5.5 $2 / $10; Opus 5.5 $4 / $20; cache reads ≈ $0.20 on Opus 5.5 / Sonnet 5.5; Batch API −50%. Token volumes below are **assumptions** to be replaced by measured `usage` from the first live runs.

Per full run (research 20 sources + strategy + 3 briefs): ≈ 95k input, 19k output tokens [A].

| Scenario | Workload | Haiku 5.5 | Sonnet 5.5 | Opus 5.5 | Other costs |
|---|---|---|---|---|---|
| 1. Local-only, imported research, mock provider | Any | $0 | $0 | $0 | Human time; electricity |
| 2. Limited live research + AI synthesis, manual trigger | ~1 full run/week → 4/month | ≈ $0.08/mo | ≈ $1.50/mo | ≈ $3.00/mo | Search provider: unknown [?] |
| 3. Scheduled: daily digest (30k/4k) ×30 + weekly full ×4 + weekly content list (25k/6k) ×4 ≈ 1.38M in / 0.22M out | per month | ≈ $0.25/mo | ≈ $5/mo | ≈ $10/mo | + retries ×1.2; search provider [?] |

Levers already planned: prompt caching of the stable config/system prefix, dedup/caching of fetched sources, per-run and per-agent token caps, monthly ceiling in `agent-limits.yaml`, cost ledger per run, warning when usage cannot be measured (e.g., `claude -p` path). Billing is in USD; no IDR conversion assumed.

Unresolved pricing questions: current Anthropic prices and rate-limit tier; whether the subscription may be used programmatically and how its quota maps to runs; search-provider pricing; whether any paid media is planned (none so far — M1 budgets are all `[ISI: …]`).

---

## 10. Proposed KPI baseline and acceptance criteria

**Baseline:** none exists. Proposal: the first three manual mock/imported runs in Phase 3 define the operational baseline (time per report, items with valid references, validation pass rate, cost per run). Business metrics are **not applicable until the store opens**; M1's outcome measures (WhatsApp inquiries, AU1/AU2 visits, AU3 qualifications, visit-to-transaction %) become MIA's business KPIs once exports exist. Leading indicators (content approved, briefs produced, questions answered) are reported separately from revenue impact, which will not be claimed without evidence.

**Operational KPIs (measured by MIA itself):** time to produce a research report; % research items with valid source + timestamp; % reports passing schema + evidence validation; recommendations accepted by reviewer (manual field); time to prepare a brief; cost per completed run; failure rate; mean run duration.

**Acceptance criteria:** the full list from prompt §14 goes into `docs/implementation/acceptance-criteria.md` (Phase 1) as a checklist with pending/passed/failed/N-A, plus two project-specific additions: (a) brand checker equivalence with `brand-oracle.mjs` on all BT/LK/CT cases; (b) `bots/punyamas` unchanged (hash of tracked files before/after each phase).

---

## 11. Exact implementation plan with approval gates

**Gate 0 (this approval) → Phase 0 artifacts.** Upon approval I will create, in `mia/`, documentation only:

1. `docs/audit/environment.md` — §2 above, with Verified / Assumptions / Unresolved sections.
2. `docs/audit/existing-projects.md` — §3 and §4 (workspace inventory, reuse map, conflict handling, read-only rule).
3. `docs/audit/integrations.md` — §8 matrix with per-integration verification steps and the untrusted-content policy.
4. `docs/audit/risks-and-constraints.md` — §12-style risk register: business constraints (D-019, CT7, consent, D-005), technical (Go on PATH, sandbox limits, no credentials), legal (ad law N70, robots/ToS, PDP), cost, divergence risk.
5. `docs/audit/phase0-report.md` — this report (so the decision record lives in the repo, not only in my plan file). Omit if you prefer only the four required files.

No code, no `git init`, no dependencies, no config files in this step. Then **STOP** and request Phase 1 approval.

**Gate 1 → Phase 1 (specification):** the 11 docs in prompt §6 (vision, MVP scope, workflows, KPI framework, system design, data model, integrations, security, cost model, roadmap, acceptance criteria) plus a one-page decision memo. Propose `git init` + `.gitignore` + `.env.example` here. STOP.

**Gate 2 → Phase 2 (foundation):** Makefile with `GO ?= ~/sdk/go1.27.2/bin/go`, `go mod init`, config loading, slog logging with redaction, SQLite store + migrations, research record schema, mock provider, basic Markdown/HTML report, unit tests, end-to-end mock run. Dependencies requested at this gate: modernc.org/sqlite, go.yaml.in/yaml/v3, golang.org/x/net (html). STOP.

**Gate 3 → Phase 3:** Research + Strategy agents on imported sample data and (if approved) an allowlisted source set; brand/business config derived from the workspace with provenance and drift check; evidence-quality validation. STOP.

**Gate 4 → Phase 4:** Content and Campaign Agent, ported brand checker + equivalence test, DRAFT/approval states. STOP.

**Gate 5 → Phase 5:** Performance Analyst on sample dataset; tracking plan; import contract proposal for the dashboard team. STOP.

**Gate 6 → Phase 6:** launchd plist generator (dry-run first), optional Telegram/email adapter (needs bot/SMTP approval). STOP.

**Gate 7 → Phase 7:** acceptance checklist run, cost inspection, limitations, hardening.

Each phase ends with the report format in prompt §15 (files changed, features, tests + results, limitations, dependencies, cost, risks, next step).

---

## 12. Questions requiring your decision

1. **Toolchain:** confirm Go 1.27.2 from `~/sdk` (Makefile `GO=` variable, as in kerjalancar) rather than installing Go on PATH. [Recommended: yes]
2. **Documentation language:** I plan English for engineering docs (as the prompt allows) and Indonesian for all customer-facing drafts and marketing deliverables. The sibling workspace is fully Indonesian — do you want engineering docs in Indonesian instead, or bilingual summaries?
3. **AI provider path for Phases 3+:** (a) Anthropic API key with separate billing, (b) headless `claude -p` on your subscription (I will not assume this is permitted; you verify the terms), or (c) mock only until further notice. Mock is built regardless.
4. **Live web research:** allow allowlisted HTTP fetching of public competitor/education pages from the app, or import-only initially? If allowed, provide (or ask me to propose for your verification) the competitor list for Surabaya/Indonesia.
5. **Relationship to `bots/punyamas`:** confirm read-only import with provenance + drift check (no symlinks, no writes), and manual handoff of approved briefs into `docs/05-marketing/`.
6. **Monthly budget ceiling** for `agent-limits.yaml` (USD), and preferred default model tier (Haiku/Sonnet/Opus 5.5) for cost control.
7. **Phase 0 artifacts:** four required files only, or also `docs/audit/phase0-report.md`?
8. **Git:** `git init` for `mia` at Phase 1, with commits authored as "Kevin Ivander" (repo-local config)? Any remote?

---

## Verification (for the Phase 0 step after approval)

- `ls docs/audit/` shows the agreed files; each file has sections "Verified", "Assumptions", "Unresolved questions".
- `grep -riE 'sk-ant|api_key=|token=' docs/` returns nothing (no secrets).
- `git -C ~/Documents/bots/punyamas status --porcelain` is identical before and after (workspace untouched).
- No new files outside `mia/docs/audit/`; no processes started; no packages installed.

**Request:** approve Phase 0 artifact creation as listed in §11 (Gate 0). I will not begin Phase 1 until you approve it separately.
