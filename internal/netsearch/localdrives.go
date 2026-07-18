package netsearch

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
)

// searchLocal walks opts.LocalRoots (or every detected drive, if none were
// explicitly selected -- the original app's blanket-search behavior),
// skipping opts.ExcludeDirs and hidden directories.
func (e *Engine) searchLocal(ctx context.Context, opts Options, results chan<- Result, log func(level, msg string)) error {
	roots := opts.LocalRoots
	if len(roots) == 0 {
		drives, err := DetectLocalDrives()
		if err != nil {
			return err
		}
		for _, d := range drives {
			roots = append(roots, d.MountPoint)
		}
	}

	exclude := make(map[string]bool, len(opts.ExcludeDirs))
	for _, ex := range opts.ExcludeDirs {
		exclude[ex] = true
	}

	for _, root := range roots {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log("INFO", "Searching "+root)

		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && (exclude[path] || strings.HasPrefix(d.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !matchGlob(opts.Pattern, d.Name()) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			select {
			case results <- Result{NetworkPath: "file://" + path, LocalPath: path, Modified: formatModTime(info.ModTime()), Size: info.Size()}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log("WARN", "error walking "+root+": "+err.Error())
		}
	}
	return nil
}
