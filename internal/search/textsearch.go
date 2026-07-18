package search

import (
	"os"
	"regexp"
	"strings"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
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
		ctxLines := append([]string(nil), lines[start:end+1]...)
		matches = append(matches, model.ContentMatch{
			LineNum:          i + 1,
			ContextStartLine: start + 1,
			ContextLines:     ctxLines,
		})
	}
	return matches
}
