package search

import (
	"bufio"
	"os"
	"regexp"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// readLines reads a whole text file into lines. Files are expected to be
// documents/source code (the original app's target use case), not
// multi-gigabyte logs, so reading fully into memory keeps the context-window
// slicing below trivially correct rather than streaming with a ring buffer.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
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
