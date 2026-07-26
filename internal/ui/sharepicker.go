package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/cassiusamicus/SearchBoar/internal/netsearch"
)

// sharePicker is the Workspace Builder tab's Network column: a tree of
// discovered SMB shares and NFS exports (see networkShareItem in
// locationstab.go, which scanShares populates). SMB share nodes are
// expandable -- their subfolders are listed live via smbclient's
// unmounted, no-privilege "ls" command (Engine.ListSMBDir) -- so a
// specific subfolder of a share can be selected, not just the whole
// share. NFS exports have no equivalent unmounted browsing mechanism (the
// only way to see inside one is to actually mount it, which needs a
// privilege prompt even just to look around), so they render as a single
// checkable leaf, same as the flat list this replaced.
//
// Tree node IDs are ordinal ("share0", "share0/Documents",
// "share0/Documents/2024", ...) rather than built from the item's own
// host/share/export-path text: an NFS export's "name" is itself a
// filesystem path (from `showmount -e`, e.g. "/srv/nfs/media") that can
// already contain "/", which would collide with netsearch.PathSelection's
// own "/"-delimited ancestor/cascade logic if used directly as part of a
// tree uid -- two unrelated exports where one's path happens to be a
// filesystem-prefix of another's would otherwise appear to be parent/child
// in the tree. Ordinal uids sidestep that entirely.
type sharePicker struct {
	app *App

	tree      *widget.Tree
	items     []networkShareItem
	selection *netsearch.PathSelection
	children  map[string][]string // uid -> subfolder names, lazily fetched
	pending   map[string]bool     // uids with an in-flight fetch, to dedupe

	// pendingSelected/pendingExcludes hold a loaded workspace's checked
	// shares/subfolder-excludes by item.key() -- independent of whatever
	// happens to be in p.items right now, since a saved workspace can
	// reference a share that hasn't been (re)discovered by a scan yet this
	// session (see ApplyPending). Whole-share checks apply the moment a
	// matching item appears in setItems; subfolder excludes apply lazily,
	// the moment that specific subfolder is actually browsed into (see
	// fetchChildren), since a not-yet-fetched subfolder has no uid to set
	// state on yet.
	pendingSelected map[string]bool
	pendingExcludes map[string][]string

	// credentials returns the SMB username/password to use for directory
	// listing -- a function rather than duplicated fields, since
	// locationstab.go already owns the entry widgets holding them.
	credentials func() (user, pass string)

	// OnChanged, if set, fires after any checkbox change, letting the
	// Selected column rebuild itself.
	OnChanged func()
}

func newSharePicker(app *App, credentials func() (user, pass string)) *sharePicker {
	return &sharePicker{
		app:             app,
		selection:       netsearch.NewPathSelection(),
		children:        map[string][]string{},
		pending:         map[string]bool{},
		pendingSelected: map[string]bool{},
		pendingExcludes: map[string][]string{},
		credentials:     credentials,
	}
}

// ApplyPending records a loaded workspace's checked shares and their
// share-relative excluded subfolders, applying them to any items already
// present and remembering the rest for whenever a matching scan discovers
// them (see setItems/fetchChildren).
func (p *sharePicker) ApplyPending(selected map[string]bool, excludes map[string][]string) {
	p.pendingSelected = selected
	p.pendingExcludes = excludes
	p.applyPendingToKnownItems()
	if p.tree != nil {
		p.tree.Refresh()
	}
}

// applyPendingToKnownItems checks the root box of every currently-known
// item with a pending whole-share selection.
func (p *sharePicker) applyPendingToKnownItems() {
	for i, item := range p.items {
		if p.pendingSelected[item.key()] {
			p.selection.SetCascade(string(shareRootUID(i)), true)
		}
	}
}

// applyPendingExcludesFor checks freshly-fetched children against any
// pending subfolder excludes for item, unchecking any that match exactly --
// called once per fetchChildren completion, since each call only reveals
// one more level of a share's tree and a multi-level exclude path can only
// be applied once every ancestor segment along it has itself been fetched.
func (p *sharePicker) applyPendingExcludesFor(item networkShareItem, subpath string, children []string) {
	pending := p.pendingExcludes[item.key()]
	if len(pending) == 0 {
		return
	}
	idx := -1
	for i, it := range p.items {
		if it.key() == item.key() {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	for _, name := range children {
		rel := name
		if subpath != "" {
			rel = subpath + "/" + name
		}
		for _, ex := range pending {
			if ex == rel {
				p.selection.SetCascade(string(shareRootUID(idx))+"/"+rel, false)
				break
			}
		}
	}
}

func (p *sharePicker) build() fyne.CanvasObject {
	p.tree = widget.NewTree(p.childUIDs, p.isBranch,
		func(branch bool) fyne.CanvasObject {
			return newTappableBox(widget.NewCheck("", nil), widget.NewLabel(""))
		},
		func(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) { p.updateNode(uid, o.(*tappableBox)) },
	)
	return p.tree
}

// setItems replaces the discovered share list (after a scan) and resets
// selection/cache state, matching drivePicker.rescan's own reset -- old
// uids (tied to the previous ordinal ordering) aren't meaningful once the
// item list changes.
func (p *sharePicker) setItems(items []networkShareItem) {
	p.items = items
	p.selection.Clear()
	p.children = map[string][]string{}
	p.pending = map[string]bool{}
	p.applyPendingToKnownItems()
	if p.tree != nil {
		p.tree.Refresh()
	}
}

func shareRootUID(i int) widget.TreeNodeID { return widget.TreeNodeID(fmt.Sprintf("share%d", i)) }

// findItem resolves uid back to the networkShareItem it belongs to and the
// share-relative subpath it represents ("" for the share's own root uid).
func (p *sharePicker) findItem(uid widget.TreeNodeID) (item networkShareItem, subpath string, ok bool) {
	for i, it := range p.items {
		root := string(shareRootUID(i))
		if string(uid) == root {
			return it, "", true
		}
		if rest, found := strings.CutPrefix(string(uid), root+"/"); found {
			return it, rest, true
		}
	}
	return networkShareItem{}, "", false
}

func (p *sharePicker) childUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	if uid == "" {
		out := make([]widget.TreeNodeID, len(p.items))
		for i := range p.items {
			out[i] = shareRootUID(i)
		}
		return out
	}
	item, subpath, ok := p.findItem(uid)
	if !ok || item.kind != "smb" {
		return nil
	}
	if children, cached := p.children[string(uid)]; cached {
		out := make([]widget.TreeNodeID, len(children))
		for i, name := range children {
			out[i] = uid + "/" + widget.TreeNodeID(name)
		}
		return out
	}
	p.fetchChildren(uid, item, subpath)
	return nil
}

// fetchChildren lists uid's subfolders in the background -- a real SMB
// call, not a local disk read, so unlike drivePicker's os.ReadDir this
// can't run synchronously on the UI thread -- and refreshes the tree once
// the result lands. Fyne re-invokes childUIDs at that point and finds the
// now-populated cache.
func (p *sharePicker) fetchChildren(uid widget.TreeNodeID, item networkShareItem, subpath string) {
	key := string(uid)
	if p.pending[key] {
		return
	}
	p.pending[key] = true
	user, pass := p.credentials()
	go func() {
		dirs, err := p.app.netEng.ListSMBDir(context.Background(), item.host, item.name, subpath, user, pass)
		runOnUI(func() {
			delete(p.pending, key)
			// Cached even on error/empty so repeatedly expanding an
			// unreachable or empty share doesn't refetch every redraw --
			// matches drivePicker.childUIDs' own no-retry-on-error
			// handling of a real ReadDir failure.
			p.children[key] = dirs
			p.applyPendingExcludesFor(item, subpath, dirs)
			if err != nil {
				p.app.setStatus("Couldn't list " + item.label() + ": " + err.Error())
			}
			p.tree.Refresh()
		})
	}()
}

// isBranch reports whether uid can be expanded: every SMB node (the share
// itself, or one of its subfolders) can be, since it's always potentially
// a directory with contents of its own (an empty one just expands to
// nothing, same reasoning as drivePicker.isBranch); NFS export nodes never
// can (see this picker's own doc comment). The synthetic "" root uid must
// always report true regardless: Fyne's Tree.walk only ever calls
// childUIDs("") to discover the top-level items at all if isBranch("")
// says the root itself is a branch (see Tree.walk/IsBranchOpen, which
// special-cases an empty uid as always-open) -- returning false there, as
// findItem's normal "no match" case would, means the tree silently never
// shows anything.
func (p *sharePicker) isBranch(uid widget.TreeNodeID) bool {
	if uid == "" {
		return true
	}
	item, _, ok := p.findItem(uid)
	return ok && item.kind == "smb"
}

func (p *sharePicker) updateNode(uid widget.TreeNodeID, box *tappableBox) {
	chk := box.box.Objects[0].(*widget.Check)
	lbl := box.box.Objects[1].(*widget.Label)

	item, subpath, ok := p.findItem(uid)
	if !ok {
		lbl.SetText(string(uid))
		return
	}
	if subpath == "" {
		lbl.SetText(item.label())
	} else {
		parts := strings.Split(subpath, "/")
		lbl.SetText(parts[len(parts)-1])
	}

	chk.OnChanged = nil
	chk.SetChecked(p.selection.Effective(string(uid)))
	path := string(uid)
	chk.OnChanged = func(v bool) {
		p.selection.SetCascade(path, v)
		p.tree.Refresh()
		if p.OnChanged != nil {
			p.OnChanged()
		}
	}
}

// SelectedShares returns every discovered share whose own root checkbox is
// checked -- i.e. will actually be mounted and searched. Checking only a
// subfolder without checking the share's own root has no effect on its
// own: the whole share still has to be mounted to reach any of its
// subfolders (see ExcludeSubpaths for narrowing what's searched within an
// otherwise-checked share).
func (p *sharePicker) SelectedShares() []networkShareItem {
	roots, _ := p.selection.Roots()
	rootSet := make(map[string]bool, len(roots))
	for _, r := range roots {
		rootSet[r] = true
	}
	var out []networkShareItem
	for i, item := range p.items {
		if rootSet[string(shareRootUID(i))] {
			out = append(out, item)
		}
	}
	return out
}

// Uncheck clears item's own root checkbox (and cascades that to any
// already-known checked subfolder beneath it), for the Selected column's
// per-row remove action.
func (p *sharePicker) Uncheck(item networkShareItem) {
	for i, it := range p.items {
		if it.key() != item.key() {
			continue
		}
		p.selection.SetCascade(string(shareRootUID(i)), false)
		if p.tree != nil {
			p.tree.Refresh()
		}
		return
	}
}

// ExcludeSubpaths returns the share-relative subfolder paths explicitly
// unchecked beneath item's otherwise-checked root, mirroring
// drivePicker.selectedRootsAndExcludes's local roots/excludes split, scoped
// to just this one share's subtree.
func (p *sharePicker) ExcludeSubpaths(item networkShareItem) []string {
	idx := -1
	for i, it := range p.items {
		if it.key() == item.key() {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	_, excludes := p.selection.Roots()
	prefix := string(shareRootUID(idx)) + "/"
	var out []string
	for _, e := range excludes {
		if rel, ok := strings.CutPrefix(e, prefix); ok {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}
