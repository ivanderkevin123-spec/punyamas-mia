// Punyamas - pemeriksa teks merek (deliverable B1, tahap 2). BUKAN kode aplikasi.
// Memeriksa teks yang akan dipublikasikan atau dicetak terhadap daftar klaim yang tidak boleh dipakai sampai terbukti (D-005, UU Perlindungan Konsumen, Etika Pariwara).
// Daftar ini adalah ASUMSI kerja; konsultan hukum periklanan menetapkan versi akhir (N70). Pemeriksa hanya menyaring kata dan frasa; ia tidak menilai makna.

export const VERSI = 'v0.1-ASUMSI';

// Setiap pola: kategori, ekspresi reguler (huruf besar atau kecil tidak dibedakan), alasan singkat.
export const POLA = [
  { kategori: 'SUPERLATIF', kata: ['terbaik', 'termurah', 'tertinggi', 'terbesar', 'terlengkap', 'terpercaya', 'tercepat', 'teraman', 'terjamin', 'terunggul', 'termahal', 'terdepan', 'termudah', 'terjujur'], re: /\b(terbaik|termurah|tertinggi|terbesar|terlengkap|terpercaya|tercepat|teraman|terjamin|terunggul|termahal|terdepan|termudah|terjujur)\b/i, alasan: 'Kata berawalan "ter" bermakna superlatif memerlukan bukti otentik (Etika Pariwara; UU 8/1999 Pasal 9 dan 17)' },
  { kategori: 'SUPERLATIF', kata: ['paling baik', 'paling murah', 'paling tinggi', 'paling besar', 'paling lengkap', 'paling cepat', 'paling aman', 'paling dipercaya', 'paling jujur', 'paling mudah', 'paling unggul', 'paling mahal'], re: /\bpaling (baik|murah|tinggi|besar|lengkap|cepat|aman|dipercaya|jujur|mudah|unggul|mahal)\b/i, alasan: 'Superlatif dengan "paling"' },
  { kategori: 'PERINGKAT', kata: ['nomor satu', 'nomor 1', 'no. 1', 'nomor wahid', 'juara'], re: /\b(nomor satu|nomor 1|no\.? ?1|nomor wahid|juara)\b/i, alasan: 'Klaim peringkat tanpa dasar yang dapat diperiksa' },
  { kategori: 'ABSOLUT', kata: ['100%', 'seratus persen', 'dijamin', 'jaminan', 'garansi', 'pasti', 'tanpa risiko'], re: /(?<![\w])100 ?%|\b(seratus persen|dijamin|jaminan|garansi|pasti|tanpa risiko)\b/i, alasan: 'Klaim mutlak atau jaminan hasil yang tidak dapat dipertanggungjawabkan' },
  { kategori: 'BERTENTANGAN_KEBIJAKAN', kata: ['tanpa KYC', 'tanpa identitas', 'tanpa KTP', 'tanpa syarat', 'tanpa potongan', 'tanpa ribet', 'tanpa antre', 'tanpa dokumen', 'tanpa biaya'], re: /\btanpa (kyc|identitas|ktp|syarat|potongan|ribet|antre|dokumen|biaya)\b/i, alasan: 'Bertentangan dengan kebijakan Punyamas (identitas wajib untuk semua transaksi, D-013; potongan dan pengurang dalam aturan harga #5 tahap 1)' },
  { kategori: 'KECEPATAN_UANG', kata: ['langsung cair', 'cair instan', 'cair cepat', 'cair hari ini', 'uang langsung', 'uang instan', 'cash instan', 'instan'], re: /\b(langsung cair|cair (instan|cepat|hari ini)|uang (langsung|instan)|cash instan|instan)\b/i, alasan: 'Pembayaran bergantung pada konfirmasi dana, batas tunai, dan proses KYC' },
  { kategori: 'KEUANGAN', kata: ['investasi', 'untung', 'keuntungan', 'cuan', 'imbal hasil', 'bunga', 'gadai', 'pinjaman'], re: /\b(investasi|untung|keuntungan|cuan|imbal hasil|bunga|gadai|pinjaman)\b/i, alasan: 'Punyamas bukan penasihat investasi, penerbit produk keuangan, atau pegadaian' },
  { kategori: 'STATUS', kata: ['resmi', 'berizin', 'terdaftar', 'bersertifikat', 'terakreditasi', 'diawasi', 'berlisensi'], re: /\b(resmi|berizin|terdaftar|bersertifikat|terakreditasi|diawasi|berlisensi)\b/i, alasan: 'Klaim status hukum atau izin hanya boleh dipakai setelah terbukti dan disetujui CEO' },
  { kategori: 'INSTITUSI', kata: ['Antam', 'Pegadaian', 'Bank Indonesia', 'OJK', 'Bappebti', 'bursa berjangka'], re: /\b(antam|pegadaian|bank indonesia|ojk|bappebti|bursa berjangka)\b/i, alasan: 'Menyebut atau menyiratkan hubungan dengan institusi lain tanpa izin' },
];

export function periksaTeks(teks) {
  const temuan = [];
  for (const p of POLA) { const m = teks.match(p.re); if (m) temuan.push({ kategori: p.kategori, cocok: m[0] }); }
  return { lolos: temuan.length === 0, temuan };
}
export const kategoriUnik = (teks) => [...new Set(periksaTeks(teks).temuan.map((t) => t.kategori))];
