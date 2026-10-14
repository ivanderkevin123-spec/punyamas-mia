# MIA — Operations Guide (Phase 4)

English for engineers; command output and report labels are Indonesian.

## 1. Toolchain

| Check | Command | Expected |
|---|---|---|
| Go binary actually used | `make go-version` | `GO=/Users/<you>/sdk/go1.27.2/bin/go` and `go version go1.27.2 darwin/arm64` |
| Wrong toolchain guard | Makefile exports `GOTOOLCHAIN=local` | a different binary fails instead of auto-downloading |
| Caches | `.toolchain/gocache`, `.toolchain/modcache` (git-ignored) | override with `GOCACHE=`/`GOMODCACHE=` if you prefer the defaults |
| Module files | every target runs in Go's default read-only module mode; `make test` and `make test-race` hash `go.mod`/`go.sum` before and after and fail if they changed; `make modcheck` compares them with HEAD | only `make tidy` (explicit, prints the diff) may change them |

Module provenance (A1). **Status: PASS, verified on 2026-10-13 by the independent pre-commit audit** (not by the documentation edit that recorded it). Evidence from that audit: a temporary project copy with a fresh module cache; `GOTOOLCHAIN=local`, `GOPROXY=https://proxy.golang.org,direct`, `GOSUMDB=sum.golang.org`, TLS verification enabled; the Go command consulted the checksum database (sumdb `lookup`/`tile` cache written, 30 lookups); every one of the 28 committed `go.sum` lines was present in the independently generated file with an identical hash, including `golang.org/x/mod v0.42.0` and `golang.org/x/tools v0.52.0`; no same-module/version hash differed; the 32 extra generated lines are test dependencies of dependencies; the project's `go.mod` and `go.sum` were untouched. Commit approval for Phase 2 and authorization for Phase 3 are separate human decisions and were not implied by that result.

Why the check exists: the archives in `.toolchain/modcache` originally came from the sibling `kerjalancar` download cache with the checksum database disabled. Re-run the check whenever `go.mod` or `go.sum` changes, on a machine with normally verified TLS. A sandboxed session whose proxy intercepts TLS fails with `x509: OSStatus -26276`; that is a blocked check to report, never a reason to disable verification.

Save the following as a script (for example `verify-a1.sh`) and run it with `bash verify-a1.sh`; it needs bash or zsh because of `<(...)`. It copies only `go.mod` into a temporary directory, never deletes or regenerates the project's `go.sum`, exits non-zero on any failure, and prints PASS only when every original line was found in a checksum-database-verified download.

```bash
#!/usr/bin/env bash
set -u
PROJECT="$HOME/Documents/punyamas/mia"
GO="$HOME/sdk/go1.27.2/bin/go"
TMP="$(mktemp -d)" || exit 1
mkdir -p "$TMP/mia" "$TMP/modcache" "$TMP/gocache" || exit 1
cp "$PROJECT/go.mod" "$TMP/mia/go.mod" || exit 1
cd "$TMP/mia" || exit 1

# Checksum verification must stay ON. GOPRIVATE, GONOSUMDB, GONOSUMCHECK, GOINSECURE and GONOPROXY
# would exempt modules from the database, so they are cleared here. GOFLAGS=-mod=mod applies to the COPY only.
if ! env -u GOPRIVATE -u GONOSUMDB -u GONOSUMCHECK -u GOINSECURE -u GONOPROXY \
     GOTOOLCHAIN=local GOFLAGS=-mod=mod \
     GOPROXY='https://proxy.golang.org,direct' GOSUMDB='sum.golang.org' \
     GOMODCACHE="$TMP/modcache" GOCACHE="$TMP/gocache" \
     "$GO" mod download all; then
  printf '%s\n' "ERROR: go mod download failed (network, TLS or checksum-database verification). A1 NOT verified."
  exit 1
fi
if [ ! -d "$TMP/modcache/cache/download/sumdb/sum.golang.org" ]; then
  printf '%s\n' "ERROR: no checksum-database lookups were recorded; the download did not use sum.golang.org. A1 NOT verified."
  exit 1
fi

MISSING="$(comm -23 <(sort "$PROJECT/go.sum") <(sort go.sum))"
if [ -n "$MISSING" ]; then
  printf '%s\n' "ERROR: original go.sum entries are missing from the independently generated file:"
  printf '%s\n' "$MISSING"
  exit 1
fi

# A same module@version with a different hash is a content mismatch (it also appears above as a missing line).
CONFLICT="$(join -j1 <(awk '{print $1"@"$2, $3}' "$PROJECT/go.sum" | sort) <(awk '{print $1"@"$2, $3}' go.sum | sort) | awk '$2 != $3')"
if [ -n "$CONFLICT" ]; then
  printf '%s\n' "ERROR: same module and version with a different checksum:"
  printf '%s\n' "$CONFLICT"
  exit 1
fi

EXTRA="$(comm -13 <(sort "$PROJECT/go.sum") <(sort go.sum))"
if [ -n "$EXTRA" ]; then
  printf '%s\n' "NOTE: entries present only in the generated file (not a mismatch by itself; analyze separately, see the guide):"
  printf '%s\n' "$EXTRA"
fi

printf '%s\n' "PASS: every original go.sum line exists in the independently generated, checksum-database-verified file."
printf '%s\n' "Temporary directory left in place for inspection: $TMP"
printf '%s\n' "Remove it afterwards with: chmod -R u+w \"$TMP\" && rm -rf \"$TMP\""
```

How to read the result:

- **PASS** means the Go command downloaded every module in the graph, verified each against `sum.golang.org` (the database is consulted because the temporary copy has no `go.sum`), and every committed line reappeared unchanged. A download made with `GOSUMDB=off`, `GONOSUMDB`, `GOPRIVATE`, `GOINSECURE` or `GOFLAGS=-insecure` proves nothing; the script clears those variables and refuses to pass when no database lookups were recorded.
- **Extra generated lines** are expected and are not mismatches by themselves: `go mod download all` resolves the full module graph including dependencies' test requirements (for example `github.com/google/go-cmp`, `github.com/google/pprof`, `modernc.org/cc/v4`), while the committed `go.sum` only covers what the build needs. Confirm each extra module is absent from `go list -deps -f '{{.Module}}' ./cmd/mia`; if one is present in the build graph but missing from the committed file, run `make tidy` deliberately and review the diff before committing.
- **A missing line or a same-version hash conflict** means the archive served by the official proxy differs from what the committed `go.sum` expects. Do not build. Find where the committed line came from (`git log -p -- go.sum`), compare the hash with `https://sum.golang.org/lookup/<module>@<version>`, and only then decide whether `go.sum` must be corrected by an explicit, reviewed `make tidy`.
- **Cleanup**: the script prints the temporary directory instead of deleting it. Go marks module-cache files read-only, so remove it with `chmod -R u+w "$TMP" && rm -rf "$TMP"` only after confirming `$TMP` is the `mktemp` path the script printed, never `$HOME` or the project.

## 2. Configuration

Files in `config/` (validated strictly at start; unknown keys are errors; secrets in YAML are rejected):

| File | Purpose |
|---|---|
| `agent-limits.yaml` | provider (`mock`), ceilings (monthly **USD 20**, run USD 2, agent USD 1), retries, timeouts, price table with `verified_at` |
| `research-sources.yaml` | workspace root and asset list (read-only), web fetching (`enabled: false`, required in Phase 2), import limits |
| `marketing-policy.yaml` | nine banned-claim categories ported from `brand-oracle.mjs` |
| `business.yaml` | facts FB1–FB14 from `B4-fakta.json`, mandatory fill-ins |
| `brand.yaml` | positioning, pillars, tone, audiences, key messages, tagline candidates from B1 |
| `content.yaml` | Phase 4 Content Agent rules applied by code: call cap per run, duplicate threshold, default content types, per-type channels/variants/required fields/rune limits/sections/checklist, marketing-language rules (`block` or `review`), base review checklist; hashed on every content draft (`content_config_hash`) |
| `scoring.yaml` | Phase 3 Strategy Agent rules applied by code: tier thresholds, duplicate thresholds, conflict heuristic (negation pattern + minimum similarity), unsupported-claim patterns, store-dependency terms, KPI/segment/pillar/effort enumerations; hashed into every strategy run and opportunity (`scoring_config_hash`) |

Environment (`.env`, git-ignored; see `.env.example`): `MIA_DATA_DIR`, `MIA_REPORTS_DIR`, `MIA_DRAFTS_DIR`, `MIA_WORKSPACE_ROOT`, `MIA_PROVIDER`, `MIA_MODEL`, `MIA_FIXTURE_KEY`, `MIA_LOG_LEVEL=debug`.

## 3. Commands

```bash
./bin/mia run research --inputs DIR [--fixture KEY] [--until STEP] [--force]
./bin/mia run research --resume RUN_ID
./bin/mia import --inputs DIR            # stops after dedup
./bin/mia status [RUN_ID] [--json]
./bin/mia report RUN_ID                  # re-render
./bin/mia validate --inputs DIR          # parse only, no database
./bin/mia config check [--json]          # exit 2 when review is required
./bin/mia approve asset --key KEY --note "..." [--by NAME]
./bin/mia run strategy --from-run RUN_ID [--fixture KEY] [--force]   # Phase 3: opportunities from the validated items of a research run
./bin/mia run strategy --resume RUN_ID
./bin/mia opportunities --run RUN_ID     # id, tier, status, segment, ordinal sum, title, labels
./bin/mia approve opportunity --id ID --decision approved|rejected|needs_more_evidence --note "..." [--by NAME]
./bin/mia export brief --id ID [--out PATH]   # approved opportunities only; default drafts/<id>.md; paths inside the workspace are refused
```

Exit codes: 0 ok · 1 error · 2 run failed / review required / export refused.

Steps of a research run: `import → workspace → dedup → research_agent → validate → report`. The report step runs even after a failure so the failure is visible.

Steps of a strategy run: `load_evidence → conflicts → strategy_agent → evaluate → report`. `load_evidence` requires the parent research run to be `completed` or `completed_with_warnings` with at least one validated item. The Strategy Agent is called at most **twice per run** (one call plus one bounded repair), counted from persisted executions including errors and timeouts; a run whose two calls are spent fails with `batas 2 panggilan` and must be started again with `--from-run` (the failed run keeps its report). `--resume` re-renders the report from stored data and never reuses a stale one; `--force` re-runs completed steps but never duplicates stored opportunities. See `docs/architecture/strategy-agent.md`.

Content commands (Phase 4; every draft is a DRAF and nothing is published, sent or scheduled):

```bash
./bin/mia run content --opportunity OPP_ID [--types instagram_caption,education] [--fixture KEY] [--regenerate]
./bin/mia run content --resume RUN_ID [--force]
./bin/mia content list --run RUN_ID | --opportunity OPP_ID
./bin/mia content show --id DRAFT_ID
./bin/mia content review --id DRAFT_ID --decision approved|rejected --note "..." [--by NAME]
./bin/mia content revise --id DRAFT_ID [--hook T] [--headline T] [--body-file PATH] [--cta T] --note "..."
./bin/mia content export --id DRAFT_ID [--out RELATIVE_NAME]   # drafts/content/<id>.md; blocked/rejected drafts are not exported
```

Steps of a content run: `load_opportunity → content_agent → evaluate_content → report`. Only opportunities whose status is `approved` with a matching latest approval record, a completed strategy run and still-validated evidence are eligible; refusals create no run and make no agent call. A second generation for an opportunity with drafts needs `--regenerate`. The Content Agent is called at most `max_calls_per_run` (2) times per run. `review approved` means approved for the next manual stage, never a publication permission; `revise` appends a version and returns the draft to `needs_review`. See `docs/architecture/content-agent.md`.

Local mock walkthrough:

```bash
make build
./bin/mia run research --inputs testdata/sample-batch          # note the printed research run id
./bin/mia run strategy --from-run <research-run-id>             # fixture key defaults to the research run's key (sample-batch)
./bin/mia opportunities --run <strategy-run-id>
./bin/mia approve opportunity --id <opp-id> --decision approved --note "Uji konten pertama"
./bin/mia export brief --id <opp-id>                            # drafts/<opp-id>.md, then hand over manually
./bin/mia run strategy --from-run <research-run-id> --fixture gates   # demo of every gate failure (blocked/rejected)
./bin/mia run content --opportunity <approved-opp-id>            # fixture key defaults to <strategy fixture key>-<topic_key>
./bin/mia content list --run <content-run-id>
./bin/mia content review --id <draft-id> --decision approved --note "Draf diperiksa"
./bin/mia content export --id <draft-id>                          # drafts/content/<draft-id>.md, labelled DRAF
./bin/mia run content --opportunity <approved-opp-id> --fixture gates --regenerate   # demo of every content gate
```

## 4. Import file formats

- **Markdown**: YAML front matter (`source_name`, `reference`, `retrieved_at`, optional `published_at`, `publisher`, `source_type`, `kind: fact|hypothesis`, `quality: A–D`, `language`, `sample`), then one item per `## Heading`; `> ` lines form the evidence excerpt; `Topik:`, `Jenis:`, `Kualitas:` lines override per item.
- **CSV**: required columns `source_name,reference,retrieved_at,kind,title,statement`; optional `excerpt,quality,topic_key,published_at,publisher,source_type,language`.
- **JSON**: array of notes with `items[]`.

Rules: facts need an excerpt; dates are `YYYY-MM-DD` or RFC 3339; files containing phone numbers, 16-digit identity numbers, e-mail addresses, or PII-looking column names are rejected whole; files over 5 MB are rejected.

## 5. Workspace drift workflow

1. `mia config check` fingerprints each asset in `research-sources.yaml`. First sight = accepted baseline.
2. A changed file is stored as `review_required`; runs continue with a warning unless `drift.block_runs: true`.
3. Review the diff details printed (rules vs `marketing-policy.yaml`, facts vs `business.yaml`), update the YAML by hand if needed, then `mia approve asset --key KEY --note "..."`.
4. MIA never writes under the workspace root.

## 6. Mock provider and fixtures

`testdata/fixtures/<agent>/<fixture-key>.json` (default key = inputs directory name; for content runs `<strategy fixture key>-<opportunity topic_key>`; fallback `default.json`). Missing fixture → empty output with a warning, never a fabricated finding. Usage tokens are estimated (runes/4) and cost is USD 0 and measurable. Strategy fixtures: `strategy/sample-batch.json` (three Tier A proposals, one duplicate that is merged, one store-dependent hypothesis), `strategy/gates.json` (one proposal per gate failure plus one valid store-dependent proposal), `strategy/conflict.json` (used by the conflict test), `strategy/default.json` (empty). The sha256 of the replayed fixture is recorded on the run and on every opportunity (`fixture_hash`). Content fixtures: `content/sample-batch-transparansi_penimbangan.json` (ten drafts: five pass every gate, five need review for placeholders or a claim to verify), `content/gates.json` (one violation per draft: price, testimonial, fictitious, cross-opportunity and unvalidated evidence, false urgency, process time, statistic plus address, guaranteed profit plus price trend, a fourth caption variant, a near-duplicate), `content/default.json` (empty). Mock output never says anything about the quality of a real model.

## 7. Data, backups, logs

- `data/mia.sqlite` (0600, WAL), `data/executions/*.request|response.txt` (0600, full prompts and raw outputs), `data/logs/mia-YYYY-MM-DD.jsonl` (redacted).
- Cost ledger semantics (A7): a row per provider call. `estimated = 0` with `unit_price_source = provider_reported` or `price_table:<date>` is actual spend; `estimated = 1` with `uncertain:timeout` or `uncertain:error` is the pre-call estimate booked for a call whose outcome is unknown (never an actual charge); `unmeasurable:estimate` is an estimate for a provider that reports no usage. Ceilings count all rows, so repeated failures cannot bypass them. At most one row exists per execution: a later reconciliation replaces the uncertain row. Reports show total, actual and uncertain figures separately. Authentication and billing rejections book nothing.
- Phase 3 tables: `opportunities` (evaluation, tier, brief JSON, config/scoring/fixture hashes, execution id) and `opportunity_evidence` (opportunity → validated item, role). Review decisions go to `approval_records` (append-only; the opportunity's `status` mirrors the latest decision, the evaluation is never edited).
- Phase 4 tables: `content_drafts` (current text, status, version, content hash, provenance hashes), `content_draft_versions` (every text version, append-only), `content_draft_evidence` (composite foreign keys: only the opportunity's own evidence). Content reviews are `approval_records` rows with `subject_type = content_draft`, `subject_version` and `subject_hash`; since migration 0003 every `approval_records` row is append-only (update/delete abort). `drafts/content/` holds exports for manual handoff.
- Backup: copy `data/mia.sqlite` while no run is active (a `mia backup` command is planned for Phase 7).
- `reports/` and `drafts/` are plain files; both are git-ignored. `drafts/<opportunity-id>.md` is the exported brief for manual handoff to `bots/punyamas` (MIA never writes there).

## 8. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `provider 'anthropic' is not available in Phase 2` | config asks for a live provider | set `provider.name: mock` |
| `web.enabled=true is not available in Phase 2` | fetching requested | set `enabled: false` |
| `config ...: contains what looks like a secret` | a key was pasted into YAML | move it to `.env` |
| `ERROR: go.mod/go.sum changed during tests` | something rewrote module files (should be impossible in read-only mode) | inspect `git diff go.mod go.sum`; only `make tidy` is allowed to change them |
| run `failed` at `import` with "tidak ada berkas riset yang valid" | all files rejected or wrong directory | run `mia validate --inputs DIR` to see per-file reasons |
| step `budget_exhausted` | ceilings reached (monthly ledger in WIB) | wait for next month, raise ceilings deliberately in `agent-limits.yaml`, or stay in mock |
| `status` shows `(STALE)` | process died mid-run | `mia run research --resume RUN_ID` |
| `tls: failed to verify certificate` during `go get` | sandboxed proxy | use the local module cache (see §1) or run outside the sandbox |
| strategy run `failed` at `load_evidence` with "berstatus ..." | the research run is not completed or has no validated items | finish the research run (`--resume`), then `run strategy --from-run` again |
| `batas 2 panggilan Agen Strategi per run sudah tercapai` | both calls of this run were spent (errors and timeouts count) | fix the cause, then start a new run with `run strategy --from-run RUN_ID` |
| `export brief` exits 2 with "hanya peluang berstatus approved" | the opportunity is not approved (or a later decision changed it) | record `--decision approved` first; blocked/rejected opportunities can never be approved |
| `run content` refused: "tidak memenuhi syarat untuk Agen Konten" | the opportunity is not approved, its history disagrees, its strategy run is not completed, or an evidence item is no longer validated | the message lists each reason; approve the opportunity or fix the evidence, then run again |
| `run content` refused: "draf sudah ada untuk peluang ini" | drafts already exist for the opportunity | review the existing drafts, or add `--regenerate` deliberately |
| `content review` refused: "diblokir gerbang kode" | a hard gate failed | `content revise` the text (gates run again) or regenerate |
| `content review` refused: "berubah di luar MIA" | the stored text no longer matches its hash | record the change with `content revise`, then review |
| `content export` refused ("ruang kerja", "..") | target inside the workspace, absolute, or escaping `drafts/content` | use a relative `--out` name; keep `MIA_DRAFTS_DIR` outside the workspace |
| `export brief` refuses the path ("baca-saja") | `--out` points inside the Punyamas workspace | export elsewhere (default `drafts/`) and hand over manually |
