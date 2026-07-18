package search

import (
	"io"
	"os"
)

// isBinaryFile sniffs the first 8KB of a file for a null byte, the same
// heuristic the original app used to decide whether a file's content is
// worth searching as text.
func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true, nil
		}
	}
	return false, nil
}
