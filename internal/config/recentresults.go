package config

import (
	"strconv"
	"strings"
)

const sectionRecentResults = "RecentResults"

// recentMatchSep separates individual matches within a RecentResult's
// matches field; recentMatchFieldSep separates fields within one match.
// Both are control characters (never comma/pipe) so they can never collide
// with a real path, filename, or line of file content.
const (
	recentMatchSep      = "\x1d"
	recentMatchFieldSep = "\x1e"
)

func joinRecentMatches(matches []RecentMatch) string {
	parts := make([]string, len(matches))
	for i, m := range matches {
		parts[i] = strconv.Itoa(m.LineNum) + "|" + strconv.Itoa(m.ContextStartLine) + "|" + strings.Join(m.ContextLines, recentMatchFieldSep)
	}
	return strings.Join(parts, recentMatchSep)
}

func splitRecentMatches(s string) []RecentMatch {
	if s == "" {
		return nil
	}
	var out []RecentMatch
	for _, part := range strings.Split(s, recentMatchSep) {
		fields := strings.SplitN(part, "|", 3)
		if len(fields) != 3 {
			continue
		}
		var lines []string
		if fields[2] != "" {
			lines = strings.Split(fields[2], recentMatchFieldSep)
		}
		out = append(out, RecentMatch{
			LineNum:          atoiOr(fields[0], 0),
			ContextStartLine: atoiOr(fields[1], 0),
			ContextLines:     lines,
		})
	}
	return out
}

// loadRecentResults parses [RecentResults], keyed "0".."9" (most-recent
// first), value = Path|DisplayPath|Modified|SizeBytes|SizeHuman|Matches.
// Matches was added later; a value with only the first 5 fields (from an
// older config.ini) still loads fine, just with no persisted match
// content, exactly as before.
func (c *Config) loadRecentResults(doc *iniDoc) {
	for _, key := range doc.keys(sectionRecentResults) {
		v, _ := doc.get(sectionRecentResults, key)
		parts := strings.SplitN(v, "|", 6)
		if len(parts) < 5 {
			continue
		}
		r := RecentResult{
			Path:        parts[0],
			DisplayPath: parts[1],
			Modified:    parts[2],
			SizeBytes:   atoi64Or(parts[3], 0),
			SizeHuman:   parts[4],
		}
		if len(parts) == 6 {
			r.Matches = splitRecentMatches(parts[5])
		}
		c.RecentResults = append(c.RecentResults, r)
	}
}

func (c *Config) saveRecentResults(doc *iniDoc) {
	sec := doc.section(sectionRecentResults)
	for i, r := range c.RecentResults {
		if i >= MaxRecentResults {
			break
		}
		value := strings.Join([]string{
			r.Path, r.DisplayPath, r.Modified, strconv.FormatInt(r.SizeBytes, 10), r.SizeHuman,
			joinRecentMatches(r.Matches),
		}, "|")
		sec.set(strconv.Itoa(i), value)
	}
}
