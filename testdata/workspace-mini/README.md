# workspace-mini (test fixture)

A minimal read-only copy of three Punyamas workspace files used by MIA tests so that the test
suite never touches `~/Documents/bots/punyamas`:

- `tools/spec-oracle/brand-oracle.mjs` — copied verbatim from the workspace (commit `0fac37a`, 2026-10-12).
- `docs/02-brand/B4-fakta.json` — copied verbatim (same commit).
- `docs/05-marketing/W1-naskah-situs-company-profile.md` — only the "Pertanyaan umum" section.

These are fixtures, not sources of truth; the real files are read by `mia config check`.
