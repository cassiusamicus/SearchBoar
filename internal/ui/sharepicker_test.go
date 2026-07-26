package ui

import (
	"reflect"
	"testing"

	"github.com/cassiusamicus/SearchBoar/internal/netsearch"
)

// newTestSharePicker builds just enough of a sharePicker for its
// selection-logic methods (SelectedShares/ExcludeSubpaths/ApplyPending) to
// be tested directly, without running build() or touching the network
// (fetchChildren, the only method that needs a real App/credentials, is
// never called by these tests).
func newTestSharePicker(items []networkShareItem) *sharePicker {
	return &sharePicker{
		items:           items,
		selection:       netsearch.NewPathSelection(),
		children:        map[string][]string{},
		pending:         map[string]bool{},
		pendingSelected: map[string]bool{},
		pendingExcludes: map[string][]string{},
	}
}

// TestIsBranchTreatsRootAsAlwaysExpandable guards a real bug this test
// suite's other cases couldn't catch (they never touch Fyne's actual
// widget.Tree walk): Tree.walk only calls childUIDs("") to discover the
// top-level items at all if isBranch("") reports the synthetic "" root
// itself as a branch (Tree.IsBranchOpen special-cases an empty uid as
// always-open, but only isBranch gates whether that matters). Since
// findItem("") never matches a real item, a naive isBranch that just
// delegated to findItem's "ok" result silently reported the root as a
// leaf -- the tree rendered nothing at all, no matter how many shares had
// just been discovered, with no error anywhere to point at why.
func TestIsBranchTreatsRootAsAlwaysExpandable(t *testing.T) {
	p := newTestSharePicker(nil) // deliberately empty -- must still be true
	if !p.isBranch("") {
		t.Fatal("isBranch(\"\") = false, want true (the tree can never discover any items otherwise)")
	}

	p2 := newTestSharePicker([]networkShareItem{{kind: "smb", host: "nas", name: "Public"}})
	if !p2.isBranch("") {
		t.Error("isBranch(\"\") = false with items present, want true")
	}
}

func TestIsBranchTrueOnlyForSMBNodes(t *testing.T) {
	items := []networkShareItem{
		{kind: "smb", host: "nas", name: "Public"},
		{kind: "nfs", host: "nas", name: "/srv/media"},
	}
	p := newTestSharePicker(items)
	if !p.isBranch(shareRootUID(0)) {
		t.Error("isBranch(smb root) = false, want true")
	}
	if p.isBranch(shareRootUID(1)) {
		t.Error("isBranch(nfs root) = true, want false (no unmounted way to browse an NFS export)")
	}
	if p.isBranch("not-a-real-uid") {
		t.Error("isBranch(unknown uid) = true, want false")
	}
}

func TestSelectedSharesReturnsOnlyCheckedRoots(t *testing.T) {
	items := []networkShareItem{
		{kind: "smb", host: "nas", name: "Public"},
		{kind: "smb", host: "nas", name: "Private"},
		{kind: "nfs", host: "nas", name: "/srv/media"},
	}
	p := newTestSharePicker(items)
	p.selection.SetCascade(string(shareRootUID(0)), true)
	p.selection.SetCascade(string(shareRootUID(2)), true)

	got := p.SelectedShares()
	want := []networkShareItem{items[0], items[2]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedShares() = %+v, want %+v", got, want)
	}
}

// TestSelectedSharesIgnoresSubfolderOnlyChecks guards the documented
// simplification: checking a subfolder without ever checking its share's
// own root has no effect on what's actually searched, since the whole
// share still has to be mounted to reach any subfolder of it.
func TestSelectedSharesIgnoresSubfolderOnlyChecks(t *testing.T) {
	items := []networkShareItem{{kind: "smb", host: "nas", name: "Public"}}
	p := newTestSharePicker(items)
	p.selection.SetCascade(string(shareRootUID(0))+"/Documents", true)

	if got := p.SelectedShares(); len(got) != 0 {
		t.Errorf("SelectedShares() = %+v, want none (root itself was never checked)", got)
	}
}

func TestExcludeSubpathsReturnsUncheckedChildrenBeneathACheckedRoot(t *testing.T) {
	items := []networkShareItem{{kind: "smb", host: "nas", name: "Public"}}
	p := newTestSharePicker(items)
	root := string(shareRootUID(0))
	p.selection.SetCascade(root, true)
	p.selection.SetCascade(root+"/Documents", false)
	p.selection.SetCascade(root+"/Photos/2024", false)

	got := p.ExcludeSubpaths(items[0])
	want := []string{"Documents", "Photos/2024"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExcludeSubpaths() = %v, want %v", got, want)
	}
}

func TestExcludeSubpathsEmptyWhenRootNotChecked(t *testing.T) {
	items := []networkShareItem{{kind: "smb", host: "nas", name: "Public"}}
	p := newTestSharePicker(items)
	p.selection.SetCascade(string(shareRootUID(0))+"/Documents", false)

	if got := p.ExcludeSubpaths(items[0]); got != nil {
		t.Errorf("ExcludeSubpaths() = %v, want nil when the share's own root isn't checked", got)
	}
}

// TestOrdinalUIDsDontCollideForNestedNFSPaths guards the whole reason tree
// uids are ordinal ("share0", "share1", ...) instead of built from the
// item's own host/share text: an NFS export's name is itself a filesystem
// path from `showmount -e` (e.g. "/srv/nfs/media") that can already
// contain "/". Two exports where one's path is a prefix of another's must
// stay independent -- checking one must not cascade to the other.
func TestOrdinalUIDsDontCollideForNestedNFSPaths(t *testing.T) {
	items := []networkShareItem{
		{kind: "nfs", host: "nas", name: "/srv/nfs"},
		{kind: "nfs", host: "nas", name: "/srv/nfs/media"},
	}
	p := newTestSharePicker(items)
	p.selection.SetCascade(string(shareRootUID(0)), true)

	got := p.SelectedShares()
	want := []networkShareItem{items[0]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedShares() = %+v, want only the explicitly checked export %+v (checking it must not cascade to an unrelated export whose path happens to nest under it)", got, want)
	}
}

func TestApplyPendingChecksAlreadyKnownItems(t *testing.T) {
	items := []networkShareItem{{kind: "smb", host: "nas", name: "Public"}}
	p := newTestSharePicker(items)

	p.ApplyPending(map[string]bool{items[0].key(): true}, nil)

	got := p.SelectedShares()
	want := []networkShareItem{items[0]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedShares() after ApplyPending = %+v, want %+v", got, want)
	}
}

// TestApplyPendingAppliesOnceMatchingItemIsScanned guards the case
// ApplyPending exists for: a workspace saved with a share that hasn't been
// (re)discovered by a scan yet this session must still show as checked the
// moment a matching scan populates the tree.
func TestApplyPendingAppliesOnceMatchingItemIsScanned(t *testing.T) {
	p := newTestSharePicker(nil)
	item := networkShareItem{kind: "smb", host: "nas", name: "Public"}

	p.ApplyPending(map[string]bool{item.key(): true}, nil)
	if got := p.SelectedShares(); len(got) != 0 {
		t.Fatalf("SelectedShares() before the matching item is known = %+v, want none", got)
	}

	p.setItems([]networkShareItem{item})
	got := p.SelectedShares()
	want := []networkShareItem{item}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedShares() after setItems = %+v, want %+v", got, want)
	}
}
