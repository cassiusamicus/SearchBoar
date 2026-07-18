package ui

import (
	"fmt"
	"time"
)

const modTimeLayout = "2006-01-02 15:04:05"

func formatModTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(modTimeLayout)
}

// parseModTime is formatModTime's inverse; the zero time.Time is returned
// (not an error) for blank/unparseable input, since callers treat a zero
// ModTime as "unknown" rather than a fatal condition.
func parseModTime(s string) time.Time {
	t, err := time.Parse(modTimeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// formatSize renders a byte count as a human string ("48.1 KB"), matching
// the original app's [Favorites] on-disk size_human field so favorites
// saved by this app and the Python app read the same either way.
func formatSize(n int64) string {
	const unit = 1024.0
	size := float64(n)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for size >= unit && i < len(units)-1 {
		size /= unit
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}
