package regexbuilder

import (
	"regexp"
	"testing"
)

func TestGenerateExactMatchIsAnchored(t *testing.T) {
	b := Builder{Type: ExactMatch, Text: "cat", CaseSensitive: true}
	pattern, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(pattern)
	if !re.MatchString("cat") {
		t.Error("expected exact match of 'cat' to match 'cat'")
	}
	if re.MatchString("category") {
		t.Error("exact match should not match a longer string containing it")
	}
}

func TestGenerateContainsIsUnanchored(t *testing.T) {
	b := Builder{Type: Contains, Text: "cat", CaseSensitive: true}
	pattern, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(pattern)
	if !re.MatchString("category") {
		t.Error("contains-text should match a substring within a longer string")
	}
}

func TestGenerateStartsEndsWith(t *testing.T) {
	starts := Builder{Type: StartsWith, Text: "foo", CaseSensitive: true}
	p, _ := starts.Generate()
	re := regexp.MustCompile(p)
	if !re.MatchString("foobar") || re.MatchString("barfoo") {
		t.Errorf("StartsWith pattern %q behaved unexpectedly", p)
	}

	ends := Builder{Type: EndsWith, Text: "bar", CaseSensitive: true}
	p, _ = ends.Generate()
	re = regexp.MustCompile(p)
	if !re.MatchString("foobar") || re.MatchString("barfoo") {
		t.Errorf("EndsWith pattern %q behaved unexpectedly", p)
	}
}

func TestGenerateExtension(t *testing.T) {
	b := Builder{Type: Extension, Text: "pdf", CaseSensitive: true}
	p, _ := b.Generate()
	re := regexp.MustCompile(p)
	if !re.MatchString("report.pdf") || re.MatchString("report.pdfx") {
		t.Errorf("Extension pattern %q behaved unexpectedly", p)
	}
}

func TestGenerateEmailAndIPIgnoreText(t *testing.T) {
	email := Builder{Type: Email, Text: "ignored", CaseSensitive: true}
	p, _ := email.Generate()
	re := regexp.MustCompile(p)
	if !re.MatchString("user@example.com") {
		t.Errorf("Email pattern %q should match a valid email", p)
	}

	ip := Builder{Type: IP, Text: "ignored", CaseSensitive: true}
	p, _ = ip.Generate()
	re = regexp.MustCompile(p)
	if !re.MatchString("192.168.1.1") {
		t.Errorf("IP pattern %q should match a valid IPv4 address", p)
	}
}

func TestGenerateCaseInsensitiveWrapsExceptCustomRegex(t *testing.T) {
	b := Builder{Type: Contains, Text: "cat", CaseSensitive: false}
	p, _ := b.Generate()
	re := regexp.MustCompile(p)
	if !re.MatchString("CAT") {
		t.Error("case-insensitive Contains should match differently-cased text")
	}

	custom := Builder{Type: CustomRegex, Text: "cat", CaseSensitive: false}
	p, _ = custom.Generate()
	if p != "cat" {
		t.Errorf("CustomRegex should not be wrapped with (?i), got %q", p)
	}
}

func TestGenerateCustomRegexPassesThrough(t *testing.T) {
	b := Builder{Type: CustomRegex, Text: `\d+`, CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if p != `\d+` {
		t.Errorf("expected custom regex to pass through unescaped, got %q", p)
	}
}

func TestGenerateRejectsInvalidCustomRegex(t *testing.T) {
	b := Builder{Type: CustomRegex, Text: `(unterminated`, CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err) // Generate itself should not error; only compilation should
	}
	if _, err := regexp.Compile(p); err == nil {
		t.Error("expected an unterminated group to fail to compile")
	}
}

func TestTestCountsMatches(t *testing.T) {
	b := Builder{Type: Contains, Text: "a", CaseSensitive: true}
	n, err := b.Test("banana")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 non-overlapping matches of 'a' in 'banana', got %d", n)
	}
}

func TestTestReturnsErrorForInvalidRegex(t *testing.T) {
	b := Builder{Type: CustomRegex, Text: `(unterminated`, CaseSensitive: true}
	if _, err := b.Test("anything"); err == nil {
		t.Error("expected an error testing an invalid custom regex")
	}
}
