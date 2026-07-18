package netsearch

import "testing"

func TestEffectiveDefaultsFalse(t *testing.T) {
	s := NewPathSelection()
	if s.Effective("/mnt/DriveG") {
		t.Error("expected unset path to default to unchecked")
	}
}

func TestSetCascadeChecksKnownChildren(t *testing.T) {
	s := NewPathSelection()
	s.SetCascade("/mnt/DriveG/sub1", false) // discovered while expanding, initially inherits parent (false)
	s.SetCascade("/mnt/DriveG", true)

	if !s.Effective("/mnt/DriveG") {
		t.Error("expected drive to be checked")
	}
	if !s.Effective("/mnt/DriveG/sub1") {
		t.Error("expected cascade to check the already-known child")
	}
	if !s.Effective("/mnt/DriveG/sub2") {
		t.Error("expected an unexpanded child to inherit the checked parent")
	}
}

func TestIndividualChildOverride(t *testing.T) {
	s := NewPathSelection()
	s.SetCascade("/mnt/DriveG", true)
	s.SetCascade("/mnt/DriveG/private", false)

	if !s.Effective("/mnt/DriveG") {
		t.Error("expected drive to remain checked")
	}
	if s.Effective("/mnt/DriveG/private") {
		t.Error("expected explicitly unchecked subfolder to stay unchecked")
	}
	if !s.Effective("/mnt/DriveG/photos") {
		t.Error("expected sibling folder to still inherit the checked parent")
	}
}

func TestRootsAndExcludes(t *testing.T) {
	s := NewPathSelection()
	s.SetCascade("/mnt/DriveG", true)
	s.SetCascade("/mnt/DriveG/private", false)
	s.SetCascade("/mnt/DriveF", true)
	s.SetCascade("/mnt/DriveH", false)

	roots, excludes := s.Roots()
	if !containsStr(roots, "/mnt/DriveG") || !containsStr(roots, "/mnt/DriveF") {
		t.Errorf("roots = %v, expected DriveG and DriveF", roots)
	}
	if containsStr(roots, "/mnt/DriveH") {
		t.Errorf("roots = %v, unchecked drive should not appear", roots)
	}
	if !containsStr(excludes, "/mnt/DriveG/private") {
		t.Errorf("excludes = %v, expected DriveG/private", excludes)
	}
}

func TestRootsSkipsRedundantNestedChecked(t *testing.T) {
	s := NewPathSelection()
	s.SetCascade("/mnt/DriveG", true)
	s.SetCascade("/mnt/DriveG/photos", true) // already implied by the checked parent

	roots, _ := s.Roots()
	if len(roots) != 1 || roots[0] != "/mnt/DriveG" {
		t.Errorf("roots = %v, expected only the top-level checked drive", roots)
	}
}

func TestClearResetsSelection(t *testing.T) {
	s := NewPathSelection()
	s.SetCascade("/mnt/DriveG", true)
	s.Clear()
	if s.Effective("/mnt/DriveG") {
		t.Error("expected Clear to reset the drive back to unchecked")
	}
	roots, excludes := s.Roots()
	if len(roots) != 0 || len(excludes) != 0 {
		t.Errorf("expected no roots/excludes after Clear, got roots=%v excludes=%v", roots, excludes)
	}
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
