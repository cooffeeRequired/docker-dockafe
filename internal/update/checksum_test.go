package update

import (
	"strings"
	"testing"
)

func TestParseSHA256SumFile(t *testing.T) {
	const sum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	content := sum + "  dockafe\nother  skip\n"
	got, err := ParseSHA256SumFile(content, "dockafe")
	if err != nil {
		t.Fatal(err)
	}
	if got != sum {
		t.Fatalf("got %s", got)
	}
	got, err = ParseSHA256SumFile(sum+" *dockafe\n", AssetName)
	if err != nil || got != sum {
		t.Fatalf("binary mode: %v %s", err, got)
	}
	_, err = ParseSHA256SumFile("deadbeef  other\n", "dockafe")
	if err == nil {
		t.Fatal("expected missing entry error")
	}
}

func TestHashReader(t *testing.T) {
	// echo -n hello | sha256sum
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	got, err := HashReader(strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
