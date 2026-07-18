package search

import (
	"path/filepath"
	"strings"
)

// excludeMatches reports whether name matches any of the given shell-glob
// patterns (path/filepath.Match syntax), matched against the filename only.
func excludeMatches(name string, patterns []string, caseSensitive bool) bool {
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		n := name
		if !caseSensitive {
			p = strings.ToLower(p)
			n = strings.ToLower(n)
		}
		if ok, err := filepath.Match(p, n); err == nil && ok {
			return true
		}
	}
	return false
}

// SplitExcludePatterns splits the comma-separated exclude-patterns field
// (e.g. "*.pyc, *.o,*.tmp") into individual trimmed glob patterns.
func SplitExcludePatterns(field string) []string {
	var out []string
	for _, p := range strings.Split(field, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
