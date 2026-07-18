package netsearch

import (
	"path/filepath"
	"strings"
)

// expandBraces expands a single {a,b,c} group in a glob pattern into
// multiple patterns, e.g. "*.{jpg,png}" -> ["*.jpg", "*.png"], mirroring
// the original LanSearch's brace-extension file-type patterns.
func expandBraces(pattern string) []string {
	start := strings.IndexByte(pattern, '{')
	end := strings.IndexByte(pattern, '}')
	if start < 0 || end < 0 || end < start {
		return []string{pattern}
	}
	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := strings.Split(pattern[start+1:end], ",")
	out := make([]string, 0, len(options))
	for _, opt := range options {
		out = append(out, prefix+opt+suffix)
	}
	return out
}

// matchGlob reports whether name matches pattern, with brace-expansion and
// case-insensitive comparison (the original used `find -iname`).
func matchGlob(pattern, name string) bool {
	name = strings.ToLower(name)
	for _, p := range expandBraces(pattern) {
		if ok, err := filepath.Match(strings.ToLower(p), name); err == nil && ok {
			return true
		}
	}
	return false
}
