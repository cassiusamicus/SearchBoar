package search

import (
	"regexp"
	"strings"
	"testing"
)

func TestExcerptLineLeavesShortLineUnchanged(t *testing.T) {
	line := "a short line with target in it"
	got := excerptLine(line, regexp.MustCompile(`target`), maxContextLineChars)
	if got != line {
		t.Errorf("got %q, want unchanged %q", got, line)
	}
}

func TestExcerptLineCentersOnMatchInLongLine(t *testing.T) {
	// A long line with the match buried in the middle -- a naive
	// head-truncation would cut it off before ever reaching "target".
	line := strings.Repeat("filler ", 200) + "target" + strings.Repeat(" more", 200)
	re := regexp.MustCompile(`target`)
	got := excerptLine(line, re, 100)

	if !strings.Contains(got, "target") {
		t.Fatalf("excerpt doesn't contain the match: %q", got)
	}
	if len([]rune(got)) > 100+2 { // +2 for the two possible ellipsis runes
		t.Errorf("excerpt is %d runes, want roughly <= 100", len([]rune(got)))
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("excerpt should be marked as cut at the start: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("excerpt should be marked as cut at the end: %q", got)
	}
}

func TestExcerptLineNoMatchTruncatesFromStart(t *testing.T) {
	line := strings.Repeat("x", 1000)
	got := excerptLine(line, regexp.MustCompile(`target`), 100)
	if strings.HasPrefix(got, "…") {
		t.Errorf("a context line with no match of its own shouldn't be marked as cut at the start: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("excerpt should be marked as cut at the end: %q", got)
	}
}

func TestExcerptLineNilRegexTruncatesFromStart(t *testing.T) {
	line := strings.Repeat("x", 1000)
	got := excerptLine(line, nil, 100)
	if strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Errorf("nil-regex excerpt = %q, want truncated from the start only", got)
	}
}

// TestExcerptLineDoesNotSplitMultiByteRunes guards against corrupting
// non-ASCII text (accented letters, smart quotes, etc., common in real
// prose) by truncating at a byte offset that lands mid-character.
func TestExcerptLineDoesNotSplitMultiByteRunes(t *testing.T) {
	line := strings.Repeat("é", 50) + "target" + strings.Repeat("ü", 50)
	got := excerptLine(line, regexp.MustCompile(`target`), 20)
	if !utf8Valid(got) {
		t.Errorf("excerpt is not valid UTF-8: %q", got)
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestMatchLinesInSliceExcerptsLongLines(t *testing.T) {
	longLine := strings.Repeat("filler ", 300) + "target" + strings.Repeat(" more", 300)
	lines := []string{"short line before", longLine, "short line after"}
	re := regexp.MustCompile(`target`)

	got := matchLinesInSlice(lines, re, 1, 1)
	if len(got) != 1 {
		t.Fatalf("got %d matches, want 1", len(got))
	}
	m := got[0]
	if len(m.ContextLines) != 3 {
		t.Fatalf("got %d context lines, want 3", len(m.ContextLines))
	}
	matchedLine := m.ContextLines[1]
	if len([]rune(matchedLine)) > maxContextLineChars+2 {
		t.Errorf("matched line excerpt is %d runes, want roughly <= %d", len([]rune(matchedLine)), maxContextLineChars)
	}
	if !strings.Contains(matchedLine, "target") {
		t.Errorf("matched line excerpt lost the actual match: %q", matchedLine)
	}
}
