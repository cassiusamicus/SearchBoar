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

// TestGenerateAllWordsMatchesAnyOrderSameLine guards the whole reason
// AllWords exists as a permutation-based alternation rather than a
// lookahead: Go's regexp package (RE2) doesn't support (?=...) at all, so
// "every word present, in any order" has no more direct expression.
func TestGenerateAllWordsMatchesAnyOrderSameLine(t *testing.T) {
	b := Builder{Type: AllWords, Text: "fat, sleek", CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(p)
	if !re.MatchString("the fat and sleek cat") {
		t.Error("expected AllWords to match when both words appear in the given order")
	}
	if !re.MatchString("the sleek fat cat") {
		t.Error("expected AllWords to match when both words appear in the opposite order")
	}
	if re.MatchString("the fat cat") {
		t.Error("expected AllWords not to match when only one word is present")
	}
}

func TestGenerateAllWordsRequiresAtLeastOneWord(t *testing.T) {
	b := Builder{Type: AllWords, Text: "  , ,  "}
	if _, err := b.Generate(); err == nil {
		t.Error("expected an error when AllWords has no real words")
	}
}

func TestGenerateAllWordsThreeWordsAnyOrder(t *testing.T) {
	b := Builder{Type: AllWords, Text: "cat, dog, bird", CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(p)
	for _, s := range []string{"cat dog bird", "bird cat dog", "dog bird cat"} {
		if !re.MatchString(s) {
			t.Errorf("expected AllWords to match %q (all three words present)", s)
		}
	}
	if re.MatchString("cat dog") {
		t.Error("expected AllWords not to match when only two of three words are present")
	}
}

func TestGenerateAnyWordMatchesEitherWord(t *testing.T) {
	b := Builder{Type: AnyWord, Text: "fear, death", CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(p)
	if !re.MatchString("fear itself") || !re.MatchString("facing death") {
		t.Errorf("expected AnyWord pattern %q to match either word", p)
	}
	if re.MatchString("nothing relevant here") {
		t.Error("expected AnyWord not to match when neither word is present")
	}
}

func TestGenerateAnyWordRequiresAtLeastOneWord(t *testing.T) {
	b := Builder{Type: AnyWord, Text: ""}
	if _, err := b.Generate(); err == nil {
		t.Error("expected an error when AnyWord has no words")
	}
}

func TestGenerateNearWordsMatchesWithinDistanceEitherOrder(t *testing.T) {
	b := Builder{Type: NearWords, Text: "fat", Text2: "sleek", Distance: 2, CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(p)
	if !re.MatchString("the fat black sleek cat") {
		t.Error("expected NearWords to match within the given distance")
	}
	if !re.MatchString("the sleek black fat cat") {
		t.Error("expected NearWords to match regardless of word order")
	}
	if re.MatchString("fat one two three sleek") {
		t.Error("expected NearWords not to match when the words are further apart than the distance")
	}
}

func TestGenerateNearWordsDefaultsDistanceWhenUnset(t *testing.T) {
	b := Builder{Type: NearWords, Text: "fat", Text2: "sleek", CaseSensitive: true}
	p, err := b.Generate()
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(p)
	if !re.MatchString("fat one two three sleek") {
		t.Error("expected NearWords to fall back to a usable default distance when Distance is unset")
	}
}

func TestGenerateNearWordsRequiresBothWords(t *testing.T) {
	b := Builder{Type: NearWords, Text: "fat", Text2: ""}
	if _, err := b.Generate(); err == nil {
		t.Error("expected an error when NearWords is missing its second word")
	}
}
