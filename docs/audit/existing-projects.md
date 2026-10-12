# Audit: Existing Projects and Reusable Assets

**Audit date:** 2026-10-12 (WIB) · **Method:** read-only inspection (`ls`, `git log/status`, `cat/sed/grep`). No file outside `~/Documents/punyamas/mia` was modified.

Legend: **[V]** verified · **[A]** assumption · **[?]** unresolved question.

## 1. Verified facts

### 1.1 Workspace layout

| Path | What it is | Relevance to MIA |
|---|---|---|
| `~/Documents/punyamas/mia` | This project. Empty except `.claude/`. Not a git repo | Target folder |
| `~/Documents/bots/punyamas` | **The Punyamas business workspace** (documents, brand, specs, mock server, Flutter apps). Git repo, branch `docs/rename-punyamas`, `main` untouched, 16 active Claw-Empire worktrees in `.climpire-worktrees/`, HEAD at audit time `446cb3f` | **Primary source of truth; read-only for MIA** |
| `~/Documents/kerjalancar` | The user's Go 1.27 web app (chi, pgx, goose, HTMX, Tailwind), modified today | Reusable engineering patterns |
| `~/Documents/bots/claw-empire` | Third-party local "AI agent office" (Node, SQLite) that drives Claude Code CLI and other providers as virtual employees | Context: how Punyamas agent tasks have been run so far |
| `~/Documents/bots/tradingview-mcp`, `~/Documents/tradingview-mcp` | Node MCP server using Chrome DevTools Protocol | Pattern reference only |
| `~/dev/xau-ea-bot` | Python Flask + launchd scheduled scripts (port 5000) | launchd pattern reference only |
| `~/Documents/forstok_project/*` | 2022 Go microservices (chi, gorm, MySQL, RabbitMQ) | Origin of the old Go module cache; not reusable |
| Trading projects (`xau-*`, `my-bot`, `my-jesse-bot`, `bots/xauusd*`) | Unrelated | None |
| `~/Documents/bots/mariemas` | Referenced in Claude project history, **directory no longer exists** (the business was renamed to Punyamas, D-019) | None |

### 1.2 `bots/punyamas` — inventory of reusable assets

All content is in Indonesian. Statuses in that repo are "draft for CEO review"; nothing is published.

| Asset | Path (relative to `bots/punyamas`) | Content | Reuse in MIA |
|---|---|---|---|
| Business context | `CONTEXT.md` | Business summary (buys used gold jewellery, bullion, silver in Indonesia; sells to refiners/aggregators), business flow (KYC → weigh/test → mechanical price rule → payment → lots → sale), systems (desktop dashboard built by the CEO on a separate PC = system of record; mobile apps planned), **AI working rules 1–7**, stage table, AI team, glossary | Source for `config/business.yaml`; rules 1–7 become agent guardrails (VERIFY labels, FACT/ASSUMPTION/OPEN split, no KYC-avoidance advice, no PII, drafts only, no physical testing conclusions) |
| Decision log | `DECISIONS.md` (D-001..D-032) | Notably: D-001 Indonesia; D-003 stage order; D-016 first store in **Surabaya**, address undecided; D-017 brand direction "trusted and transparent"; **D-019 name changed to Punyamas, trademark check pending, irreversible spending deferred**; D-020 official logo and palette (navy `#1B2F4A`, gold `#786647`, `#B59864`); D-028 name used provisionally, agent cost confirmed per batch; D-032 site expanded to 17 pages, still no buying prices | Hard constraints for every output |
| Open questions | `OPEN-QUESTIONS.md` (Q1..Q91) | Q2 location (partly decided), **Q55 final name, Q56 tagline, Q61 digital channels (domain, social accounts, WhatsApp Business, Google Business Profile) deferred until name is safe**, marketing budget unknown | "Requires confirmation" fields in config |
| Brand strategy | `docs/02-brand/B1-arah-dan-strategi-merek.md` | Audiences **AU1–AU5** (individual sellers, repeat sellers, business buyers, end consumers, professional partners) with wants/fears/channels; positioning (clarity, not highest price); pillars **PM1–PM4** (visible, clear, orderly, calm) with allowed/forbidden claims; tone **KP1–KP4** (calm, clear, meticulous, humble) and language rules (Bapak/Ibu, kami/Anda, units g/permil/Rp, no emoji/slang); key messages per audience; taglines **TG1–TG4** (passed) vs TG5–TG6 (rejected); **9 banned-claim categories** (SUPERLATIF, PERINGKAT, ABSOLUT, BERTENTANGAN_KEBIJAKAN, KECEPATAN_UANG, KEUANGAN, STATUS, INSTITUSI); approved texts **CT1–CT10**; banned examples **LK1–LK10**; checker test cases **BT1–BT12**; name risk analysis; sources and research limits (§15) | Direct source for `config/brand.yaml` and `config/marketing-policy.yaml`; CT/LK/BT rows become the equivalence test fixture for the ported checker |
| Brand text checker | `tools/spec-oracle/brand-oracle.mjs` (25 lines, `VERSI = 'v0.1-ASUMSI'`) | `POLA[]` of 9 regexes + `periksaTeks(teks)` returning `{lolos, temuan}` | Port rules as data; Go implementation must reproduce every BT/LK/CT verdict; drift check compares the YAML rules with this file |
| Facts file | `docs/02-brand/B4-fakta.json` (v0.1) | FB1–FB14 with status `RENCANA` (planned, must be written in intent language) or `KEBIJAKAN` (policy); `angka_boleh: ["10"]`; `isian_wajib` (PT name, addresses, NIB, NPWP, directors, contact, founding date, bank) | Approved-claims list and the "missing business information" list |
| Brand guide and tokens | `docs/02-brand/B3-brand-guide-dan-token.md` | Design tokens, status system, number/unit/date formatting (§8), accessibility | Report formatting rules |
| Pre-launch marketing plan | `docs/05-marketing/M1-rencana-pra-peluncuran.md` (v0.1, owner "Tata (Marketing)") | 7 preconditions before any public activity; audiences and messages; channels **M1 site, M2 signage/print, M3 WhatsApp Business, M4 social media, M5 manufacturer/trader relations** (all costs `[ISI: …]`); sequence tied to roadmap milestones L03/L06/L09/L21/L40/L42; process measures and outcome measures (WhatsApp inquiries, AU1/AU2 visits, AU3 qualifications, visit-to-transaction %); 7 unknowns | KPI framework input; MIA outputs map to M1–M5 and must respect the preconditions |
| Website copy and static site | `docs/05-marketing/W1-naskah-situs-company-profile.md`, `docs/05-marketing/situs/` (17 pages, no external resources, no trackers, `noindex` draft), builder `tools/site/` | 13 FAQ questions, 4 education articles (karat/kadar, valuing, preparing items, why identity is recorded), calculator without fixed prices | Seed "customer question" research items tagged as **internal hypotheses** (not external evidence) |
| Prior research style | `docs/02-brand/penelusuran-merek-2026-10-06.md`, B1 §15 | Explicit "read directly / read only via search summaries / not researched / limitations" disclosure | Template for MIA evidence-quality labels |
| App specs and contract | `docs/03-apps/*`, `packages/kontrak/openapi.json` (title "API Aplikasi Mobile Punyamas", version `0.1.0-ASUMSI`), `server-tiruan/` (tsx, port 8791) | Endpoints for queue (`/antrean/*`), attendance (`/absensi/*`), items, weighing, testing, quotes, payments, receipts, members | Integration boundary documentation only; **no marketing endpoints exist** |
| Roadmap and status | `docs/00-peta-jalan/P1..P5` | Critical path is outside the repo: location, trademark check, PT founding, dashboard completion, professional review | Timing constraints for any public campaign |
| Verification tooling | `node tools/spec-oracle/validate-docs.mjs`, `*-selfcheck.mjs` | Document/oracle consistency checks (Node 22) | Not run by MIA; pattern for MIA's own self-checks |

Repository rules stated in its README (binding for MIA when it reads from there): agent work enters via Claw-Empire worktree branches and is merged only after CEO approval; **never store real customer personal data**; legal/tax/regulatory statements must be sourced and labelled **VERIFY**.

### 1.3 `kerjalancar` — reusable engineering patterns

| Pattern | Detail | Reuse |
|---|---|---|
| Toolchain selection | `Makefile`: `GO ?= go`; docs say `GO=~/sdk/go1.27.2/bin/go make test`; `.toolchain/{gocache,gopath,modcache}` keep caches local | Same approach for MIA |
| Layout | `cmd/{server,migrate}`, `internal/{auth,config,db,web,workspace}`, `docs/`, `.env.example`, `.golangci.yml`, `.github/` | Same shape (without web/auth) |
| Config | `.env.example` with variable names only (APP_ENV, PORT, LOG_LEVEL, DATABASE_URL, …) | Same discipline |
| Tests | `go test -race -count=1 ./...`; DB tests skip when `TEST_DATABASE_URL` unset | Same discipline (SQLite in temp dir instead) |
| Docs set | ARCHITECTURE, DECISIONS, RISK_REGISTER, ASSUMPTION_REGISTER, COMPETITOR_RESEARCH, ROADMAP, PROJECT_STATUS | Mirrors the prompt's required docs |

Not reusable as code: Postgres (pgx/goose), sessions/auth, HTMX UI — all out of MIA scope.

### 1.4 Claw-Empire context

Punyamas documents were produced by AI "employees" orchestrated by Claw-Empire using the Claude Code CLI under the user's subscription. D-028 records that agent cost is confirmed per batch because quota is shared with interactive Claude sessions. MIA is a different thing: a deterministic application with optional AI calls, not an agent office. MIA does not integrate with Claw-Empire.

## 2. Potential conflicts and how MIA avoids them

| Overlap | Finding | Handling |
|---|---|---|
| Operational dashboard (system of record for transactions, future CRM) | **Not on this machine** (CONTEXT.md: built by the CEO on a separate PC) | No integration in MVP. Later: a file-based export contract (aggregated inquiry/visit counts per channel and period, no PII) agreed with the dashboard owner |
| Mobile apps, `server-tiruan`, OpenAPI contract | Running on port 8791 now; endpoints are operational flows | MIA never calls them; MIA binds no port in the MVP |
| Brand/business source of truth | Lives in `bots/punyamas`; copying risks divergence | MIA config files carry `source_of_truth: {path, commit, date}`; `mia config check` diffs the 9 claim categories against `brand-oracle.mjs` and facts against `B4-fakta.json` and warns on drift; MIA never writes into `bots/punyamas` |
| Marketing deliverables (M1, W1, site) | Owned by the workspace / "Tata" | MIA writes to `mia/reports/` and `mia/drafts/`; moving approved briefs into `docs/05-marketing/` is a manual, CEO-approved step |
| Claw-Empire worktrees | 16 active worktrees in `bots/punyamas` | Read-only access only; MIA reads the checked-out branch, never the worktrees |
| CRM / lead management | Out of scope and planned for the dashboard | MIA stores aggregated metrics only; schema has no customer fields; importers reject columns that look like phone numbers or identity numbers |
| Port and process collisions | 5000, 6379, 8791, 8765–8784 in use/reserved | MIA is a CLI; any future viewer binds 127.0.0.1 outside those ranges |

## 3. Assumptions

- **[A]** `bots/punyamas` branch `docs/rename-punyamas` is the current working state; `main` is stale by design (README rule).
- **[A]** The brand checker list (`v0.1-ASUMSI`) will change after the advertising-law consultant review (N70); MIA must treat it as versioned input, not a constant.
- **[A]** The CEO (the user) is the approver for all MIA outputs, consistent with D-005.
- **[A]** The dashboard owner can eventually export aggregated marketing-relevant counts; the format is undefined.

## 4. Unresolved questions

- **[?]** Should engineering docs for MIA be in English (plan default) or Indonesian like the sibling workspace?
- **[?]** Confirm read-only import with provenance and drift check as the relationship to `bots/punyamas` (no symlinks, no writes).
- **[?]** Who owns the "Tata (Marketing)" role going forward: Claw-Empire agents, MIA, or both with MIA as the evidence base?
- **[?]** Exact export format and cadence from the operational dashboard, when it exists.
