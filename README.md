# MIA — Punyamas Marketing Intelligence Agent

Internal marketing-intelligence and decision-support tool for Punyamas (an Indonesian gold-buying business in pre-launch). MIA turns approved research inputs into evidence-backed findings, ranks opportunities from validated evidence, and (in a later phase) drafts brand-safe Indonesian briefs. It is **not** a dashboard, CRM, or publisher, and it never performs external actions.

**Status:** Phase 5 (workflow orchestration and quality gate) implemented on top of Phases 2–4. Provider mode is **mock only**: no paid API calls, no live web fetching, and nothing is ever published, sent or scheduled. The Punyamas workspace at `~/Documents/bots/punyamas` is read **read-only** with provenance and drift detection; approved briefs and content drafts are exported to `drafts/` (always labelled DRAF) and handed over manually.

Documentation: `docs/audit/` (Phase 0), `docs/product/`, `docs/architecture/` (Phase 1; Phase 3: `strategy-agent.md`; Phase 4: `content-agent.md`; Phase 5: `workflow.md`), `docs/implementation/` (roadmap, acceptance criteria, phase reports), `docs/phase1-decision-memo.md` (Indonesian), `docs/implementation/operations.md` (how to run).

## Requirements

- Go **1.27.2 darwin/arm64** at `~/sdk/go1.27.2` (the Go 1.17 on `PATH` is not used). Verify with `make go-version`.
- No services, no network, no credentials are needed in mock mode.

## Quick start

```bash
make go-version        # must print go1.27.2 darwin/arm64
make test              # unit, integration and workflow tests (mock provider)
make build             # bin/mia
./bin/mia config check                                  # fingerprint workspace assets, report drift
./bin/mia run research --inputs testdata/sample-batch   # end-to-end mock research run → prints "Run <id>"
./bin/mia run strategy --from-run <research-run-id>     # Strategy Agent on the validated evidence of that run
./bin/mia opportunities --run <strategy-run-id>         # list opportunities with tier, status and labels
./bin/mia approve opportunity --id <opp-id> --decision approved --note "..."
./bin/mia export brief --id <opp-id>                    # drafts/<opp-id>.md (approved opportunities only)
./bin/mia run content --opportunity <opp-id>            # Content Agent: DRAF content from an approved opportunity
./bin/mia content list --run <content-run-id>           # status, placeholders, claims to verify, readiness
./bin/mia content review --id <draft-id> --decision approved --note "..."   # not a publication permission
./bin/mia content export --id <draft-id>                # drafts/content/<draft-id>.md, labelled DRAF
./bin/mia workflow start --inputs testdata/sample-batch # Phase 5: one resumable workflow over all steps
./bin/mia workflow run --id <wf-id>                     # safe automatic steps only; never approves or publishes
./bin/mia workflow show --id <wf-id>                    # live status, blockers and next actions from the database
./bin/mia workflow export --id <wf-id>                  # handoff-ready drafts only, labelled DRAF
./bin/mia status                                        # list runs
```

Reports are written to `reports/<run-id>/research.md|html`, `strategy.md|html` and `content.md|html`; the database and logs live under `data/`; exported briefs under `drafts/` (all git-ignored).

## Layout

```
cmd/mia                      CLI (research + strategy commands)
internal/config              YAML + .env loading, strict validation, config and scoring hashes
internal/orchestrator        run/step state machine, timeouts, bounded retries, budget gate, resume; research and strategy workflows
internal/research/importer   Markdown/CSV/JSON notes with provenance and PII guard
internal/research/workspace  read-only workspace fingerprints, drift, parsers (brand-oracle, facts, FAQ)
internal/research/dedup      canonical URLs, content hashes, near-duplicate titles
internal/agent/research      Market Research Agent prompt, parser, evidence validation
internal/agent/strategy      Strategy Agent prompt (evidence in an untrusted block), strict parser, structural validation
internal/strategy            deterministic gates (evidence, conflicts, brand/claims, operational, duplicates), scoring, tiers, briefs
internal/agent/content       Content Agent prompt (one approved opportunity in an untrusted block), strict content_drafts.v1 parser, per-type structure
internal/content             eligibility, evidence/claim/language/placeholder/duplicate gates, readiness, review and revision services
internal/claims              factual-claim scanner shared by the Strategy and Content agents
internal/safepath            export path guard (drafts directory only, never the workspace) and atomic writes
internal/workflow            workflow assessment (gates A–E, status, blockers, next actions) and advance/resume with reservations and a lease
internal/provider            Provider interface; mock (fixtures, zero cost); registry
internal/budget              per-run / per-agent / monthly (USD 20) ceilings and cost ledger
internal/brand               claim checker ported from brand-oracle.mjs (data-driven rules)
internal/validate            suspicious-content detector
internal/store               SQLite (pure Go), embedded migrations (0001 research, 0002 strategy, 0003 content, 0004 workflow), append-only triggers
internal/report              Markdown + HTML templates (Indonesian labels): research, strategy, content and workflow reports, handoff brief, content export
config/                      brand, business facts, claim policy, sources, limits, scoring rules, content rules, workflow limits
schemas/                     agent output schema
testdata/                    sample batch, mock fixtures (research, strategy, content), workspace-mini, failure fixtures, golden reports
```

## Safety properties

- The model gets no tools; all inputs are placed in a delimited untrusted-data block; instruction-like content is flagged and excluded.
- Every research item carries a source, retrieval time and evidence excerpt; facts without source evidence are rejected; interpretations must cite items.
- The Strategy Agent only proposes: code verifies every cited evidence ID against validated items, shows both sides of conflicting evidence, rejects unsupported prices, hours, addresses, guarantees and statistics, blocks text that fails the brand checker, labels store-dependent ideas, merges duplicates without losing corroboration, and recomputes tiers from the evidence.
- At most two Strategy Agent calls per run; approvals are append-only; nothing is approved automatically; exported briefs can never be written into the Punyamas workspace.
- The Content Agent works only from an opportunity approved in the database; every draft cites only that opportunity's validated evidence; unsupported prices, hours, addresses, statistics, testimonials, promises, discounts, deadlines and price trends block the draft; missing information appears as `[ISI: ...]` and keeps the draft from ever being ready; reviews and versions are append-only and a changed text returns to review.
- The workflow derives its status from the database every time, reserves run IDs before starting runs, holds a lease while advancing, and never approves, publishes or sends anything; failed stages are resumed only on `workflow resume`.
- A failed required step always yields a failed run and a report with a failure header; runs can be resumed from the last completed step and the report is always re-rendered.
- Budget ceilings are checked before every provider call (including mock, with simulated usage); failed or timed-out calls are booked as uncertain estimates.
- Secrets live only in `.env`; logs redact key-like values and truncate untrusted excerpts.
