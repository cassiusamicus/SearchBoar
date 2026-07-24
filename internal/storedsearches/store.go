// Package storedsearches implements the "Stored Searches" dialog's
// save/load/delete logic over a *config.Config.
package storedsearches

import (
	"errors"
	"sort"

	"github.com/cassiusamicus/SearchBoar/internal/config"
)

var (
	ErrNotFound = errors.New("stored search not found")
	ErrExists   = errors.New("a stored search with that name already exists")
)

type Store struct {
	cfg *config.Config
}

func LoadStore(cfg *config.Config) *Store {
	return &Store{cfg: cfg}
}

func (s *Store) Save() error {
	return s.cfg.Save()
}

// Add saves a new named search, rejecting a duplicate name (the original
// dialog only offered Save/Load/Delete, not overwrite-in-place).
func (s *Store) Add(name string, ss config.StoredSearch) error {
	if _, exists := s.cfg.FavoriteSearches[name]; exists {
		return ErrExists
	}
	ss.Name = name
	s.cfg.FavoriteSearches[name] = ss
	return nil
}

func (s *Store) Delete(name string) error {
	if _, exists := s.cfg.FavoriteSearches[name]; !exists {
		return ErrNotFound
	}
	delete(s.cfg.FavoriteSearches, name)
	return nil
}

func (s *Store) Get(name string) (config.StoredSearch, bool) {
	ss, ok := s.cfg.FavoriteSearches[name]
	return ss, ok
}

// List returns every stored search, sorted by name for a stable UI order.
func (s *Store) List() []config.StoredSearch {
	names := make([]string, 0, len(s.cfg.FavoriteSearches))
	for n := range s.cfg.FavoriteSearches {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]config.StoredSearch, len(names))
	for i, n := range names {
		out[i] = s.cfg.FavoriteSearches[n]
	}
	return out
}
