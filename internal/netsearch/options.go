// Package netsearch implements LanSearch's network file search: local
// drives, SMB shares, and NFS exports, searched with a glob pattern. It has
// no GUI dependency.
package netsearch

// Options controls a single network search run.
type Options struct {
	Pattern string // shell glob, e.g. "*.{jpg,png}" -- matches the original LanSearch's Pattern Builder output

	// LocalRoots are the specific local paths to search (from the drive
	// picker). If empty, every mounted local drive is searched (the
	// original app's blanket behavior).
	LocalRoots  []string
	ExcludeDirs []string // paths (and their descendants) to skip within LocalRoots
	SearchLocal bool
	SearchSMB   bool
	SearchNFS   bool

	CIDR     string // network range for SMB/NFS host discovery; "" = autodetect
	Username string
	Password string
}

// Result is one matched file, from any source.
type Result struct {
	NetworkPath string // display path: local path, //host/share/..., or host:export/...
	LocalPath   string // actual filesystem path to open (under a mount point for SMB/NFS)
	Modified    string
	Size        int64
}

// LogLine is one line for the search log pane.
type LogLine struct {
	Level   string // INFO, WARN, ERROR
	Message string
}
