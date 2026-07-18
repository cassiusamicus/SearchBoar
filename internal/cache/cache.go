// Package cache is a small on-disk (SQLite) store, in the same config
// directory as config.ini, holding two related but distinct things:
//
//   - file_text: extracted text per file, keyed by path + modification time
//   - size, so repeat searches over the same tree skip re-extracting a
//     file (PDF/DOCX parsing, or just the disk/network read) whose content
//     hasn't changed since it was last searched. A cached entry is only
//     ever read back for a file whose mtime/size still match what was
//     cached, so a changed file is always re-extracted rather than served
//     stale.
//   - common_terms/term_matches: a user-curated list of "terms I search for
//     often" (the Common Search Terms tab) and the last set of matches
//     found for each, refreshed on demand rather than kept continuously in
//     sync -- so the list is fast to check but can go stale between
//     refreshes, same tradeoff as file_text.
//
// Neither table is populated proactively; nothing is written until the user
// actually searches for it (or explicitly adds/reindexes a term), so this
// never grows into a background filesystem index.
package cache

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Cache wraps a SQLite database of cached file text. A nil *Cache is valid
// and behaves as a disabled cache (every method is a safe no-op), so callers
// that don't want caching can simply leave the field unset.
type Cache struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS file_text (
	path      TEXT PRIMARY KEY,
	mtime_ns  INTEGER NOT NULL,
	size      INTEGER NOT NULL,
	text      TEXT NOT NULL,
	cached_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS common_terms (
	term            TEXT PRIMARY KEY,
	added_at        INTEGER NOT NULL,
	last_indexed_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS term_matches (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	term               TEXT NOT NULL,
	path               TEXT NOT NULL,
	display_path       TEXT NOT NULL DEFAULT '',
	mtime_ns           INTEGER NOT NULL,
	size               INTEGER NOT NULL,
	line_num           INTEGER NOT NULL,
	context_start_line INTEGER NOT NULL,
	context_lines      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_term_matches_term ON term_matches(term);
`

// DefaultPath returns ~/.config/searchboar/cache.db, alongside config.ini.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "searchboar", "cache.db")
}

// Open creates/opens the cache database at path, creating its parent
// directory if needed.
func Open(path string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// The cache is written from multiple search worker goroutines
	// concurrently; SQLite only allows one writer at a time, and
	// modernc.org/sqlite has no built-in busy-retry, so a single
	// connection serializes writes instead of surfacing SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

// GetText returns the cached text for path if present and still fresh (its
// stored mtime and size match what's passed in).
func (c *Cache) GetText(path string, mtime time.Time, size int64) (string, bool) {
	if c == nil {
		return "", false
	}
	var mtimeNS, gotSize int64
	var text string
	err := c.db.QueryRow(`SELECT mtime_ns, size, text FROM file_text WHERE path = ?`, path).
		Scan(&mtimeNS, &gotSize, &text)
	if err != nil {
		return "", false
	}
	if mtimeNS != mtime.UnixNano() || gotSize != size {
		return "", false
	}
	return text, true
}

// PutText stores (or replaces) path's extracted text. Errors are dropped --
// a failed cache write should never fail the search it's serving.
func (c *Cache) PutText(path string, mtime time.Time, size int64, text string) {
	if c == nil {
		return
	}
	_, _ = c.db.Exec(
		`INSERT INTO file_text(path, mtime_ns, size, text, cached_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET mtime_ns = excluded.mtime_ns, size = excluded.size,
		   text = excluded.text, cached_at = excluded.cached_at`,
		path, mtime.UnixNano(), size, text, time.Now().Unix(),
	)
}

// Stats reports the cache's current row count and total cached text size,
// for showing the user roughly how big the cache has grown.
func (c *Cache) Stats() (rows int, textBytes int64, err error) {
	if c == nil {
		return 0, 0, nil
	}
	err = c.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(LENGTH(text)), 0) FROM file_text`).Scan(&rows, &textBytes)
	return rows, textBytes, err
}

// Clear deletes every cached entry.
func (c *Cache) Clear() error {
	if c == nil {
		return nil
	}
	_, err := c.db.Exec(`DELETE FROM file_text`)
	return err
}

// Close closes the underlying database.
func (c *Cache) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}
