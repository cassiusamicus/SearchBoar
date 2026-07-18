package netsearch

import "testing"

func TestShellQuoteEscapesEmbeddedQuotes(t *testing.T) {
	got := shellQuote(`it's a share`)
	want := `'it'\''s a share'`
	if got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}

func TestShellQuotePlainString(t *testing.T) {
	got := shellQuote("//192.168.1.5/home")
	want := "'//192.168.1.5/home'"
	if got != want {
		t.Errorf("shellQuote = %q, want %q", got, want)
	}
}
