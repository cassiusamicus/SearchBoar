// Package model holds the small set of types shared between the search
// engines (internal/search, internal/netsearch) and the UI layer.
package model

import "time"

// FileEntry describes a file on disk, independent of why it matched.
type FileEntry struct {
	Path    string
	Name    string
	ModTime time.Time
	Size    int64
}

// ContentMatch is one block of context lines around a content-search hit.
// LineNum is the 1-indexed line the match itself was found on;
// ContextStartLine is the 1-indexed line the first entry in ContextLines
// corresponds to.
type ContentMatch struct {
	LineNum          int
	ContextStartLine int
	ContextLines     []string
}

// FileResult is a file that matched a search, with zero or more content
// matches (zero for a filename-only search).
type FileResult struct {
	FileEntry
	Matches []ContentMatch
}
