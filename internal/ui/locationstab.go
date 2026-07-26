package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/cassiusamicus/SearchBoar/internal/netsearch"
)

// networkShareItem is one discovered SMB share or NFS export, shown as a
// node in the Workspace Builder tab's share tree (sharePicker).
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

// mountLabel matches the DisplayPrefix netsearch.Engine.ResolveRoots
// assigns a resolved network root (see engine.go's MountJob.Label) -- used
// to find which share a search result's root actually came from, distinct
// from label()'s "SMB   "/"NFS   "-prefixed display text.
func (i networkShareItem) mountLabel() string {
	if i.kind == "smb" {
		return fmt.Sprintf("//%s/%s", i.host, i.name)
	}
	return fmt.Sprintf("%s:%s", i.host, i.name)
}

// selectedRow is one entry in the Workspace Builder tab's Selected column:
// either a checked local root or a checked network share.
type selectedRow struct {
	label   string
	isLocal bool
	path    string           // local root path, if isLocal
	share   networkShareItem // share item, if !isLocal
}

// locationsTab is the "Workspace Builder" tab: three columns -- Selected (a
// live, flat list of every path currently checked, across both other
// columns, with a remove action per row), Local (a tree of local drives and
// their subfolders, each with a checkbox), and Network (discovered SMB
// shares/NFS exports; SMB shares are themselves expandable trees, so a
// specific subfolder of a share can be selected, not just the whole share
// -- see sharepicker.go for why NFS exports can't be). Network shares must
// be explicitly discovered (Scan for Shares) and checked before they're
// searched -- mounting every share on every LAN host automatically (the
// original design) meant a wall of privilege-escalation prompts.
//
// Local search has no on/off toggle of its own: the tree already means
// "nothing checked" as "search everything" (see
// drivePicker.selectedRootsAndExcludes), so a separate "Local drives"
// checkbox on top of that said nothing the tree didn't already say on its
// own.
type locationsTab struct {
	app *App

	picker      *drivePicker
	sharePicker *sharePicker

	smbCheck, nfsCheck   *widget.Check
	cidrEntry            *widget.Entry
	userEntry, passEntry *widget.Entry

	shareStatus  *widget.Label
	scanBtn      *widget.Button
	scanProgress *widget.ProgressBarInfinite

	selectedList *widget.List
	selectedRows []selectedRow

	workspaceSelect *widget.Select
}

func newLocationsTab(a *App) *locationsTab {
	t := &locationsTab{app: a, picker: newDrivePicker()}
	t.sharePicker = newSharePicker(a, func() (string, string) { return t.userEntry.Text, t.passEntry.Text })
	return t
}

func (t *locationsTab) build() fyne.CanvasObject {
	t.smbCheck = widget.NewCheck("SMB shares", func(bool) { t.refreshSelectedColumn() })
	t.nfsCheck = widget.NewCheck("NFS exports", func(bool) { t.refreshSelectedColumn() })

	t.cidrEntry = widget.NewEntry()
	t.cidrEntry.SetPlaceHolder("auto or 192.168.1.0/24")
	// Prefilled from the most recent successful scan (if any), else
	// today's own best guess -- a blank field still resolves to
	// DetectLocalCIDR at scan time regardless, but prefilling it here
	// means the field isn't just showing an empty placeholder for a value
	// that's actually already known.
	if t.app.cfg.LastScanCIDR != "" {
		t.cidrEntry.SetText(t.app.cfg.LastScanCIDR)
	} else if detected, err := netsearch.DetectLocalCIDR(); err == nil {
		t.cidrEntry.SetText(detected)
	}

	t.userEntry = widget.NewEntry()
	t.userEntry.SetPlaceHolder("Username")
	t.passEntry = widget.NewPasswordEntry()
	t.passEntry.SetPlaceHolder("Password")

	t.shareStatus = widget.NewLabel("Not scanned yet")
	// Shown/started only while a scan is actually running (see
	// scanShares) -- an indeterminate bar rather than a percentage since
	// there's no way to know host/share counts in advance, but its motion
	// alone is what answers "is this still working" during a scan that can
	// legitimately take up to two minutes (nmap's own budget).
	t.scanProgress = widget.NewProgressBarInfinite()
	t.scanProgress.Hide()
	t.scanBtn = widget.NewButton("Scan for Shares", func() { t.scanShares() })

	t.picker.OnChanged = func() { t.refreshSelectedColumn() }
	t.sharePicker.OnChanged = func() { t.refreshSelectedColumn() }

	localCard := widget.NewCard("Local", "Check drives and folders to search", t.picker.build())

	networkHeader := container.NewVBox(
		container.NewHBox(t.smbCheck, t.nfsCheck),
		container.NewBorder(nil, nil, widget.NewLabel("Range:"), nil, t.cidrEntry),
		widget.NewLabel("SMB credentials (this session only):"),
		container.NewBorder(nil, nil, widget.NewLabel("User:"), nil, t.userEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Pass:"), nil, t.passEntry),
		container.NewHBox(t.scanBtn, t.shareStatus),
		t.scanProgress,
	)
	networkCard := widget.NewCard("Network", "", container.NewBorder(networkHeader, nil, nil, nil, t.sharePicker.build()))

	// Row content is wrapped in tappableBox (a proper widget.Widget via
	// BaseWidget+CreateRenderer), not a bare *fyne.Container -- widget.List
	// silently renders nothing for rows whose template isn't a real Widget
	// (see settings.go's buildCustomProgramsSettings, which uses the same
	// label+spacer+delete-icon-button row shape for the same reason).
	t.selectedList = widget.NewList(
		func() int { return len(t.selectedRows) },
		func() fyne.CanvasObject {
			return newTappableBox(widget.NewLabel(""), layout.NewSpacer(), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateSelectedRow(id, o.(*tappableBox)) },
	)
	selectedCard := widget.NewCard("Selected", "Everything currently checked, local and network", t.selectedList)

	// Three columns via nested HSplits (container has no native three-pane
	// split): Selected | Local | Network, per the user's own ordering.
	rightTwo := container.NewHSplit(localCard, networkCard)
	rightTwo.Offset = 0.5
	main := container.NewHSplit(selectedCard, rightTwo)
	main.Offset = 0.22

	// HSplit's own MinSize is the sum of its children's, which would
	// otherwise pin the whole window's minimum width (as happened before
	// with the Pattern Builder's horizontal radio group). Wrapping it in a
	// horizontal scroll lets the window shrink freely; the columns scroll
	// together if the window is narrower than they need.
	body := container.NewHScroll(main)

	t.refreshSelectedColumn()

	// The workspace bar (name + Load/Save/Delete) sits in a full-width bar
	// across the top of the tab, not one more card competing for space in
	// a column -- switching/saving a workspace is common enough to need to
	// be seen immediately, not found by scanning down a column of cards.
	return container.NewBorder(t.buildWorkspaceBar(), nil, nil, nil, body)
}

// scanShares discovers SMB/NFS hosts and shares on the configured network
// range and populates the share tree; it never mounts anything. Runs in
// the background since host/share discovery can take a while (nmap alone
// has a 2-minute budget) -- the progress bar and disabled button are the
// only feedback that it's actually working during that wait, not stuck.
func (t *locationsTab) scanShares() {
	cidr := t.cidrEntry.Text
	if cidr == "" {
		if detected, err := netsearch.DetectLocalCIDR(); err == nil {
			cidr = detected
		}
	}
	user, pass := t.userEntry.Text, t.passEntry.Text
	scanSMB, scanNFS := t.smbCheck.Checked, t.nfsCheck.Checked

	t.scanBtn.Disable()
	t.scanProgress.Show()
	t.scanProgress.Start()
	t.shareStatus.SetText("Scanning...")
	t.app.setStatus("Scanning for network shares...")

	go func() {
		var items []networkShareItem
		usedFallback := false
		if scanSMB {
			shares, fallback, err := t.app.netEng.DiscoverSMB(context.Background(), cidr, user, pass)
			usedFallback = usedFallback || fallback
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
			t.sharePicker.setItems(items)
			t.scanBtn.Enable()
			t.scanProgress.Stop()
			t.scanProgress.Hide()

			switch {
			case len(items) == 0 && usedFallback:
				// nmap missing and zero shares found were previously
				// indistinguishable -- worth saying explicitly, since the
				// fallback probe is slower and less thorough than a real
				// nmap scan.
				t.shareStatus.SetText("Found nothing (no nmap installed, used a slower fallback scan -- install nmap for faster, more thorough results)")
			case len(items) == 0:
				t.shareStatus.SetText("Found nothing")
			default:
				t.shareStatus.SetText(fmt.Sprintf("Found %d share(s)/export(s) on %d host(s)", len(items), countUniqueHosts(items)))
				t.app.cfg.LastScanCIDR = cidr
				t.app.cfg.Save()
			}
			t.app.setStatus(t.shareStatus.Text)
			t.refreshSelectedColumn()
		})
	}()
}

func countUniqueHosts(items []networkShareItem) int {
	hosts := map[string]bool{}
	for _, item := range items {
		hosts[item.host] = true
	}
	return len(hosts)
}

// refreshSelectedColumn rebuilds the Selected column's rows from both
// pickers' current state -- called whenever either picker's checkbox state
// changes (via their OnChanged hooks) or a scan completes.
func (t *locationsTab) refreshSelectedColumn() {
	var rows []selectedRow
	roots, _ := t.picker.selectedRootsAndExcludes()
	for _, r := range roots {
		rows = append(rows, selectedRow{label: t.picker.driveLabel(r), isLocal: true, path: r})
	}
	for _, item := range t.sharePicker.SelectedShares() {
		rows = append(rows, selectedRow{label: item.label(), share: item})
	}
	t.selectedRows = rows
	if t.selectedList != nil {
		t.selectedList.Refresh()
	}
}

func (t *locationsTab) updateSelectedRow(id widget.ListItemID, box *tappableBox) {
	lbl := box.box.Objects[0].(*widget.Label)
	btn := box.box.Objects[2].(*widget.Button)
	if id >= len(t.selectedRows) {
		lbl.SetText("")
		btn.OnTapped = nil
		return
	}
	row := t.selectedRows[id]
	lbl.SetText(row.label)
	btn.OnTapped = func() {
		if row.isLocal {
			t.picker.selection.SetCascade(row.path, false)
			t.picker.tree.Refresh()
		} else {
			t.sharePicker.Uncheck(row.share)
		}
		t.refreshSelectedColumn()
	}
}

// locationOptions builds the netsearch.LocationOptions for the current
// picker/checkbox/share-selection state.
func (t *locationsTab) locationOptions() netsearch.LocationOptions {
	roots, _ := t.picker.selectedRootsAndExcludes()

	var smbShares []netsearch.SMBShare
	var nfsExports []netsearch.NFSExport
	for _, item := range t.sharePicker.SelectedShares() {
		if item.kind == "smb" {
			smbShares = append(smbShares, netsearch.SMBShare{Host: item.host, Share: item.name})
		} else {
			nfsExports = append(nfsExports, netsearch.NFSExport{Host: item.host, Export: item.name})
		}
	}

	return netsearch.LocationOptions{
		LocalRoots: roots,
		// Always true: the tree itself already means "nothing checked" as
		// "search everything" (see selectedRootsAndExcludes), so there's
		// no separate on/off state for local search to carry.
		SearchLocal:        true,
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
// that particular resolved local root.
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

// excludeDirsForShare translates a checked share's unchecked subfolders
// (share-relative, from sharePicker.ExcludeSubpaths) into real absolute
// paths under mountPath -- the same shape search.Options.ExcludeDirs
// already expects for local roots (see excludeDirsFor). Called once a
// network root has actually been mounted (see searchcontrol.go's
// runUnifiedSearch), since only then does a real filesystem path exist to
// join the share-relative exclusions against.
func (t *locationsTab) excludeDirsForShare(displayPrefix, mountPath string) []string {
	for _, item := range t.sharePicker.SelectedShares() {
		if item.mountLabel() != displayPrefix {
			continue
		}
		subpaths := t.sharePicker.ExcludeSubpaths(item)
		if len(subpaths) == 0 {
			return nil
		}
		out := make([]string, len(subpaths))
		for i, sp := range subpaths {
			out[i] = filepath.Join(mountPath, sp)
		}
		return out
	}
	return nil
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
	t.refreshSelectedColumn()
}
