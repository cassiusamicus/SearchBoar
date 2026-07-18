package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

// startTab is the "Start" tab: a quick-access dashboard, not a full
// picker. Quick Files/Containing entries and a one-folder Browse button
// cover the common case; "Open Search Builder"/"Open Search Locations"
// links go to the full versions for anything more elaborate. Recent
// results and the last search term/paths are persisted to config so they
// survive a restart.
type startTab struct {
	app *App

	quickFileEntry *widget.Entry
	quickContent   *widget.Entry
	locationLabel  *widget.Label
	resultsList    *widget.List
}

func newStartTab(a *App) *startTab {
	return &startTab{app: a}
}

func (t *startTab) build() fyne.CanvasObject {
	t.quickFileEntry = widget.NewEntry()
	t.quickFileEntry.SetPlaceHolder(".* (all files)")
	t.quickContent = widget.NewEntry()
	t.quickContent.SetPlaceHolder("Text or regex to search for")
	t.quickContent.OnSubmitted = func(string) { t.searchNow() }

	searchNowBtn := widget.NewButtonWithIcon("Search Now", theme.SearchIcon(), func() { t.searchNow() })
	searchNowBtn.Importance = widget.HighImportance
	openBuilderBtn := widget.NewButton("Open Search Builder  →", func() {
		t.commitQuickFields()
		t.app.tabs.SelectIndex(tabIndexBuilder)
	})
	quickSearchCard := widget.NewCard("Quick Search", "", container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Files:"), nil, t.quickFileEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Containing:"), nil, t.quickContent),
		container.NewHBox(searchNowBtn, openBuilderBtn),
	))

	t.locationLabel = widget.NewLabel("")
	t.locationLabel.Wrapping = fyne.TextWrapWord
	browseBtn := widget.NewButton("Browse for a folder...", func() { t.quickBrowse() })
	openLocationsBtn := widget.NewButton("Open Search Locations  →", func() {
		t.app.tabs.SelectIndex(tabIndexLocations)
	})
	quickLocationCard := widget.NewCard("Quick Location", "", container.NewVBox(
		t.locationLabel,
		container.NewHBox(browseBtn, openLocationsBtn),
	))

	t.resultsList = widget.NewList(
		func() int { return len(t.app.cfg.RecentResults) },
		func() fyne.CanvasObject { return newTappableBox(widget.NewIcon(theme.FileIcon()), widget.NewLabel("")) },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateResultRow(id, o.(*tappableBox)) },
	)
	viewAllBtn := widget.NewButton("View All Results  →", func() { t.app.tabs.SelectIndex(tabIndexResults) })
	clearBtn := widget.NewButton("Clear History", func() { t.clear() })
	resultsCard := widget.NewCard("Recent Results", "", container.NewBorder(
		nil, container.NewHBox(viewAllBtn, clearBtn), nil, nil,
		container.NewVScroll(t.resultsList),
	))

	t.refresh()

	return container.NewVBox(quickSearchCard, quickLocationCard, resultsCard)
}

func (t *startTab) updateResultRow(id widget.ListItemID, box *tappableBox) {
	if id >= len(t.app.cfg.RecentResults) {
		return
	}
	r := t.app.cfg.RecentResults[id]
	path := r.Path
	label := r.DisplayPath
	if label == "" {
		label = path
	}
	box.SetObjects([]fyne.CanvasObject{widget.NewIcon(theme.FileIcon()), widget.NewLabel(filepath.Base(path) + "  —  " + label)})
	box.OnDoubleTapped = func() {
		if err := t.app.openResult(path); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	}
	box.OnSecondaryTapped = func(e *fyne.PointEvent) {
		rec := fileRecord{Path: path, Name: filepath.Base(path), ModifiedStr: r.Modified, Size: r.SizeBytes, SizeHuman: r.SizeHuman}
		menu := t.app.fileContextMenu(rec, nil)
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

// refresh re-reads the quick fields and location summary from the
// authoritative state (Search Builder's fields, the location picker,
// config's persisted history) -- called at startup and whenever the Start
// tab becomes visible, so it can never go stale while the user edits
// elsewhere.
func (t *startTab) refresh() {
	if t.quickFileEntry == nil {
		return // build() hasn't run yet
	}
	t.quickFileEntry.SetText(t.app.builder.fileEntry.Text)
	t.quickContent.SetText(t.app.builder.contentCombo.Text)
	t.locationLabel.SetText(t.locationSummary())
	t.resultsList.Refresh()
}

// commitQuickFields writes the quick entries back into the Search
// Builder's real fields -- called before acting on them (searching, or
// navigating to the full Search Builder tab) so an edit made here is never
// silently lost.
func (t *startTab) commitQuickFields() {
	t.app.builder.fileEntry.SetText(t.quickFileEntry.Text)
	t.app.builder.contentCombo.SetText(t.quickContent.Text)
}

func (t *startTab) searchNow() {
	t.commitQuickFields()
	t.app.startSearch()
}

func (t *startTab) quickBrowse() {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		t.app.locations.picker.selection.Clear()
		t.app.locations.picker.selection.SetCascade(uri.Path(), true)
		if t.app.locations.picker.tree != nil {
			t.app.locations.picker.tree.Refresh()
		}
		t.locationLabel.SetText(t.locationSummary())
	}, t.app.win)
	d.Show()
}

func (t *startTab) locationSummary() string {
	var parts []string
	if t.app.locations.localCheck.Checked {
		roots, _ := t.app.locations.picker.selectedRootsAndExcludes()
		switch {
		case len(roots) == 0:
			parts = append(parts, "all local drives")
		case len(roots) <= 3:
			parts = append(parts, strings.Join(roots, ", "))
		default:
			parts = append(parts, fmt.Sprintf("%s, and %d more", strings.Join(roots[:3], ", "), len(roots)-3))
		}
	}
	if t.app.locations.smbCheck.Checked {
		parts = append(parts, "SMB shares")
	}
	if t.app.locations.nfsCheck.Checked {
		parts = append(parts, "NFS exports")
	}
	if len(parts) == 0 {
		return "Searching: (nothing selected -- see Search Locations)"
	}
	return "Searching: " + strings.Join(parts, " + ")
}

func (t *startTab) clear() {
	t.app.cfg.Recent.LastFilePattern = ""
	t.app.cfg.Recent.ContentPatterns = nil
	t.app.cfg.Recent.Paths = nil
	t.app.cfg.RecentResults = nil
	t.app.cfg.Save()
	t.refresh()
}

// recordSearch persists what was just searched for/where, and the first
// MaxRecentResults results, so the Start tab reflects it (including after
// a restart). Must be called from the UI goroutine (it re-reads config
// state and refreshes widgets directly, no internal runOnUI hop).
func (t *startTab) recordSearch(filePattern string, searchPaths []string, results []recentResultSource) {
	t.app.cfg.Recent.LastFilePattern = filePattern
	t.app.cfg.Recent.Paths = searchPaths

	recent := make([]config.RecentResult, 0, len(results))
	for i, r := range results {
		if i >= config.MaxRecentResults {
			break
		}
		recent = append(recent, config.RecentResult{
			Path: r.Path, DisplayPath: r.DisplayPath, Modified: r.Modified, SizeBytes: r.Size, SizeHuman: formatSize(r.Size),
		})
	}
	t.app.cfg.RecentResults = recent
	t.app.cfg.Save()

	t.refresh()
}

// recentResultSource is the minimal shape searchcontrol.go needs to hand
// off to recordSearch without importing the ui package's model dependency
// twice over.
type recentResultSource struct {
	Path        string
	DisplayPath string
	Modified    string
	Size        int64
}
