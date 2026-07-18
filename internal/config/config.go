// Package config reads and writes SearchBoar's on-disk configuration at
// ~/.config/searchboar/config.ini, in the exact INI shape the original
// Python/GTK3 app used, so existing user data (recent paths, window
// geometry, column widths, favorites, stored searches, custom programs)
// loads with zero migration.
package config

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

const (
	MaxRecentPaths           = 5
	MaxRecentContentPatterns = 10
	MaxFavorites             = 100
	MaxRecentResults         = 10
)

// Config is the fully-parsed, in-memory representation of config.ini.
type Config struct {
	path string

	Recent                RecentConfig
	Window                WindowState
	Columns               map[string]int
	Favorites             []FavoriteRecord
	FavoriteCategoryOrder []string
	FavoriteSearches      map[string]StoredSearch
	CustomPrograms        map[string]string
	RecentResults         []RecentResult
}

type RecentConfig struct {
	// Paths holds the search locations used by the most recently run
	// search (not a rolling history despite the name -- see AddPath vs.
	// direct assignment), shown on the Start tab.
	Paths           []string
	ContentPatterns []string
	LastFilePattern string
}

// RecentResult is a lightweight (no content-match text) record of one
// file found by the most recently completed search, shown on the Start
// tab so recent results survive an app restart.
type RecentResult struct {
	Path        string
	DisplayPath string
	Modified    string
	SizeBytes   int64
	SizeHuman   string
}

// WindowState mirrors the original app's convention: X/Y default to -1 to
// mean "no saved position" (Width/Height are still restored on their own).
type WindowState struct {
	X, Y, Width, Height int
}

type FavoriteRecord struct {
	Filepath   string
	Filename   string
	SearchTerm string
	Modified   string
	SizeBytes  int64
	SizeHuman  string
	Category   string
}

type StoredSearch struct {
	Name           string
	FilePattern    string
	ContentPattern string
	Directory      string
}

// DefaultPath returns ~/.config/searchboar/config.ini, matching the
// original app's Path.home() / '.config' / 'searchboar' / 'config.ini'.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "searchboar", "config.ini")
}

// New returns an empty Config bound to path, with the same defaults the
// original app used when no config file exists yet.
func New(path string) *Config {
	return &Config{
		path:                  path,
		Window:                WindowState{X: -1, Y: -1},
		Columns:               map[string]int{},
		FavoriteSearches:      map[string]StoredSearch{},
		CustomPrograms:        map[string]string{},
		FavoriteCategoryOrder: []string{"Uncategorized"},
	}
}

// Load reads path if it exists, or returns a fresh default Config (bound to
// path for a later Save) if it doesn't.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return New(path), nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	doc, err := parseIni(f)
	if err != nil {
		return nil, err
	}

	cfg := New(path)
	cfg.loadRecent(doc)
	cfg.loadWindow(doc)
	cfg.loadColumns(doc)
	cfg.loadFavorites(doc)
	cfg.loadFavoriteSearches(doc)
	cfg.loadCustomPrograms(doc)
	cfg.loadRecentResults(doc)
	return cfg, nil
}

// Save atomically writes the config back to its bound path, creating the
// parent directory if necessary.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}

	doc := newIniDoc()
	c.saveRecent(doc)
	c.saveWindow(doc)
	c.saveColumns(doc)
	c.saveFavorites(doc)
	c.saveFavoriteSearches(doc)
	c.saveCustomPrograms(doc)
	c.saveRecentResults(doc)

	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".config-*.ini.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := doc.write(tmp); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, c.path)
}

func splitPipe(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == '|' {
			out = append(out, v[start:i])
			start = i + 1
		}
	}
	return out
}

func joinPipe(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "|"
		}
		out += p
	}
	return out
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func atoi64Or(s string, def int64) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
