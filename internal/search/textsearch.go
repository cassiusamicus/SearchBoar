package search

import (
	"os"
	"regexp"
	"strings"

	"github.com/cassiusamicus/SearchBoar/internal/model"
)

// readText reads a whole text file's raw content. Files are expected to be
// documents/source code (the original app's target use case), not
// multi-gigabyte logs, so reading fully into memory is fine; the result is
// also what gets stored in the extraction cache, so it must be the file's
// real content, not a line-limited scan of it.
func readText(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// splitLines splits extracted text into lines the way bufio.ScanLines would
// (no trailing empty line for text ending in "\n", and "\r\n" endings
// collapse the same as "\n"), so line numbers match whether the text was
// just extracted or came back from the cache.
func splitLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// matchLinesInSlice finds every line matching re and returns one
// ContentMatch per hit, each carrying `before`/`after` lines of context.
func matchLinesInSlice(lines []string, re *regexp.Regexp, before, after int) []model.ContentMatch {
	var matches []model.ContentMatch
	for i, line := range lines {
		if !re.MatchString(line) {
			continue
		}
		start := i - before
		if start < 0 {
			start = 0
		}
		end := i + after
		if end >= len(lines) {
			end = len(lines) - 1
		}
		ctxLines := make([]string, end-start+1)
		for j := start; j <= end; j++ {
			ctxLines[j-start] = excerptLine(lines[j], re, maxContextLineChars)
		}
		matches = append(matches, model.ContentMatch{
			LineNum:          i + 1,
			ContextStartLine: start + 1,
			ContextLines:     ctxLines,
		})
	}
	return matches
}

// maxContextLineChars caps how much of a single line ends up in a
// ContentMatch's ContextLines. Most source files and prose are wrapped
// short enough that this never triggers, but real-world text sometimes
// isn't -- an entire paragraph written as one unwrapped line, a long
// transcript line, a minified file -- and without a cap, "N lines of
// context" around a match buried in the middle of such a line dumps the
// whole thing: possibly several KB of text that doesn't even fit on
// screen, with the user's actual context-line setting effectively
// ignored and the real hit lost somewhere in the middle.
const maxContextLineChars = 400

// excerptLine returns line unchanged if it's already short enough,
// otherwise a maxRunes-wide window centered on re's first match within
// it, so the actual hit stays visible rather than being buried past
// whatever the first screenful happened to show -- with an ellipsis
// marking whichever side got cut. A context line that doesn't itself
// contain the term (re is nil, or doesn't match this particular line) has
// no match position to center on, so the window is taken from the start
// instead. Operates on runes, not bytes, so multi-byte UTF-8 characters
// (accented letters, smart quotes, etc., common in real prose) never get
// split mid-character at the truncation boundary.
func excerptLine(line string, re *regexp.Regexp, maxRunes int) string {
	runes := []rune(line)
	if len(runes) <= maxRunes {
		return line
	}
	mid := maxRunes / 2
	if re != nil {
		if loc := re.FindStringIndex(line); loc != nil {
			mid = len([]rune(line[:(loc[0]+loc[1])/2]))
		}
	}
	start := mid - maxRunes/2
	if start < 0 {
		start = 0
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}
	excerpt := string(runes[start:end])
	if start > 0 {
		excerpt = "…" + excerpt
	}
	if end < len(runes) {
		excerpt += "…"
	}
	return excerpt
}
