package config

import "strconv"

const sectionRecentResults = "RecentResults"

// loadRecentResults parses [RecentResults], keyed "0".."9" (most-recent
// first), value = Path|DisplayPath|Modified|SizeBytes|SizeHuman.
func (c *Config) loadRecentResults(doc *iniDoc) {
	for _, key := range doc.keys(sectionRecentResults) {
		v, _ := doc.get(sectionRecentResults, key)
		parts := splitPipe(v)
		if len(parts) != 5 {
			continue
		}
		c.RecentResults = append(c.RecentResults, RecentResult{
			Path:        parts[0],
			DisplayPath: parts[1],
			Modified:    parts[2],
			SizeBytes:   atoi64Or(parts[3], 0),
			SizeHuman:   parts[4],
		})
	}
}

func (c *Config) saveRecentResults(doc *iniDoc) {
	sec := doc.section(sectionRecentResults)
	for i, r := range c.RecentResults {
		if i >= MaxRecentResults {
			break
		}
		value := joinPipe([]string{r.Path, r.DisplayPath, r.Modified, strconv.FormatInt(r.SizeBytes, 10), r.SizeHuman})
		sec.set(strconv.Itoa(i), value)
	}
}
