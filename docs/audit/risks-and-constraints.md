# Audit: Risks and Constraints

**Audit date:** 2026-10-12 (WIB). Constraints are taken from the master prompt and from the Punyamas workspace (`bots/punyamas`: CONTEXT.md, DECISIONS.md, OPEN-QUESTIONS.md, B1, M1). Risks are qualitative (likelihood × impact: L/M/H) and will be re-scored at each phase gate.

Legend: **[V]** verified · **[A]** assumption · **[?]** unresolved question.

## 1. Business constraints (binding on every MIA output)

| ID | Constraint | Source | Effect on MIA |
|---|---|---|---|
| BC1 | Business is in **pre-launch**: PT not founded, no store address, no operations | CONTEXT.md; B4 FB1, FB4 [V] | All content uses intent language ("kami akan…"); no claims of current operation |
| BC2 | Name **Punyamas has not passed a trademark check**; irreversible spending (signage, print, long-term domain, social handles, PT name) deferred until the trademark agent finishes (N68) | D-019, D-028 [V] | Briefs are "concepts that can be renamed"; MIA never recommends buying domains/handles or publishing |
| BC3 | **No public activity** before the 7 preconditions in M1 §2 (name safe, domain/accounts owned, PT data, brand text check, tagline approved, promo-contact consent, CEO approval of text/assets) | M1 [V] | Every brief carries a precondition checklist; campaign plans are sequenced after milestones L03/L06/L09/L21/L40 |
| BC4 | **No buying prices, price comparisons, or superlatives** in public text; no testimonials or performance figures before opening | B1 CT7, PM table, D-032 [V] | Content agent blocks these; checker enforces 9 categories |
| BC5 | **Banned-claim categories**: SUPERLATIF, PERINGKAT, ABSOLUT, BERTENTANGAN_KEBIJAKAN, KECEPATAN_UANG, KEUANGAN, STATUS, INSTITUSI (version `v0.1-ASUMSI`, final by advertising-law consultant N70) | B1 §9, `brand-oracle.mjs` [V] | Rules are versioned data in `config/marketing-policy.yaml`; drift check against the source file |
| BC6 | Identity recorded for **every** transaction (KYC); never market "tanpa KYC/KTP" | D-013, B1 LK2 [V] | Content explains identity recording as protection (CT3) |
| BC7 | **Separate, revocable consent** before any promotional contact | M1 §2 item 6; T1-#4 D16 [V] | MIA never generates outreach lists or messages to individuals; it may draft consent-collection copy |
| BC8 | First store in **Surabaya**; address, hours, contact unknown | D-016, Q2, M1 unknowns [V] | `[ISI: …]` placeholders; geography for research scoping |
| BC9 | Tone KP1–KP4 (calm, clear, meticulous, humble); Bapak/Ibu, kami/Anda; units g, permil, Rp; no emoji/slang | B1 §7 [V] | Style rules in `config/brand.yaml`; validation in content agent |
| BC10 | Legal/tax/regulatory statements must be sourced and labelled **VERIFY** | CONTEXT.md rule 1 [V] | Research items citing regulation get `requires_verify: true` |
| BC11 | **No real customer personal data** in documents, prompts, marketing, or repos | CONTEXT.md rule 5; repo `.gitignore` [V] | Schema has no PII fields; importers reject PII-like columns |
| BC12 | AI prepares drafts only; no publishing, sending, payment, or transaction without CEO approval | CONTEXT.md rule 6; master prompt §12 [V] | Approval states in the store; no outbound side effects in MVP |
| BC13 | Customer-facing language is Indonesian; English allowed for engineering docs | master prompt §2 [V] | Templates and checker are Indonesian; code/docs English unless the user decides otherwise [?] |

## 2. Technical constraints

| ID | Constraint | Status |
|---|---|---|
| TC1 | Go on PATH is 1.17.5 (x86_64); the usable toolchain is `~/sdk/go1.27.2/bin/go` (arm64), selected via Makefile variable | [V] |
| TC2 | No application-level AI credentials; `~/.config/anthropic` empty; only the subscription-based Claude Code CLI exists | [V] |
| TC3 | Default `node` is v10; any Node tooling must `nvm use 22` explicitly (a reason to avoid Node for MIA's runtime) | [V] |
| TC4 | Sandboxed development session: local port binding, `sysctl`, `crontab`, Docker socket and Redis connections are blocked; outbound HTTP only to declared hosts. Tests must not depend on binding ports or on external services | [V] |
| TC5 | Ports 5000, 6379, 8791 are in use; 8765–8784 reserved by the Punyamas workspace | [V] |
| TC6 | `bots/punyamas` has 16 active Claw-Empire worktrees; it is read-only for MIA | [V] |
| TC7 | The operational dashboard is on another machine; no API or DB is reachable | [V] |
| TC8 | New Go modules must be fetched from `proxy.golang.org` (reachable) after dependency approval; `modernc.org/sqlite` and `go.yaml.in/yaml/v3` are already in a local cache | [V] |

## 3. Legal and compliance constraints

| ID | Item | Status |
|---|---|---|
| LC1 | Indonesian consumer-protection and advertising ethics (UU 8/1999 Art. 9 and 17; Etika Pariwara) underlie the banned-claim list; final list pending consultant (N70) | [V] as cited in B1; [A] until professional review |
| LC2 | Personal-data protection policy exists in the workspace (`docs/01-foundation/04-*`); MIA must not create new personal-data processing | [V] |
| LC3 | Web research must respect `robots.txt`, site terms, and copyright: store excerpts, not full copies, and link to sources | [A] policy; per-site terms [?] |
| LC4 | Trademark status of the name is unverified; MIA outputs must not create reliance on the name | [V] |
| LC5 | Anthropic subscription terms for automated/application use of `claude -p` must be verified by the user before that adapter is enabled | [?] |

## 4. Security constraints (from master prompt §12 and workspace rules)

- Secrets only in `.env` (git-ignored); `.env.example` with names only; logs redact tokens and any value matching key/token patterns.
- Agents have no shell, no filesystem write outside `reports/`, `drafts/`, and the SQLite file; the application, not the model, decides what to fetch.
- Retrieved content is untrusted; instructions inside it are never followed (see `integrations.md` §5).
- Human approval is a stored state transition (`draft → reviewed → approved`), required before any content is considered usable; MVP has no publish/send capability at all.
- No connection to the operational dashboard without explicit authorization; no customer or transaction records in MIA.
- Development stays in `~/Documents/punyamas/mia`; nothing is installed system-wide.

## 5. Cost constraints

- Mock mode must be fully functional with zero paid calls.
- Per-run, per-agent, and monthly token/cost ceilings in `config/agent-limits.yaml`; runs abort cleanly when exceeded.
- Cost ledger per run from provider `usage`; explicit warning when cost is not measurable (CLI adapter).
- Cost model scenarios (from the Phase 0 plan, prices to be re-verified): local-only $0; manual live runs ≈ $0.08–$3 per month depending on model tier; scheduled daily+weekly ≈ $0.25–$10 per month, excluding any search-provider fees (unknown).
- Monthly budget ceiling value: **[?]** to be set by the user.

## 6. Risk register

| ID | Risk | L | I | Mitigation | Owner |
|---|---|---|---|---|---|
| R1 | Brand/business config in MIA drifts from `bots/punyamas` (rules, facts, decisions change there) | H | M | `source_of_truth` metadata; `mia config check` diff against `brand-oracle.mjs` and `B4-fakta.json`; re-sync step at every phase gate | MIA |
| R2 | Generated content contains banned claims, invented facts, prices, or operational claims | M | H | Ported checker with equivalence tests; `[ISI: …]` placeholders; facts limited to B4 list; DRAFT state; human approval required | MIA + CEO |
| R3 | Fabricated or unverifiable research (invented sources, quotes, statistics) | M | H | Every item requires source + URL/file + retrieval timestamp + excerpt; items without evidence are typed `hypothesis`; schema validation rejects citations without a stored snapshot | MIA |
| R4 | Prompt injection via fetched pages | M | M | Untrusted-content policy; no tools for the model; suspicious-content detector; excerpts rendered as quotes only | MIA |
| R5 | Unexpected AI spend | L | M | Budgets, retry caps, caching, dedup, cost ledger, monthly ceiling, mock default | MIA + CEO |
| R6 | Subscription CLI used outside its terms | M | H | Adapter disabled by default; enabled only after the user verifies terms; API-key path preferred for automation | CEO |
| R7 | Web scraping breaches site terms or robots rules | M | M | Allowlist off by default; robots check; polite rate; manual import fallback | MIA + CEO |
| R8 | Marketing activity started before preconditions (name clearance, consent) are met | L | H | Every brief carries the M1 precondition checklist; MIA has no publishing capability | CEO |
| R9 | Scope creep toward CRM/dashboard/publisher | M | M | Out-of-scope list in `docs/product/mvp-scope.md`; phase gates | CEO + MIA |
| R10 | Toolchain confusion (Go 1.17 on PATH, node v10 default) breaks builds or launchd jobs | M | L | Makefile `GO` variable; launchd plist uses the absolute binary path; no Node runtime dependency | MIA |
| R11 | Sandbox limitations hide integration failures until the user runs the binary | M | L | Tests avoid ports/services; smoke tests documented for the user to run outside the sandbox | MIA |
| R12 | No performance data exists for a long time (store not open) | H | L | Performance agent reports "cannot compute" honestly; sample dataset for testing; tracking plan prepared in advance | MIA |
| R13 | Accidental writes into `bots/punyamas` or its worktrees | L | H | No code path writes there; verification compares `git status` before/after each phase | MIA |
| R14 | Indonesian text quality from the model (register, Bapak/Ibu, units) below brand standard | M | M | Style rules in prompts and post-validation; CEO review; examples CT1–CT10 as few-shot references | MIA + CEO |

## 7. Assumptions

- **[A]** The user (CEO) is the sole approver and the only operator of MIA during the MVP.
- **[A]** No paid advertising will run during the MVP period; performance data, if any, will come from WhatsApp Business and site analytics exports after launch.
- **[A]** The banned-claim list will be revised by a consultant; MIA treats it as versioned configuration.
- **[A]** Reports are consumed locally (Markdown/HTML files opened by the user); no hosting.

## 8. Unresolved questions

- **[?]** Monthly budget ceiling and default model tier.
- **[?]** AI provider path (API key vs subscription CLI vs mock only).
- **[?]** Whether live web fetching is allowed in Phase 3 and the initial allowlist.
- **[?]** Engineering documentation language (English vs Indonesian).
- **[?]** Git initialization timing and commit identity for `mia`.
