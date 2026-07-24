package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/cassiusamicus/SearchBoar/internal/config"
)

// buildWorkspaceBar is the Workspace Builder tab's workspace name + Load/
// Save/Delete row: save the current location selection (checked drives/
// folders, excluded subfolders, Local/SMB/NFS scope, and any checked
// SMB/NFS shares) under a name, then load or delete it later. This is the
// same list the Start tab's quick-select dropdown reads (see starttab.go's
// refreshWorkspaces), so a workspace saved here is immediately available
// there too. A full-width bar across the top of the tab, not one more card
// competing for space in the sidebar -- switching/saving a workspace is
// common enough to need to be seen immediately, not found by scanning
// down a column of cards.
func (t *locationsTab) buildWorkspaceBar() fyne.CanvasObject {
	t.workspaceSelect = widget.NewSelect(nil, nil)
	t.workspaceSelect.PlaceHolder = "(none selected)"
	t.refreshWorkspaceOptions()

	loadBtn := widget.NewButton("Load", func() { t.loadSelectedWorkspace() })
	saveBtn := widget.NewButton("Save Current as...", func() { t.promptSaveWorkspace() })
	deleteBtn := widget.NewButton("Delete", func() { t.deleteSelectedWorkspace() })

	return widget.NewCard("Workspace", "Save/load a named set of search locations", container.NewBorder(
		nil, nil, widget.NewLabel("Name:"), container.NewHBox(loadBtn, saveBtn, deleteBtn),
		t.workspaceSelect,
	))
}

// refreshWorkspaceOptions reloads the dropdown's option list from the
// store -- called after any add/delete here and mirrored on the Start
// tab's own quick-select so both stay in sync without one owning the other.
func (t *locationsTab) refreshWorkspaceOptions() {
	if t.workspaceSelect == nil {
		return
	}
	t.workspaceSelect.SetOptions(workspaceNames(t.app.wsStore.List()))
}

func workspaceNames(list []config.LocationWorkspace) []string {
	names := make([]string, len(list))
	for i, w := range list {
		names[i] = w.Name
	}
	return names
}

// currentWorkspaceSnapshot captures the tab's current location selection as
// a config.LocationWorkspace, ready to be saved under a name.
func (t *locationsTab) currentWorkspaceSnapshot() config.LocationWorkspace {
	roots, excludes := t.picker.selectedRootsAndExcludes()

	var smbShares, nfsExports []string
	for _, item := range t.shareItems {
		if !t.selectedShareKeys[item.key()] {
			continue
		}
		ref := item.host + ":" + item.name
		if item.kind == "smb" {
			smbShares = append(smbShares, ref)
		} else {
			nfsExports = append(nfsExports, ref)
		}
	}

	return config.LocationWorkspace{
		SearchLocal: t.localCheck.Checked,
		SearchSMB:   t.smbCheck.Checked,
		SearchNFS:   t.nfsCheck.Checked,
		LocalRoots:  roots,
		ExcludeDirs: excludes,
		SMBShares:   smbShares,
		NFSExports:  nfsExports,
	}
}

// applyWorkspace replaces the tab's current location selection with w's.
// Selected SMB/NFS shares are remembered by (host, name) key regardless of
// whether they're in the current shareItems list (nothing's been scanned
// yet this session, or a scan just hasn't found that host) -- they'll show
// as checked automatically once a matching scan populates the share list,
// consistent with the rest of this tab's explicit-opt-in-only design for
// network shares (see scanShares' doc comment).
func (t *locationsTab) applyWorkspace(w config.LocationWorkspace) {
	t.localCheck.SetChecked(w.SearchLocal)
	t.smbCheck.SetChecked(w.SearchSMB)
	t.nfsCheck.SetChecked(w.SearchNFS)

	t.picker.selection.Clear()
	for _, r := range w.LocalRoots {
		t.picker.selection.SetCascade(r, true)
	}
	for _, e := range w.ExcludeDirs {
		t.picker.selection.SetCascade(e, false)
	}
	if t.picker.tree != nil {
		t.picker.tree.Refresh()
	}

	t.selectedShareKeys = map[string]bool{}
	for _, ref := range w.SMBShares {
		if host, name, ok := splitHostRef(ref); ok {
			t.selectedShareKeys[networkShareItem{kind: "smb", host: host, name: name}.key()] = true
		}
	}
	for _, ref := range w.NFSExports {
		if host, name, ok := splitHostRef(ref); ok {
			t.selectedShareKeys[networkShareItem{kind: "nfs", host: host, name: name}.key()] = true
		}
	}
	if t.shareList != nil {
		t.shareList.Refresh()
	}
}

func splitHostRef(ref string) (host, name string, ok bool) {
	idx := strings.Index(ref, ":")
	if idx < 0 {
		return "", "", false
	}
	return ref[:idx], ref[idx+1:], true
}

func (t *locationsTab) promptSaveWorkspace() {
	dialog.ShowEntryDialog("Save Workspace", "Name:", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		snap := t.currentWorkspaceSnapshot()
		if _, exists := t.app.wsStore.Get(name); exists {
			dialog.ShowConfirm("Overwrite Workspace", fmt.Sprintf("A workspace named %q already exists. Overwrite it?", name), func(ok bool) {
				if ok {
					t.saveWorkspaceAs(name, snap)
				}
			}, t.app.win)
			return
		}
		t.saveWorkspaceAs(name, snap)
	}, t.app.win)
}

func (t *locationsTab) saveWorkspaceAs(name string, snap config.LocationWorkspace) {
	t.app.wsStore.Put(name, snap)
	t.app.wsStore.Save()
	t.refreshWorkspaceOptions()
	t.workspaceSelect.SetSelected(name)
	t.app.start.refreshWorkspaces()
	t.app.setStatus("Saved workspace \"" + name + "\"")
}

func (t *locationsTab) loadSelectedWorkspace() {
	name := t.workspaceSelect.Selected
	if name == "" {
		t.app.setStatus("Select a workspace to load")
		return
	}
	w, ok := t.app.wsStore.Get(name)
	if !ok {
		return
	}
	t.applyWorkspace(w)
	t.app.setStatus("Loaded workspace \"" + name + "\"")
}

func (t *locationsTab) deleteSelectedWorkspace() {
	name := t.workspaceSelect.Selected
	if name == "" {
		t.app.setStatus("Select a workspace to delete")
		return
	}
	dialog.ShowConfirm("Delete Workspace", fmt.Sprintf("Are you sure you want to delete %q?", name), func(ok bool) {
		if !ok {
			return
		}
		t.app.wsStore.Delete(name)
		t.app.wsStore.Save()
		t.workspaceSelect.ClearSelected()
		t.refreshWorkspaceOptions()
		t.app.start.refreshWorkspaces()
	}, t.app.win)
}
