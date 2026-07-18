package netsearch

import (
	"path/filepath"
	"sort"
	"strings"
)

// PathSelection tracks explicit checked/unchecked overrides for a tree of
// filesystem paths, backing the drive picker: checking a drive or folder
// cascades that state to its already-expanded subfolders, but any
// subfolder can still be individually re-checked/unchecked afterward.
type PathSelection struct {
	explicit map[string]bool
}

func NewPathSelection() *PathSelection {
	return &PathSelection{explicit: map[string]bool{}}
}

// Clear removes every explicit checked/unchecked override, resetting the
// selection to its default (everything unchecked/inherited).
func (s *PathSelection) Clear() {
	s.explicit = map[string]bool{}
}

// SetCascade sets path's explicit state and applies the same state to
// every already-known path nested under it.
func (s *PathSelection) SetCascade(path string, checked bool) {
	s.explicit[path] = checked
	prefix := strings.TrimRight(path, string(filepath.Separator)) + string(filepath.Separator)
	for p := range s.explicit {
		if strings.HasPrefix(p, prefix) {
			s.explicit[p] = checked
		}
	}
}

// Effective returns path's checked state: its own explicit value if set,
// else the nearest explicit ancestor's value, else false (unchecked by
// default).
func (s *PathSelection) Effective(path string) bool {
	for p := path; ; {
		if v, ok := s.explicit[p]; ok {
			return v
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}

// Roots computes the minimal set of top-level checked paths to search
// (skipping any path whose parent is already checked, to avoid redundant
// walks) and the top-level unchecked paths nested beneath a checked
// ancestor, which the caller should exclude while walking each root.
func (s *PathSelection) Roots() (roots, excludes []string) {
	paths := make([]string, 0, len(s.explicit))
	for p := range s.explicit {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		if s.explicit[p] && !s.hasCheckedAncestor(p) {
			roots = append(roots, p)
		}
	}
	for _, p := range paths {
		if !s.explicit[p] && s.hasCheckedAncestor(p) {
			excludes = append(excludes, p)
		}
	}
	return roots, excludes
}

func (s *PathSelection) hasCheckedAncestor(path string) bool {
	for p := filepath.Dir(path); ; {
		if v, ok := s.explicit[p]; ok {
			return v
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false
		}
		p = parent
	}
}
