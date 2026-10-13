# Laporan Fase 2 — Fondasi Minimal dan Alur Mock

**Tanggal:** 2026-10-12 (WIB) · **Status:** Fase 2 selesai; Fase 3 belum dimulai dan menunggu persetujuan · **Biaya eksternal:** USD 0 (tidak ada panggilan berbayar, tidak ada pengambilan web).

## 1. Ringkasan

MIA kini berjalan ujung ke ujung dalam mode mock: `mia run research --inputs DIR` mengimpor catatan riset (Markdown/CSV/JSON) dengan jejak sumber, membaca aset ruang kerja Punyamas secara baca-saja dengan deteksi perubahan, mendeteksi duplikat, memanggil Agen Riset Pasar lewat penyedia tiruan (fixture deterministik, biaya nol), memvalidasi bukti dan klaim, lalu menulis laporan Markdown dan HTML berbahasa Indonesia. Run yang gagal selalu menghasilkan laporan berjudul "RUN GAGAL" dan dapat dilanjutkan dari langkah terakhir yang selesai. Plafon biaya (bulanan USD 20, per run USD 2, per agen USD 1) diperiksa sebelum setiap panggilan, termasuk dalam mode mock.

## 2. Berkas yang dibuat

| Area | Berkas |
|---|---|
| Build | `Makefile` (variabel `GO` default `~/sdk/go1.27.2/bin/go`, `GOTOOLCHAIN=local`, cache lokal `.toolchain/`), `go.mod`, `go.sum`, `.env.example`, `.gitignore` |
| CLI | `cmd/mia/main.go` (+ `main_test.go`) |
| Inti | `internal/domain`, `internal/ids`, `internal/config`, `internal/logx`, `internal/store` (+ `migrations/0001_init.sql`), `internal/orchestrator`, `internal/report` (+ dua templat), `internal/budget`, `internal/brand`, `internal/validate`, `internal/provider` (+ `mock`, `registry`), `internal/agent/research`, `internal/research/{importer,workspace,dedup}`, `internal/testutil` |
| Konfigurasi | `config/agent-limits.yaml`, `config/research-sources.yaml`, `config/marketing-policy.yaml`, `config/business.yaml`, `config/brand.yaml`; `schemas/research_findings.v1.json` |
| Data uji | `testdata/sample-batch/` (3 berkas fiktif berlabel contoh), `testdata/fixtures/research/sample-batch.json`, `testdata/workspace-mini/` (salinan baca-saja tiga berkas ruang kerja untuk uji), `testdata/failures/`, `testdata/golden/research-sample.md` |
| Dokumentasi | `README.md`, `docs/implementation/operations.md`, pembaruan `docs/implementation/acceptance-criteria.md` dan `roadmap.md`, laporan ini |

## 3. Fitur yang selesai

1. Konfigurasi YAML dengan validasi ketat (kunci tak dikenal ditolak, rahasia di YAML ditolak, penyedia selain mock dan `web.enabled: true` ditolak pada Fase 2), pemuatan `.env`, hash konfigurasi per run.
2. Logging JSON terstruktur dengan penyamaran kunci rahasia dan pemotongan kutipan tak tepercaya.
3. Penyimpanan SQLite (driver Go murni, WAL, migrasi tertanam) dengan tabel run, langkah, sumber, butir riset, bukti, aset ruang kerja, eksekusi agen, biaya, dan persetujuan.
4. Importer Markdown/CSV/JSON: sumber, referensi, waktu ambil, tanggal terbit, kutipan bukti wajib untuk fakta; penolakan berkas yang memuat nomor telepon, NIK 16 digit, email, atau kolom berbau data pribadi; batas ukuran berkas.
5. Pembaca ruang kerja baca-saja: sidik jari sha256 per aset, baseline otomatis pada impor pertama, status `unchanged/changed/missing/review_required`, perbandingan aturan `brand-oracle.mjs` dan fakta `B4-fakta.json` dengan konfigurasi MIA, `mia approve asset` untuk menerima perubahan setelah ditinjau, opsi `drift.block_runs`.
6. Dedup: kanonikalisasi URL, hash isi ternormalisasi, kemiripan judul (Jaccard ≥ 0,8).
7. Orkestrator deterministik: enam langkah berurutan, batas waktu per langkah, retry terbatas (`max_retries`), klasifikasi kesalahan penyedia (auth/billing tidak diulang), gerbang anggaran, ringkasan run, `--resume`, `--until`, deteksi run basi (`stale`).
8. Agen Riset Pasar: prompt dengan blok DATA TIDAK TEPERCAYA, keluaran JSON terstruktur, satu percobaan perbaikan terbatas, validasi referensi bukti (fakta harus mengutip butir fakta bersumber; interpretasi harus mengutip butir).
9. Pemeriksa klaim merek yang dipindahkan sebagai data dari `brand-oracle.mjs` (9 kategori) dan terbukti setara pada kasus BT1–BT12 dan TG1–TG6 dari dokumen B1.
10. Laporan Markdown dan HTML: fakta bersumber, interpretasi, hipotesis, sumber, ditolak/duplikat/mencurigakan, aset ruang kerja, eksekusi dan biaya, asumsi, langkah berikutnya.
11. Perintah CLI: `run research`, `import`, `status`, `report`, `validate`, `config check`, `approve asset`, `version`.

## 4. Uji yang dijalankan dan hasilnya

- `make test`: 12 paket `ok`, 0 gagal; 54 fungsi uji. Detektor race `ok` pada orchestrator, budget, store.
- Uji alur (end-to-end mock): run lengkap dengan laporan golden byte-identik; kegagalan penyedia diulang tepat `1+max_retries` kali lalu run gagal dengan header "RUN GAGAL"; kesalahan auth tidak diulang; batas waktu ditegakkan; plafon bulanan menghentikan run sebelum panggilan; `--resume` melanjutkan tanpa mengulang langkah yang selesai; impor ulang batch yang sama memakai ulang 13 butir tanpa duplikasi; berkas yang ditolak dilaporkan dan run tetap berjalan; konten mencurigakan dikecualikan dari prompt; `--until` menghasilkan run `partial`; deteksi dan persetujuan perubahan aset; pemblokiran run saat `drift.block_runs`.
- Demo nyata (2026-10-12): `mia config check` terhadap `~/Documents/bots/punyamas` mencatat baseline 8 aset tanpa perbedaan; `mia run research --inputs testdata/sample-batch` selesai dengan peringatan: 4 sumber, 21 butir (8 impor + 13 pertanyaan umum dari W1), 5 fakta, 2 interpretasi, 17 hipotesis, 3 temuan agen diterima, 1 ditolak (fixture yang sengaja tidak valid), 1 tanda klaim terlarang, biaya USD 0,0000. Status git `bots/punyamas` identik sebelum dan sesudah.

Cakupan pernyataan per paket (uji paket itu sendiri; paket `report`, `provider/mock`, `registry` diuji lewat uji alur sehingga tercatat 0 %):

| Paket | Cakupan |
|---|---|
| internal/agent/research | 94,9 % |
| internal/brand | 95,8 % |
| internal/research/dedup | 95,6 % |
| internal/validate | 83,3 % |
| internal/research/importer | 82,8 % |
| internal/orchestrator | 80,0 % |
| internal/research/workspace | 79,6 % |
| internal/budget | 76,9 % |
| internal/logx | 75,0 % |
| internal/config | 64,0 % |
| internal/store | 29,6 % (sebagian besar jalur dieksekusi lewat uji orkestrator) |
| cmd/mia | 11,0 % (hanya uji asap; perintah lain dicek manual) |

## 5. Keterbatasan yang diketahui

- Agen Strategi, skor peluang, deteksi konflik antar-fakta (F11), dan persetujuan peluang (F12) belum ada: Fase 3.
- Penyedia nyata (Anthropic API) dan pengambilan web belum diimplementasikan; konfigurasi menolaknya secara eksplisit.
- Pemeriksa klaim menyaring kata; kasus LK1–LK10 dan CT1–CT10 belum dijadikan uji (Fase 3).
- `report`, `mock`, dan `registry` belum punya uji unit sendiri; `store` punya cakupan langsung rendah.
- Provenance `sha256` di `config/*.yaml` sengaja kosong; nilai aktual tercatat di basis data oleh `mia config check`.
- Di sesi sandbox, `go get` gagal verifikasi TLS; modul diambil dari cache unduhan proyek `kerjalancar` (`GOPROXY=file://`). Semua checksum `go.sum` cocok dengan `go.sum` proyek itu kecuali `golang.org/x/mod` dan `golang.org/x/tools` (dependensi tidak langsung dari graf modul, bukan bagian dari biner). Verifikasi ulang dengan `GOSUMDB=sum.golang.org go mod verify` saat jaringan normal.
- `git init` dan penulisan ke `.git` diblokir di dalam sandbox; dijalankan dengan pengabaian sandbox atas permintaan Anda.

## 6. Dependensi yang ditambahkan

Langsung: `modernc.org/sqlite v1.60.1`, `go.yaml.in/yaml/v3 v3.0.5` (keduanya disetujui pada gerbang Fase 2). Tidak langsung: dependensi transitif modernc (libc, mathutil, memory, dll.) sebagaimana tercatat di `go.mod`. Tidak ada perubahan konfigurasi mesin.

## 7. Biaya

USD 0. Tidak ada panggilan API berbayar, tidak ada pencarian web, tidak ada akun eksternal yang dibuat.

## 8. Risiko tersisa

- Ketergantungan pada kualitas fixture: mode mock tidak menambah pengetahuan baru; interpretasi nyata baru hadir saat penyedia disetujui.
- Perubahan konfigurasi mengubah hash dan dapat memerlukan pembaruan laporan golden.
- Teks Indonesia dari model nyata belum teruji terhadap aturan nada merek.

## 9. Langkah berikutnya (memerlukan persetujuan)

Fase 3: Agen Strategi dengan `config/scoring.yaml`, deteksi konflik, persetujuan peluang, uji kesetaraan LK/CT; opsional bila disetujui: allowlist sumber web dari daftar di `docs/architecture/integrations.md` §5 dan adapter Anthropic API dengan API key dan plafon. Keputusan yang dibutuhkan: aktivasi pengambilan web (ya/tidak dan daftar host), jalur penyedia AI (Opsi A atau tetap mock), model default, nilai plafon per run/agen.

## 10. Addendum — remediasi temuan audit (2026-10-13)

Audit independen Fase 2 (PASS WITH CONDITIONS) menemukan A1–A7. A2–A7 diperbaiki tanpa mengubah skema basis data, dan A1 kemudian diverifikasi oleh audit pra-commit independen (lihat butir A1); rinciannya ada di `docs/implementation/acceptance-criteria.md` §8 dan `docs/implementation/operations.md`.

- **A1 (verifikasi checksum dependensi): PASS.** Diverifikasi 2026-10-13 oleh audit pra-commit independen (bukan oleh pembaruan dokumen ini, yang tidak mengulang verifikasi): salinan proyek sementara dengan cache modul baru, GOTOOLCHAIN=local, proxy resmi, GOSUMDB=sum.golang.org, verifikasi TLS aktif. Basis data checksum dihubungi (cache lookup sumdb tertulis, 30 pencarian); seluruh 28 baris `go.sum` yang di-commit ada di berkas yang dihasilkan secara independen dengan hash identik, termasuk `golang.org/x/mod v0.42.0` dan `golang.org/x/tools v0.52.0`; tidak ada konflik hash untuk modul dan versi yang sama; 32 baris tambahan adalah dependensi uji dari dependensi, bukan ketidakcocokan; `go.mod` dan `go.sum` proyek tidak berubah. Prosedur pengulangan ada di `operations.md` §1. Persetujuan commit Fase 2 dan otorisasi Fase 3 adalah keputusan manusia yang terpisah dan belum diberikan.
- **A2:** butir, tautan run, dan bukti disimpan dalam satu transaksi (`InsertItemWithEvidence`); uji injeksi kegagalan membuktikan tidak ada butir yatim dan impor ulang pulih.
- **A3:** hitungan ringkasan dibangun ulang dari basis data sebelum laporan dan saat finalisasi; run yang dilanjutkan membawa peringatan dan hitungan berkas dari eksekusi sebelumnya; laporan selalu dirender ulang saat `--resume` (sebelumnya laporan gagal yang lama tertinggal).
- **A4:** keluaran Markdown meloloskan tanda baca Markdown dan menetralkan baris baru pada setiap nilai tak tepercaya; tabel tidak dapat dirusak oleh `|`, tag HTML dan tautan dari sumber tidak dirender; teks Indonesia biasa tidak berubah.
- **A5:** Makefile memakai mode modul baca-saja; hanya `make tidy` yang boleh mengubah `go.mod`/`go.sum`; `make test` dan `make test-race` gagal bila berkas modul berubah.
- **A6:** bagian fakta, interpretasi, dan hipotesis hanya memuat butir tervalidasi; butir lain tercantum di bagian 7 dengan statusnya; run sebagian memuat spanduk "RUN BELUM LENGKAP".
- **A7:** panggilan penyedia yang gagal atau habis waktu dibukukan sebagai estimasi tidak pasti dari estimasi pra-panggilan (`uncertain:timeout|error`, `estimated = 1`); plafon menghitung baris ini sehingga kegagalan berulang tidak melewati plafon; satu baris per eksekusi (rekonsiliasi mengganti, bukan menambah); laporan memisahkan biaya aktual dan tidak pasti; penolakan autentikasi/penagihan tidak dibukukan. Tidak ada penyedia nyata yang diaktifkan dan tidak ada tagihan aktual yang diklaim.
- Laporan golden `testdata/golden/research-sample.md` dibuat ulang karena format laporan berubah (A4/A6/A7); versi sebelumnya tetap ada di riwayat git.
