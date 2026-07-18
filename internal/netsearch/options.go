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

	// SelectedSMBShares/SelectedNFSExports are the specific shares/exports
	// to mount and search (from a prior Scan for Shares in the Search
	// Locations tab). Unlike local drives, there is no "empty means
	// everything" fallback here: mounting every share on every discovered
	// LAN host is what caused a wall of privilege-escalation prompts in
	// the original design, so network shares are opt-in only.
	SelectedSMBShares  []SMBShare
	SelectedNFSExports []NFSExport

	CIDR     string // network range for SMB/NFS host discovery; "" = autodetect
	Username string
	Password string
}

// SMBShare identifies one discovered (not yet necessarily mounted) SMB
// share.
type SMBShare struct {
	Host  string
	Share string
}

// NFSExport identifies one discovered NFS export.
type NFSExport struct {
	Host   string
	Export string
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
