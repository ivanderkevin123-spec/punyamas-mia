# MIA — Punyamas Marketing Intelligence Agent

Internal marketing-intelligence and decision-support tool for Punyamas (an Indonesian gold-buying business in pre-launch). MIA turns approved research inputs into evidence-backed findings, ranks opportunities from validated evidence, and (in a later phase) drafts brand-safe Indonesian briefs. It is **not** a dashboard, CRM, or publisher, and it never performs external actions.

**Status:** Phase 3 (Strategy Agent) implemented on top of the Phase 2 foundation. Provider mode is **mock only**: no paid API calls, no live web fetching. The Punyamas workspace at `~/Documents/bots/punyamas` is read **read-only** with provenance and drift detection; approved briefs are exported to `drafts/` and handed over manually.

Documentation: `docs/audit/` (Phase 0), `docs/product/`, `docs/architecture/` (Phase 1; Phase 3: `strategy-agent.md`), `docs/implementation/` (roadmap, acceptance criteria, phase reports), `docs/phase1-decision-memo.md` (Indonesian), `docs/implementation/operations.md` (how to run).

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
./bin/mia status                                        # list runs
```

Reports are written to `reports/<run-id>/research.md|html` and `reports/<run-id>/strategy.md|html`; the database and logs live under `data/`; exported briefs under `drafts/` (all git-ignored).

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
internal/provider            Provider interface; mock (fixtures, zero cost); registry
internal/budget              per-run / per-agent / monthly (USD 20) ceilings and cost ledger
internal/brand               claim checker ported from brand-oracle.mjs (data-driven rules)
internal/validate            suspicious-content detector
internal/store               SQLite (pure Go), embedded migrations (0001 research, 0002 strategy)
internal/report              Markdown + HTML templates (Indonesian labels): research report, strategy report, handoff brief
config/                      brand, business facts, claim policy, sources, limits, scoring rules
schemas/                     agent output schema
testdata/                    sample batch, mock fixtures (research, strategy), workspace-mini, failure fixtures, golden reports
```

## Safety properties

- The model gets no tools; all inputs are placed in a delimited untrusted-data block; instruction-like content is flagged and excluded.
- Every research item carries a source, retrieval time and evidence excerpt; facts without source evidence are rejected; interpretations must cite items.
- The Strategy Agent only proposes: code verifies every cited evidence ID against validated items, shows both sides of conflicting evidence, rejects unsupported prices, hours, addresses, guarantees and statistics, blocks text that fails the brand checker, labels store-dependent ideas, merges duplicates without losing corroboration, and recomputes tiers from the evidence.
- At most two Strategy Agent calls per run; approvals are append-only; nothing is approved automatically; exported briefs can never be written into the Punyamas workspace.
- A failed required step always yields a failed run and a report with a failure header; runs can be resumed from the last completed step and the report is always re-rendered.
- Budget ceilings are checked before every provider call (including mock, with simulated usage); failed or timed-out calls are booked as uncertain estimates.
- Secrets live only in `.env`; logs redact key-like values and truncate untrusted excerpts.
