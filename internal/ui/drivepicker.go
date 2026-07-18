package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
)

// drivePicker is the "select and deselect all the desired drives and
// paths to be searched" tree: local drives at the top level (labeled
// Internal/Removable/SD Card), lazily-expandable subfolders underneath,
// each with a checkbox. Checking a drive or folder cascades to its
// already-expanded children; any subfolder can still be individually
// re-checked/unchecked afterward.
type drivePicker struct {
	tree      *widget.Tree
	drives    []netsearch.Drive
	selection *netsearch.PathSelection
	children  map[string][]string // lazy subdirectory listing cache
}

func newDrivePicker() *drivePicker {
	return &drivePicker{selection: netsearch.NewPathSelection(), children: map[string][]string{}}
}

func (p *drivePicker) build() fyne.CanvasObject {
	p.rescan()

	p.tree = widget.NewTree(p.childUIDs, p.isBranch,
		func(branch bool) fyne.CanvasObject {
			return newTappableBox(widget.NewCheck("", nil), widget.NewLabel(""))
		},
		func(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) { p.updateNode(uid, o.(*tappableBox)) },
	)

	rescanBtn := widget.NewButton("Search for new storage devices", func() {
		p.rescan()
		p.tree.Refresh()
	})

	return container.NewBorder(rescanBtn, nil, nil, nil, p.tree)
}

func (p *drivePicker) rescan() {
	drives, err := netsearch.DetectLocalDrives()
	if err != nil {
		return
	}
	p.drives = drives
	p.children = map[string][]string{}
}

func (p *drivePicker) childUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	if uid == "" {
		out := make([]widget.TreeNodeID, len(p.drives))
		for i, d := range p.drives {
			out[i] = d.MountPoint
		}
		return out
	}
	if children, ok := p.children[uid]; ok {
		return children
	}
	entries, err := os.ReadDir(uid)
	if err != nil {
		p.children[uid] = nil
		return nil
	}
	var children []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		children = append(children, filepath.Join(uid, e.Name()))
	}
	sort.Strings(children)
	p.children[uid] = children
	return children
}

// isBranch always reports true for a non-root node: every node in this
// tree is, by construction, a directory (childUIDs only ever lists
// subdirectories), so it's always potentially expandable. An empty
// directory just expands to show nothing.
func (p *drivePicker) isBranch(uid widget.TreeNodeID) bool {
	return true
}

func (p *drivePicker) driveLabel(mountPoint string) string {
	for _, d := range p.drives {
		if d.MountPoint == mountPoint {
			return d.Label
		}
	}
	return mountPoint
}

func (p *drivePicker) updateNode(uid widget.TreeNodeID, box *tappableBox) {
	chk := box.box.Objects[0].(*widget.Check)
	lbl := box.box.Objects[1].(*widget.Label)

	label := filepath.Base(uid)
	if isDrivePath(p.drives, uid) {
		label = p.driveLabel(uid)
	}
	lbl.SetText(label)

	chk.OnChanged = nil
	chk.SetChecked(p.selection.Effective(uid))
	path := uid
	chk.OnChanged = func(v bool) {
		p.selection.SetCascade(path, v)
		p.tree.Refresh()
	}
}

func isDrivePath(drives []netsearch.Drive, path string) bool {
	for _, d := range drives {
		if d.MountPoint == path {
			return true
		}
	}
	return false
}

// SelectedRootsAndExcludes returns the netsearch.Options-ready root/exclude
// lists. If nothing has been explicitly checked at all, it returns (nil,
// nil) meaning "search every drive" (the original app's default
// behavior), matching the expectation that an untouched picker doesn't
// silently mean "search nothing."
func (p *drivePicker) selectedRootsAndExcludes() (roots, excludes []string) {
	roots, excludes = p.selection.Roots()
	if len(roots) == 0 && len(excludes) == 0 {
		return nil, nil
	}
	return roots, excludes
}
