package config

const sectionRecent = "Recent"

func (c *Config) loadRecent(doc *iniDoc) {
	if v, ok := doc.get(sectionRecent, "paths"); ok {
		c.Recent.Paths = splitPipe(v)
	}
	if v, ok := doc.get(sectionRecent, "content_patterns"); ok {
		c.Recent.ContentPatterns = splitPipe(v)
	}
}

func (c *Config) saveRecent(doc *iniDoc) {
	sec := doc.section(sectionRecent)
	sec.set("paths", joinPipe(c.Recent.Paths))
	sec.set("content_patterns", joinPipe(c.Recent.ContentPatterns))
}

// AddPath records dir as the most-recently-used search directory, capped at
// MaxRecentPaths entries with duplicates moved to the front.
func (r *RecentConfig) AddPath(dir string) {
	r.Paths = prependDedup(r.Paths, dir, MaxRecentPaths)
}

// AddContentPattern records pattern as the most-recently-used content
// search pattern, capped at MaxRecentContentPatterns entries. Empty
// patterns are ignored, matching the original app (which only records
// non-empty content searches).
func (r *RecentConfig) AddContentPattern(pattern string) {
	if pattern == "" {
		return
	}
	r.ContentPatterns = prependDedup(r.ContentPatterns, pattern, MaxRecentContentPatterns)
}

func prependDedup(list []string, v string, max int) []string {
	out := make([]string, 0, max)
	out = append(out, v)
	for _, existing := range list {
		if existing == v {
			continue
		}
		out = append(out, existing)
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}
