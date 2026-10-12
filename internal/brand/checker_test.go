package brand

import (
	"reflect"
	"testing"

	"github.com/ivanderkevin123-spec/punyamas-mia/internal/config"
	"github.com/ivanderkevin123-spec/punyamas-mia/internal/testutil"
)

// Cases BT1–BT12 and TG1–TG6 are copied from bots/punyamas/docs/02-brand/B1-arah-dan-strategi-merek.md §8–9
// so the Go port must reproduce the workspace checker's verdicts.
func TestCheckerMatchesWorkspaceCases(t *testing.T) {
	cfg, err := config.Load(testutil.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(cfg.Policy.Rules)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id   string
		text string
		want []string // nil = LOLOS
	}{
		{"BT1", "Terima kasih atas kunjungannya.", nil},
		{"BT2", "Kami pastikan jawaban paling lambat tiga hari kerja.", nil},
		{"BT3", "Harga terbaru kami tampilkan di layar.", nil},
		{"BT4", "Hasil uji tertulis diberikan kepada Anda.", nil},
		{"BT5", "Tertib administrasi sejak hari pertama.", nil},
		{"BT6", "Ini penawaran yang terbaik untuk Anda.", []string{"SUPERLATIF"}},
		{"BT7", "TANPA KYC!", []string{"BERTENTANGAN_KEBIJAKAN"}},
		{"BT8", "Dapatkan 100% harga pasar.", []string{"ABSOLUT"}},
		{"BT9", "Garansi uang kembali.", []string{"ABSOLUT"}},
		{"BT10", "Dokumen resmi kami lengkap.", []string{"STATUS"}},
		{"BT11", "Kami paling jujur dan paling cepat.", []string{"SUPERLATIF"}},
		{"BT12", "Punyamas, pilihan terpercaya.", []string{"SUPERLATIF"}},
		{"TG1", "Ditimbang dan diuji di depan Anda.", nil},
		{"TG2", "Harga jelas, dasar hitungnya Anda lihat.", nil},
		{"TG3", "Punya emas? Kami timbang di depan Anda.", nil},
		{"TG4", "Emas Anda, dihitung dengan terang.", nil},
		{"TG5", "Harga tertinggi untuk emas Anda.", []string{"SUPERLATIF"}},
		{"TG6", "Jual emas tanpa ribet, uang langsung cair.", []string{"BERTENTANGAN_KEBIJAKAN", "KECEPATAN_UANG"}},
	}
	for _, tc := range cases {
		res := c.Check(tc.text)
		got := res.Categories()
		if tc.want == nil {
			if !res.Passes {
				t.Errorf("%s: expected LOLOS, got %+v", tc.id, res.Findings)
			}
			continue
		}
		if res.Passes || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: categories = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestAbsolutMatchTrimsLeadingSpace(t *testing.T) {
	c, _ := New([]config.Rule{{Category: "ABSOLUT", Pattern: `(?i)(?:^|[^\w])100 ?%`}})
	res := c.Check("Dapatkan 100% harga pasar.")
	if res.Passes || res.Findings[0].Match != "100%" {
		t.Fatalf("got %+v", res)
	}
}
