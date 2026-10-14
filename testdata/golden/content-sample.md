# Laporan Draf Konten MIA — Punyamas

> **DRAF — BELUM DIPUBLIKASIKAN.** MIA tidak memiliki fitur publikasi, pengiriman, atau penjadwalan. Persetujuan peninjau di MIA hanya untuk tahap berikutnya; publikasi tetap memerlukan persetujuan CEO (D-005), syarat M1 §2, dan pengisian semua [ISI: ...].

| Run konten | Peluang | Run strategi | Run riset | Status | Dibuat (UTC) | Selesai (UTC) | Penyedia AI | Konfigurasi | Aturan konten | Fixture |
|---|---|---|---|---|---|---|---|---|---|---|
| `run_0003` | `opp_0001` | `run_0002` | `run_0001` | **Selesai dengan peringatan** | 2026-10-12T10:00:00Z | 2026-10-12T10:00:00Z | mock | `testconfig` | `testcontent` | `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` |

Draf diusulkan model dan diperiksa kode: bukti harus milik peluang yang disetujui dan tervalidasi; harga, alamat, jam buka, statistik, testimoni, janji, diskon, tenggat, dan tren harga tanpa dukungan diblokir; informasi yang belum ada ditulis sebagai [ISI: ...]. Tidak ada draf yang disetujui otomatis. Keluaran mock tidak menguji kualitas model AI sungguhan.

## 1. Peluang sumber

- **Tunjukkan penimbangan dan pengujian di depan penjual sebagai pembeda utama** `opp_0001` · tingkat A · status DISETUJUI oleh peninjau · segmen AU1
- **Kebutuhan pelanggan:** Penjual perorangan ingin melihat sendiri proses penimbangan dan dasar harga karena khawatir ditipu timbangan atau kadar.
- **Interpretasi strategis (bukan fakta):** Interpretasi: ada toko contoh yang menimbang di luar pandangan pembeli, sementara artikel edukasi menyarankan penimbangan di depan penjual; kejelasan proses tampak menjadi kebutuhan yang belum selalu dipenuhi.
- Fakta peluang `item_0004` `catatan-lapangan-contoh.md#2` kualitas A — Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli; pembeli hanya menerima angka akhir.
- Fakta peluang `item_0001` `artikel-edukasi-contoh.json#1` kualitas B — Artikel contoh menyarankan agar penjual meminta penimbangan dilakukan di depan mata dan menanyakan kadar yang dipakai sebagai dasar harga.
- **Riwayat keputusan peluang (hanya-tambah):** 2026-10-12T10:00:00Z approved oleh CEO (uji)
- **Jejak peluang:** run strategi `run_0002` · run riset `run_0001` · aturan skor `testscoring` · fixture strategi `003a6b3c725213094941698d6aafa3589e267dfe25046ea4cfa669d444d3f6a0`

## 2. Ringkasan

- Draf diusulkan agen: 10 · ditolak validasi struktural: 0 · duplikat tidak disimpan: 0 · disimpan: 10
- Status: perlu tinjauan 5 · lolos gerbang 5 · disetujui 0 · ditolak 0 · diblokir 0
- Isian [ISI: ...] belum diisi: 6 · klaim perlu verifikasi: 1 · tautan bukti: 8
- Biaya: USD 0.0000 (aktual USD 0.0000, estimasi tidak pasti USD 0.0000) · Plafon bulanan: USD 20.00

## 3. Status langkah

| Langkah | Status | Percobaan | Catatan |
|---|---|---|---|
| load\_opportunity | completed | 1 |  |
| content\_agent | completed | 1 |  |
| evaluate\_content | completed | 1 |  |


### Peringatan
- catatan agen: Keluaran mock deterministik untuk peluang transparansi\_penimbangan (sample-batch).
- 5 draf memerlukan tinjauan (isian \[ISI: ...\], klaim yang perlu verifikasi, atau temuan)

## 4. Perlu tinjauan (5)

### Caption Instagram — varian 2 — `draft_0002`

- **Status:** perlu tinjauan — ada isian, klaim, temuan, atau perubahan teks yang harus diperiksa · **Versi:** 1 · **Kanal:** instagram · **Segmen:** AU1
- **Tujuan:** Memperkenalkan pembeda proses yang terlihat kepada penjual perorangan (AU1) sebelum toko beroperasi.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status needs\_review: belum disetujui peninjau untuk versi ini — 1 penanda isian belum diisi: \[ISI: kontak resmi Punyamas\]
- **Hook:** Apa yang terjadi di meja penimbangan?
- **Isi draf (DRAF, belum dipublikasikan):**

> Di sebagian toko, penimbangan dilakukan di meja belakang dan pembeli hanya menerima angka akhir. Kami akan melakukannya di depan Anda: berat bersih, kadar, dan dasar hitung akan ditampilkan. Untuk pertanyaan sebelum toko dibuka, hubungi \[ISI: kontak resmi Punyamas\].

- **Ajakan (CTA):** Tanyakan proses penimbangan kepada kami melalui \[ISI: kontak resmi Punyamas\].
- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0004` `catatan-lapangan-contoh.md#2` fact kualitas A — Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli; pembeli hanya menerima angka akhir. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
- **Fakta terdukung (bukti peluang atau fakta usaha):** Ada toko yang menimbang di meja belakang sehingga pembeli hanya menerima angka akhir.
- **Isian wajib diisi:** `[ISI: kontak resmi Punyamas]`
- Risiko: Pesan dapat terdengar menuduh pesaing bila tidak ditulis netral.
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
  - [ ] Isi \[ISI: kontak resmi Punyamas\] dengan data terverifikasi sebelum digunakan.
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → needs\_review
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `5fa0b0bc2af43f0f64df450220287a1ebb7faaed4b4584979b30444190ef63f6`

### Caption Instagram — varian 3 — `draft_0003`

- **Status:** perlu tinjauan — ada isian, klaim, temuan, atau perubahan teks yang harus diperiksa · **Versi:** 1 · **Kanal:** instagram · **Segmen:** AU1
- **Tujuan:** Memperkenalkan pembeda proses yang terlihat kepada penjual perorangan (AU1) sebelum toko beroperasi.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status needs\_review: belum disetujui peninjau untuk versi ini — 1 klaim perlu verifikasi — 1 temuan gerbang belum diselesaikan
- **Hook:** Harga datang dengan dasar hitungnya.
- **Isi draf (DRAF, belum dipublikasikan):**

> Banyak penjual belum terbiasa membaca dasar hitung harga. Harga beli akan dihitung dari berat bersih, kadar, dan pengurang alat, dan setiap komponennya akan ditampilkan kepada Anda. Penawaran berlaku 10 menit agar Anda punya waktu menimbang keputusan dengan tenang.

- **Ajakan (CTA):** Pelajari cara membaca dasar hitung sebelum menjual.
- **Bukti pendukung (milik peluang, tervalidasi):** tidak ada
- **Fakta usaha** `FB8` (KEBIJAKAN): Harga beli dihitung dengan rumus yang dapat dijelaskan: berat bersih, kadar, pengurang alat, dan harga per gram murni ditampilkan; penawaran berlaku 10 menit.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Penawaran berlaku 10 menit dan dasar hitung ditampilkan.
- **Klaim perlu verifikasi:** Banyak penjual belum terbiasa membaca dasar hitung harga.
- **Temuan claims KLAIM\_PERLU\_VERIFIKASI** (review): klaim fakta tanpa bukti tervalidasi atau fakta usaha — "Banyak penjual belum terbiasa membaca dasar hitung harga."
- Asumsi: Penjelasan dasar hitung mengurangi keraguan.
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
  - [ ] Verifikasi atau hapus klaim: "Banyak penjual belum terbiasa membaca dasar hitung harga.".
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → needs\_review
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `98497a29d55a1fa4eee48033611971833e796f9261967ab164f016fe0cfb882c`

### Naskah video pendek — varian 1 — `draft_0008`

- **Status:** perlu tinjauan — ada isian, klaim, temuan, atau perubahan teks yang harus diperiksa · **Versi:** 1 · **Kanal:** instagram\_reels · **Segmen:** AU1
- **Tujuan:** Menunjukkan perbedaan proses yang terlihat dan tidak terlihat dalam video pendek.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status needs\_review: belum disetujui peninjau untuk versi ini — 1 penanda isian belum diisi: \[ISI: tanggal pembukaan toko\]
- **Hook:** Pernahkah barang Anda ditimbang tanpa Anda lihat?
- **Isi draf (DRAF, belum dipublikasikan):**

> Pembuka: Seorang penjual menyerahkan perhiasan di meja, lalu hanya menerima angka akhir.
> Isi: Narasi: Saat menjual emas, Anda berhak melihat timbangan dan hasil uji. Kami akan menimbang dan menguji di depan Anda, lalu menjelaskan dasar hitungnya.
> Penutup: Teks di layar: Ditimbang dan diuji di depan Anda. Informasi pembukaan: \[ISI: tanggal pembukaan toko\].

- **Ajakan (CTA):** Ikuti akun ini untuk kabar pembukaan.
- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0004` `catatan-lapangan-contoh.md#2` fact kualitas A — Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli; pembeli hanya menerima angka akhir. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
- **Fakta usaha** `FB7` (KEBIJAKAN): Barang customer ditimbang dan diuji di depan customer; barang bernilai lebih tinggi diuji dengan lebih dari satu metode; hasil uji yang belum konsisten dijelaskan dan barang dapat dibawa pulang.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Ada toko yang menimbang di luar pandangan pembeli.
- **Isian wajib diisi:** `[ISI: tanggal pembukaan toko]`
- Risiko: Adegan tidak boleh menampilkan toko pesaing yang dapat dikenali.
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
  - [ ] Isi \[ISI: tanggal pembukaan toko\] dengan data terverifikasi sebelum digunakan.
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → needs\_review
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `3b14f3bc48531fe9ab7a7943bf5870b39876fbc5779b1ff134ee6d0341a59d15`

### Pesan WhatsApp (draf, tidak dikirim) — varian 1 — `draft_0009`

- **Status:** perlu tinjauan — ada isian, klaim, temuan, atau perubahan teks yang harus diperiksa · **Versi:** 1 · **Kanal:** whatsapp · **Segmen:** AU1
- **Tujuan:** Menyiapkan pesan kabar persiapan untuk kontak yang sudah memberi persetujuan promosi.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status needs\_review: belum disetujui peninjau untuk versi ini — 2 penanda isian belum diisi: \[ISI: syarat dan periode promosi\], \[ISI: alamat cabang\]
- **Isi draf (DRAF, belum dipublikasikan):**

> Selamat siang, Bapak atau Ibu. Punyamas sedang mempersiapkan tempat pembelian emas di Surabaya. Di tempat kami nanti, barang akan ditimbang dan diuji di depan Anda, dan harga akan datang dengan dasar hitungnya. Syarat dan periode promosi: \[ISI: syarat dan periode promosi\]. Alamat: \[ISI: alamat cabang\].

- **Ajakan (CTA):** Balas pesan ini bila ingin menerima kabar pembukaan.
- **Bukti pendukung (milik peluang, tervalidasi):** tidak ada
- **Fakta usaha** `FB1` (RENCANA): Punyamas adalah usaha pembelian emas dan perak di Surabaya, Jawa Timur. Usaha dalam persiapan dan belum beroperasi.
- **Fakta usaha** `FB7` (KEBIJAKAN): Barang customer ditimbang dan diuji di depan customer; barang bernilai lebih tinggi diuji dengan lebih dari satu metode; hasil uji yang belum konsisten dijelaskan dan barang dapat dibawa pulang.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Punyamas sedang mempersiapkan tempat pembelian emas di Surabaya.
- **Isian wajib diisi:** `[ISI: syarat dan periode promosi]`
- **Isian wajib diisi:** `[ISI: alamat cabang]`
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
  - [ ] Pesan promosi WhatsApp hanya untuk kontak yang memberi persetujuan promosi terpisah; MIA tidak mengirim pesan.
  - [ ] Isi \[ISI: syarat dan periode promosi\] dengan data terverifikasi sebelum digunakan.
  - [ ] Isi \[ISI: alamat cabang\] dengan data terverifikasi sebelum digunakan.
  - [ ] Fakta FB1 berstatus RENCANA: pastikan ditulis dalam bahasa niat.
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → needs\_review
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `0ac51f66584fd5f86b771d4486d08481394c92dafc404edbe6c7571aefb5cee7`

### Artikel pendek atau FAQ — varian 1 — `draft_0010`

- **Status:** perlu tinjauan — ada isian, klaim, temuan, atau perubahan teks yang harus diperiksa · **Versi:** 1 · **Kanal:** situs · **Segmen:** AU1
- **Tujuan:** Menjawab pertanyaan umum tentang proses penimbangan dan dasar harga.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status needs\_review: belum disetujui peninjau untuk versi ini — 2 penanda isian belum diisi: \[ISI: harga buyback terverifikasi\], \[ISI: kontak resmi Punyamas\]
- **Headline:** Tanya jawab: bagaimana penimbangan di Punyamas akan dilakukan
- **Isi draf (DRAF, belum dipublikasikan):**

> Pertanyaan: Apakah saya boleh melihat barang saya ditimbang?
> Jawaban: Ya. Kami akan menimbang dan menguji barang di depan Anda.
> Pertanyaan: Bagaimana harga akan dihitung?
> Jawaban: Dari berat bersih, kadar, dan pengurang alat; setiap komponennya akan ditampilkan. Harga per gram yang berlaku: \[ISI: harga buyback terverifikasi\].
> Pertanyaan: Apakah saya boleh menolak penawaran?
> Jawaban: Boleh. Barang Anda kembali utuh.

- **Ajakan (CTA):** Kirim pertanyaan Anda melalui \[ISI: kontak resmi Punyamas\].
- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0001` `artikel-edukasi-contoh.json#1` fact kualitas B — Artikel contoh menyarankan agar penjual meminta penimbangan dilakukan di depan mata dan menanyakan kadar yang dipakai sebagai dasar harga. (sumber: CONTOH — artikel edukasi fiktif (pengujian))
- **Fakta usaha** `FB7` (KEBIJAKAN): Barang customer ditimbang dan diuji di depan customer; barang bernilai lebih tinggi diuji dengan lebih dari satu metode; hasil uji yang belum konsisten dijelaskan dan barang dapat dibawa pulang.
- **Fakta usaha** `FB8` (KEBIJAKAN): Harga beli dihitung dengan rumus yang dapat dijelaskan: berat bersih, kadar, pengurang alat, dan harga per gram murni ditampilkan; penawaran berlaku 10 menit.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Dasar hitung terdiri dari berat bersih, kadar, dan pengurang alat.
- **Isian wajib diisi:** `[ISI: harga buyback terverifikasi]`
- **Isian wajib diisi:** `[ISI: kontak resmi Punyamas]`
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
  - [ ] Isi \[ISI: harga buyback terverifikasi\] dengan data terverifikasi sebelum digunakan.
  - [ ] Isi \[ISI: kontak resmi Punyamas\] dengan data terverifikasi sebelum digunakan.
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → needs\_review
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `40ed60516d46dbbf817cc767bd42889befd72fa80a12ede447ee460800a99fa2`

## 5. Lolos gerbang kode, menunggu tinjauan (5)

### Caption Instagram — varian 1 — `draft_0001`

- **Status:** draf — lolos gerbang kode, menunggu tinjauan manusia · **Versi:** 1 · **Kanal:** instagram · **Segmen:** AU1
- **Tujuan:** Memperkenalkan pembeda proses yang terlihat kepada penjual perorangan (AU1) sebelum toko beroperasi.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status draft: belum disetujui peninjau untuk versi ini
- **Hook:** Ditimbang dan diuji di depan Anda.
- **Isi draf (DRAF, belum dipublikasikan):**

> Saat menjual perhiasan emas, Bapak atau Ibu berhak melihat prosesnya. Panduan edukasi yang kami baca menyarankan agar penimbangan dilakukan di depan penjual dan kadar yang dipakai sebagai dasar harga ditanyakan. Kami akan menimbang dan menguji barang di depan Bapak atau Ibu, dan harga akan datang dengan dasar hitungnya.

- **Ajakan (CTA):** Simpan unggahan ini sebagai pengingat sebelum menjual perhiasan.
- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0001` `artikel-edukasi-contoh.json#1` fact kualitas B — Artikel contoh menyarankan agar penjual meminta penimbangan dilakukan di depan mata dan menanyakan kadar yang dipakai sebagai dasar harga. (sumber: CONTOH — artikel edukasi fiktif (pengujian))
- **Fakta usaha** `FB7` (KEBIJAKAN): Barang customer ditimbang dan diuji di depan customer; barang bernilai lebih tinggi diuji dengan lebih dari satu metode; hasil uji yang belum konsisten dijelaskan dan barang dapat dibawa pulang.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Panduan edukasi menyarankan penimbangan di depan penjual.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Barang ditimbang dan diuji di depan pemiliknya.
- Asumsi: Calon penjual mencari informasi proses sebelum datang.
- Risiko: Kata 'berhak' dapat terdengar menggurui bila nadanya tidak tenang.
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
  - [ ] Pastikan foto pendamping tidak menampilkan wajah atau identitas orang. (usulan agen)
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → draft
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `6719bd4e8b9fb3936281150f7cf612f729ddc8e08dd0a7585fbcc85ba9f9e11e`

### Variasi headline/hook untuk pengujian — varian 1 — `draft_0004`

- **Status:** draf — lolos gerbang kode, menunggu tinjauan manusia · **Versi:** 1 · **Kanal:** instagram · **Segmen:** AU1
- **Tujuan:** Menguji sudut pesan yang paling mudah dipahami AU1.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status draft: belum disetujui peninjau untuk versi ini
- **Headline:** Ditimbang dan diuji di depan Anda
- **Isi draf (DRAF, belum dipublikasikan):**

> Sudut uji: proses yang terlihat (pilar PM1).

- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0004` `catatan-lapangan-contoh.md#2` fact kualitas A — Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli; pembeli hanya menerima angka akhir. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → draft
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `650230b63c9941e468e1275f8bd2bb8702646aa09fb2fbc336bf1ac44c571a52`

### Variasi headline/hook untuk pengujian — varian 2 — `draft_0005`

- **Status:** draf — lolos gerbang kode, menunggu tinjauan manusia · **Versi:** 1 · **Kanal:** instagram · **Segmen:** AU1
- **Tujuan:** Menguji sudut pesan yang paling mudah dipahami AU1.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status draft: belum disetujui peninjau untuk versi ini
- **Headline:** Harga jelas, dasar hitungnya Anda lihat
- **Isi draf (DRAF, belum dipublikasikan):**

> Sudut uji: dasar hitung yang ditampilkan (pilar PM2).

- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0001` `artikel-edukasi-contoh.json#1` fact kualitas B — Artikel contoh menyarankan agar penjual meminta penimbangan dilakukan di depan mata dan menanyakan kadar yang dipakai sebagai dasar harga. (sumber: CONTOH — artikel edukasi fiktif (pengujian))
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → draft
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `a5c8e7e7c4c9d23bd86df4efb0a61e81d526d6fce702d9b4eb214e644caa8713`

### Variasi headline/hook untuk pengujian — varian 3 — `draft_0006`

- **Status:** draf — lolos gerbang kode, menunggu tinjauan manusia · **Versi:** 1 · **Kanal:** instagram · **Segmen:** AU1
- **Tujuan:** Menguji sudut pesan yang paling mudah dipahami AU1.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status draft: belum disetujui peninjau untuk versi ini
- **Headline:** Lihat sendiri proses penimbangannya
- **Isi draf (DRAF, belum dipublikasikan):**

> Sudut uji: ajakan memeriksa proses dengan tenang (pilar PM1 dan PM4).

- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0004` `catatan-lapangan-contoh.md#2` fact kualitas A — Penimbangan dilakukan di meja belakang yang tidak terlihat dari sisi pembeli; pembeli hanya menerima angka akhir. (sumber: CONTOH — catatan lapangan fiktif untuk pengujian MIA)
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → draft
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `504f1539f1cd51224e3b74cbaa51e75a21431eec9e71aee3b714123750011063`

### Konten edukasi emas — varian 1 — `draft_0007`

- **Status:** draf — lolos gerbang kode, menunggu tinjauan manusia · **Versi:** 1 · **Kanal:** situs · **Segmen:** AU1
- **Tujuan:** Mengedukasi calon penjual tentang hal yang perlu diperiksa saat menjual perhiasan emas.
- **Siap serah terima ke proses persetujuan publikasi:** TIDAK — status draft: belum disetujui peninjau untuk versi ini
- **Headline:** Tiga hal yang perlu Anda lihat saat menjual perhiasan emas
- **Isi draf (DRAF, belum dipublikasikan):**

> Pertama, minta penimbangan dilakukan di depan Anda. Kedua, tanyakan kadar yang dipakai sebagai dasar harga. Ketiga, minta rincian dasar hitung: berat bersih, kadar, dan pengurang. Dua langkah pertama juga disarankan oleh panduan edukasi yang kami baca. Kami berencana menjalankan ketiganya di depan setiap penjual.

- **Ajakan (CTA):** Bagikan panduan ini kepada keluarga yang berencana menjual perhiasan.
- **Bukti pendukung (milik peluang, tervalidasi):**
  - `item_0001` `artikel-edukasi-contoh.json#1` fact kualitas B — Artikel contoh menyarankan agar penjual meminta penimbangan dilakukan di depan mata dan menanyakan kadar yang dipakai sebagai dasar harga. (sumber: CONTOH — artikel edukasi fiktif (pengujian))
- **Fakta usaha** `FB7` (KEBIJAKAN): Barang customer ditimbang dan diuji di depan customer; barang bernilai lebih tinggi diuji dengan lebih dari satu metode; hasil uji yang belum konsisten dijelaskan dan barang dapat dibawa pulang.
- **Fakta usaha** `FB8` (KEBIJAKAN): Harga beli dihitung dengan rumus yang dapat dijelaskan: berat bersih, kadar, pengurang alat, dan harga per gram murni ditampilkan; penawaran berlaku 10 menit.
- **Fakta terdukung (bukti peluang atau fakta usaha):** Panduan edukasi menyarankan penimbangan di depan penjual dan menanyakan kadar.
- **Interpretasi pemasaran (bukan fakta):** Penjual lebih tenang bila prosesnya terlihat.
- **Checklist tinjauan manusia:**
  - [ ] Persetujuan CEO wajib sebelum publikasi apa pun (D-005); persetujuan di MIA bukan izin publikasi.
  - [ ] Syarat M1 §2 terpenuhi sebelum kegiatan publik (nama lolos pemeriksaan merek D-019, akun atas nama usaha).
  - [ ] Semua \[ISI: ...\] diisi dengan data terverifikasi; tidak ada harga, alamat, jam buka, atau angka yang dikarang.
  - [ ] Teks akhir lolos pemeriksa teks merek setelah diedit.
  - [ ] Fakta berstatus RENCANA ditulis dalam bahasa niat (kami akan ...).
- **Riwayat versi (hanya-tambah):** v1 oleh agent 2026-10-12T10:00:00Z → draft
- **Riwayat tinjauan (hanya-tambah):** belum ada
- **Provenance:** peluang `opp_0001` · run konten `run_0003` · run strategi `run_0002` · run riset `run_0001` · eksekusi `exec_0003` · konfigurasi `testconfig` · aturan konten `testcontent` · aturan skor `testscoring` · fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d` · hash isi v1 `961148e4bc7157bc7aeedfe932aa2d151b6fce946ad063b8fb13c634f0953b1f`

## 6. Disetujui peninjau — bukan izin publikasi (0)

_Tidak ada._

## 7. Ditolak peninjau (0)

_Tidak ada._

## 8. Diblokir gerbang kode (0)

_Tidak ada._

## 9. Eksekusi agen dan biaya

| Eksekusi | Agen | Penyedia | Model | Hasil | Token masuk | Token keluar | Biaya |
|---|---|---|---|---|---|---|---|
| `exec_0003` | content | mock | mock | ok | 2339 | 2704 | USD 0.0000 |

## 10. Aturan yang diterapkan kode

- Kelayakan: hanya peluang berstatus approved dengan riwayat persetujuan, run strategi selesai, dan bukti yang masih tervalidasi.
- Bukti: referensi harus milik peluang ini dan tervalidasi; referensi fiktif, belum tervalidasi, atau lintas peluang memblokir draf.
- Klaim: fakta tanpa bukti tervalidasi (A–C) atau fakta usaha yang disetujui dicatat sebagai klaim perlu verifikasi.
- Bahasa: harga, jam buka, alamat, statistik, klaim pelanggan, angka tanpa sumber, janji keuntungan, tren harga, testimoni, urgensi palsu, diskon, dan janji waktu proses memblokir draf; ajakan mendesak dan isian tidak baku memerlukan tinjauan. Aturan ini mencocokkan kata dan pola, bukan makna.
- Isian: setiap [ISI: ...] dicatat; draf dengan isian tidak pernah siap serah terima.
- Duplikat: draf yang hampir sama (jenis dan kanal sama) tidak disimpan dan dilaporkan.
- Status: draf baru tidak pernah disetujui otomatis; perubahan teks setelah tinjauan mengembalikan draf ke perlu tinjauan.

## 11. Langkah berikutnya

- Tinjau setiap draf dan catat keputusan dengan: mia content review --id \<id\> --decision approved\|rejected --note "...".
- Isi setiap \[ISI: ...\] dan verifikasi klaim yang ditandai melalui mia content revise; revisi mengembalikan draf ke tinjauan.
- Ekspor draf untuk serah terima manual dengan mia content export --id \<id\>; MIA tidak menerbitkan atau mengirim apa pun.

---
_Dibuat 2026-10-12T10:00:00Z oleh MIA (mode mock). DRAF — belum dipublikasikan. Jejak: konfigurasi `testconfig`, aturan konten `testcontent`, fixture `8738f753f9c5b7a1c718280d70dd36f251bff4694a0c1a22436f20a2f293803d`._
