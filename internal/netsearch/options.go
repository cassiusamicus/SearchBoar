// Package netsearch discovers and mounts search locations: local drives,
// SMB shares, and NFS exports. It does not walk or match files itself --
// once a location is a real local path (a drive's mount point, or a
// freshly mounted SMB/NFS share), internal/search's engine does the actual
// walking/matching, so local and network search share one pattern
// language (regex) and one set of features (content search, PDF/DOCX
// extraction, ripgrep) instead of two parallel implementations.
package netsearch

// LocationOptions controls which locations ResolveRoots discovers.
type LocationOptions struct {
	// LocalRoots are the specific local paths to search (from the Search
	// Locations picker). If empty, every mounted local drive is used.
	LocalRoots []string

	SearchLocal bool
	SearchSMB   bool
	SearchNFS   bool

	CIDR     string // network range for SMB/NFS host discovery; "" = autodetect
	Username string
	Password string
}

// ResolvedRoot is one real filesystem path ready to be walked by
// internal/search, plus the display prefix (if any) to translate its
// results' local mount-point paths back into a network-style path.
type ResolvedRoot struct {
	Path          string
	DisplayPrefix string // "" for local roots; "//host/share" or "host:export" otherwise
}

// LogLine is one line for the search log pane.
type LogLine struct {
	Level   string // INFO, WARN, ERROR
	Message string
}
