package fsutil

import "os"

// DeleteFile removes a single file (not a directory), matching the
// original app's os.remove behind its "Delete file" context menu action.
func DeleteFile(path string) error {
	return os.Remove(path)
}
