package config

import (
	"bytes"
	"os"
	"testing"
)

func TestIniRoundTripByteIdentical(t *testing.T) {
	orig, err := os.ReadFile("testdata/sample_config.ini")
	if err != nil {
		t.Fatal(err)
	}

	doc, err := parseIni(bytes.NewReader(orig))
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doc.write(&out); err != nil {
		t.Fatal(err)
	}

	if out.String() != string(orig) {
		t.Fatalf("round trip mismatch\n--- want ---\n%s\n--- got ---\n%s", orig, out.String())
	}
}

func TestIniIgnoresFullLineComments(t *testing.T) {
	src := "[Section]\n# a comment\nkey = value\n; also a comment\n"
	doc, err := parseIni(bytes.NewReader([]byte(src)))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := doc.get("Section", "key")
	if !ok || v != "value" {
		t.Fatalf("expected key=value, got %q ok=%v", v, ok)
	}
}

func TestIniDoesNotStripInlineHash(t *testing.T) {
	// configparser does not strip inline comments by default; a '#'
	// appearing after a value must be preserved verbatim.
	src := "[Section]\nkey = value#not-a-comment\n"
	doc, err := parseIni(bytes.NewReader([]byte(src)))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := doc.get("Section", "key")
	if v != "value#not-a-comment" {
		t.Fatalf("expected inline # to be preserved, got %q", v)
	}
}

func TestIniFirstEqualsWins(t *testing.T) {
	src := "[Section]\nkey = a=b=c\n"
	doc, err := parseIni(bytes.NewReader([]byte(src)))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := doc.get("Section", "key")
	if v != "a=b=c" {
		t.Fatalf("expected value after first '=' to be kept whole, got %q", v)
	}
}
