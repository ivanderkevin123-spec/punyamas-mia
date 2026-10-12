# MIA — Operations Guide (Phase 2)

English for engineers; command output and report labels are Indonesian.

## 1. Toolchain

| Check | Command | Expected |
|---|---|---|
| Go binary actually used | `make go-version` | `GO=/Users/<you>/sdk/go1.27.2/bin/go` and `go version go1.27.2 darwin/arm64` |
| Wrong toolchain guard | Makefile exports `GOTOOLCHAIN=local` | a different binary fails instead of auto-downloading |
| Caches | `.toolchain/gocache`, `.toolchain/modcache` (git-ignored) | override with `GOCACHE=`/`GOMODCACHE=` if you prefer the defaults |

Module provenance (2026-10-12): `go.sum` entries for every module except `golang.org/x/exp` were cross-checked against the sumdb-verified `go.sum` of the sibling `kerjalancar` project; `go mod verify` passes. When network TLS is available, re-verify with `GOFLAGS= GOSUMDB=sum.golang.org go mod verify`.

## 2. Configuration

Files in `config/` (validated strictly at start; unknown keys are errors; secrets in YAML are rejected):

| File | Purpose |
|---|---|
| `agent-limits.yaml` | provider (`mock`), ceilings (monthly **USD 20**, run USD 2, agent USD 1), retries, timeouts, price table with `verified_at` |
| `research-sources.yaml` | workspace root and asset list (read-only), web fetching (`enabled: false`, required in Phase 2), import limits |
| `marketing-policy.yaml` | nine banned-claim categories ported from `brand-oracle.mjs` |
| `business.yaml` | facts FB1–FB14 from `B4-fakta.json`, mandatory fill-ins |
| `brand.yaml` | positioning, pillars, tone, audiences, key messages, tagline candidates from B1 |

Environment (`.env`, git-ignored; see `.env.example`): `MIA_DATA_DIR`, `MIA_WORKSPACE_ROOT`, `MIA_PROVIDER`, `MIA_MODEL`, `MIA_FIXTURE_KEY`, `MIA_LOG_LEVEL=debug`.

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
```

Exit codes: 0 ok · 1 error · 2 run failed / review required.

Steps of a research run: `import → workspace → dedup → research_agent → validate → report`. The report step runs even after a failure so the failure is visible.

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

`testdata/fixtures/<agent>/<fixture-key>.json` (default key = inputs directory name; fallback `default.json`). Missing fixture → empty output with a warning, never a fabricated finding. Usage tokens are estimated (runes/4) and cost is USD 0 and measurable.

## 7. Data, backups, logs

- `data/mia.sqlite` (0600, WAL), `data/executions/*.request|response.txt` (0600, full prompts and raw outputs), `data/logs/mia-YYYY-MM-DD.jsonl` (redacted).
- Backup: copy `data/mia.sqlite` while no run is active (a `mia backup` command is planned for Phase 7).
- `reports/` and `drafts/` are plain files; both are git-ignored.

## 8. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `provider 'anthropic' is not available in Phase 2` | config asks for a live provider | set `provider.name: mock` |
| `web.enabled=true is not available in Phase 2` | fetching requested | set `enabled: false` |
| `config ...: contains what looks like a secret` | a key was pasted into YAML | move it to `.env` |
| run `failed` at `import` with "tidak ada berkas riset yang valid" | all files rejected or wrong directory | run `mia validate --inputs DIR` to see per-file reasons |
| step `budget_exhausted` | ceilings reached (monthly ledger in WIB) | wait for next month, raise ceilings deliberately in `agent-limits.yaml`, or stay in mock |
| `status` shows `(STALE)` | process died mid-run | `mia run research --resume RUN_ID` |
| `tls: failed to verify certificate` during `go get` | sandboxed proxy | use the local module cache (see §1) or run outside the sandbox |
