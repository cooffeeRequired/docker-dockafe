package update

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"v1.0.1":              "1.0.1",
		"1.0.1":               "1.0.1",
		"v1.0.0-3-gc467c45":   "1.0.0",
		"1.0.1-dirty":         "1.0.1",
		" V2.3.4 ":            "2.3.4",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	if Compare("1.0.0", "1.0.1") >= 0 {
		t.Fatal("1.0.0 should be < 1.0.1")
	}
	if Compare("v1.0.1", "1.0.1") != 0 {
		t.Fatal("v1.0.1 should equal 1.0.1")
	}
	if Compare("1.0.0-3-gabc", "1.0.0") != 0 {
		t.Fatal("git describe should normalize to base")
	}
	if Compare("1.1.0", "1.0.9") <= 0 {
		t.Fatal("1.1.0 should be > 1.0.9")
	}
}
