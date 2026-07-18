package ui

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
)

// locationsTab is the "Search Locations" tab: a tree of local drives and
// their subfolders, each with a checkbox (the main content of the tab),
// plus which location types to search (Local/SMB/NFS) and the network
// settings SMB/NFS discovery needs.
type locationsTab struct {
	app *App

	picker *drivePicker

	localCheck, smbCheck, nfsCheck *widget.Check
	cidrEntry                      *widget.Entry
	userEntry, passEntry           *widget.Entry
}

func newLocationsTab(a *App) *locationsTab {
	return &locationsTab{app: a, picker: newDrivePicker()}
}

func (t *locationsTab) build() fyne.CanvasObject {
	t.localCheck = widget.NewCheck("Local drives", nil)
	t.localCheck.SetChecked(true)
	t.smbCheck = widget.NewCheck("SMB shares", nil)
	t.smbCheck.SetChecked(true)
	t.nfsCheck = widget.NewCheck("NFS exports", nil)
	t.nfsCheck.SetChecked(true)

	t.cidrEntry = widget.NewEntry()
	t.cidrEntry.SetPlaceHolder("auto or 192.168.1.0/24")
	t.userEntry = widget.NewEntry()
	t.userEntry.SetPlaceHolder("Username")
	t.passEntry = widget.NewPasswordEntry()
	t.passEntry.SetPlaceHolder("Password")

	sidebar := container.NewVBox(
		widget.NewCard("Search In", "", container.NewVBox(t.localCheck, t.smbCheck, t.nfsCheck)),
		widget.NewCard("Network Settings", "", container.NewVBox(
			container.NewBorder(nil, nil, widget.NewLabel("Range:"), nil, t.cidrEntry),
			widget.NewLabel("SMB credentials (optional):"),
			container.NewBorder(nil, nil, widget.NewLabel("User:"), nil, t.userEntry),
			container.NewBorder(nil, nil, widget.NewLabel("Pass:"), nil, t.passEntry),
		)),
	)

	tree := widget.NewCard("Storage Locations", "Check drives and folders to search", t.picker.build())

	return container.NewBorder(nil, nil, nil, sidebar, tree)
}

// locationOptions builds the netsearch.LocationOptions for the current
// picker/checkbox state.
func (t *locationsTab) locationOptions() netsearch.LocationOptions {
	roots, _ := t.picker.selectedRootsAndExcludes()
	return netsearch.LocationOptions{
		LocalRoots:  roots,
		SearchLocal: t.localCheck.Checked,
		SearchSMB:   t.smbCheck.Checked,
		SearchNFS:   t.nfsCheck.Checked,
		CIDR:        t.cidrEntry.Text,
		Username:    t.userEntry.Text,
		Password:    t.passEntry.Text,
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
