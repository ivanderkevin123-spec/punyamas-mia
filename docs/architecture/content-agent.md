# MIA — Content Agent (Phase 4)

**Status:** implemented 2026-10-14, mock provider only, uncommitted pending approval. English for engineers; §0 is the business-facing summary. Contract: `schemas/content_drafts.v1.json`. Rules: `config/content.yaml`.

## 0. Ringkasan untuk pemilik usaha (Bahasa Indonesia)

Agen Konten mengubah **satu peluang strategi yang sudah disetujui** menjadi draf konten berbahasa Indonesia: caption Instagram (3 variasi), variasi headline untuk pengujian (3), konten edukasi, naskah video pendek, pesan WhatsApp, dan artikel pendek/FAQ (masing-masing 1). Semua hasil adalah **DRAF**: MIA tidak menerbitkan, mengirim, atau menjadwalkan apa pun. Model hanya mengusulkan; kode memeriksa bahwa peluang benar-benar disetujui di basis data, bahwa setiap bukti milik peluang itu dan masih tervalidasi, bahwa tidak ada harga, alamat, jam buka, statistik, testimoni, janji keuntungan, diskon, tenggat palsu, atau janji waktu proses yang dikarang, dan bahwa setiap informasi yang belum ada ditulis sebagai `[ISI: ...]`. Draf baru tidak pernah disetujui otomatis. Peninjau mencatat keputusan (`approved`/`rejected`) dengan riwayat yang hanya bisa bertambah; persetujuan di MIA berarti "boleh lanjut ke tahap berikutnya", bukan izin publikasi. Draf yang diubah setelah ditinjau kembali ke status perlu tinjauan. Ekspor ditulis ke `drafts/content/` dan tidak pernah ke ruang kerja Punyamas.

## 1. Position in the architecture

```
research run → strategy run → approve opportunity (append-only, one transaction)
                                   ↓ eligibility (database only)
content run: load_opportunity → content_agent → evaluate_content → report
                                   ↓ content_drafts + content_draft_versions + content_draft_evidence
review (approval_records, versioned) · revise (new version, back to needs_review) · export (drafts/content, DRAF)
```

A content run is a `research_runs` row with `kind = content` and `parent_run_id` = the strategy run. It reuses the orchestrator (`execute`, step timeouts, bounded retries, budget gate, resume, report always re-rendered, counts rebuilt from the store) and the shared agent-call loop `invokeAgent` (`internal/orchestrator/agentcall.go`) that the Strategy Agent also uses: budget check before every call, every call stored as an execution with request and response files, failed and timed-out calls booked as uncertain cost, one bounded repair call, and a per-run call cap counted from stored executions (`max_calls_per_run: 2` in `content.yaml`).

## 2. Eligibility (`internal/content/source.go`)

Checked from the database only, before a run is created (refusal = no run, no agent call) and again once per invocation (a resumed run stops if the approval was withdrawn):

1. the opportunity exists and its status is `approved`;
2. its latest `approval_records` decision is `approved` (a status set without a decision is refused);
3. it has a tier (not blocked or rejected by the strategy gates);
4. its strategy run is a `strategy` run, completed or completed with warnings, over the same research run;
5. its strategy execution exists in that run with outcome ok/repaired;
6. it has evidence links, and every linked item is still validated and linked to the research run.

The storage layer repeats the core rule: a trigger refuses any `content_drafts` insert whose opportunity is not `approved`.

## 3. Contract `content_drafts.v1` (`internal/agent/content`)

The prompt contains static Indonesian rules, brand tone and language rules, and — inside the `DATA TIDAK TEPERCAYA` block — the opportunity and its brief, only the opportunity's own validated evidence, and the approved business facts FB1–FB14 with their RENCANA/KEBIJAKAN status. The agent returns `{"drafts":[...],"notes":""}`; fields are listed in the schema. Parsing is strict (unknown fields rejected), so the agent cannot set status, identifiers or hashes. Structural validation per type: requested types only, allowed channel and segment, required fields, rune limits (0 = must be empty), body sections in order (`Pembuka/Isi/Penutup` for video, `Pertanyaan/Jawaban` for FAQ), variants numbered 1..limit without repeats. Excess variants are rejected; a shortfall is a warning. Evidence is referenced by ref (stable across ID generators) and resolved by code to item IDs through `opportunity_evidence` only.

## 4. Code gates (`internal/content/gates.go`)

Public text = hook, headline, body, call_to_action. `[ISI: ...]` markers are extracted first (normalised, unique) and removed before scanning, so `[ISI: kontak resmi Punyamas]` does not trip the STATUS brand rule; a marker whose label contains digits is blocked (`PLACEHOLDER_BERNILAI`), an empty label needs review.

| Gate | Rule | Outcome |
|---|---|---|
| Evidence | each ref must be evidence of this opportunity and validated; fictitious (`BUKTI_TIDAK_ADA`), unvalidated (`BUKTI_BELUM_VALID`) or other opportunity's (`BUKTI_LINTAS_PELUANG`) refs; unknown FB IDs (`FAKTA_TIDAK_ADA`) | blocked |
| No evidence | neither evidence refs nor business facts (`TANPA_BUKTI`) | needs review |
| Claims | `kind = fact` without a resolved validated fact of quality A–C or an approved business fact → `claims_requiring_verification` (`KLAIM_PERLU_VERIFIKASI`); interpretations and hypotheses are labelled, never shown as facts | needs review |
| Brand checker | the nine `marketing-policy.yaml` categories | blocked |
| Factual claims | shared `internal/claims` scanner: HARGA, JAM_BUKA, ALAMAT, STATISTIK, KLAIM_PELANGGAN, and numbers absent from cited evidence, cited business facts or `angka_boleh` (list markers and identifiers ignored) | blocked |
| Language rules | JANJI_UNTUNG, TREN_HARGA, TESTIMONI, URGENSI_PALSU, DISKON_PROMO, WAKTU_PROSES | blocked |
| Language rules | TEKANAN (`segera`, `ayo`, `!`), PLACEHOLDER_TIDAK_BAKU (`[ISI]`, `TODO`, `XXX`) | needs review |
| Duplicates | same type and channel, identical hash or token Jaccard ≥ 0.85 against an earlier draft of the run or an existing draft of the opportunity | not stored, reported with the matching draft |

Status: any blocking finding → `blocked`; placeholders, claims to verify or review findings → `needs_review`; otherwise `draft`. Review checklist: `review_checklist` from config, type checklist (WhatsApp consent), one item per placeholder and per claim to verify, RENCANA facts, then up to ten agent items (marked as agent suggestions).

**Limits (documented, not hidden):** rules match wording and patterns, not meaning. Paraphrased promises ("tidak akan pernah rugi"), implied urgency, invented facts phrased without numbers or keywords, and misleading framing can pass. Human review (D-005) remains mandatory; the mock fixtures are hand-written and say nothing about real model behaviour.

## 5. State machine and review (`internal/content/review.go`)

```
            code gates                    human review (note required)
proposal ─────────────► draft ────────────────► approved ──┐
              │        needs_review ──────────► rejected   │ text change (revise or out-of-band)
              └──────► blocked (cannot be reviewed)        ▼
                          ▲         revise ────────────► needs_review (new version) | blocked
```

- `content review` appends an `approval_records` row (`subject_type = content_draft`, `subject_version`, `subject_hash`) and mirrors the status in one transaction; it refuses blocked drafts, a decision already recorded for the same version, approval when the source opportunity is no longer approved, and drafts whose text no longer matches their hash.
- `content revise` re-runs every gate on the edited text, appends a `content_draft_versions` row (author `human`, note), bumps the version and sets `needs_review` (or `blocked`). Whitespace-only edits are refused as non-material. Evidence links are kept; a revision cannot add evidence.
- Storage guarantees: `approval_records`, `content_draft_versions` and `content_draft_evidence` are append-only (triggers); drafts cannot be deleted; a draft cannot be inserted as approved/rejected; status `approved` requires a review record for the current version and hash; any text change on an approved or rejected draft resets it to `needs_review`.
- **Readiness** ("siap serah terima ke proses persetujuan publikasi") is computed, never stored: approved for the current version and hash, text matches the hash, zero placeholders, zero claims to verify, zero open findings. A draft with `[ISI: ...]` is never ready. Readiness is not a publication permission; MIA has no publication feature.

## 6. Storage — migration `0003_content.sql`

`approval_records` gains `subject_version`, `subject_hash` and append-only triggers (for every subject). New tables: `content_drafts` (type CHECK, status CHECK, version, content hash, execution id, config/content-config/scoring/fixture hashes, strategy and research run ids; `UNIQUE(id, opportunity_id)`, `UNIQUE(run_id, proposal_index)`, `UNIQUE(opportunity_id, content_type, channel, content_hash)`), `content_draft_evidence` with composite foreign keys to `content_drafts(id, opportunity_id)` and `opportunity_evidence(opportunity_id, item_id)`, and `content_draft_versions`. Drafts, their first version and evidence links are written in one transaction. The migration upgrades a Phase 3 database in place (tested with Phase 3 data).

## 7. CLI and export

`run content --opportunity ID [--types] [--fixture] [--regenerate]`, `run content --resume RUN_ID [--force]`, `content list|show|review|revise|export`, `report RUN_ID`. A second generation for an opportunity that already has drafts is refused unless `--regenerate`; regenerated near-duplicates are skipped and named. `content export` writes `drafts/content/<draft-id>.md` (or `--out` relative name) through a temporary file and rename; absolute paths, `..`, symlink escapes and any target inside the workspace root (also a misconfigured `MIA_DRAFTS_DIR`) are refused (`internal/safepath`); blocked and rejected drafts are not exported; export never writes the database. The export always carries the DRAF banner, status, readiness, placeholders, claims to verify, evidence, versions, review history and provenance. Terminal output strips control characters.

## 8. Reports

`reports/<run>/content.md|html`: not-published banner, source opportunity with approval history and evidence, summary counts, steps, drafts grouped by status (needs review, passed gates, approved, rejected, blocked) with readiness, body as an escaped blockquote, evidence, business facts, supported and unsupported claims, placeholders, findings, assumptions, risks, checklist, versions, reviews and provenance; executions and costs. Markdown uses `text/cell/code/quote` escaping (a body cannot add headings, links, HTML or fake fields); HTML uses `html/template` auto-escaping. Since Phase 5 the status, placeholder, claim and evidence counts are derived from the stored drafts at render time, so `mia report <content-run>` stays current after reviews and revisions; only the agent-output counters come from the run summary.

## 9. Known limitations

1. Language and claim gates are lexical (see §4); human review stays mandatory.
2. Claim classification depends on the agent declaring claims; undeclared factual statements are caught only by the patterns and number check.
3. Readiness ends inside MIA; publication approval, M1 §2 preconditions and real contact or promotion data are outside MIA.
4. Duplicate detection is token-based; reworded near-copies below the threshold are kept.
5. A run whose call cap is spent must be restarted as a new run; a failed run keeps its report.
6. Only the mock provider exists; output quality of a real model is untested.
