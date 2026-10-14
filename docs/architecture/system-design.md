# MIA — System Design

> **Phase 5 update (2026-10-14):** workflow orchestration over the existing steps (status precedence, gates A–E, reservation-based idempotency, lease, resume) is specified in `workflow.md`.

> **Phase 4 update (2026-10-14):** the Content Agent workflow (steps `load_opportunity → content_agent → evaluate_content → report`, gates, review state machine, export) is specified in `content-agent.md`.

> **Phase 3 update (2026-10-13):** the Strategy Agent workflow (steps `load_evidence → conflicts → strategy_agent → evaluate → report`, gates, scoring, approvals) is specified in `strategy-agent.md`; this document keeps the Phase 1 design.

**Status:** Phase 1 draft · 2026-10-12 · English. Inputs: `docs/audit/*.md` (historical), Phase 1 re-verification (§1), approved decisions (Go 1.27.2 native, mock-only AI, no live fetching, read-only `bots/punyamas`, USD 20/month ceiling, separate `mia` repo).

## 0. Ringkasan untuk pemilik usaha (Bahasa Indonesia)

Arsitektur yang direkomendasikan adalah **satu aplikasi baris perintah (CLI) berbahasa Go** dengan **orkestrator deterministik**, **penyimpanan SQLite lokal**, **laporan Markdown/HTML**, dan **lapisan penyedia yang dapat ditukar** (AI dan sumber riset) yang pada awalnya hanya berisi penyedia tiruan (mock). Dua agen logis (Riset Pasar dan Strategi) berjalan sebagai langkah berurutan di dalam satu proses; agen Konten dan Analis Kinerja ditambahkan belakangan tanpa mengubah orkestrator. Tidak ada server web, layanan latar belakang, basis data eksternal, atau kerangka multi-agen.

## 1. Verified inputs and Phase 1 re-verification

| Item | Phase 0 (historical) | Re-checked 2026-10-12 in Phase 1 | Status |
|---|---|---|---|
| Native Go | `~/sdk/go1.27.2/bin/go` = go1.27.2 darwin/arm64 | Same | [V] |
| Go on PATH | go1.17.5 darwin/amd64 | Same (not to be used) | [V] |
| `mia` folder | Not a git repository | Still not a repository; `git init` was attempted in Phase 1 and **blocked** (sandbox denies creating `.git`; the override was declined by the session's permission classifier). `.gitignore` written. Owner must run `git init -b main` or grant the permission | [V] discrepancy recorded |
| Commit identity | No global identity; local identity in `kerjalancar` ("Kevin Ivander", email matching the account) | Same; `mia` has no local identity yet → **no commits will be made** until the owner approves setting it | [V] |
| `bots/punyamas` | HEAD `446cb3f`, 1 untracked entry, unchanged by Phase 0 | HEAD now `d7ee05b` with 3 status entries, changed by the owner's other sessions, **not by MIA**; still read-only for MIA | [V] discrepancy recorded |
| AI credentials | None | None; no paid call made | [V] |

## 2. Architecture options

| Criterion | Option 1 — Minimal local app, deterministic workflow (CLI + SQLite, agents as functions) | Option 2 — Modular monolith with pluggable AI and research providers (recommended) | Option 3 — Multi-agent framework (agent runtime, message bus, tool-using agents) |
|---|---|---|---|
| Initial development effort | Lowest (≈ Phase 2–3 scope) | Low–moderate: Option 1 plus explicit `Provider`, `Agent`, `Importer`, `Reporter` interfaces and a registry | High: framework integration or custom runtime, agent protocols, tool sandboxing |
| Operational cost | USD 0 in mock; live calls bounded by budget | Same as Option 1 | Higher: more model calls per task (agent chatter), harder to bound |
| Reliability | High: linear steps, no concurrency | High: same execution model; interfaces add no runtime risk | Lower: emergent loops, retries across agents, harder failure attribution |
| Complexity | Minimal, but adapters would be ad-hoc later | Moderate, contained in interfaces | High |
| Testing | Easy (table tests, golden files) | Easy: mock provider implements the same interface as live providers; workflow tests run end to end | Hard: non-deterministic multi-agent runs |
| Extensibility | Weak: adding a provider means editing orchestration code | Strong: new provider/agent = new package implementing an interface + config entry | Strong in theory, but expensive |
| Security | Strong: no tools for the model, no network by default | Same, plus allow/block lists at the provider boundary | Weaker: tool-using agents widen the prompt-injection surface |
| One-machine suitability | Excellent (single binary, launchd) | Excellent | Poor fit (services, queues, often Node/Python runtimes) |
| Risk of unnecessary abstraction | Low now, higher later (rewrite when adding Content/Performance agents and live providers) | Low–moderate: interfaces are few (4) and justified by the roadmap (Phases 3–6 add providers and agents) | High: abstractions without demonstrated need |

**Recommendation: Option 2.** The audit shows a single-machine, single-operator workload with a roadmap that adds providers (live AI, search, importers) and agents (Content, Performance) in later phases. Option 2 keeps Option 1's execution model (one process, deterministic sequential steps, no agent conversations) and adds only the interfaces that those additions need. Option 3 contradicts the cost ceiling, the security requirements and the "no elaborate multi-agent framework" rule.

## 3. Component diagram

```
                 ┌────────────────────────────────────────────────────────────┐
                 │  mia CLI  (cmd/mia)   run · import · report · status ·      │
                 │                       validate · approve · config check    │
                 └───────────────┬────────────────────────────────────────────┘
                                 │ commands
                 ┌───────────────▼────────────────────────────────────────────┐
                 │  Orchestrator (internal/orchestrator)                      │
                 │  run state machine · step sequencing · timeouts · retries  │
                 │  budget gate · schema validation · persistence · summary   │
                 └──┬──────────┬───────────┬───────────┬───────────┬──────────┘
                    │          │           │           │           │
     ┌──────────────▼──┐ ┌─────▼──────┐ ┌──▼────────┐ ┌▼─────────┐ ┌▼─────────────┐
     │ Inputs          │ │ Agents     │ │ Validation│ │ Scoring  │ │ Reports      │
     │ importers       │ │ research   │ │ schemas   │ │ config-  │ │ md + html    │
     │ (md/csv/json)   │ │ strategy   │ │ claim     │ │ driven   │ │ templates    │
     │ workspace       │ │ [content]  │ │ checker   │ │ determ-  │ │              │
     │ (read-only)     │ │ [perf]     │ │ facts     │ │ inistic  │ │              │
     │ [fetcher: off]  │ └─────┬──────┘ │ dedup     │ └──────────┘ └──────────────┘
     └─────────────────┘       │        └───────────┘
                               │ Provider interface (no tools, untrusted-data block)
                     ┌─────────▼──────────────────────────────┐
                     │ Providers (internal/provider)           │
                     │ mock (fixtures) ─ default               │
                     │ [anthropic: API key, later, approved]   │
                     │ [claudecli: not enabled]                │
                     └─────────┬──────────────────────────────┘
                               │ usage → CostRecord
     ┌─────────────────────────▼──────────────────────────────┐
     │ Store (internal/store): SQLite data/mia.sqlite, WAL     │
     │ runs · steps · sources · items · evidence · opportunities│
     │ experiments · imported_assets · executions · costs ·     │
     │ approvals                                                │
     └──────────────────────────────────────────────────────────┘
     Config (config/*.yaml + .env) ──► every component at start   Logs: slog JSON, redacted
```

Brackets mark components defined now but implemented in later phases or disabled by default.

## 4. Components and responsibilities

| Component | Package (planned) | Responsibility | Deterministic? |
|---|---|---|---|
| CLI | `cmd/mia` | Parse commands/flags, load config, call orchestrator, print summaries, exit codes | Yes |
| Config | `internal/config` | Load and validate `config/*.yaml` and `.env`; expose typed settings; compute config snapshot hash per run; `source_of_truth` metadata | Yes |
| Orchestrator | `internal/orchestrator` | Create run, execute ordered steps, enforce per-step timeout and retry caps, call budget gate before each provider call, validate outputs, persist, write `RunSummary`, support `--resume` | Yes |
| Importers | `internal/research/importer` | Markdown/CSV/JSON notes → `Source` + `ResearchItem` + `Evidence`; PII column rejection; provenance validation | Yes |
| Workspace importer | `internal/research/workspace` | Read-only reading of the approved `bots/punyamas` files; fingerprinting; drift detection; parsing of `brand-oracle.mjs` `POLA` and `B4-fakta.json` | Yes |
| Fetcher (Phase 3, off by default) | `internal/research/fetch` | Allowlisted HTTP GET with robots check, rate limit, size cap, snapshot + hash, text extraction | Yes |
| Dedup | `internal/research/dedup` | URL canonicalization, content hash, normalized-title similarity | Yes |
| Agents | `internal/agent/research`, `internal/agent/strategy` (+ `content`, `performance` later) | Build prompts from validated inputs, call `Provider`, parse structured output | Prompting is code; the model is not |
| Validation | `internal/validate` | JSON schema per agent output, semantic checks (evidence refs resolve, kinds consistent), claim checker (`internal/brand`), facts check | Yes |
| Scoring | `internal/scoring` | Weighted score from validated dimension values; breakdown JSON | Yes |
| Providers | `internal/provider` (+ `provider/mock`, later `provider/anthropic`) | `Complete(ctx, Request) (Response, Usage, error)`; mock replays fixtures keyed by agent + input hash | Mock: yes |
| Budget | `internal/budget` | Pre-call estimate, ledger, per-run/per-agent/monthly ceilings, "unmeasurable" marking | Yes |
| Store | `internal/store` | SQLite access, embedded migrations, repositories, transactions | Yes |
| Reports | `internal/report` | Render Markdown and HTML from run data using templates; Indonesian section labels | Yes |
| Logging | `internal/log` | `log/slog` JSON handler with redaction of secrets and of long untrusted excerpts | Yes |

## 5. Data flow (Workflow A → B)

1. `mia run research` → Orchestrator creates `ResearchRun` and `RunStep[]` (`import`, `workspace`, `dedup`, `research_agent`, `validate`, `report`).
2. Importers and the workspace importer write `Source`, `ResearchItem`, `Evidence`, `ImportedAsset` inside one transaction per file.
3. Dedup marks duplicates; counts go to the summary.
4. Research Agent builds a request: system instructions (static, cacheable), config excerpts (audiences, policy categories), and an **untrusted-data block** with the validated items. The provider returns structured findings; `AgentExecution` stores the raw response and `CostRecord` stores usage.
5. Validation turns findings into `ResearchItem` rows with `origin = agent`, each linked by `Evidence(from_item_id)` to the source-backed items it cites; invalid ones are stored as `rejected` with reasons.
6. Report renders from the store, never from the raw model text.
7. `mia run strategy --from-run` repeats steps 4–6 with the Strategy Agent (grouping, scoring proposal, experiments) and the deterministic scorer; `Opportunity` and `Experiment` rows are written; the strategy report is rendered.
8. `mia approve …` writes `ApprovalRecord` rows; nothing else changes.

## 6. Failure boundaries

| Boundary | Isolation rule |
|---|---|
| Per file | A parse failure affects only that file; the step continues and reports the failure |
| Per step | A step has its own timeout, retry cap, and status; a required step failure fails the run; an optional step failure yields `completed_with_warnings` |
| Per provider call | Budget gate before; usage ledger after; timeout via context; errors classified (`auth`, `rate_limit`, `timeout`, `invalid_output`, `budget`) |
| Per run | One run never reads another run's partial state except through completed, validated rows; `--resume` restarts from the last completed step |
| Workspace | Read-only file access; any write path to the workspace is a programming error caught by tests (the store and reporters only accept paths under the MIA root) |
| Report | Rendered only from validated rows; a run that is not `completed` renders a failure header first |

## 7. Agent design

Each logical agent is a Go package implementing a small interface (illustrative, not implementation):

```go
// illustrative only — Phase 2 will define the final shapes
type Agent interface {
    Name() string                                   // "research", "strategy", ...
    Limits() AgentLimits                            // timeout, max calls, token caps from agent-limits.yaml
    BuildRequest(in Input) (provider.Request, error) // static system prompt + config + untrusted-data block
    ParseOutput(resp provider.Response) (Output, error) // JSON schema validation happens here
}
```

Rules: no agent calls another agent; the orchestrator decides order; an agent may make at most `max_calls` provider calls per step (default 1, plus 1 bounded repair call on schema failure); the model receives no tools; outputs are JSON matching a versioned schema stored in `schemas/`. Mock execution satisfies every agent: the mock provider returns a fixture for `(agent, schema_version, input_hash)` or a configurable canned response, so the whole workflow is testable with zero cost.

Adding the Content Agent (Phase 4) or Performance Agent (Phase 5) means adding a package implementing `Agent`, a schema file, templates, and a step name in the orchestrator's step table; the orchestrator's execution loop does not change.

## 8. Adding a provider without touching orchestration

```go
// illustrative only
type Provider interface {
    Name() string
    Complete(ctx context.Context, req Request) (Response, Usage, error)
    Capabilities() Capabilities // measurable_cost bool, max_input_tokens, supports_json_schema
}
```

Providers are registered by name in a factory driven by `config/agent-limits.yaml → provider.name`. The orchestrator calls `budget.Allow(estimate)` → `provider.Complete` → `budget.Record(usage)`. A new provider (e.g. `anthropic` after an API key and billing are approved) implements the interface and reports `Usage` from the API response; a provider that cannot report usage sets `measurable_cost = false`, which forces the summary to print "biaya tidak dapat diukur" and counts estimated tokens against the ceiling. Research providers (`SearchProvider`, `Fetcher`) follow the same pattern with allow/block lists enforced in the application, not in the provider.

## 9. Toolchain and Makefile design

Verified: `~/sdk/go1.27.2/bin/go version` → `go1.27.2 darwin/arm64`; `go version` on PATH → `go1.17.5 darwin/amd64` (must not be used).

Makefile design (conceptual; created in Phase 2):

```make
# Go executable: defaults to the verified native toolchain; override with GO=/path/to/go
GO ?= $(HOME)/sdk/go1.27.2/bin/go
GOFLAGS ?= -mod=mod
export GOTOOLCHAIN ?= local      # never auto-download a different toolchain

.PHONY: go-version build test test-race vet run-mock lint

go-version:    ## Print the Go binary and version actually used
	@echo "GO=$(GO)" && $(GO) version

build:         ## Build bin/mia with the selected toolchain
	$(GO) build -o bin/mia ./cmd/mia

test:          ## Unit, integration and workflow tests (mock provider only)
	$(GO) test -count=1 ./...

test-race:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

run-mock:      ## End-to-end mock run on the sample batch
	./bin/mia run research --inputs testdata/sample-batch --then strategy
```

Developer verification: `make go-version` must print `go1.27.2 darwin/arm64`; `go.mod` will declare `go 1.27`; `GOTOOLCHAIN=local` makes a wrong binary fail loudly instead of downloading another toolchain. Module cache: default `~/go/pkg/mod` (modules fetched from `proxy.golang.org` after approval); an optional `GOMODCACHE=.toolchain/modcache` mirrors the `kerjalancar` pattern if isolation is preferred.

## 10. Planned repository layout

```
mia/
  cmd/mia/                  CLI entry
  internal/config           YAML + .env loading, validation, snapshot hash
  internal/orchestrator     run/step state machine, resume, summary
  internal/research/{importer,workspace,fetch,dedup}
  internal/agent/{research,strategy}      (+ content, performance later)
  internal/provider/{mock}                (+ anthropic later)
  internal/validate  internal/brand  internal/scoring  internal/budget
  internal/store            SQLite, migrations (embedded SQL), repositories
  internal/report           templates (md, html), renderers
  internal/log              slog setup, redaction
  schemas/                  JSON schemas for agent outputs (versioned)
  config/                   brand.yaml business.yaml marketing-policy.yaml research-sources.yaml agent-limits.yaml scoring.yaml
  testdata/                 fixtures, sample batch, golden reports
  imports/  data/  reports/  drafts/   (git-ignored except README placeholders)
  docs/                     audit/ product/ architecture/ implementation/
  Makefile  .env.example  .gitignore  go.mod
```

## 11. Configuration and logging

- Config precedence: defaults in code → `config/*.yaml` → environment (`.env`, loaded only by the CLI, never committed). Secrets only via environment. Every run stores `config_hash` so reports can be reproduced.
- `config/research-sources.yaml` holds the workspace asset list (relative paths confirmed in `docs/audit/existing-projects.md` §1.2), the web allowlist (empty until approved), and blocklist patterns.
- Logging: `log/slog` JSON to stderr and to `data/logs/mia-<date>.jsonl`; redaction of any value whose key matches `(?i)key|token|secret|password` and truncation of untrusted excerpts to 200 characters in logs (full text stays in the store).

## 12. Scheduling (Phase 6 design note)

`mia schedule render --interval 86400` will print a launchd plist (label `com.punyamas.mia.<job>`, absolute path to `bin/mia`, `WorkingDirectory` = MIA root, stdout/stderr to `data/logs/`), following the pattern already used on this machine (`com.kevinivander.xauea.*`). Installation (`launchctl bootstrap`) stays a manual, documented step; MIA never installs itself.

## 13. Non-functional requirements

| Requirement | Target |
|---|---|
| Runtime dependencies | Single static binary; SQLite embedded (pure Go driver) |
| Run duration (mock, sample batch) | < 10 s |
| Memory | < 300 MB for 1,000 research items |
| Determinism | Same inputs + same fixtures + same config → byte-identical Markdown report (golden test) |
| Cost in mock mode | USD 0, enforced by the provider having no network code |
| Portability | macOS arm64 primary; Linux build possible, not required |
