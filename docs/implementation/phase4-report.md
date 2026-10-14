# Laporan Fase 4 — Agen Konten (mode mock)

**Tanggal:** 2026-10-14 · **Status:** diimplementasikan dan diverifikasi, **belum di-commit** (menunggu persetujuan) · **Dasar:** commit Fase 3 `4ccd4fa` di `main` (= `origin/main`) · **Penyedia AI:** mock saja (USD 0) · **Dependensi baru:** tidak ada (`go.mod`/`go.sum` tidak berubah). Rincian desain: `docs/architecture/content-agent.md`.

## 1. Ringkasan

Agen Konten membuat draf konten pemasaran berbahasa Indonesia dari **satu peluang strategi berstatus `approved`**. Kelayakan peluang dibaca dari basis data, bukan dari input pengguna. Model hanya mengusulkan; kode memutuskan bukti, klaim, bahasa, penanda `[ISI: ...]`, duplikat, dan status. Semua draf berstatus DRAF. MIA tidak menerbitkan, mengirim, atau menjadwalkan apa pun, dan persetujuan peninjau di MIA bukan izin publikasi.

## 2. Fitur yang selesai

1. **Run konten** `load_opportunity → content_agent → evaluate_content → report` sebagai `research_runs` berjenis `content` dengan `parent_run_id` = run strategi. Memakai ulang orkestrator (batas waktu, retry terbatas, gerbang anggaran, resume, laporan selalu dirender ulang).
2. **Kelayakan** dari basis data: status `approved`, keputusan terakhir di riwayat juga `approved`, punya tingkat, run strategi selesai dengan run riset yang sama, eksekusi strategi berhasil, dan semua bukti masih tervalidasi. Penolakan tidak membuat run dan tidak memanggil agen; resume memeriksa ulang.
3. **Kontrak `content_drafts.v1`** (parser ketat; agen tidak dapat menetapkan status, ID, atau hash) untuk enam jenis konten dengan batas di `config/content.yaml`: caption Instagram (3 variasi), variasi headline (3), edukasi, naskah video pendek, pesan WhatsApp draf, artikel/FAQ (masing-masing 1).
4. **Gerbang kode:** bukti harus milik peluang dan tervalidasi; harga, jam buka, alamat, statistik, klaim pelanggan, angka tanpa sumber, janji keuntungan, tren harga, testimoni, urgensi palsu, diskon, dan janji waktu proses memblokir draf; ajakan mendesak dan isian tidak baku memerlukan tinjauan; klaim fakta tanpa dukungan dicatat sebagai perlu verifikasi.
5. **Penanda `[ISI: ...]`** dicatat dari teks, masuk checklist, dan membuat draf tidak pernah siap serah terima.
6. **Duplikat** dalam run dan terhadap draf lama dilaporkan dan tidak disimpan; pembuatan ulang memerlukan `--regenerate`.
7. **Status dan tinjauan:** `draft`, `needs_review`, `approved`, `rejected`, dan `blocked` (gerbang keras gagal; tidak dapat ditinjau). Tinjauan dan versi bersifat hanya-tambah; revisi atau perubahan teks di luar MIA mengembalikan draf ke `needs_review`.
8. **Laporan** Markdown dan HTML serta **ekspor** DRAF ke `drafts/content/` dengan penjaga path.
9. **Penguatan pendukung:** pemindai klaim faktual dipakai bersama (`internal/claims`), loop panggilan agen dipakai bersama (`agentcall.go`), pemuat bukti dipakai bersama (`strategy.LoadEvidence`), penjaga path (`internal/safepath`), dan keputusan peluang kini atomik (`RecordOpportunityDecision`).

## 3. Perintah CLI yang benar-benar dijalankan

Dijalankan dengan `bin/mia` (versi `0.4.0-phase4`) di direktori data, laporan, dan draf sementara dalam `$TMPDIR`, dengan `MIA_WORKSPACE_ROOT=testdata/workspace-mini`:

| Perintah | Hasil |
|---|---|
| `run research --inputs testdata/sample-batch`, `run strategy --from-run …`, `opportunities --run …` | run riset dan strategi selesai |
| `run content --opportunity …` sebelum disetujui | ditolak: status `proposed` dan tidak ada keputusan |
| `approve opportunity --decision approved` lalu `run content --opportunity …` | 10 draf: 5 lolos gerbang, 5 perlu tinjauan, 6 isian, 1 klaim perlu verifikasi |
| `content list --run …`, `content show --id …` | status, peluang asal, isian, klaim, kesiapan tampil |
| `content review --decision approved` | dicatat; "bukan izin publikasi" |
| `content export --id …` | berkas DRAF di `drafts/content/` |
| `content export --out ../keluar`, ekspor dengan `MIA_DRAFTS_DIR` di ruang kerja | ditolak (exit 1) |
| `run content --opportunity …` kedua kali tanpa `--regenerate` | ditolak |
| `content revise --body-file …` | versi 2, kembali ke `needs_review` |
| `run content … --fixture gates --regenerate` lalu `content review` pada draf diblokir | 9 diblokir, 1 ditolak struktural, 1 duplikat; tinjauan ditolak |

## 4. Migrasi dan tabel

Migrasi `0003_content.sql`: kolom `subject_version` dan `subject_hash` pada `approval_records` serta pemicu hanya-tambah untuk semua subjek; tabel `content_drafts`, `content_draft_versions`, dan `content_draft_evidence` (foreign key komposit ke `opportunity_evidence`). Pemicu menolak draf untuk peluang yang tidak disetujui, draf baru berstatus `approved`/`rejected`, persetujuan tanpa catatan tinjauan untuk versi saat ini, peninjauan draf diblokir, dan penghapusan draf. Perubahan teks pada draf yang sudah ditinjau mengembalikannya ke `needs_review`. Migrasi diuji pada basis data Fase 3 berisi data.

## 5. Hasil pengujian (2026-10-14)

| Perintah | Hasil |
|---|---|
| `GOPROXY=off GOSUMDB=off make test` | semua paket lulus; hash `go.mod`/`go.sum` sama sebelum dan sesudah |
| `GOPROXY=off GOSUMDB=off make test-race` | lulus |
| `make vet` | bersih |
| `make modcheck` | `go.mod/go.sum unchanged vs HEAD` |

Uji baru mencakup 19 butir yang diminta (pemetaan lengkap di `acceptance-criteria.md` §10). Satu uji lama diperbarui secara sah: `TestMigrateAndIntegrity` kini mengharapkan 3 migrasi. Laporan golden strategi tidak berubah setelah refaktor.

## 6. Contoh keluaran dari fixture

Caption Instagram varian 1 (status `draft`, setelah disetujui: siap serah terima):

> Hook: "Ditimbang dan diuji di depan Anda." — Isi: "Kami akan menimbang dan menguji barang di depan Bapak atau Ibu, dan harga akan datang dengan dasar hitungnya." (bukti `artikel-edukasi-contoh.json#1`, fakta usaha FB7)

Pesan WhatsApp (status `needs_review`): dua isian `[ISI: syarat dan periode promosi]` dan `[ISI: alamat cabang]`, checklist persetujuan promosi terpisah, dan FB1 berstatus RENCANA. Laporan lengkap: `testdata/golden/content-sample.md`.

## 7. Bukti penegakan aturan

- **Status persetujuan:** tujuh status peluang tidak layak ditolak tanpa run atau panggilan agen; pemicu basis data menolak draf untuk peluang yang tidak disetujui.
- **Bukti:** referensi fiktif, belum tervalidasi, dan lintas peluang memblokir draf dan tidak pernah ditautkan; foreign key komposit menolak tautan lintas peluang.
- **Isian:** draf berisi `[ISI: ...]` yang disetujui tetap "TIDAK siap serah terima".
- **Ekspor:** label DRAF, metadata tinjauan, dan isian selalu ada; traversal, path absolut, symlink keluar, dan ruang kerja ditolak; ekspor gagal tidak mengubah draf dan tidak meninggalkan berkas.

## 8. Berkas yang berubah

18 berkas terlacak diubah dan 33 berkas baru dibuat (termasuk laporan ini). Isinya hanya kode, uji, fixture, konfigurasi, dan dokumentasi Fase 4, ditambah refaktor pendukung di atas. Tidak ada commit dan tidak ada push.

## 9. Risiko dan keterbatasan

1. Aturan bahasa dan klaim bersifat leksikal. Parafrase dapat lolos, sehingga tinjauan manusia (D-005) tetap wajib.
2. Fixture mock ditulis tangan dan tidak menguji kualitas model sungguhan. Efektivitas pemasaran draf belum dibuktikan dan memerlukan eksperimen.
3. Kesiapan serah terima berakhir di MIA. Syarat M1 §2, persetujuan publikasi, dan data kontak atau promosi nyata berada di luar MIA.
4. Temuan audit A8–A16 tetap terbuka. Temuan baru: templat strategi dan brief Fase 3 menampilkan referensi bukti dalam kurung tanpa escape (risiko rendah; templat konten Fase 4 sudah memakai code span).

## 10. Konfirmasi

Tidak ada commit, push, akses web, penyedia berbayar, atau perubahan pada ruang kerja `~/Documents/bots/punyamas`. Fase 5 belum dimulai.

Catatan jujur tentang ruang kerja: HEAD tetap `6ebe592`, tetapi pada 2026-10-14 pukul 09.11 WIB muncul direktori tak terlacak `dashboard/` (salinan proyek Next.js bertanggal 4 Oktober, dengan folder `.claude/` yang diubah pukul 09.14). Direktori itu dibuat proses lain di mesin ini. Isinya tidak memuat artefak MIA, dan satu-satunya jalur tulis MIA adalah data, laporan, dan ekspor di direktori yang dikonfigurasi, dengan tujuan di dalam ruang kerja ditolak.
