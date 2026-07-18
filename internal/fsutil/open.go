// Package fsutil implements the small set of OS-shelling-out operations the
// UI layer needs for file actions: opening a file/directory with the
// default handler, listing "open with" candidates, and deleting a file.
package fsutil

import (
	"os/exec"
	"path/filepath"
)

// OpenPath opens path (file or directory) with the OS default handler.
func OpenPath(path string) error {
	return exec.Command("xdg-open", path).Start()
}

// ShowInFileManager opens path's parent directory with the OS default
// handler.
func ShowInFileManager(path string) error {
	return OpenPath(filepath.Dir(path))
}

// OpenWith launches path using the given command (a candidate from
// CandidatePrograms or a user-entered command).
func OpenWith(command, path string) error {
	return exec.Command(command, path).Start()
}
