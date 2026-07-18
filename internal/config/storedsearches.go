package config

const sectionFavoriteSearches = "FavoriteSearches"

// loadFavoriteSearches parses [FavoriteSearches]; a value must have exactly
// 3 pipe-delimited parts (file_pattern|content_pattern|directory) to be
// considered valid, matching the original app.
func (c *Config) loadFavoriteSearches(doc *iniDoc) {
	for _, name := range doc.keys(sectionFavoriteSearches) {
		v, _ := doc.get(sectionFavoriteSearches, name)
		parts := splitPipe(v)
		if len(parts) != 3 {
			continue
		}
		c.FavoriteSearches[name] = StoredSearch{
			Name:           name,
			FilePattern:    parts[0],
			ContentPattern: parts[1],
			Directory:      parts[2],
		}
	}
}

func (c *Config) saveFavoriteSearches(doc *iniDoc) {
	sec := doc.section(sectionFavoriteSearches)
	for _, name := range sortedKeys(c.FavoriteSearches) {
		s := c.FavoriteSearches[name]
		sec.set(name, joinPipe([]string{s.FilePattern, s.ContentPattern, s.Directory}))
	}
}
