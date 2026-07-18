package netsearch

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

func formatModTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// walkAndMatch walks root, matching each file's name against pattern, and
// emits a Result whose NetworkPath is prefix + the path relative to root
// (e.g. "//host/share/relpath" or "host:export/relpath") -- used by the
// SMB/NFS search paths, where the display path is not the local mount
// point but a network-style path.
func walkAndMatch(ctx context.Context, root, pattern, prefix string, results chan<- Result) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchGlob(pattern, d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		networkPath := prefix + "/" + filepath.ToSlash(rel)

		select {
		case results <- Result{NetworkPath: networkPath, LocalPath: path, Modified: formatModTime(info.ModTime()), Size: info.Size()}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}
