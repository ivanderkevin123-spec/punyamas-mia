# MIA — MVP Scope

**Status:** Phase 1 draft for owner review · 2026-10-12 · Decisions applied: mock-only AI, no live fetching in Phases 1–2, read-only `bots/punyamas`, USD 20/month hard ceiling, Go 1.27.2 native toolchain.

## 0. Ringkasan untuk pemilik usaha (Bahasa Indonesia)

MVP MIA (dibangun di Fase 2 dan 3) berisi: **menjalankan riset secara manual** dari berkas impor dan dokumen ruang kerja Punyamas, **catatan riset yang selalu bersumber**, **Agen Riset Pasar** dan **Agen Strategi** (dengan penyedia AI tiruan/mock), **validasi keluaran terstruktur**, **penyimpanan lokal (SQLite)**, **laporan Markdown/HTML**, **pencatatan biaya dan status eksekusi**, **pemeriksa klaim merek** yang diturunkan dari `brand-oracle.mjs`, dan **uji otomatis**. Agen Konten (Fase 4), Agen Analis Kinerja (Fase 5), penjadwalan dan notifikasi (Fase 6) menyusul melalui antarmuka yang sudah disiapkan. Yang **tidak** dibangun: dashboard web, CRM, integrasi ke dashboard operasional, pengambilan data web otomatis tanpa persetujuan, dan tindakan eksternal apa pun.

## 1. Classification of features

Legend: **R** = Required for MVP (Phase 2–3) · **O** = Optional after MVP (Phase 4–6) · **X** = Explicitly out of scope.

### 1.1 Execution and inputs

| # | Feature | Class | Phase | Notes |
|---|---|---|---|---|
| F1 | CLI `mia` with `run`, `import`, `report`, `status`, `validate`, `config check` | R | 2 | Manual execution first; no server, no ports |
| F2 | Importers for Markdown, CSV, JSON research notes with mandatory provenance fields | R | 2 | Rejects rows without source name + reference + date; rejects PII-like columns |
| F3 | Read-only import of Punyamas workspace assets (B1, B4-fakta.json, brand-oracle.mjs, M1, CONTEXT, DECISIONS, W1 FAQ) with path, hash, timestamp, commit | R | 2 | Paths confirmed in `docs/audit/existing-projects.md` §1.2; workspace never written |
| F4 | Drift detection and review queue for imported assets | R | 2 | Workflow D |
| F5 | Allowlisted HTTP fetch of public pages (robots-aware, rate-limited, snapshot + hash) | O | 3 | **Disabled by default; activation needs explicit approval of the allowlist** |
| F6 | Google Trends and analytics CSV importers | O | 3/5 | Manual exports only |
| F7 | Search-provider adapter (Tavily/Brave/SerpAPI/Anthropic server tool) | O | 3+ | Only after pricing and terms verified |
| F8 | Scheduled execution via launchd plist generator (dry-run first) | O | 6 | Only after manual runs are stable |
| F9 | Telegram/email notifications | O | 6 | Needs bot/SMTP approval |
| F10 | Web dashboard / UI | X | — | Audit found no requirement; Markdown/HTML reports suffice |
| F11 | Crawling, scraping beyond allowlist, CAPTCHA/paywall bypass | X | — | Prohibited |

### 1.2 Agents and orchestration

| # | Feature | Class | Phase | Notes |
|---|---|---|---|---|
| F12 | Deterministic orchestrator: run state machine, step order, timeouts, bounded retries, budget enforcement, persistence, run summary | R | 2 | No agent-to-agent conversations, no recursion |
| F13 | Mock AI provider replaying fixtures (deterministic, zero cost) | R | 2 | Initial and default provider mode |
| F14 | Provider interface + cost ledger (usage from provider when available; "unmeasurable" flag otherwise) | R | 2 | Enables later adapters without orchestrator changes |
| F15 | Market Research Agent: findings from inputs, facts vs hypotheses, evidence links | R | 3 | Uses mock provider until a live provider is approved |
| F16 | Strategy Agent: scoring model (config-driven), rationale, missing evidence, experiment proposals | R | 3 | Score recomputed by code; model proposes dimension values + rationale |
| F17 | Structured output validation (JSON schema per agent; reject/repair bounded) | R | 2–3 | Failed validation = failed step, never a success report |
| F18 | Content and Campaign Agent (Indonesian briefs, brand checker, `[ISI: …]` placeholders, DRAFT state) | O | 4 | Interfaces defined in MVP; excluded to keep Phase 2–3 small (see §2) |
| F19 | Performance Analyst Agent on sample/imported aggregated data | O | 5 | Honest "cannot compute" when data absent; tracking plan |
| F20 | Anthropic Messages API adapter | O | 3+ | Only with API key and explicit billing approval; within USD 20/month ceiling |
| F21 | Subscription CLI (`claude -p`) adapter | X for now | — | Not enabled; see `docs/architecture/integrations.md` §3 (terms assessment) |

### 1.3 Storage, reports, safety, quality

| # | Feature | Class | Phase | Notes |
|---|---|---|---|---|
| F22 | SQLite store (`data/mia.sqlite`) with embedded migrations | R | 2 | Single machine; see `docs/architecture/data-model.md` |
| F23 | Markdown + static HTML reports (executive summary, findings with sources, opportunities, experiments, assumptions, costs, next steps) | R | 2–3 | Templates via Go `text/template` and `html/template` |
| F24 | Brand/claim checker ported from `brand-oracle.mjs` as data-driven rules + equivalence test on B1 cases | R | 3 | Required for flagging unsupported claims in findings and recommendations, not only briefs |
| F25 | Approved-facts check against `B4-fakta.json`; missing-fact flags | R | 3 | Any output mentioning address, hours, prices, guarantees is flagged |
| F26 | Structured logging (JSON) with secret redaction | R | 2 | `log/slog` |
| F27 | Configuration: YAML files + `.env` for secrets, validated at start | R | 2 | `config/{brand,business,marketing-policy,research-sources,agent-limits,scoring}.yaml` |
| F28 | Budget controls: per-run, per-agent, monthly ceiling (USD 20), request/token caps, retry caps, concurrency cap | R | 2 | Enforced in mock mode too (counts simulated usage) |
| F29 | Deduplication of sources and research items (URL canonicalization, content hash, normalized-title similarity) | R | 2–3 | Unit-tested |
| F30 | Approval records on opportunities (and later briefs) | R | 3 | Human decisions stored separately from model output |
| F31 | Unit, integration, and workflow tests; mock end-to-end test | R | 2–3 | `go test ./...` |
| F32 | Dashboard integration (any direction) | X | — | Out of scope until explicitly approved |
| F33 | Customer records, lead lists, PII of any kind | X | — | Prohibited by workspace rule 5 |

## 2. Why the Content Agent is deferred to Phase 4

Including it in Phase 2–3 would add: Indonesian generation prompts and few-shot examples, brief templates, a second approval flow, and more fixtures, roughly doubling the validation surface before the research/strategy core is proven. The MVP still ships the two pieces the Content Agent depends on, the brand checker (F24) and the facts check (F25), so Phase 4 adds only the agent and templates behind the existing `Agent` and `Provider` interfaces.

## 3. MVP boundaries in one table

| Dimension | MVP (Phases 2–3) | Later | Never |
|---|---|---|---|
| Inputs | Imported files, workspace documents | Allowlisted public pages, Trends/analytics CSV, search provider | Unapproved crawling, private data |
| Agents | Research, Strategy (mock provider) | Content, Performance; live Anthropic API | Autonomous publishers, agent swarms |
| Outputs | Markdown/HTML reports, ranked opportunities, experiment proposals | Indonesian briefs (DRAFT), performance reviews, notifications | Published content, sent messages, ad spend |
| Storage | SQLite, local files | Same | Warehouse, vector DB |
| Cost | USD 0 | ≤ USD 20/month hard ceiling | Unbounded or unmeasured spend |
| Interfaces | CLI | launchd scheduling | Web dashboard, dashboard/DB integration |

## 4. Acceptance reference

Testable criteria for every R item are in `docs/implementation/acceptance-criteria.md`; phase mapping is in `docs/implementation/roadmap.md`.
