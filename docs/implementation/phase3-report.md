# Laporan Fase 3 — Agen Strategi (mode mock)

**Tanggal:** 2026-10-13 · **Status:** diimplementasikan dan diverifikasi, **belum di-commit** (menunggu persetujuan) · **Dasar:** commit Fase 2 `4d510e0` di cabang `main` · **Penyedia AI:** mock saja (USD 0) · **Dependensi baru:** tidak ada (`go.mod`/`go.sum` tidak berubah).

## 1. Ringkasan

Agen Strategi membaca butir riset **tervalidasi** dari satu run riset, mengusulkan peluang dan brief kampanye berbahasa Indonesia, lalu kode memutuskan: kutipan bukti diverifikasi, bukti berkonflik ditampilkan dua sisi, klaim faktual tanpa dukungan ditolak, teks yang gagal pemeriksa merek diblokir, peluang yang memerlukan toko diberi label, usulan ganda digabung tanpa kehilangan bukti, dan tingkat A/B/C dihitung ulang dari bukti dengan nilai ordinal 0–3. Peluang disimpan dengan ID bukti, hash konfigurasi, hash aturan skor, dan hash fixture. Tidak ada yang disetujui otomatis; CEO mencatat keputusan (hanya-tambah) dan brief yang disetujui diekspor ke `drafts/` untuk serah terima manual. Rincian desain: `docs/architecture/strategy-agent.md`.

## 2. Berkas yang dibuat atau diubah

Baru: `config/scoring.yaml`; `internal/store/migrations/0002_strategy.sql`; `internal/agent/strategy/{strategy.go,strategy_test.go}`; `internal/strategy/{evaluate.go,evaluate_test.go}`; `internal/orchestrator/{strategy.go,strategy_test.go}`; `internal/report/{strategy.go,strategy_test.go}`; `internal/report/templates/{strategy.md.tmpl,strategy.html.tmpl,brief.md.tmpl}`; `cmd/mia/{strategy_cmd.go,strategy_cmd_test.go}`; `testdata/fixtures/strategy/{sample-batch,gates,conflict,default}.json`; `testdata/golden/strategy-sample.md`; `docs/architecture/strategy-agent.md`; laporan ini.

Diubah: `internal/domain/domain.go` (status peluang, tingkat, `StrategyCounts`, `ParentRunID`, hash), `internal/config/config.go` (`Scoring`, hash, validasi, `MIA_DRAFTS_DIR`), `internal/store/repo.go` (peluang, bukti peluang, eksekusi, riwayat persetujuan), `internal/orchestrator/orchestrator.go` (antarmuka `RenderStrategy`, `FromRun`, loop langkah bersama, hitungan strategi), `cmd/mia/main.go` (perintah baru, `report` sesuai jenis run, versi `0.3.0-phase3`), `internal/store/store_test.go` (jumlah migrasi = 2), `.env.example`, `README.md`, `docs/implementation/{operations,roadmap,acceptance-criteria}.md`, catatan penunjuk di `docs/architecture/{system-design,data-model}.md`.

## 3. Fitur yang selesai

1. Alur run strategi: `load_evidence → conflicts → strategy_agent → evaluate → report`, run terpisah dengan `parent_run_id`; `--resume` selalu merender ulang laporan; `--force` tidak menggandakan peluang.
2. Kontrak keluaran `strategy_proposals.v1` dengan parser ketat dan validasi struktural; bukti dikirim di dalam blok DATA TIDAK TEPERCAYA, diurutkan deterministik; satu percobaan perbaikan; **maksimal 2 panggilan per run** dihitung dari eksekusi tersimpan (termasuk galat dan habis waktu).
3. Lima gerbang kode: bukti (ID harus ada dan tervalidasi; tanpa fakta A–C → hipotesis, maks. C), konflik (kedua sisi, maks. C), merek dan klaim (diblokir / ditolak: HARGA, JAM_BUKA, ALAMAT, STATISTIK, KLAIM_PELANGGAN, angka tanpa dukungan), ketergantungan operasional (label), duplikat (topik sama + bukti bersama ≥ 50% atau judul ≥ 0,8; bukti digabung).
4. Penilaian ordinal 0–3 lima dimensi; kekuatan bukti dihitung kode (rubrik 0–3); dimensi lain dibatasi kode; jumlah ordinal berlabel; tingkat A/B/C dari `config/scoring.yaml`.
5. Deteksi konflik deterministik (topik sama, sumber berbeda, polaritas negasi berbeda, pernyataan mirip ≥ 0,5) ditulis ke `conflict_group` butir riset.
6. Penyimpanan: tabel `opportunities` dan `opportunity_evidence`; hash konfigurasi, aturan skor, fixture, dan ID eksekusi per peluang dan per run.
7. Tinjauan: `approve opportunity` (append-only ke `approval_records`, status peluang mengikuti keputusan terakhir, evaluasi tidak pernah diubah; `blocked`/`rejected` tidak dapat ditinjau), `export brief` (hanya `approved`; `drafts/<id>.md`; jalur di dalam ruang kerja Punyamas ditolak).
8. Laporan strategi Markdown + HTML (tingkat A/B/C, diblokir, ditolak, gerbang, konflik, dimensi, hash, biaya, eksekusi) dan brief serah terima dengan header DRAF.

## 4. Uji yang dijalankan dan hasilnya (2026-10-13)

| Perintah | Hasil |
|---|---|
| `GOPROXY=off GOSUMDB=off make test` | semua paket lulus; hash `go.mod`/`go.sum` identik sebelum dan sesudah |
| `GOPROXY=off GOSUMDB=off make test-race` | lulus |
| `make vet` | bersih |
| `make modcheck` | `go.mod/go.sum unchanged vs HEAD` |
| `make go-version` | `go1.27.2 darwin/arm64` dari `~/sdk/go1.27.2` |

Uji baru (29 fungsi uji): `internal/strategy` (13: ketertelusuran bukti, referensi tidak ada/belum tervalidasi, hipotesis maks. C, rubrik kekuatan bukti, konflik dua sisi, blokir merek, klaim tanpa dukungan, angka dan ID yang lolos, label operasional, dedup, tingkat oleh kode, keterukuran KPI, deteksi konflik), `internal/agent/strategy` (3: parser, struktural, blok tidak tepercaya dan perbaikan), `internal/orchestrator` (11: ujung-ke-ujung + golden, hash per run, fixture gerbang dengan butir `manual:unvalidated#1`, konflik, prasyarat run induk, maks. 2 panggilan, plafon anggaran, habis waktu, lanjutkan tanpa laporan basi dan tanpa duplikasi, tanpa jaringan/penyedia berbayar, riwayat persetujuan hanya-tambah), `internal/report` (1: nilai berbahaya di laporan, HTML, dan brief), `cmd/mia` (1: alur CLI terisolasi: run → opportunities → approve → export → penolakan jalur ruang kerja → keputusan kedua → blokir/tolak tidak dapat disetujui). Uji Fase 2 tetap lulus; `TestMigrateAndIntegrity` diperbarui ke dua migrasi.

## 5. Verifikasi asal bukti dan anggaran

- Setiap tautan bukti peluang merujuk butir tervalidasi run induk (diuji); butir belum tervalidasi tidak pernah masuk prompt maupun tautan.
- Hash fixture pada run = sha256 berkas fixture yang diputar; hash aturan skor = 16 heksa awal sha256 `config/scoring.yaml`; hash konfigurasi run induk diwariskan.
- Biaya mock USD 0 dan terukur; panggilan yang habis waktu dibukukan sebagai estimasi tidak pasti dan dihitung terhadap plafon; plafon bulanan memblokir panggilan sebelum terjadi (`budget_exhausted`, nol panggilan).
- Kapabilitas penyedia mock: `Network=false`; prompt dan keluaran tersimpan dengan mode 0600.

## 6. Keputusan desain dan penyimpangan dari rencana

1. **Konflik memerlukan kemiripan pernyataan.** Heuristik awal (topik sama + polaritas negasi berbeda) menghasilkan konflik palsu pada data contoh (dua fakta tentang toko yang berbeda). Ditambahkan ambang `conflict.min_similarity: 0.5` (Jaccard token setelah kata negasi dihapus). Konsekuensi: kontradiksi yang diparafrase tanpa kata negasi tidak terdeteksi (keterbatasan terdokumentasi).
2. **Gerbang operasional memindai bidang eksekusi saja** (pesan, penawaran, kanal, uji, sumber daya, ketergantungan, tindakan) dengan frasa ketergantungan (`di toko`, `toko kami`, `konter`, `petugas`, …); kata `toko` telanjang di bidang interpretasi menandai toko pesaing dan sebelumnya memberi label salah.
3. **Batas 2 panggilan per run menghitung kegagalan.** Run yang menghabiskan dua panggilan (termasuk habis waktu) gagal tertutup dengan pesan `batas 2 panggilan` dan status langkah yang jujur (`timed_out` bila hasil terakhir habis waktu); `--resume` hanya menyelamatkan run yang gagal di langkah lain atau setelah satu kegagalan fatal. Run baru dibuat dengan `--from-run`.
4. **Usulan yang ditolak tidak dihitung sebagai hipotesis** agar hitungan ringkasan tidak menyesatkan.
5. Fixture contoh: referensi `pertanyaan-pelanggan-contoh.csv#1` diperbaiki menjadi `#2` (baris 1 CSV adalah header); kata `resmi` dihapus dari fixture karena kategori STATUS pemeriksa merek.
6. Tidak ada adapter penyedia berbayar, tidak ada pengambilan web, tidak ada dependensi baru.

## 7. Keterbatasan yang diketahui

Lihat `docs/architecture/strategy-agent.md` §10: deteksi konflik dan klaim bersifat leksikal (tinjauan manusia tetap wajib, D-005); label operasional bergantung pada bidang eksekusi dan bendera `requires_store`; nilai dimensi selain kekuatan bukti adalah pendapat agen dalam batas kode; fixture mock ditulis tangan sehingga belum ada bukti perilaku model nyata; temuan audit A8–A16 tetap terbuka (`acceptance-criteria.md` §9).

## 8. Biaya

USD 0. Tidak ada panggilan penyedia nyata, tidak ada akun eksternal.

## 9. Ruang kerja Punyamas

MIA tidak menulis ke `~/Documents/bots/punyamas`: semua uji memakai `testdata/workspace-mini`, dan `export brief` menolak jalur di dalam akar ruang kerja (diuji). Catatan jujur: selama tugas ini HEAD repositori ruang kerja bergerak dari `a178aee` ke commit lain oleh proses lain di mesin ini (agen Claw-Empire), dengan `git status --porcelain` kosong sebelum dan sesudah; perubahan itu bukan berasal dari MIA.

## 10. Langkah berikutnya (memerlukan persetujuan)

Tinjau diff lengkap (termasuk berkas baru) dan laporan golden `testdata/golden/strategy-sample.md`; bila disetujui, commit Fase 3. Fase 4 (Agen Konten) belum dimulai dan tidak akan dimulai tanpa persetujuan terpisah.
