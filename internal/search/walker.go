package search

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type candidate struct {
	path string
	info os.FileInfo
}

// collectCandidates walks opts.Dir, applying the hidden-file rule,
// recursive/non-recursive scoping, exclude globs, size filters, and the
// filename regex -- everything a file must pass before its content (if
// any) is even considered.
func collectCandidates(ctx context.Context, opts Options, filenameRe *regexp.Regexp) ([]candidate, error) {
	var out []candidate
	root := opts.Dir

	excludeDirs := make(map[string]bool, len(opts.ExcludeDirs))
	for _, d := range opts.ExcludeDirs {
		excludeDirs[d] = true
	}

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil // skip unreadable entries rather than aborting the whole walk
		}

		name := d.Name()
		hidden := strings.HasPrefix(name, ".")

		if d.IsDir() {
			if path == root {
				return nil
			}
			if excludeDirs[path] {
				return filepath.SkipDir
			}
			if hidden && !opts.IncludeHidden {
				return filepath.SkipDir
			}
			if !opts.Recursive {
				return filepath.SkipDir
			}
			return nil
		}

		if hidden && !opts.IncludeHidden {
			return nil
		}
		if excludeMatches(name, opts.ExcludeGlobs, opts.CaseSensitive) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if opts.MinSizeBytes > 0 && info.Size() < opts.MinSizeBytes {
			return nil
		}
		if opts.MaxSizeBytes > 0 && info.Size() > opts.MaxSizeBytes {
			return nil
		}
		if filenameRe != nil && !filenameRe.MatchString(name) {
			return nil
		}
		out = append(out, candidate{path: path, info: info})
		return nil
	}

	err := filepath.WalkDir(root, walkFn)
	return out, err
}
