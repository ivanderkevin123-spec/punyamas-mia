package claims_test

import (
	"strings"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/claims"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

func scanner(t *testing.T) *claims.Scanner {
	t.Helper()
	cfg, err := config.Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	s, err := claims.New(cfg.Scoring.UnsupportedClaims, cfg.Business.AllowedNumbers)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func cats(fs []claims.Finding) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Category)
	}
	return strings.Join(out, ",")
}

func TestCategoriesMatchConfiguredRules(t *testing.T) {
	s := scanner(t)
	for text, want := range map[string]string{
		"Kami membeli Rp 1.000.000 per gram.":          "HARGA",
		"Buka pukul 08.00 sampai 17.00.":               "JAM_BUKA",
		"Kunjungi Jl. Contoh No. 5.":                   "ALAMAT",
		"90% penjual puas.":                            "STATISTIK",
		"Pelanggan kami mengatakan prosesnya jelas.":   "KLAIM_PELANGGAN",
		"Kami akan menimbang di depan Bapak atau Ibu.": "",
	} {
		if got := cats(s.Categories(text)); got != want {
			t.Errorf("%q: categories %q, want %q", text, got, want)
		}
	}
}

func TestUnsupportedNumbers(t *testing.T) {
	s := scanner(t)
	text := "Penawaran berlaku 10 menit; 12 pengunjung bertanya; target O4, pilar PM1, fakta FB5, aturan #5, M1 §2; 3 hari."
	got := s.UnsupportedNumbers(text, "Petugas mencatat 12 pengunjung.")
	if strings.Join(got, ",") != "3" {
		t.Fatalf("unsupported numbers = %v, want [3] (10 is approved, 12 is in the evidence, identifiers are ignored)", got)
	}
	if all := s.Scan("Rp 5 per gram"); cats(all) != "HARGA,ANGKA" {
		t.Fatalf("scan = %s", cats(all))
	}
	if got := s.UnsupportedNumbers("angka 7 dan 7"); len(got) != 2 {
		t.Fatalf("repeated numbers are reported per occurrence: %v", got)
	}
}
