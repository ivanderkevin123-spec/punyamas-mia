# MIA — Punyamas Marketing Intelligence Agent

Internal marketing-intelligence and decision-support tool for Punyamas (an Indonesian gold-buying business in pre-launch). MIA turns approved research inputs into evidence-backed findings and, in later phases, ranked opportunities and brand-safe Indonesian draft briefs. It is **not** a dashboard, CRM, or publisher, and it never performs external actions.

**Status:** Phase 2 (minimal foundation and mock workflow). Provider mode is **mock only**: no paid API calls, no live web fetching. The Punyamas workspace at `~/Documents/bots/punyamas` is read **read-only** with provenance and drift detection.

Documentation: `docs/audit/` (Phase 0), `docs/product/`, `docs/architecture/`, `docs/implementation/` (Phase 1), `docs/phase1-decision-memo.md` (Indonesian), `docs/implementation/operations.md` (how to run).

## Requirements

- Go **1.27.2 darwin/arm64** at `~/sdk/go1.27.2` (the Go 1.17 on `PATH` is not used). Verify with `make go-version`.
- No services, no network, no credentials are needed in Phase 2.

## Quick start

```bash
make go-version        # must print go1.27.2 darwin/arm64
make test              # unit, integration and workflow tests (mock provider)
make build             # bin/mia
./bin/mia config check                                  # fingerprint workspace assets, report drift
./bin/mia run research --inputs testdata/sample-batch   # end-to-end mock run
./bin/mia status                                        # list runs
```

Reports are written to `reports/<run-id>/research.md` and `.html`; the database and logs live under `data/` (git-ignored).

## Layout

```
cmd/mia                      CLI
internal/config              YAML + .env loading, strict validation, config hash
internal/orchestrator        run/step state machine, timeouts, bounded retries, budget gate, resume
internal/research/importer   Markdown/CSV/JSON notes with provenance and PII guard
internal/research/workspace  read-only workspace fingerprints, drift, parsers (brand-oracle, facts, FAQ)
internal/research/dedup      canonical URLs, content hashes, near-duplicate titles
internal/agent/research      Market Research Agent prompt, parser, evidence validation
internal/provider            Provider interface; mock (fixtures, zero cost); registry
internal/budget              per-run / per-agent / monthly (USD 20) ceilings and cost ledger
internal/brand               claim checker ported from brand-oracle.mjs (data-driven rules)
internal/validate            suspicious-content detector
internal/store               SQLite (pure Go), embedded migrations
internal/report              Markdown + HTML templates (Indonesian labels)
config/                      brand, business facts, claim policy, sources, limits, price table
schemas/                     agent output schema
testdata/                    sample batch, mock fixtures, workspace-mini, failure fixtures, golden report
```

## Safety properties (Phase 2)

- The model gets no tools; all inputs are placed in a delimited untrusted-data block; instruction-like content is flagged and excluded.
- Every research item carries a source, retrieval time and evidence excerpt; facts without source evidence are rejected; interpretations must cite items.
- A failed required step always yields a failed run and a report with a failure header; runs can be resumed from the last completed step.
- Budget ceilings are checked before every provider call (including mock, with simulated usage).
- Secrets live only in `.env`; logs redact key-like values and truncate untrusted excerpts.
