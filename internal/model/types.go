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

	// DisplayPath is shown to the user in place of Path when set --
	// used for files found under a mounted network share, where Path is a
	// local mount-point path (e.g. "/tmp/searchboar-smb-123/doc.pdf") but
	// the user should see the network-style path (e.g.
	// "//host/share/doc.pdf"). Empty for ordinary local files, where Path
	// itself is already what should be shown.
	DisplayPath string
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
