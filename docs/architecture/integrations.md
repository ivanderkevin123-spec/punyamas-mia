# MIA — Integrations and Data Sources

**Status:** Phase 1 draft · 2026-10-12 · English with Indonesian summary. Starting point: `docs/audit/integrations.md` (Phase 0, historical). Nothing in this document is activated: **no fetching, no scraping, no credentials, no accounts**. Terms and pricing quoted below were fetched on 2026-10-12 and must be re-read by the owner before any activation.

## 0. Ringkasan (Bahasa Indonesia)

Pada MVP, sumber data MIA hanyalah **berkas impor** dan **dokumen ruang kerja Punyamas** (baca-saja). Integrasi lain (situs kompetitor, data tren, penyedia AI berbayar, analitik, iklan, notifikasi, data dashboard) adalah **kandidat** yang masing-masing memerlukan verifikasi syarat layanan, biaya, dan persetujuan pemilik. Untuk penyedia AI, dokumen resmi Anthropic yang dibaca hari ini menyatakan bahwa login langganan (OAuth) ditujukan untuk pemakaian biasa oleh individu, sedangkan pengembang yang membuat produk/layanan harus memakai **API key**; karena itu jalur **API dengan tagihan terpisah** (Opsi A) direkomendasikan bila MIA perlu AI nyata, dan jalur CLI langganan (Opsi B) **tidak** direkomendasikan untuk eksekusi otomatis. Daftar kandidat kompetitor di §5 adalah usulan untuk ditinjau manusia, bukan temuan terverifikasi.

Legend: **[V]** verified in Phase 0 or Phase 1 · **[V1]** verified in Phase 1 (today) · **[A]** assumption · **[C]** candidate, unverified · **[?]** unresolved.

## 1. Integration feasibility matrix

| # | Integration | Business value | Availability status | Authentication | Cost implications | Terms and restrictions | Data sensitivity | Technical complexity | MVP / phase | Fallback | Verification required before activation |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Local imported files (Markdown, CSV, JSON notes; manual competitor observations; interview notes) | Highest-quality evidence when the owner records field observations and interviews (level A) | Available [V] | None | None | None; owner must not import personal data | Low–medium (PII guard enforced) | Low | **MVP (Phase 2)** | — (primary input) | None |
| 2 | Punyamas workspace documents (`CONTEXT.md`, `DECISIONS.md`, `OPEN-QUESTIONS.md`, `docs/02-brand/B1-…`, `docs/02-brand/B4-fakta.json`, `tools/spec-oracle/brand-oracle.mjs`, `docs/05-marketing/M1-…`, `docs/05-marketing/W1-…`) | Brand rules, facts, claim policy, marketing plan, internal hypotheses | Available locally [V]; paths confirmed in `docs/audit/existing-projects.md` §1.2 | None | None | Read-only by policy; content changes without notice (owner's other sessions) | Internal business documents; no customer data | Low–medium (parsing `POLA` and JSON) | **MVP (Phase 2)** with drift detection | — | Confirm asset list in `config/research-sources.yaml` at Phase 2 gate |
| 3 | Approved public websites (competitor and education pages) | Competitor positioning, customer questions, terminology | Network reachable for declared hosts [V]; per-site access untested [?] | None | None (bandwidth) | robots.txt, site terms, copyright (store short excerpts + link), JS-rendered pages may be unreadable | Public | Medium (fetch, robots, extraction, snapshots) | Phase 3, **off by default** | Manual import of observations (row 1) | Per-source checklist §4; owner approval of allowlist |
| 4a | Google Trends (manual CSV export) | Relative search interest for terms like "jual emas", seasonality | Site reachable [V]; no official API [V] | Google account for export (owner) | None | Export by hand in the UI; automated access via unofficial libraries not proposed | Public aggregate | Low (CSV importer) | Phase 3 (import only) | — | None beyond owner doing the export |
| 4b | Search provider API (Tavily, Brave Search, SerpAPI) | Discover pages and questions at scale | No keys [V] | API key | Paid or freemium; **pricing not verified** [?] | Each provider's terms; result licensing | Public | Medium | Phase 3+ (optional) | Manual search + import | Pricing, terms, rate limits per provider |
| 4c | Anthropic server-side `web_search` tool (via the API) | Same as 4b without a second vendor | Requires API key (row 5a) | API key | **Verified [V1]:** USD 10 per 1,000 searches plus standard token costs for search results counted as input tokens; errors not billed | Anthropic Commercial Terms; `allowed_domains` restriction supported by the tool | Public | Low once 5a exists | Phase 3+ (optional) | Manual search | Owner approval; budget gate |
| 5a | AI provider — Anthropic Messages API (API key, separate billing) | Research synthesis, grouping, scoring proposals, drafting | Endpoint reachable [V]; **no key, no account verified** [?] | API key (Console) | **Verified [V1] prices** in `cost-model.md` §2; fits the USD 20 ceiling at planned volumes | Commercial Terms of Service; rate limits by usage tier (**not verified** [?]) | Prompts contain public/internal text only; no customer data | Medium (adapter + usage ledger) | Phase 3+ **only after approval** | Mock provider | Owner creates Console account and key; confirm tier/rate limits; record price-table date |
| 5b | AI provider — Claude Code CLI `claude -p` under the owner's subscription | Zero marginal cost on paper | Installed (2.1.226) [V] | OAuth login | Uses subscription quota; cost reported as a client-side estimate only | See §3: documented intent is ordinary individual use; programmatic/backend use should use API keys | Same as 5a | Low (subprocess) but fragile (CLI output changes) | **Not enabled** | Mock | Not proposed |
| 5c | Mock provider | Deterministic tests, zero cost | Available by design | None | USD 0 | None | None | Low | **MVP default** | — | None |
| 6 | Website analytics (GA4 or similar) | Traffic, pages, conversions for the company-profile site | No property exists: site unpublished, no domain (Q61) [V] | OAuth / service account | Free tier | Google terms; privacy policy on the site must disclose analytics | Aggregated visitor data (no PII in MIA) | Medium (API) or Low (CSV) | Phase 5 (CSV import first) | CSV export | Site live, property created, owner approval |
| 7 | Search performance analytics (Google Search Console) | Queries bringing visitors; question discovery | No property [V] | OAuth | Free | Google terms | Aggregate | Low (CSV) | Phase 5 (CSV import) | CSV export | Site live, verification of domain |
| 8 | Advertising performance (Meta, TikTok, Google Ads exports) | Cost per inquiry if paid media is ever used | No accounts; no paid media planned (M1 budgets `[ISI: …]`) [V] | Platform accounts | Ad spend (outside MIA's budget) | Platform terms | Aggregate | Low (CSV) | Phase 5 (CSV import) | — | Owner decision to run paid media at all |
| 9 | Telegram or email notifications (run summaries only) | Owner gets a message when a scheduled run finishes or fails | No bot token / SMTP [V] | Bot token or SMTP credentials | Telegram Bot API free; email depends on provider | Creating a bot is an external account action → approval | Run summaries only; no customer data | Low | Phase 6 | Read reports locally | Owner creates bot or provides SMTP; approval |
| 10 | Future operational dashboard data (aggregated inquiry/visit/transaction counts) | Business KPIs B1–B5 | Dashboard on another PC; no interface exists [V] | To be defined by the dashboard owner | None | Must exclude customer records; privacy review | Aggregates only | Low (CSV contract) | Phase 5+ **out of scope until explicitly approved** | Manual counts | Written export contract agreed with the dashboard owner |

## 2. Verified facts gathered in Phase 1 (2026-10-12)

| Fact | Source read today | Verbatim or close quotation |
|---|---|---|
| OAuth login is for ordinary individual use; developers should use API keys | Claude Code docs, *Legal and compliance* (`code.claude.com/docs/en/legal-and-compliance`) | "OAuth authentication is intended exclusively for purchasers of Claude Free, Pro, Max, Team, and Enterprise subscription plans and is designed to support ordinary use of Claude Code and other native Anthropic applications." / "Developers building products or services that interact with Claude's capabilities, including those using the Agent SDK, should use API key authentication through Claude Console or a supported cloud provider." / "Advertised usage limits for Pro and Max plans assume ordinary, individual usage of Claude Code and the Agent SDK." |
| Scripted calls are steered to API keys | Claude Code docs, *Run Claude Code programmatically* (`code.claude.com/docs/en/headless`) | "`--bare` is the recommended mode for scripted and SDK calls, and will become the default for `-p` in a future release." / "Set `ANTHROPIC_API_KEY` before running it, because bare mode doesn't use your subscription login." / JSON output includes `total_cost_usd` which is a "client-side estimate". |
| Consumer terms restrict automated access except via API key | Anthropic Consumer Terms of Service (effective October 8, 2025), as summarized by the fetch tool with quoted clauses | Section 3 prohibits accessing the Services "through automated or non-human means, whether through a bot, script, or otherwise" "Except when you are accessing our Services via an Anthropic API Key." **Owner should read the clause directly before relying on this summary.** |
| Current API prices | Anthropic pricing page (`platform.claude.com/docs/en/about-claude/pricing`) | See `cost-model.md` §2 |

## 3. AI provider options evaluated

| Criterion | A — Anthropic API with explicit billing | B — Subscription CLI (`claude -p`) | C — Mock-only |
|---|---|---|---|
| Current applicable terms | Commercial Terms; documented path for "developers building products or services" [V1] | Consumer Terms; documentation states OAuth is for "ordinary use" and recommends API keys for scripted/SDK calls; consumer terms restrict automated access except via API key [V1]. Automated scheduled backend use is therefore **not supported by the documented terms** | No terms issue |
| Authentication | API key in `.env` (0600) | OAuth session on the owner's machine; `--bare` mode explicitly ignores it | None |
| Scheduling restrictions | None beyond rate limits | Launchd-driven unattended use conflicts with "ordinary, individual usage" | None |
| Reliability | Stable versioned API; SDK available | CLI output format changes between versions (docs list many version-specific behaviours) | Deterministic |
| Cost predictability | Per-token prices verified; ledger from `usage`; hard ceiling enforceable | Cost "client-side estimate"; quota shared with the owner's interactive sessions (D-028) | USD 0 |
| Rate limits | By usage tier; **not verified for a new account** [?] | Subscription limits, shared | None |
| Model availability | Opus 5.5, Sonnet 5.5, Haiku 5.5 listed on the pricing page [V1] | Whatever the CLI exposes | Fixtures |
| Security | Key confined to `.env`; no tools given to the model | CLI has tools by default; must be restricted with flags; larger surface | None |
| Operational complexity | Adapter + ledger (Phase 3) | Subprocess management, parsing, version pinning | Lowest |
| **Assessment** | **Recommended when live AI is needed**, within the USD 20 ceiling | **Not recommended**; not enabled | **Recommended now (Phases 2–3)** |

Model choice under Option A is the owner's decision. The pricing page's own guidance is "Choose Haiku for simple tasks, Sonnet for most production workloads, and Opus for the most complex reasoning". `cost-model.md` shows that even Opus 5.5 fits the ceiling at the planned volumes; a cheaper tier reduces cost further. The adapter will make the model a configuration value.

## 4. Per-source approval checklist (applies to every web source before it enters the allowlist)

| # | Check | Recorded where |
|---|---|---|
| 1 | Source accessibility: plain HTTP GET returns readable text without JavaScript | `research-sources.yaml → web.allow[].access_check` with date |
| 2 | Applicable terms of use: read; automated access not prohibited; note the URL and date | `terms_note`, `terms_checked_at` |
| 3 | robots.txt: path allowed for a generic user agent; crawl-delay honoured | enforced at runtime; `robots_checked_at` |
| 4 | Rate limits: documented or polite default (≤ 1 request per 10 s per host; ≤ 20 pages per run) | `max_requests_per_hour`, `max_pages_per_run` |
| 5 | Copyright and permitted reuse: store short excerpts (≤ 1,000 chars) with attribution; never republish | design rule; `excerpt_policy` |
| 6 | Authentication requirements: none (login-gated sources are rejected) | `requires_auth = false` |
| 7 | Data privacy: no personal data of third parties (reviews naming people are not stored verbatim) | PII guard + `privacy_note` |
| 8 | Source attribution: canonical URL, publisher, retrieval and publication dates stored | `sources` table |
| 9 | Technical feasibility: tested fetch + extraction on one page, recorded | `feasibility_checked_at` |
| 10 | Preference order: official API or manual export > allowlisted fetch > nothing | `collection_method` |

## 5. Proposed competitor and source research shortlist (for human review)

All entries are **[C] candidates**: MIA has not accessed any of them; no features, prices, offers, or market positions are claimed. "Public source to verify" names where the owner (or Phase 3, if approved) would look. Indonesian labels are the ones that will appear in reports.

### 5.1 Pesaing langsung (direct competitors: buy gold or jewellery from the public in Indonesia / Surabaya)

| Candidate | Why it matters to Punyamas | Public source to verify | Proposed collection method |
|---|---|---|---|
| PT Pegadaian (state-owned; its public pages describe gold-related retail services; whether and how it buys gold from the public is to be verified) [C] | Largest trusted brand customers compare against; positioning on official status is exactly what Punyamas may not claim (STATUS category) | Official website and its gold-services pages; physical outlets in Surabaya | Allowlisted fetch of public pages after checklist; manual outlet observation |
| PT Aneka Tambang — Logam Mulia (certified bars; a publicly advertised buyback programme is to be verified) [C] | Reference point for buyback transparency and education content; relevant to AU1 holders of bars | Official Logam Mulia website buyback/education pages | Allowlisted fetch after checklist |
| Toko emas tradisional di Surabaya (jewellery shops in market clusters; exact names to be collected in the field) [C] | The actual alternative for AU1/AU2 sellers; pricing practices, trust cues, signage | Field visits; Google Maps listings (read manually; automated collection of Maps content is not proposed because platform terms restrict it [A]) | Owner's manual observation notes imported as level-A evidence |
| Online "jual emas / beli emas" services with physical buyback in East Java (to be identified) [C] | Convenience and process transparency comparisons | To be identified by the owner's manual search; each becomes a checklist entry | Manual first |

### 5.2 Layanan emas yang berdekatan (adjacent gold-related services)

| Candidate | Why | Public source to verify | Method |
|---|---|---|---|
| Gold pawn services (Pegadaian gadai emas; sharia banks offering gadai emas) [C] | Alternative for people who need cash but want to keep their gold; informs messaging about "selling vs. pawning" (note: Punyamas copy itself may not use the word *gadai*, KEUANGAN category) | Official pages of the institutions | Allowlisted fetch after checklist |
| Digital gold platforms (e.g. platforms that sell and buy back digital gold with physical withdrawal) [C] | They shape expectations about spread, transparency, and instant pricing | Each platform's public FAQ/pricing pages | Allowlisted fetch after checklist; names to be confirmed by owner |
| Jewellery manufacturers and brands based in or selling into Surabaya (e.g. a Surabaya-based manufacturer is believed to exist; name and role to verify) [C] | Supply side and brand presence in the city; possible AU3 buyers rather than competitors | Company websites | Manual review |

### 5.3 Pesaing pencarian dan konten (search and content competitors)

| Candidate | Why | Public source to verify | Method |
|---|---|---|---|
| Finance/news media articles answering "cara jual emas", "harga emas hari ini", "jual emas di mana" (national outlets) [C] | They own the search results Punyamas's education pages would compete with | Search results for the Indonesian queries; article URLs recorded | Manual search with recorded queries; allowlisted fetch of specific articles after checklist |
| Education pages of the institutions in §5.1–5.2 [C] | Direct content competitors for AU1 questions | Their education/FAQ sections | Allowlisted fetch |
| YouTube and TikTok content on selling gold (creators and institutions) [C] | Format and question discovery | Manual viewing; no automated collection | Manual notes only |

### 5.4 Sumber wawasan pelanggan non-pesaing (non-competitor sources of customer insight)

| Candidate | Why | Source | Method |
|---|---|---|---|
| Owner's field interviews and WhatsApp/visit questions (after launch) [C] | Highest evidence quality (A); directly tests the FAQ hypotheses | Owner's notes (no personal data) | Import (Markdown template) |
| Google Trends for "jual emas", "harga emas", "toko emas Surabaya", "jual perhiasan bekas" [C] | Seasonality and relative interest | Trends UI | Manual CSV export → import |
| Public forums and communities discussing selling gold in Indonesian (e.g. large Indonesian forums and subreddits) [C] | Fears and questions in customers' own words (level C evidence) | Public threads | Manual reading and excerpting; allowlisted fetch only if robots/terms permit |
| Marketplace listings and reviews of gold jewellery (manual reading) [C] | Language, terminology, price anchoring (not for claims) | Public listings | Manual notes; no scraping |
| Regulator and industry references for context, labelled VERIFY (trade ministry/commodity regulator pages on physical gold trade; statistics office) [C] | Context for education content; must carry VERIFY labels per workspace rule 1 | Official government pages | Allowlisted fetch after checklist |

## 6. Proposed `config/research-sources.yaml` shape (conceptual)

```yaml
workspace:
  root: ~/Documents/bots/punyamas      # read-only
  assets:
    - key: context      ; path: CONTEXT.md
    - key: decisions    ; path: DECISIONS.md
    - key: b1           ; path: docs/02-brand/B1-arah-dan-strategi-merek.md
    - key: facts        ; path: docs/02-brand/B4-fakta.json
    - key: brand_oracle ; path: tools/spec-oracle/brand-oracle.mjs
    - key: m1           ; path: docs/05-marketing/M1-rencana-pra-peluncuran.md
    - key: w1           ; path: docs/05-marketing/W1-naskah-situs-company-profile.md
web:
  enabled: false                      # requires approval record to flip
  allow: []                           # filled only after §4 checklist per host
  block: ["*/login*", "*/checkout*", "*/account*"]
  defaults: { max_requests_per_hour: 60, max_pages_per_run: 20, request_interval_s: 10, max_bytes: 2000000 }
```

## 7. Unresolved

- **[?]** Whether the owner will open an Anthropic Console account (Option A) and when; rate-limit tier.
- **[?]** Which §5 candidates the owner approves for Phase 3, and the exact URLs after the checklist.
- **[?]** Search provider choice and pricing (4b), if any.
- **[?]** Notification channel (Telegram vs email) for Phase 6.
- **[?]** Dashboard export contract (row 10), only when the dashboard owner is ready.
