# MIA — Cost Model

**Status:** Phase 1 · 2026-10-12. Hard ceiling for external operating cost: **USD 20 per month** (approved). No paid call was made in Phase 0 or Phase 1. Prices in §2 were read today from Anthropic's pricing page; everything in §3 marked **[A]** is an illustrative workload assumption to be replaced by measured usage.

## 0. Ringkasan (Bahasa Indonesia)

- **Skenario A (lokal, tanpa panggilan berbayar):** biaya eksternal **USD 0**. Ini mode awal MIA.
- **Skenario B (riset langsung terbatas + AI lewat API):** dengan asumsi 4 run penuh per bulan, perkiraan **USD 0,1 sampai 3,5 per bulan** tergantung model, plus biaya pencarian bila dipakai (USD 10 per 1.000 pencarian untuk alat pencarian Anthropic; penyedia lain belum diverifikasi).
- **Skenario C (multi-agen terjadwal):** perkiraan **USD 0,3 sampai 16 per bulan** tergantung model dan jumlah pencarian; model termahal mendekati plafon, sehingga plafon USD 20 ditegakkan otomatis sebelum setiap panggilan.
- Harga per token diverifikasi 2026-10-12; volume token masih **asumsi** sampai diukur pada run nyata. Tagihan Anthropic dalam USD; tidak ada konversi ke rupiah yang diasumsikan.

## 1. Cost categories

| Category | Applies in | Measured how |
|---|---|---|
| LLM tokens (input, cache write, cache read, output) | B, C | Provider `usage` per call → `cost_records` |
| Web search tool calls (Anthropic server tool) or third-party search API | B, C (optional) | Count per call; price table |
| Web fetching bandwidth | B, C (if approved) | Not billed by anyone; bounded by size caps |
| Notifications (Telegram free; email provider varies) | C | Provider statement |
| Owner time (review, approvals, imports) | A, B, C | Not in the USD ceiling; tracked by KPI O1/O5 |
| Electricity and the existing machine | A, B, C | Not counted |

## 2. Verified price table (read 2026-10-12 from `platform.claude.com/docs/en/about-claude/pricing`; the page shows no date, so the fetch date is the verification date)

| Model (API id) | Base input / MTok | Cache write 5 min / 1 h | Cache hit / MTok | Output / MTok | Batch input / output / MTok |
|---|---|---|---|---|---|
| Claude Opus 5.5 (`claude-opus-5-5`) | USD 4 | USD 5 / USD 8 | USD 0.20 | USD 20 | USD 2 / USD 10 |
| Claude Sonnet 5.5 (`claude-sonnet-5-5`) | USD 2 | USD 2.50 / USD 4 | USD 0.10 | USD 10 | USD 1 / USD 5 |
| Claude Haiku 5.5 (`claude-haiku-5-5`), prompts ≤ 100,000 tokens | USD 0.10 | USD 0.125 / USD 0.20 | USD 0.01 | USD 0.50 | USD 0.05 / USD 0.25 |
| Claude Haiku 5.5, prompts > 100,000 tokens | USD 0.50 | USD 0.625 / USD 1 | USD 0.05 | USD 2.50 | USD 0.25 / USD 1.25 |

Other verified statements from the same page: the Batch API gives "a 50% discount on both input and output tokens"; web search is "available on the Claude API for $10 per 1,000 searches, plus standard token costs for search-generated content"; errors during web search are not billed. Not extracted: web fetch tool pricing (listed on the same page; verify before use). Not verified: rate limits and usage tier for a new account; third-party search provider prices; whether prices change after today.

The price table will be stored in `config/agent-limits.yaml → price_table` with `verified_at: 2026-10-12` and used only for pre-call estimates and for calls whose provider does not report cost; the ledger prefers provider-reported usage × table prices.

## 3. Formulas

```
cost_call  = in_tokens × p_in + cache_write_tokens × p_cw + cache_read_tokens × p_cr
           + out_tokens × p_out + searches × p_search
cost_run   = Σ cost_call (over agent executions in the run) × retry_factor
cost_month = Σ cost_run (runs in the calendar month, WIB) + notifications
allow(call) = cost_month_so_far + cost_run_so_far + estimate(call) ≤ min(ceiling_month, ceiling_run, ceiling_agent)
estimate(call) = est_in_tokens × p_in + max_out_tokens × p_out (+ p_search if the call may search)
```

Required inputs to compute any scenario exactly: the four token counts per call (from `usage`), number of searches, chosen model, retry factor (measured O6), runs per month.

## 4. Workload assumptions [A] (replace with measurements after the first live runs)

| Unit | Input tokens | Output tokens | Notes |
|---|---|---|---|
| Research agent, full run (≈ 20 sources) | 55,000 | 6,000 | Includes ~6k static prefix (instructions + config) |
| Strategy agent (grouping + scoring + gaps) | 25,000 | 7,000 | Two calls |
| Content agent, 3 briefs (Phase 4) | 25,000 | 9,000 | Three calls |
| Daily digest (Phase 6) | 30,000 | 4,000 | One call |
| Full run total (research + strategy) | 80,000 | 13,000 | |
| Retry factor | ×1.5 (B), ×1.2 (C) | | Covers repairs and retries until measured |
| Searches, if enabled | 20 per full run; 5 per daily digest | | Each search also adds ≈ 2,000 input tokens of results [A] |

## 5. Scenario A — Local-only, zero paid calls (Phases 2–3 default)

| Item | Value |
|---|---|
| Provider | Mock (fixtures); no network code path |
| Research inputs | Imported files, workspace documents |
| External cost | **USD 0** (guaranteed by design: the budget gate still runs with simulated usage so limits are tested) |
| Ceiling behaviour | Not reachable |
| Trade-off | No new external knowledge; synthesis quality limited to fixtures; good for building, testing, and validating evidence rules |

## 6. Scenario B — Limited live research and AI (manual or bounded runs, after approval)

Assumptions: 4 full runs/month, retry factor 1.5, optional 80 searches/month.

| Cost line | Haiku 5.5 | Sonnet 5.5 | Opus 5.5 | Formula |
|---|---|---|---|---|
| Tokens: 4 × 80k in, 4 × 13k out, × 1.5 | USD 0.09 | USD 1.74 | USD 3.48 | (320k × p_in + 52k × p_out) × 1.5 |
| Searches (if enabled): 80 × USD 0.01 | USD 0.80 | USD 0.80 | USD 0.80 | searches × 0.01 |
| Search result tokens: 80 × 2k = 160k in | USD 0.02 | USD 0.32 | USD 0.64 | 160k × p_in |
| **Total without search** | **≈ USD 0.09** | **≈ USD 1.74** | **≈ USD 3.48** | |
| **Total with Anthropic web search** | **≈ USD 0.91** | **≈ USD 2.86** | **≈ USD 4.92** | |

Ceilings proposed: `ceiling_month = 20`, `ceiling_run = 2.00`, `ceiling_agent = 1.00`, alert at 80 % of month (USD 16). Unknowns: third-party search pricing; rate-limit tier. Trade-off: human review of every run; no automation. Budget exhaustion: the run stops before the call that would exceed any ceiling; the report lists skipped steps.

## 7. Scenario C — Expanded multi-agent operation (Phase 6+, after approval)

Assumptions: 30 daily digests, 4 weekly full runs, 4 weekly content runs, retry factor 1.2, optional 230 searches/month.

| Cost line | Haiku 5.5 | Sonnet 5.5 | Opus 5.5 | Formula |
|---|---|---|---|---|
| Tokens: 1.32M in, 208k out, × 1.2 | USD 0.28 | USD 5.66 | USD 11.33 | (1.32M × p_in + 208k × p_out) × 1.2 |
| Searches: 230 × USD 0.01 | USD 2.30 | USD 2.30 | USD 2.30 | |
| Search result tokens: 460k in | USD 0.05 | USD 0.92 | USD 1.84 | |
| Prompt caching saving on the 6k static prefix (≈ 38 calls) | −USD 0.02 | −USD 0.40 | −USD 0.80 | 38 × 6k × (p_in − p_cr) minus cache writes |
| **Total without search** | **≈ USD 0.28** | **≈ USD 5.3** | **≈ USD 10.5** | |
| **Total with search** | **≈ USD 2.6** | **≈ USD 8.5** | **≈ USD 14.7** | |

Opus 5.5 with search approaches the ceiling; the monthly gate will refuse calls past USD 20 regardless of schedule, and the daily digest is the first job to be skipped (configurable priority). Trade-offs: more automation, more review load, more drift exposure; notifications needed so failures are seen. Batch API (50 % off) is applicable to the daily digest if latency does not matter, roughly halving its token cost; not assumed above.

## 8. Budget-exhaustion and failure behaviour (all scenarios)

1. Pre-call gate refuses any call whose estimate would breach a ceiling; the step ends `budget_exhausted`; the run ends `partial` (optional steps) or `failed` (required steps); the report says what was not done.
2. Provider errors (`auth`, `billing`) stop the run with no retry; `rate_limit`/`timeout` retry at most 2 times; retries count toward the ledger only when the provider bills them (errors in web search are not billed, per the page).
3. If usage cannot be measured (provider without usage data), the estimate is booked with `estimated = true`, the summary prints *"Biaya tidak dapat diukur; estimasi digunakan"*, and the ceiling applies to estimates.
4. Monthly ledger resets on the first day of the month (WIB); a report `reports/cost/<YYYY-MM>.md` lists spend by run, agent, and category.

## 9. Verification requirements before any paid call

- Owner confirms an Anthropic Console account and key exist (Option A) and the billing method.
- Re-read the pricing page; update `price_table.verified_at`.
- Confirm rate limits for the account tier.
- Run one bounded smoke test with `ceiling_run = 0.10` and compare provider-reported usage with the estimate.
