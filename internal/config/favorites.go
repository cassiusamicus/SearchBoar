package config

import "strconv"

const (
	sectionFavorites        = "Favorites"
	sectionFavoriteCats     = "FavoriteCategories"
	defaultFavoriteCategory = "Uncategorized"
)

// loadFavorites parses [Favorites], accepting the three value formats the
// original app could have written over time:
//
//	6 fields: filename|search_term|modified|size_bytes|size_human|category
//	5 fields: filename|search_term|modified|size_bytes|size_human  (category -> Uncategorized)
//	4 fields: filename|modified|size_bytes|size_human              (no search term)
//
// The section key is the favorite's full filepath (case-preserved).
func (c *Config) loadFavorites(doc *iniDoc) {
	for _, path := range doc.keys(sectionFavorites) {
		v, _ := doc.get(sectionFavorites, path)
		parts := splitPipe(v)

		var rec FavoriteRecord
		rec.Filepath = path

		switch len(parts) {
		case 6:
			rec.Filename, rec.SearchTerm, rec.Modified = parts[0], parts[1], parts[2]
			rec.SizeBytes = atoi64Or(parts[3], 0)
			rec.SizeHuman = parts[4]
			rec.Category = parts[5]
		case 5:
			rec.Filename, rec.SearchTerm, rec.Modified = parts[0], parts[1], parts[2]
			rec.SizeBytes = atoi64Or(parts[3], 0)
			rec.SizeHuman = parts[4]
			rec.Category = defaultFavoriteCategory
		case 4:
			rec.Filename, rec.Modified = parts[0], parts[1]
			rec.SizeBytes = atoi64Or(parts[2], 0)
			rec.SizeHuman = parts[3]
			rec.Category = defaultFavoriteCategory
		default:
			continue // malformed entry, skip
		}
		c.Favorites = append(c.Favorites, rec)
	}

	if v, ok := doc.get(sectionFavoriteCats, "category_order"); ok {
		if order := splitPipe(v); len(order) > 0 {
			c.FavoriteCategoryOrder = order
		}
	}
}

func (c *Config) saveFavorites(doc *iniDoc) {
	sec := doc.section(sectionFavorites)
	for _, rec := range c.Favorites {
		value := joinPipe([]string{
			rec.Filename,
			rec.SearchTerm,
			rec.Modified,
			strconv.FormatInt(rec.SizeBytes, 10),
			rec.SizeHuman,
			rec.Category,
		})
		sec.set(rec.Filepath, value)
	}

	order := c.FavoriteCategoryOrder
	if len(order) == 0 {
		order = []string{defaultFavoriteCategory}
	}
	doc.section(sectionFavoriteCats).set("category_order", joinPipe(order))
}
