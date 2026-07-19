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

	quickFileEntry  *widget.Entry
	quickContent    *widget.Entry
	locationLabel   *widget.Label
	workspaceSelect *widget.Select
	resultsList     *widget.List
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
	favoriteBtn := widget.NewButtonWithIcon("Save as Favorite Search", theme.HistoryIcon(), func() {
		t.commitQuickFields()
		t.app.favSearches.promptSaveCurrentSearch()
	})
	quickSearchCard := widget.NewCard("Quick Search", "", container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Files:"), nil, t.quickFileEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Containing:"), nil, t.quickContent),
		container.NewHBox(searchNowBtn, openBuilderBtn, favoriteBtn),
	))

	t.locationLabel = widget.NewLabel("")
	t.locationLabel.Wrapping = fyne.TextWrapWord
	localOnlyBtn := widget.NewButton("Local Storage Only", func() { t.localOnly() })
	browseBtn := widget.NewButton("Browse for a folder...", func() { t.quickBrowse() })
	openLocationsBtn := widget.NewButton("Open Search Locations  →", func() {
		t.app.tabs.SelectIndex(tabIndexLocations)
	})
	t.workspaceSelect = widget.NewSelect(nil, func(name string) { t.selectWorkspace(name) })
	t.workspaceSelect.PlaceHolder = "Workspace..."
	t.refreshWorkspaces()
	quickLocationCard := widget.NewCard("Quick Location", "", container.NewVBox(
		t.locationLabel,
		container.NewHBox(localOnlyBtn, t.workspaceSelect, browseBtn, openLocationsBtn),
	))

	t.resultsList = widget.NewList(
		func() int { return len(t.app.cfg.RecentResults) },
		func() fyne.CanvasObject { return newTappableBox(widget.NewIcon(theme.FileIcon()), widget.NewLabel("")) },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateResultRow(id, o.(*tappableBox)) },
	)
	viewAllBtn := widget.NewButton("Open Detailed Results  →", func() { t.app.tabs.SelectIndex(tabIndexResults) })
	clearBtn := widget.NewButton("Clear History", func() { t.clear() })
	resultsCard := widget.NewCard("Quick Results", "Your most recent search hits", container.NewBorder(
		nil, container.NewHBox(viewAllBtn, clearBtn), nil, nil,
		container.NewVScroll(t.resultsList),
	))

	t.refresh()

	// Two columns instead of one long stacked column: quick search/location
	// controls on the left, Quick Results filling the right -- a single
	// VBox left the whole right half of the window empty on anything wider
	// than a narrow laptop screen.
	left := container.NewVBox(quickSearchCard, quickLocationCard)
	return container.NewGridWithColumns(2, left, resultsCard)
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

// localOnly is a one-click shortcut for the common case: search local
// drives only, skipping SMB/NFS network scanning entirely (and the
// mount-authorization prompts that come with it).
func (t *startTab) localOnly() {
	t.app.locations.localCheck.SetChecked(true)
	t.app.locations.smbCheck.SetChecked(false)
	t.app.locations.nfsCheck.SetChecked(false)
	t.locationLabel.SetText(t.locationSummary())
}

// selectWorkspace applies a saved workspace (built/managed on the Search
// Locations tab) directly from the Start tab, so switching between a few
// regular search areas doesn't require a trip to the full tab.
func (t *startTab) selectWorkspace(name string) {
	w, ok := t.app.wsStore.Get(name)
	if !ok {
		return
	}
	t.app.locations.applyWorkspace(w)
	t.locationLabel.SetText(t.locationSummary())
}

// refreshWorkspaces reloads the quick-select's options from the store --
// called on build and whenever a workspace is saved/deleted from the
// Search Locations tab, so both stay in sync.
func (t *startTab) refreshWorkspaces() {
	if t.workspaceSelect == nil {
		return
	}
	t.workspaceSelect.SetOptions(workspaceNames(t.app.wsStore.List()))
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
		matches := make([]config.RecentMatch, len(r.Matches))
		for j, m := range r.Matches {
			matches[j] = config.RecentMatch{LineNum: m.LineNum, ContextStartLine: m.ContextStartLine, ContextLines: m.ContextLines}
		}
		recent = append(recent, config.RecentResult{
			Path: r.Path, DisplayPath: r.DisplayPath, Modified: r.Modified, SizeBytes: r.Size, SizeHuman: formatSize(r.Size),
			Matches: matches,
		})
	}
	t.app.cfg.RecentResults = recent
	t.app.cfg.Save()

	t.refresh()
}

// recentResultSource is the minimal shape searchcontrol.go needs to hand
// off to recordSearch without importing the ui package's model dependency
// twice over. Matches is carried through too (not just path/size/date) so
// a restored result on a future launch still shows real match context
// instead of an unexplained "no content preview" -- see model.ContentMatch.
type recentResultSource struct {
	Path        string
	DisplayPath string
	Modified    string
	Size        int64
	Matches     []recentMatchSource
}

type recentMatchSource struct {
	LineNum          int
	ContextStartLine int
	ContextLines     []string
}
