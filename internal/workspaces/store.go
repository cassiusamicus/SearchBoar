// Package workspaces implements the Workspace Builder tab's "Workspaces"
// feature: save/load/delete a named set of search locations (which local
// roots/excluded subfolders, whether local/SMB/NFS are in scope, and which
// SMB shares/NFS exports were selected), over a *config.Config.
package workspaces

import (
	"errors"
	"sort"

	"github.com/cassiusamicus/SearchBoar/internal/config"
)

var ErrNotFound = errors.New("workspace not found")

type Store struct {
	cfg *config.Config
}

func LoadStore(cfg *config.Config) *Store {
	return &Store{cfg: cfg}
}

func (s *Store) Save() error {
	return s.cfg.Save()
}

// Put saves w under name, overwriting any existing workspace of that name --
// unlike Favorite Searches, a workspace is meant to be refined over time
// under a stable name, so callers wanting "don't clobber" behavior should
// check Get first (the Workspace Builder tab's save dialog does, prompting
// to confirm an overwrite).
func (s *Store) Put(name string, w config.LocationWorkspace) {
	w.Name = name
	s.cfg.Workspaces[name] = w
}

func (s *Store) Delete(name string) error {
	if _, exists := s.cfg.Workspaces[name]; !exists {
		return ErrNotFound
	}
	delete(s.cfg.Workspaces, name)
	return nil
}

func (s *Store) Get(name string) (config.LocationWorkspace, bool) {
	w, ok := s.cfg.Workspaces[name]
	return w, ok
}

// List returns every saved workspace, sorted by name for a stable UI order.
func (s *Store) List() []config.LocationWorkspace {
	names := make([]string, 0, len(s.cfg.Workspaces))
	for n := range s.cfg.Workspaces {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]config.LocationWorkspace, len(names))
	for i, n := range names {
		out[i] = s.cfg.Workspaces[n]
	}
	return out
}
