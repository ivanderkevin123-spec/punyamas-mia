# Memo Keputusan Fase 1 — Punyamas Marketing Intelligence Agent (MIA)

**Tanggal:** 2026-10-12 (WIB) · **Status:** Fase 1 selesai; menunggu persetujuan pemilik untuk Fase 2 · **Bahasa:** Indonesia (ringkasan bisnis); rincian teknis berbahasa Inggris ada di dokumen yang dirujuk.

## 1. Ringkasan eksekutif

MIA adalah alat bantu internal untuk riset pasar, pemeringkatan peluang, dan penyusunan draf brief pemasaran Punyamas. Fase 1 menghasilkan spesifikasi lengkap: visi, cakupan MVP, alur kerja, kerangka KPI, desain sistem, model data, integrasi, keamanan, model biaya, peta jalan, dan kriteria penerimaan. Rekomendasi: **aplikasi CLI Go (toolchain Go 1.27.2 di `~/sdk`), SQLite lokal, orkestrator deterministik, penyedia AI tiruan (mock) sebagai mode awal, laporan Markdown/HTML**, dua agen logis pada MVP (Riset Pasar dan Strategi), dan ruang kerja `bots/punyamas` sebagai sumber kebenaran **baca-saja** dengan deteksi perubahan. Biaya eksternal pada Fase 2–3 adalah **USD 0**; plafon keras **USD 20 per bulan** berlaku bila nanti AI atau pencarian berbayar disetujui. Tidak ada kode aplikasi, dependensi, layanan, atau panggilan berbayar yang dibuat pada Fase 1.

## 2. Tujuan bisnis

Membantu Punyamas menarik calon penjual emas yang tepat dan membangun kepercayaan, dengan keputusan pemasaran yang **berbasis bukti**, konsisten dengan aturan merek, dan terukur. MIA tidak menjanjikan kenaikan penjualan; yang diukur adalah KPI operasional (waktu, kualitas bukti, biaya) sejak Fase 3 dan KPI bisnis (pertanyaan masuk, biaya per pertanyaan, konversi) hanya setelah toko beroperasi dan sumber datanya disetujui (`docs/product/kpi-framework.md`).

## 3. MVP yang direkomendasikan (Fase 2–3)

- Eksekusi manual lewat CLI; impor berkas Markdown/CSV/JSON dan dokumen ruang kerja Punyamas (baca-saja, dengan jejak sumber dan deteksi perubahan).
- Catatan riset yang selalu bersumber (sumber, URL/berkas, waktu ambil, tanggal terbit, kutipan bukti, tautan bukti ke klaim), dedup, dan pemisahan fakta / interpretasi / hipotesis / rekomendasi / keputusan.
- Agen Riset Pasar dan Agen Strategi (skor dengan rumus terbuka dan alasan tertulis), validasi keluaran terstruktur, pemeriksa klaim yang diturunkan dari `brand-oracle.mjs` dan pemeriksaan fakta dari `B4-fakta.json`.
- Penyimpanan SQLite, laporan Markdown/HTML, pencatatan status dan biaya, persetujuan manusia, uji otomatis.
- **Ditunda:** Agen Konten (Fase 4), Analis Kinerja (Fase 5), penjadwalan dan notifikasi (Fase 6). **Di luar cakupan:** dashboard web, CRM, integrasi dashboard operasional, tindakan eksternal apa pun. Rincian: `docs/product/mvp-scope.md`.

## 4. Arsitektur yang direkomendasikan

**Monolit modular dengan penyedia yang dapat ditukar** (Opsi 2 di `docs/architecture/system-design.md`): satu proses, langkah berurutan yang deterministik, antarmuka `Provider`/`Agent`/`Importer`/`Reporter`, tanpa percakapan antar-agen, tanpa alat untuk model. Makefile akan memakai variabel `GO` yang default ke `~/sdk/go1.27.2/bin/go` dan `GOTOOLCHAIN=local`; perintah `make go-version` memverifikasi toolchain.

## 5. Alternatif yang dipertimbangkan

- **Aplikasi minimal tanpa antarmuka penyedia:** paling cepat, tetapi menuntut penulisan ulang saat agen Konten/Kinerja dan penyedia nyata ditambahkan.
- **Kerangka multi-agen:** biaya, kompleksitas, dan permukaan serangan lebih besar; bertentangan dengan plafon biaya dan kebutuhan determinisme.
- Bahasa lain (Node 22, Python 3.12): tersedia, tetapi Node default di mesin ini v10 dan Python default 3.9 (akhir masa dukungan); Go 1.27.2 native sudah dipakai di proyek Anda yang lain.

## 6. Pertukaran utama

| Pilihan | Keuntungan | Harga yang dibayar |
|---|---|---|
| Mock dulu | Biaya nol, uji deterministik | Belum ada pengetahuan baru dari luar sampai sumber disetujui |
| Tanpa dashboard web | Sederhana, aman | Laporan dibaca sebagai berkas |
| Skor heuristik | Transparan, dapat diaudit | Bukan prediksi statistik; harus dinyatakan demikian di laporan |
| Pemeriksa klaim berbasis kata | Konsisten dengan ruang kerja | Tidak menilai makna; tinjauan manusia tetap wajib |
| Baca-saja ke `bots/punyamas` | Tidak ada tabrakan dengan worktree Claw-Empire | Konfigurasi MIA bisa tertinggal; dideteksi, bukan otomatis diperbarui |

## 7. Kelayakan sumber data

Tersedia sekarang: berkas impor dan dokumen ruang kerja. Kandidat yang memerlukan verifikasi dan persetujuan: situs publik (daftar di §8), Google Trends (ekspor manual), penyedia pencarian (harga belum diverifikasi; alat pencarian Anthropic USD 10 per 1.000 pencarian, terverifikasi 2026-10-12), analitik situs dan Search Console (belum ada properti; situs belum terbit), ekspor iklan (belum ada akun), notifikasi Telegram/email (perlu token/SMTP), data dashboard (belum ada antarmuka; di luar cakupan sampai disetujui). Rincian: `docs/architecture/integrations.md` §1.

**Penyedia AI.** Dokumen resmi Anthropic yang dibaca hari ini menyatakan login langganan (OAuth) "dirancang untuk pemakaian biasa" dan pengembang yang membuat produk/layanan "sebaiknya memakai autentikasi API key"; mode skrip `--bare` tidak memakai login langganan; Ketentuan Konsumen membatasi akses otomatis kecuali lewat API key. Kesimpulan: **Opsi A (API dengan tagihan terpisah)** direkomendasikan bila AI nyata dibutuhkan; **Opsi B (CLI langganan) tidak direkomendasikan dan tidak diaktifkan**; **Opsi C (mock) dipakai sekarang**. Pilihan model (Opus 5.5 / Sonnet 5.5 / Haiku 5.5) adalah keputusan pemilik; semuanya muat di bawah plafon pada volume yang direncanakan.

## 8. Usulan daftar kompetitor (untuk ditinjau manusia; belum diverifikasi)

- **Pesaing langsung:** PT Pegadaian (layanan emas ritel milik negara), buyback Logam Mulia Antam, toko emas tradisional di Surabaya (nama dikumpulkan di lapangan), layanan jual-beli emas daring dengan buyback fisik di Jawa Timur (diidentifikasi pemilik).
- **Layanan berdekatan:** gadai emas (Pegadaian, bank syariah), platform emas digital, pabrikan/merek perhiasan yang berbasis di Surabaya.
- **Pesaing pencarian dan konten:** artikel media keuangan nasional untuk kueri "cara jual emas" dan sejenisnya, halaman edukasi institusi di atas, kanal YouTube/TikTok (catatan manual).
- **Sumber wawasan non-pesaing:** wawancara lapangan pemilik (bukti kualitas tertinggi), Google Trends, forum publik berbahasa Indonesia, ulasan dan listing marketplace (dibaca manual), halaman regulator (berlabel VERIFY).
Tidak ada klaim fitur, harga, atau penawaran. Setiap sumber harus lolos daftar periksa 10 butir (`integrations.md` §4) sebelum masuk allowlist. Pengambilan data **tidak diaktifkan**.

## 9. Model biaya dan harga yang belum terselesaikan

Harga per token diverifikasi 2026-10-12 dari halaman harga Anthropic (Opus 5.5 USD 4/20, Sonnet 5.5 USD 2/10, Haiku 5.5 USD 0,10/0,50 per juta token masuk/keluar; Batch API diskon 50 %; pencarian web USD 10 per 1.000). Volume token masih asumsi. Perkiraan: Skenario A **USD 0**; Skenario B (4 run/bulan) **≈ USD 0,1–3,5** tanpa pencarian, **≈ USD 0,9–4,9** dengan pencarian; Skenario C (harian + mingguan) **≈ USD 0,3–10,5** tanpa pencarian, **≈ USD 2,6–14,7** dengan pencarian. Belum diverifikasi: batas laju akun baru, harga penyedia pencarian pihak ketiga, harga alat web fetch. Plafon USD 20 ditegakkan sebelum setiap panggilan; biaya yang tidak dapat diukur dilaporkan secara eksplisit. Rincian dan rumus: `docs/architecture/cost-model.md`.

## 10. Pertimbangan keamanan dan hukum

Rahasia hanya di `.env`; model tidak punya alat; konten web dan impor diperlakukan sebagai data tidak tepercaya dengan pendeteksi instruksi tersembunyi; allowlist kosong sampai disetujui; robots.txt dihormati; tidak ada bypass login/CAPTCHA; tidak ada data pribadi pelanggan (penjaga PII pada impor); semua tindakan eksternal tidak ada dalam kode. Hukum: aturan klaim merek (UU 8/1999, Etika Pariwara, versi final menunggu konsultan N70), hak cipta (kutipan pendek dengan atribusi), nama merek belum lolos pemeriksaan (D-019), ketentuan Anthropic (§7). Rincian: `docs/architecture/security.md`.

## 11. Asumsi dan pertanyaan terbuka

Asumsi: CEO adalah satu-satunya operator dan pemberi persetujuan; toko belum beroperasi selama MVP; volume token sesuai §4 model biaya; daftar klaim akan berubah setelah tinjauan konsultan. Perbedaan dengan audit Fase 0 yang dicatat hari ini: HEAD `bots/punyamas` adalah `d7ee05b` saat Fase 1 dimulai dan `0fac37a` saat Fase 1 berakhir (akhir Fase 0: `446cb3f`), akibat commit dari sesi lain milik pemilik, bukan MIA (MIA tidak pernah menulis ke sana); `git init` di `mia` diblokir oleh sandbox dan izin sesi sehingga **repositori belum dibuat** (hanya `.gitignore`); identitas commit belum ada di `mia`.

Terbuka: (1) kapan dan apakah membuat akun Anthropic Console untuk Opsi A; (2) model default; (3) kandidat kompetitor mana yang disetujui untuk Fase 3 dan apakah pengambilan web diaktifkan; (4) penyedia pencarian; (5) nilai `ceiling_run`/`ceiling_agent` (usulan USD 2 / USD 1); (6) kanal notifikasi Fase 6; (7) format ekspor dashboard (kelak).

## 12. Dependensi yang memerlukan persetujuan

Fase 2: `modernc.org/sqlite`, `go.yaml.in/yaml/v3`. Fase 3 (opsional): `golang.org/x/net/html` bila pengambilan web disetujui; `github.com/anthropics/anthropic-sdk-go` bila Opsi A disetujui. Tidak ada yang dipasang pada Fase 1.

## 13. Tahapan implementasi

Fase 2 fondasi dan alur mock (USD 0) → Fase 3 agen Riset dan Strategi dengan sumber yang disetujui → Fase 4 Konten → Fase 5 Analisis kinerja → Fase 6 Penjadwalan dan notifikasi → Fase 7 Evaluasi dan pengerasan. Setiap fase punya tujuan, cakupan, prasyarat, modul, dependensi, rencana uji, kriteria penerimaan, risiko, biaya, dan gerbang persetujuan (`docs/implementation/roadmap.md`).

## 14. Kriteria persetujuan Fase 2

Fase 2 dapat dimulai bila pemilik: (a) menyetujui memo ini dan sebelas dokumen Fase 1; (b) membuat repositori `mia` (`git init -b main`) atau mengizinkan sesi membuatnya; (c) menetapkan identitas commit lokal; (d) menyetujui dua dependensi Fase 2; (e) mengonfirmasi daftar aset ruang kerja (`integrations.md` §6); (f) menegaskan bahwa Fase 2 berjalan **mock-only, USD 0, tanpa pengambilan web, tanpa integrasi dashboard**.
