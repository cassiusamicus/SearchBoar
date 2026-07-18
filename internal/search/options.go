package search

// Options controls a single search run, mirroring the fields the original
// SearchBoar app's Basic/Advanced tabs collected.
type Options struct {
	Dir            string
	FilePattern    string // regex; empty/".*" matches everything
	ContentPattern string // regex; ignored unless ContentEnabled
	ContentEnabled bool

	Recursive     bool
	CaseSensitive bool
	IncludeHidden bool

	ContextBefore int
	ContextAfter  int

	MinSizeBytes int64 // 0 = no minimum
	MaxSizeBytes int64 // 0 = no maximum

	// ExcludeGlobs are shell-glob patterns (path/filepath.Match syntax,
	// e.g. "*.pyc") matched against the filename only. This is real glob
	// matching, unlike the original app, which labeled this field as
	// glob-like but actually compiled it as regex.
	ExcludeGlobs []string

	// ExcludeDirs are absolute directory paths (and their descendants) to
	// skip entirely -- used by the Search Locations picker, where a
	// subfolder nested under an otherwise-checked drive can be individually
	// unchecked.
	ExcludeDirs []string
}

// Progress is a periodic, best-effort snapshot of an in-flight search;
// consumers should not assume every intermediate value is delivered.
type Progress struct {
	Searched int
	Found    int
	Total    int
}
