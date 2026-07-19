package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
)

// networkShareItem is one discovered SMB share or NFS export, shown as a
// checkable row in the Workspace Builder tab's share picker.
type networkShareItem struct {
	kind string // "smb" or "nfs"
	host string
	name string // share name (smb) or export path (nfs)
}

func (i networkShareItem) key() string { return i.kind + "|" + i.host + "|" + i.name }

func (i networkShareItem) label() string {
	if i.kind == "smb" {
		return fmt.Sprintf("SMB   //%s/%s", i.host, i.name)
	}
	return fmt.Sprintf("NFS   %s:%s", i.host, i.name)
}

// locationsTab is the "Workspace Builder" tab: a tree of local drives and
// their subfolders, each with a checkbox (the main content of the tab),
// which location types to search (Local/SMB/NFS), the network settings
// SMB/NFS discovery needs, and a share picker -- network shares must be
// explicitly discovered and checked before they're searched. Mounting
// every share on every LAN host automatically (the original design) meant
// a wall of privilege-escalation prompts on any network with more than a
// couple of hosts.
type locationsTab struct {
	app *App

	picker *drivePicker

	localCheck, smbCheck, nfsCheck *widget.Check
	cidrEntry                      *widget.Entry
	userEntry, passEntry           *widget.Entry

	shareItems        []networkShareItem
	selectedShareKeys map[string]bool
	shareList         *widget.List
	shareStatus       *widget.Label

	workspaceSelect *widget.Select
}

func newLocationsTab(a *App) *locationsTab {
	return &locationsTab{app: a, picker: newDrivePicker(), selectedShareKeys: map[string]bool{}}
}

func (t *locationsTab) build() fyne.CanvasObject {
	// Local-only by default: scanning/mounting SMB+NFS on every search is
	// expensive and, for most searches, unwanted -- the Start tab's "Local
	// Storage Only" button and these same checkboxes here are still how you
	// opt into network locations for a given search.
	t.localCheck = widget.NewCheck("Local drives", nil)
	t.localCheck.SetChecked(true)
	t.smbCheck = widget.NewCheck("SMB shares", nil)
	t.nfsCheck = widget.NewCheck("NFS exports", nil)

	t.cidrEntry = widget.NewEntry()
	t.cidrEntry.SetPlaceHolder("auto or 192.168.1.0/24")
	t.userEntry = widget.NewEntry()
	t.userEntry.SetPlaceHolder("Username")
	t.passEntry = widget.NewPasswordEntry()
	t.passEntry.SetPlaceHolder("Password")

	searchBtn := widget.NewButtonWithIcon("Search", theme.SearchIcon(), func() { t.app.startSearch() })
	searchBtn.Importance = widget.HighImportance

	sidebar := container.NewVBox(
		widget.NewCard("Search In", "", container.NewVBox(t.localCheck, t.smbCheck, t.nfsCheck)),
		widget.NewCard("Network Settings", "", container.NewVBox(
			container.NewBorder(nil, nil, widget.NewLabel("Range:"), nil, t.cidrEntry),
			widget.NewLabel("SMB credentials (this session only):"),
			container.NewBorder(nil, nil, widget.NewLabel("User:"), nil, t.userEntry),
			container.NewBorder(nil, nil, widget.NewLabel("Pass:"), nil, t.passEntry),
		)),
		searchBtn,
	)

	tree := widget.NewCard("Storage Locations", "Check drives and folders to search", t.picker.build())

	sharesCol := t.buildSharesColumn()

	main := container.NewHSplit(tree, sharesCol)
	main.Offset = 0.5

	// HSplit's minimum width is the sum of both children's natural widths,
	// which would otherwise pin the whole window's minimum size (as
	// happened before with the Pattern Builder's horizontal radio group).
	// Wrapping it in a horizontal scroll lets the window shrink freely;
	// the two columns scroll together if the window is narrower than they
	// need.
	body := container.NewBorder(nil, nil, nil, sidebar, container.NewHScroll(main))

	// The workspace bar (name + Load/Save/Delete) used to be one more card
	// stacked in the middle of the right-hand sidebar -- easy to miss, and
	// no more prominent than "Network Settings" below it, even though
	// switching/saving a workspace is one of the more common things to do
	// on this tab. A full-width bar across the top puts it somewhere it
	// can't be missed, ahead of everything else on the page.
	return container.NewBorder(t.buildWorkspaceBar(), nil, nil, nil, body)
}

func (t *locationsTab) buildSharesColumn() fyne.CanvasObject {
	t.shareList = widget.NewList(
		func() int { return len(t.shareItems) },
		func() fyne.CanvasObject { return newTappableBox(widget.NewCheck("", nil), widget.NewLabel("")) },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateShareRow(id, o.(*tappableBox)) },
	)
	t.shareStatus = widget.NewLabel("Not scanned yet")
	scanBtn := widget.NewButton("Scan for shares", func() { t.scanShares() })

	return widget.NewCard("Network Shares", "Scan, then check the shares you want searched", container.NewBorder(
		container.NewVBox(scanBtn, t.shareStatus), nil, nil, nil,
		container.NewVScroll(t.shareList),
	))
}

func (t *locationsTab) updateShareRow(id widget.ListItemID, box *tappableBox) {
	if id >= len(t.shareItems) {
		return
	}
	item := t.shareItems[id]
	chk := box.box.Objects[0].(*widget.Check)
	lbl := box.box.Objects[1].(*widget.Label)
	lbl.SetText(item.label())

	chk.OnChanged = nil
	chk.SetChecked(t.selectedShareKeys[item.key()])
	key := item.key()
	chk.OnChanged = func(v bool) { t.selectedShareKeys[key] = v }
}

// scanShares discovers SMB/NFS hosts and shares on the configured network
// range and populates the share picker; it never mounts anything. Runs in
// the background since host/share discovery can take a while (nmap alone
// has a 2-minute budget).
func (t *locationsTab) scanShares() {
	cidr := t.cidrEntry.Text
	if cidr == "" {
		if detected, err := netsearch.DetectLocalCIDR(); err == nil {
			cidr = detected
		}
	}
	user, pass := t.userEntry.Text, t.passEntry.Text
	scanSMB, scanNFS := t.smbCheck.Checked, t.nfsCheck.Checked

	t.shareStatus.SetText("Scanning...")
	t.app.setStatus("Scanning for network shares...")

	go func() {
		var items []networkShareItem
		if scanSMB {
			shares, err := t.app.netEng.DiscoverSMB(context.Background(), cidr, user, pass)
			if err != nil {
				runOnUI(func() { t.app.setStatus("SMB scan error: " + err.Error()) })
			}
			for _, s := range shares {
				items = append(items, networkShareItem{kind: "smb", host: s.Host, name: s.Share})
			}
		}
		if scanNFS {
			exports, err := t.app.netEng.DiscoverNFS(context.Background(), cidr)
			if err != nil {
				runOnUI(func() { t.app.setStatus("NFS scan error: " + err.Error()) })
			}
			for _, ex := range exports {
				items = append(items, networkShareItem{kind: "nfs", host: ex.Host, name: ex.Export})
			}
		}

		runOnUI(func() {
			t.shareItems = items
			t.shareList.Refresh()
			t.shareStatus.SetText(fmt.Sprintf("Found %d share(s)/export(s)", len(items)))
			t.app.setStatus(fmt.Sprintf("Found %d network share(s)", len(items)))
		})
	}()
}

// locationOptions builds the netsearch.LocationOptions for the current
// picker/checkbox/share-selection state.
func (t *locationsTab) locationOptions() netsearch.LocationOptions {
	roots, _ := t.picker.selectedRootsAndExcludes()

	var smbShares []netsearch.SMBShare
	var nfsExports []netsearch.NFSExport
	for _, item := range t.shareItems {
		if !t.selectedShareKeys[item.key()] {
			continue
		}
		if item.kind == "smb" {
			smbShares = append(smbShares, netsearch.SMBShare{Host: item.host, Share: item.name})
		} else {
			nfsExports = append(nfsExports, netsearch.NFSExport{Host: item.host, Export: item.name})
		}
	}

	return netsearch.LocationOptions{
		LocalRoots:         roots,
		SearchLocal:        t.localCheck.Checked,
		SearchSMB:          t.smbCheck.Checked,
		SearchNFS:          t.nfsCheck.Checked,
		SelectedSMBShares:  smbShares,
		SelectedNFSExports: nfsExports,
		CIDR:               t.cidrEntry.Text,
		Username:           t.userEntry.Text,
		Password:           t.passEntry.Text,
	}
}

// excludeDirsFor returns the picker's unchecked-subfolder exclusions that
// fall under root, for passing to search.Options.ExcludeDirs when walking
// that particular resolved root.
func (t *locationsTab) excludeDirsFor(root string) []string {
	_, excludes := t.picker.selectedRootsAndExcludes()
	prefix := strings.TrimRight(root, string(filepath.Separator)) + string(filepath.Separator)
	var out []string
	for _, e := range excludes {
		if strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

func (t *locationsTab) anyLocationSelected() bool {
	return t.localCheck.Checked || t.smbCheck.Checked || t.nfsCheck.Checked
}

// restoreCheckedPaths pre-checks the picker with the locations used by the
// last search (loaded from config), so the Workspace Builder tab reflects
// them even before the user expands the tree to see the actual nodes.
func (t *locationsTab) restoreCheckedPaths(paths []string) {
	for _, p := range paths {
		t.picker.selection.SetCascade(p, true)
	}
	if t.picker.tree != nil {
		t.picker.tree.Refresh()
	}
}
