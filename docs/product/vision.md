# MIA — Product Vision

**Status:** Phase 1 draft for owner review · 2026-10-12 · English (engineering); ringkasan bisnis dalam bahasa Indonesia di bagian 0.
**Inputs:** `docs/audit/*.md` (Phase 0, historical), the approved Phase 1 decisions (Go 1.27.2 native toolchain, bilingual docs, mock-only AI, no live fetching, read-only `bots/punyamas`, USD 20/month ceiling, separate `mia` git repository).

## 0. Ringkasan untuk pemilik usaha (Bahasa Indonesia)

MIA (Marketing Intelligence Agent) adalah **alat bantu internal** untuk Punyamas: ia mengumpulkan bukti dari sumber yang disetujui, menyusun temuan riset pasar yang selalu merujuk sumbernya, memeringkat peluang pemasaran dengan rumus yang terbuka, dan menyiapkan draf brief kampanye berbahasa Indonesia yang mematuhi aturan merek yang sudah ada di ruang kerja `bots/punyamas`. MIA **bukan** dashboard operasional, bukan CRM, bukan sistem transaksi, dan tidak pernah menerbitkan, mengirim pesan, atau membelanjakan uang sendiri. Semua keluaran berstatus **draf** sampai disetujui CEO. Pada tahap awal MIA berjalan **tanpa biaya API** (mode mock); biaya eksternal, bila nanti disetujui, dibatasi **USD 20 per bulan** sebagai plafon keras.

## 1. Vision

MIA is an internal marketing intelligence and decision-support system that turns approved research inputs into evidence-backed findings, transparently ranked opportunities, measurable experiment proposals, and brand-safe draft briefs, so that Punyamas can make marketing decisions faster, more consistently, and with less manual research, while every claim stays traceable to its source or is explicitly labelled a hypothesis.

## 2. Business problem

| Problem | Evidence from the audit | Consequence without MIA |
|---|---|---|
| Marketing for a pre-launch gold-buying business must be built on trust and transparency, with strict claim rules (9 banned categories) and CEO approval of every public text | `bots/punyamas` B1 §9, D-005, M1 §2 (audit `existing-projects.md` §1.2) | Hand-written content is slow to check and easy to get wrong |
| Customer needs, questions and competitor positioning are currently **hypotheses** (the FAQ and audiences were drafted internally, not researched externally) | B1 §15 research limits; W1 FAQ is internal copy (audit `existing-projects.md`) | Campaigns risk being built on untested assumptions |
| Research done by AI employees so far has no persistent evidence store; findings live in prose documents | Workspace structure (audit `existing-projects.md` §1.2, §1.4) | Research is repeated, provenance is lost, drift goes unnoticed |
| Marketing budget and channels are undecided; spending must stay small and measurable | M1 costs all `[ISI: …]`; Q61 open; USD 20/month ceiling (approved) | Any tool that spends freely or cannot report cost is unacceptable |

## 3. Intended users

| User | Role with MIA | Language |
|---|---|---|
| CEO / owner (primary, sole operator during MVP) | Runs research, reviews findings, approves opportunities and briefs, sets budgets | Indonesian summaries and drafts; English technical docs acceptable |
| Future marketing staff ("Tata (Marketing)" role, human or AI-assisted) | Prepares imports, reads reports, drafts briefs from approved opportunities | Indonesian |
| Engineering (small team, initially one person) | Operates, extends providers and importers, maintains tests | English |

## 4. Primary use cases

1. **Manual research run** from imported notes/files and, later, approved sources → evidence-backed research report (Workflow A).
2. **Opportunity prioritization** with a documented scoring model and written rationale (Workflow B).
3. **Brief preparation** (later phase) from an approved opportunity into an Indonesian draft that passes the brand checker and lists missing facts (Workflow C).
4. **Source drift detection** for the imported brand rules, facts, claim policy and marketing plan (Workflow D).
5. **Honest failure reporting** for unavailable sources, missing providers, timeouts, budget exhaustion, invalid outputs (Workflow E).
6. **Performance analysis** (later phase) on approved, aggregated campaign data without storing customer records (Workflow F).

## 5. Business outcomes (measured, not promised)

| Outcome | How it is observed | Caveat |
|---|---|---|
| Less manual research time per report | Operational KPI O1 (time to prepare a research report) vs. a manual baseline the owner records | Baseline must be measured first |
| Higher evidence quality in marketing decisions | O2 (% findings with valid source references), O3 (% reports passing validation) | Quality of sources is bounded by what is approved |
| Faster path from opportunity to brief | O5 (time from opportunity selection to brief readiness) | Only after Phase 4 |
| Brand-safe content by construction | O9 (% drafts passing the claim checker on first attempt) | Checker filters words, not meaning; human review stays mandatory |
| Marketing effectiveness (inquiries, cost per inquiry, conversion) | Business KPIs B1–B5 | **Unavailable** until the store opens and data sources are approved; MIA will not claim revenue impact without evidence |

## 6. Non-goals

- Operational dashboard, POS, transaction records, inventory, pricing engine.
- CRM, lead lists, customer profiles, WhatsApp mass messaging.
- Autonomous publishing, ad buying, bidding, or sending of any external communication.
- Gold price prediction or investment advice (also forbidden by brand rule KEUANGAN).
- Multi-tenant SaaS, web dashboard, vector database, distributed infrastructure.
- Replacing the Punyamas workspace documents or the Claw-Empire workflow.

## 7. Assumptions

- **[A]** The CEO is the only approver and operator during the MVP.
- **[A]** The store opens later than the MVP; performance data will not exist for months.
- **[A]** Research inputs during Phases 2–3 are imported files and workspace documents; live fetching, if approved, comes in Phase 3 at the earliest.
- **[A]** Indonesian marketing text generated by a model needs human editing; MIA reduces, not removes, that work.
- **[A]** The banned-claim list will change after the advertising-law consultant review (N70); MIA treats it as versioned input.

## 8. Success criteria for the MVP (Phase 2–3)

1. A complete research → strategy run finishes in mock mode with zero paid calls and produces a readable Markdown/HTML report.
2. 100 % of research items in the report carry a source reference, retrieval timestamp and evidence-quality label; items without external evidence are labelled hypotheses.
3. Every ranked opportunity shows its score breakdown and rationale; the score is recomputed by deterministic code from validated inputs.
4. Imported workspace assets are tracked with path, hash and timestamp; a changed source is detected and flagged for review.
5. Budget, retry and timeout limits are enforced and visible in the run summary; cost is recorded or explicitly reported as unmeasurable.
6. The `bots/punyamas` working tree is unchanged after every run and every phase.

## 9. How MIA differs from the Punyamas dashboard

| Aspect | Punyamas operational dashboard (separate project, other PC) | MIA |
|---|---|---|
| Purpose | System of record for transactions, stock, queue, attendance, members; future CRM | Marketing research, prioritization, drafting, evaluation |
| Data | Customers, items, prices, payments (personal and financial data) | Public sources, imported notes, aggregated campaign metrics; **no customer records** |
| Actions | Executes business operations | Produces drafts and reports; **no external actions** |
| Integration | None in the MVP; later a file-based export of aggregated marketing metrics agreed with the dashboard owner | Reads exports only; never writes to the dashboard |
| Users | Staff, members, CEO | CEO and marketing |

MIA supports marketing by making evidence, priorities and drafts explicit; it does not store leads, run campaigns or talk to customers.
