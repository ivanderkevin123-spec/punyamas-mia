# Laporan Strategi Pemasaran MIA — Punyamas

| Run strategi | Run riset (bukti) | Status | Dibuat (UTC) | Selesai (UTC) | Penyedia AI | Konfigurasi | Aturan skor | Fixture |
|---|---|---|---|---|---|---|---|---|
| `run_0002` | `run_0001` | **Selesai** | 2026-10-12T10:00:00Z | 2026-10-12T10:00:00Z | mock | `testconfig` | `testscoring` | `003a6b3c725213094941698d6aafa3589e267dfe25046ea4cfa669d444d3f6a0` |

Semua peluang di bawah adalah **usulan** yang menunggu tinjauan manusia; tidak ada yang disetujui otomatis. Fakta bersumber, interpretasi strategis, dan rekomendasi dipisahkan. Tingkat dihitung ulang oleh kode dari bukti tervalidasi; nilai dimensi adalah ordinal 0–3.

## 1. Ringkasan

- Bukti tervalidasi tersedia: 16 butir · kelompok konflik: 0
- Usulan agen: 5 · digabung (duplikat): 1 · dievaluasi: 4
- Tingkat A: 3 · Tingkat B: 0 · Tingkat C: 1 · diblokir (merek): 0 · ditolak (bukti/klaim): 0
- Hipotesis: 1 · bukti berkonflik: 0 · memerlukan toko/operasional: 1
- Biaya: USD 0.0000 (aktual USD 0.0000, estimasi tidak pasti USD 0.0000) · Plafon bulanan: USD 20.00

## 2. Status langkah

| Langkah | Status | Percobaan | Catatan |
|---|---|---|---|
| load\_evidence | completed | 1 |  |
| conflicts | completed | 1 |  |
| strategy\_agent | completed | 1 |  |
| evaluate | completed | 1 |  |



## 3. Tingkat A (3)

### [A] Tunjukkan penimbangan dan pengujian di depan penjual sebagai pembeda utama — `opp_0001`

- **Status:** usulan — menunggu tinjauan manusia · **Segmen:** AU1 · **Topik:** `transparansi_penimbangan`
- **Tingkat:** A · **Jumlah ordinal:** 13/15 (jumlah ordinal, bukan probabilitas) · menggabungkan usulan #4
- **Kebutuhan pelanggan:** Penjual perorangan ingin melihat sendiri proses penimbangan dan dasar harga karena khawatir ditipu timbangan atau kadar.
- **Interpretasi strategis (bukan fakta):** Interpretasi: ada toko contoh yang menimbang di luar pandangan pembeli, sementara artikel edukasi menyarankan penimbangan di depan penjual; kejelasan proses tampak menjadi kebutuhan yang belum selalu dipenuhi.
- **Fakta pendukung (ID bukti):**
  - `item_0004` `catatan-lapangan-contoh.md#2` kualitas A — Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli; pembeli hanya menerima angka akhir. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
  - `item_0001` `artikel-edukasi-contoh.json#1` kualitas B — Artikel contoh menyarankan agar penjual meminta penimbangan dilakukan di depan mata dan menanyakan kadar yang dipakai sebagai dasar harga. (sumber: CONTOH — artikel edukasi fiktif (pengujian))
- **Penilaian:** Kekuatan bukti 3 (code) · Niat pelanggan 2 (agent) · Kesesuaian strategis 3 (agent) · Kemudahan pengujian 3 (agent) · Keterukuran 2 (agent)
- **Pesan (bahasa niat):** Kami akan menimbang dan menguji barang Bapak atau Ibu di depan Bapak atau Ibu, dan harga akan datang dengan dasar hitungnya.
- **Konsep penawaran:** Seri konten edukasi pendek: apa yang terjadi di meja penimbangan dan bagaimana dasar harga dijelaskan. · **Kanal:** Media sosial dan WhatsApp Business (setelah syarat M1 terpenuhi)
- **Uji:** Uji dua versi pesan (proses terlihat vs dasar hitung) pada konten edukasi yang sama; bandingkan pertanyaan masuk per versi. · **Metrik:** O4 · **Kriteria:** Minimal satu brief dari peluang ini disetujui peninjau dan pesan lolos pemeriksa teks merek tanpa revisi klaim.
- **Sumber daya:** Penulis konten internal, tinjauan CEO, aset visual dari brand guide B3. · **Upaya:** rendah
- Risiko: Pesan dapat terdengar menuduh pesaing bila tidak ditulis netral.
- Asumsi: Calon penjual mencari informasi proses sebelum datang.
- Bukti yang kurang: Belum ada data pertanyaan masuk nyata; bukti berasal dari catatan contoh dan artikel contoh.
- Ketergantungan: Syarat M1 §2 (nama aman, akun atas nama usaha) sebelum publikasi.
- **Tindakan berikutnya:** Susun dua draf konten edukasi dan ajukan ke pemeriksa teks merek serta tinjauan CEO.

### [A] Jawab pertanyaan tentang KTP sebelum calon penjual datang — `opp_0002`

- **Status:** usulan — menunggu tinjauan manusia · **Segmen:** AU1 · **Topik:** `pertanyaan_identitas`
- **Tingkat:** A · **Jumlah ordinal:** 12/15 (jumlah ordinal, bukan probabilitas)
- **Kebutuhan pelanggan:** Calon penjual bertanya apakah KTP diperlukan dan ragu tentang pencatatan identitas.
- **Interpretasi strategis (bukan fakta):** Interpretasi: penjelasan singkat mengapa identitas dicatat (sebagai perlindungan, bukan hambatan) dapat mengurangi keraguan sebelum kunjungan.
- **Fakta pendukung (ID bukti):**
  - `item_0005` `catatan-lapangan-contoh.md#3` kualitas A — Dua pengunjung yang diamati bertanya kepada petugas apakah KTP diperlukan untuk menjual perhiasan. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
- **Hipotesis yang dikutip:**
  - `item_0011` `workspace:w1#3` — Calon pelanggan mungkin bertanya: Apakah saya perlu membawa identitas?
- **Penilaian:** Kekuatan bukti 3 (code) · Niat pelanggan 2 (agent+cap (kurang dari dua sumber independen)) · Kesesuaian strategis 2 (agent) · Kemudahan pengujian 3 (agent) · Keterukuran 2 (agent)
- **Pesan (bahasa niat):** Untuk setiap transaksi, kami akan mencatat identitas. Data Bapak atau Ibu hanya dipakai untuk keperluan yang diwajibkan.
- **Konsep penawaran:** Tanya-jawab singkat tentang identitas pada halaman edukasi dan balasan otomatis WhatsApp. · **Kanal:** Halaman tanya-jawab situs dan WhatsApp Business
- **Uji:** Bandingkan jumlah pertanyaan tentang KTP sebelum dan sesudah tanya-jawab dipublikasikan. · **Metrik:** O4 · **Kriteria:** Brief disetujui dan teks tanya-jawab lolos pemeriksa teks merek.
- **Sumber daya:** Penulis konten, tinjauan kepatuhan. · **Upaya:** rendah
- Risiko: Penjelasan yang terlalu teknis dapat menakutkan calon penjual.
- Asumsi: Keraguan tentang identitas menghambat kunjungan.
- Bukti yang kurang: Belum ada pertanyaan nyata lewat kanal usaha.
- Ketergantungan: Kanal WhatsApp Business aktif (Q61).
- **Tindakan berikutnya:** Tulis draf tanya-jawab identitas berdasarkan FB5 dan CT3.

### [A] Edukasi mengapa penawaran berbeda antar toko — `opp_0003`

- **Status:** usulan — menunggu tinjauan manusia · **Segmen:** AU2 · **Topik:** `praktik_harga_pesaing`
- **Tingkat:** A · **Jumlah ordinal:** 12/15 (jumlah ordinal, bukan probabilitas)
- **Kebutuhan pelanggan:** Penjual berulang ingin harga yang konsisten dan paham mengapa penawaran berbeda antar toko.
- **Interpretasi strategis (bukan fakta):** Interpretasi: menjelaskan komponen penawaran (berat bersih, kadar, pengurang) tanpa menyebut angka dapat membangun kepercayaan tanpa bersaing lewat klaim harga.
- **Fakta pendukung (ID bukti):**
  - `item_0002` `artikel-edukasi-contoh.json#2` kualitas B — Artikel contoh menyatakan bahwa penawaran harga untuk barang yang sama dapat berbeda antar toko karena perbedaan metode uji dan potongan. (sumber: CONTOH — artikel edukasi fiktif (pengujian))
  - `item_0003` `catatan-lapangan-contoh.md#1` kualitas A — Petugas mencatat bahwa papan di depan toko menampilkan angka per gram untuk emas 24K dan 22K dan menyebut "harga tertinggi" sebagai slogan. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
- **Penilaian:** Kekuatan bukti 3 (code) · Niat pelanggan 2 (agent) · Kesesuaian strategis 3 (agent) · Kemudahan pengujian 2 (agent) · Keterukuran 2 (agent)
- **Pesan (bahasa niat):** Harga akan datang dengan dasar hitungnya: berat bersih, kadar, dan pengurang akan kami tampilkan.
- **Konsep penawaran:** Artikel edukasi tentang komponen penawaran tanpa angka harga. · **Kanal:** Halaman edukasi situs
- **Uji:** Terbitkan artikel setelah syarat M1 terpenuhi; ukur pertanyaan lanjutan tentang dasar harga. · **Metrik:** O9 · **Kriteria:** Draf lolos pemeriksa teks merek pada percobaan pertama.
- **Sumber daya:** Penulis konten, tinjauan Finance \& Pricing. · **Upaya:** sedang
- Risiko: Pembaca dapat menafsirkan penjelasan sebagai janji harga.
- Asumsi: Penjual berulang membaca konten edukasi.
- Bukti yang kurang: Belum ada data penjual berulang.
- Ketergantungan: Aturan harga \#5 sebagai sumber penjelasan.
- **Tindakan berikutnya:** Susun kerangka artikel dari FB8 dan CT2.

## 4. Tingkat B (0)

_Tidak ada._

## 5. Tingkat C — hipotesis, konflik, atau bukti lemah (1)

### [C] Layanan penjemputan emas ke rumah — `opp_0004`

- **Status:** usulan — menunggu tinjauan manusia · **Segmen:** AU1 · **Topik:** `durasi_proses`
- **Tingkat:** C · **Jumlah ordinal:** 3/15 (jumlah ordinal, bukan probabilitas) · **HIPOTESIS** (tanpa fakta A/B) · **MEMERLUKAN KONFIRMASI OPERASIONAL/TOKO**
- **Kebutuhan pelanggan:** Calon penjual ingin proses yang singkat dan tidak repot.
- **Interpretasi strategis (bukan fakta):** Interpretasi: penjemputan ke rumah mungkin menjawab keinginan proses cepat, tetapi belum ada bukti permintaan nyata dan memerlukan staf serta prosedur keamanan.
- **Fakta pendukung (ID bukti):** tidak ada
- **Hipotesis yang dikutip:**
  - `item_0006` `pertanyaan-pelanggan-contoh.csv#2` — Calon penjual ingin tahu berapa lama proses dari datang sampai pembayaran selesai.
- **Penilaian:** Kekuatan bukti 0 (code) · Niat pelanggan 1 (agent+cap (tanpa fakta tervalidasi berkualitas A–C)) · Kesesuaian strategis 1 (agent) · Kemudahan pengujian 0 (agent) · Keterukuran 1 (agent)
- **Batas tingkat:** evidence — hipotesis tanpa fakta A/B
- Gerbang evidence (lolos): tidak ada fakta tervalidasi berkualitas A/B yang dikutip: dicatat sebagai hipotesis, tingkat maksimal C
- Gerbang operational (catatan): memerlukan toko, staf, atau kapabilitas operasional yang belum terverifikasi (usaha belum beroperasi); istilah operasional: petugas, staf
- **Pesan (bahasa niat):** Kami sedang mempelajari cara membuat proses lebih singkat.
- **Konsep penawaran:** Belum ada; perlu bukti permintaan. · **Kanal:** Belum ditentukan
- **Uji:** Survei singkat kepada calon penjual tentang preferensi lokasi transaksi. · **Metrik:** B1 · **Kriteria:** Minimal sepuluh jawaban survei yang menunjukkan preferensi.
- **Sumber daya:** Petugas lapangan, prosedur keamanan, asuransi. · **Upaya:** tinggi
- Risiko: Keamanan petugas dan barang.
- Asumsi: Ada permintaan penjemputan.
- Bukti yang kurang: Belum ada data permintaan.
- Bukti yang kurang: Belum ada fakta tervalidasi berkualitas A/B yang mendukung; kumpulkan bukti sebelum dijalankan.
- Ketergantungan: Staf dan prosedur keamanan.
- Ketergantungan: Konfirmasi operasional (toko, staf, atau kapabilitas terkait) sebelum dijalankan.
- Ketergantungan: Data KPI B1 belum tersedia sebelum toko beroperasi.
- **Tindakan berikutnya:** Tunda sampai ada bukti permintaan.

## 6. Diblokir pemeriksa teks merek (0)

_Tidak ada._

## 7. Ditolak gerbang bukti atau klaim faktual (0)

_Tidak ada._

## 8. Eksekusi agen dan biaya

| Eksekusi | Agen | Penyedia | Model | Hasil | Token masuk | Token keluar | Biaya |
|---|---|---|---|---|---|---|---|
| `exec_0002` | strategy | mock | mock | ok | 2363 | 2492 | USD 0.0000 |

## 9. Aturan yang diterapkan kode

- Gerbang 1 bukti: setiap ID yang dikutip harus ada dan tervalidasi; tanpa fakta A/B → hipotesis, tingkat maksimal C.
- Gerbang 2 konflik: bukti dalam kelompok konflik ditampilkan kedua sisinya; tingkat maksimal C sampai diselesaikan.
- Gerbang 3 merek dan klaim: gagal pemeriksa teks merek → diblokir; harga, jam buka, alamat, jaminan, statistik, klaim pelanggan, atau angka tanpa dukungan bukti → ditolak.
- Gerbang 4 operasional: peluang yang memerlukan toko, staf, atau kapabilitas belum terverifikasi diberi label; tidak dianggap fakta.
- Gerbang 5 duplikat: topik sama dan bukti bersama ≥ 50% atau judul mirip ≥ 0,8 → digabung; bukti pendukung dipertahankan.
- Tingkat A: bukti ≥ 2, niat ≥ 2, kesesuaian ≥ 2, tanpa batas; Tingkat B: bukti ≥ 1, kesesuaian ≥ 2, tanpa batas C; selainnya C. Kekuatan bukti dihitung kode; nilai agen lain dibatasi kode.

## 10. Langkah berikutnya

- Tinjau setiap peluang dan catat keputusan dengan: mia approve opportunity --id \<id\> --decision approved\|rejected\|needs\_more\_evidence --note "...".
- Brief peluang yang disetujui dapat diekspor untuk serah terima manual: mia export brief --id \<id\>.
- Peluang yang memerlukan toko atau kapabilitas operasional menunggu konfirmasi operasional; jangan dianggap fakta.

---
_Dibuat 2026-10-12T10:00:00Z oleh MIA (mode mock). Laporan dirender dari rekaman tersimpan; jejak: konfigurasi `testconfig`, aturan skor `testscoring`, fixture `003a6b3c725213094941698d6aafa3589e267dfe25046ea4cfa669d444d3f6a0`._
