package dedup

import "testing"

func TestCanonicalURL(t *testing.T) {
	cases := map[string]string{
		"HTTPS://Example.com/Path/?utm_source=x&b=2&a=1#frag": "https://example.com/Path?a=1&b=2",
		"https://example.com/a/":                               "https://example.com/a",
		"not a url":                                            "not a url",
	}
	for in, want := range cases {
		if got := CanonicalURL(in); got != want {
			t.Errorf("CanonicalURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContentHashNormalizes(t *testing.T) {
	a := ContentHash("Harga beli, per gram!")
	b := ContentHash("harga   beli per GRAM")
	if a != b {
		t.Fatal("normalized variants should hash equally")
	}
	if a == ContentHash("harga jual per gram") {
		t.Fatal("different statements must differ")
	}
}

func TestFindDuplicates(t *testing.T) {
	cands := []Candidate{
		{ID: "1", Title: "Toko A memasang papan harga per gram", ContentHash: "h1"},
		{ID: "2", Title: "Toko A memasang papan harga per gram", ContentHash: "h1"},
		{ID: "3", Title: "Toko A memasang papan harga per gram emas", ContentHash: "h3"},
		{ID: "4", Title: "Pengunjung menanyakan KTP", ContentHash: "h4"},
	}
	m := FindDuplicates(cands, 0.8)
	if len(m) != 2 {
		t.Fatalf("matches = %d, want 2: %+v", len(m), m)
	}
	if m[0].ID != "2" || m[0].DuplicateOf != "1" || m[1].ID != "3" || m[1].DuplicateOf != "1" {
		t.Fatalf("unexpected matches: %+v", m)
	}
}

func TestJaccard(t *testing.T) {
	if Jaccard("emas perak sawit", "emas perak sawit") != 1 {
		t.Fatal("identical should be 1")
	}
	if Jaccard("emas perak", "sawit karet") != 0 {
		t.Fatal("disjoint should be 0")
	}
}
