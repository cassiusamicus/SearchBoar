// Package favorites implements the category-tree business logic behind
// SearchBoar's Favorite Results tab, operating on an *config.Config so the
// UI layer only has to call Store methods and Save.
package favorites

import (
	"errors"

	"github.com/cassiusamicus/SearchBoar/internal/config"
)

const DefaultCategory = "Uncategorized"

var (
	ErrLimitReached     = errors.New("favorites limit reached")
	ErrDuplicate        = errors.New("file already in favorites")
	ErrNotFound         = errors.New("favorite not found")
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryExists   = errors.New("category already exists")
	ErrDefaultCategory  = errors.New("cannot delete the default Uncategorized category")
)

// Category is a named group of favorites, in display order.
type Category struct {
	Name  string
	Items []config.FavoriteRecord
}

// Store wraps a *config.Config with favorites/category business logic.
type Store struct {
	cfg *config.Config
}

func LoadStore(cfg *config.Config) *Store {
	return &Store{cfg: cfg}
}

// Save persists the underlying config.
func (s *Store) Save() error {
	return s.cfg.Save()
}

// Categories groups favorites by category, in the config's saved category
// order, preserving each favorite's relative position within its category.
func (s *Store) Categories() []Category {
	order := s.cfg.FavoriteCategoryOrder
	if len(order) == 0 {
		order = []string{DefaultCategory}
	}

	cats := make([]Category, len(order))
	index := map[string]int{}
	for i, name := range order {
		cats[i] = Category{Name: name}
		index[name] = i
	}

	for _, f := range s.cfg.Favorites {
		idx, ok := index[f.Category]
		if !ok {
			// A favorite references a category missing from the saved
			// order (e.g. a hand-edited config file) -- surface it
			// defensively rather than dropping the favorite.
			idx = len(cats)
			cats = append(cats, Category{Name: f.Category})
			index[f.Category] = idx
		}
		cats[idx].Items = append(cats[idx].Items, f)
	}
	return cats
}

// Add appends a new favorite, enforcing the original app's 100-item cap and
// duplicate-path detection.
func (s *Store) Add(rec config.FavoriteRecord) error {
	if len(s.cfg.Favorites) >= config.MaxFavorites {
		return ErrLimitReached
	}
	for _, f := range s.cfg.Favorites {
		if f.Filepath == rec.Filepath {
			return ErrDuplicate
		}
	}
	if rec.Category == "" {
		rec.Category = DefaultCategory
	}
	s.ensureCategory(rec.Category)
	s.cfg.Favorites = append(s.cfg.Favorites, rec)
	return nil
}

// Remove deletes the favorite at path, if any.
func (s *Store) Remove(path string) {
	out := s.cfg.Favorites[:0]
	for _, f := range s.cfg.Favorites {
		if f.Filepath != path {
			out = append(out, f)
		}
	}
	s.cfg.Favorites = out
}

// Contains reports whether path is already favorited.
func (s *Store) Contains(path string) bool {
	for _, f := range s.cfg.Favorites {
		if f.Filepath == path {
			return true
		}
	}
	return false
}

func (s *Store) AddCategory(name string) error {
	for _, c := range s.cfg.FavoriteCategoryOrder {
		if c == name {
			return ErrCategoryExists
		}
	}
	s.cfg.FavoriteCategoryOrder = append(s.cfg.FavoriteCategoryOrder, name)
	return nil
}

// RenameCategory renames a category and repoints every favorite in it.
func (s *Store) RenameCategory(oldName, newName string) error {
	found := false
	for i, c := range s.cfg.FavoriteCategoryOrder {
		if c == oldName {
			s.cfg.FavoriteCategoryOrder[i] = newName
			found = true
			break
		}
	}
	if !found {
		return ErrCategoryNotFound
	}
	for i := range s.cfg.Favorites {
		if s.cfg.Favorites[i].Category == oldName {
			s.cfg.Favorites[i].Category = newName
		}
	}
	return nil
}

// MoveToCategory moves an existing favorite into an existing category.
func (s *Store) MoveToCategory(path, category string) error {
	exists := false
	for _, c := range s.cfg.FavoriteCategoryOrder {
		if c == category {
			exists = true
			break
		}
	}
	if !exists {
		return ErrCategoryNotFound
	}
	for i := range s.cfg.Favorites {
		if s.cfg.Favorites[i].Filepath == path {
			s.cfg.Favorites[i].Category = category
			return nil
		}
	}
	return ErrNotFound
}

// DeleteCategory removes a category, moving its members into Uncategorized
// first (a no-op move for members already there).
func (s *Store) DeleteCategory(name string) error {
	if name == DefaultCategory {
		return ErrDefaultCategory
	}
	idx := -1
	for i, c := range s.cfg.FavoriteCategoryOrder {
		if c == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrCategoryNotFound
	}
	s.ensureCategory(DefaultCategory)
	for i := range s.cfg.Favorites {
		if s.cfg.Favorites[i].Category == name {
			s.cfg.Favorites[i].Category = DefaultCategory
		}
	}
	s.cfg.FavoriteCategoryOrder = append(s.cfg.FavoriteCategoryOrder[:idx], s.cfg.FavoriteCategoryOrder[idx+1:]...)
	return nil
}

// MoveUp/MoveDown swap a favorite with its neighbor within its own
// category, the Go equivalent of the original app's Alt+Up/Alt+Down
// reordering (the only reorder mechanism kept, since Fyne's Tree has no
// built-in drag-and-drop).
func (s *Store) MoveUp(path string) error   { return s.move(path, -1) }
func (s *Store) MoveDown(path string) error { return s.move(path, 1) }

func (s *Store) move(path string, dir int) error {
	fav := -1
	for i, f := range s.cfg.Favorites {
		if f.Filepath == path {
			fav = i
			break
		}
	}
	if fav < 0 {
		return ErrNotFound
	}
	category := s.cfg.Favorites[fav].Category

	var sameCategoryIdx []int
	for i, f := range s.cfg.Favorites {
		if f.Category == category {
			sameCategoryIdx = append(sameCategoryIdx, i)
		}
	}
	pos := -1
	for i, idx := range sameCategoryIdx {
		if idx == fav {
			pos = i
			break
		}
	}
	newPos := pos + dir
	if newPos < 0 || newPos >= len(sameCategoryIdx) {
		return nil // already at the edge of its category; no-op
	}
	a, b := sameCategoryIdx[pos], sameCategoryIdx[newPos]
	s.cfg.Favorites[a], s.cfg.Favorites[b] = s.cfg.Favorites[b], s.cfg.Favorites[a]
	return nil
}

func (s *Store) ensureCategory(name string) {
	for _, c := range s.cfg.FavoriteCategoryOrder {
		if c == name {
			return
		}
	}
	s.cfg.FavoriteCategoryOrder = append(s.cfg.FavoriteCategoryOrder, name)
}
