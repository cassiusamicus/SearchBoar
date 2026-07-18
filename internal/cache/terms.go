package cache

import (
	"strings"
	"time"
)

// TermInfo describes one entry in the Common Search Terms list.
type TermInfo struct {
	Term          string
	AddedAt       time.Time
	LastIndexedAt time.Time // zero if never indexed
	MatchCount    int
}

// TermMatch is one content match found while indexing a common term,
// shaped like model.FileResult/model.ContentMatch but flattened (one row
// per match, not grouped per file) to match how it's stored.
type TermMatch struct {
	Path        string
	DisplayPath string
	ModTime     time.Time
	Size        int64

	LineNum          int
	ContextStartLine int
	ContextLines     []string
}

// AddTerm adds term to the common-terms list if it isn't already present.
// It does not index it -- call SaveTermMatches (after running a search) to
// populate/refresh its matches.
func (c *Cache) AddTerm(term string) error {
	if c == nil {
		return nil
	}
	_, err := c.db.Exec(
		`INSERT INTO common_terms(term, added_at, last_indexed_at) VALUES (?, ?, 0)
		 ON CONFLICT(term) DO NOTHING`,
		term, time.Now().Unix(),
	)
	return err
}

// RemoveTerm deletes term and every match indexed for it.
func (c *Cache) RemoveTerm(term string) error {
	if c == nil {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM term_matches WHERE term = ?`, term); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM common_terms WHERE term = ?`, term); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ListTerms returns every common term, oldest-added first, with its current
// indexed match count.
func (c *Cache) ListTerms() ([]TermInfo, error) {
	if c == nil {
		return nil, nil
	}
	rows, err := c.db.Query(`
		SELECT t.term, t.added_at, t.last_indexed_at, COUNT(m.id)
		FROM common_terms t
		LEFT JOIN term_matches m ON m.term = t.term
		GROUP BY t.term, t.added_at, t.last_indexed_at
		ORDER BY t.added_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TermInfo
	for rows.Next() {
		var term string
		var addedAt, lastIndexedAt int64
		var count int
		if err := rows.Scan(&term, &addedAt, &lastIndexedAt, &count); err != nil {
			return nil, err
		}
		info := TermInfo{Term: term, AddedAt: time.Unix(addedAt, 0), MatchCount: count}
		if lastIndexedAt > 0 {
			info.LastIndexedAt = time.Unix(lastIndexedAt, 0)
		}
		out = append(out, info)
	}
	return out, rows.Err()
}

// SaveTermMatches replaces every stored match for term with matches, and
// stamps term's last-indexed time -- the whole thing runs in one
// transaction, so a search that finds zero matches still correctly clears
// out stale matches from a previous run rather than leaving them behind.
func (c *Cache) SaveTermMatches(term string, matches []TermMatch) error {
	if c == nil {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM term_matches WHERE term = ?`, term); err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO term_matches(term, path, display_path, mtime_ns, size, line_num, context_start_line, context_lines)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, m := range matches {
		if _, err := stmt.Exec(term, m.Path, m.DisplayPath, m.ModTime.UnixNano(), m.Size,
			m.LineNum, m.ContextStartLine, strings.Join(m.ContextLines, "\n")); err != nil {
			stmt.Close()
			tx.Rollback()
			return err
		}
	}
	stmt.Close()
	if _, err := tx.Exec(`UPDATE common_terms SET last_indexed_at = ? WHERE term = ?`, time.Now().Unix(), term); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// GetTermMatches returns every currently-indexed match for term.
func (c *Cache) GetTermMatches(term string) ([]TermMatch, error) {
	if c == nil {
		return nil, nil
	}
	rows, err := c.db.Query(`
		SELECT path, display_path, mtime_ns, size, line_num, context_start_line, context_lines
		FROM term_matches WHERE term = ? ORDER BY path, line_num`, term)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TermMatch
	for rows.Next() {
		var m TermMatch
		var mtimeNS int64
		var lines string
		if err := rows.Scan(&m.Path, &m.DisplayPath, &mtimeNS, &m.Size, &m.LineNum, &m.ContextStartLine, &lines); err != nil {
			return nil, err
		}
		m.ModTime = time.Unix(0, mtimeNS)
		m.ContextLines = strings.Split(lines, "\n")
		out = append(out, m)
	}
	return out, rows.Err()
}
