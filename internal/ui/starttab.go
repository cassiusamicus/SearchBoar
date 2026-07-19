package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

// startTab is the "Start" tab: a quick-access dashboard, not a full
// picker. Abbreviated Files/Containing entries and a one-folder Browse
// button cover the common case; "Search Builder"/"Workspace Builder"
// links go to the full versions for anything more elaborate.
// The results panel is the same list+cards+Prev/Next resultsView the
// Detailed Results tab uses (see resultsview.go), just laid out compactly
// -- a list under the location controls, cards with Prev/Next filling the
// right column -- so this tab is a real second view of the same results,
// not a separate frozen snapshot. Recent results and the last search term/
// paths are persisted to config so they survive a restart (see
// recordSearch/restoreLastSearch).
type startTab struct {
	app *App

	quickFileEntry    *widget.Entry
	quickContent      *widget.Entry
	locationLabel     *widget.Label
	savedSearchSelect *widget.Select
	workspaceSelect   *widget.Select
	view              *resultsView
}

func newStartTab(a *App) *startTab {
	// Fixed sort (no dropdown here, unlike Detailed Results) -- this is
	// meant to be a quick glance, not a second full results tab.
	return &startTab{app: a, view: newResultsView(a, "Number of hits", false)}
}

func (t *startTab) build() fyne.CanvasObject {
	t.quickFileEntry = widget.NewEntry()
	t.quickFileEntry.SetPlaceHolder(".* (all files)")
	t.quickContent = widget.NewEntry()
	t.quickContent.SetPlaceHolder("Text or regex to search for")
	t.quickContent.OnSubmitted = func(string) { t.searchNow() }

	// Search and Location rows follow the same shape -- primary action,
	// then the button to the full tab (no "Open" prefix, just the
	// destination's name), then a quick-select dropdown for something
	// already saved -- so the two cards read as one consistent pattern
	// instead of two differently-organized ones.
	searchNowBtn := widget.NewButtonWithIcon("Search Now", theme.SearchIcon(), func() { t.searchNow() })
	searchNowBtn.Importance = widget.HighImportance
	openBuilderBtn := widget.NewButton("Search Builder  →", func() {
		t.commitQuickFields()
		t.app.tabs.SelectIndex(tabIndexBuilder)
	})
	t.savedSearchSelect = widget.NewSelect(nil, func(name string) { t.selectSavedSearch(name) })
	t.savedSearchSelect.PlaceHolder = "Saved Searches..."
	t.refreshSavedSearches()
	searchCard := widget.NewCard("Search", "", container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Files:"), nil, t.quickFileEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Containing:"), nil, t.quickContent),
		container.NewHBox(searchNowBtn, openBuilderBtn, t.savedSearchSelect),
	))

	t.locationLabel = widget.NewLabel("")
	t.locationLabel.Wrapping = fyne.TextWrapWord
	browseBtn := widget.NewButton("Browse for Folder", func() { t.quickBrowse() })
	openLocationsBtn := widget.NewButton("Workspace Builder  →", func() {
		t.app.tabs.SelectIndex(tabIndexLocations)
	})
	t.workspaceSelect = widget.NewSelect(nil, func(name string) { t.selectWorkspace(name) })
	t.workspaceSelect.PlaceHolder = "Saved Workspaces..."
	t.refreshWorkspaces()
	locationCard := widget.NewCard("Location", "", container.NewVBox(
		t.locationLabel,
		container.NewHBox(browseBtn, openLocationsBtn, t.workspaceSelect),
	))

	// view.build() must run before wiring viewAllBtn/clearBtn below, since
	// they reference t.view.list/scroll.
	t.view.build()

	viewAllBtn := widget.NewButton("Open Detailed Results  →", func() { t.app.tabs.SelectIndex(tabIndexResults) })
	clearBtn := widget.NewButton("Clear History", func() { t.clear() })

	// Left column: Search/Location cards pinned to the top, then the
	// result list filling whatever space is left below them -- a fixed
	// VBox of just the two cards left the rest of the column empty.
	resultsListCard := widget.NewCard("Results", "", container.NewBorder(
		nil, container.NewHBox(viewAllBtn, clearBtn), nil, nil,
		t.view.list,
	))
	left := container.NewBorder(
		container.NewVBox(searchCard, locationCard), nil, nil, nil,
		resultsListCard,
	)

	// Right column: Prev/Next + count above the cards, mirroring Detailed
	// Results' header, then the cards filling the rest of the column.
	navHeader := container.NewHBox(t.view.prevBtn, t.view.nextBtn, layout.NewSpacer(), t.view.countLabel)
	right := container.NewBorder(navHeader, nil, nil, nil, t.view.scroll)

	t.refresh()

	// A resizable split (drag the divider) instead of a fixed 50/50 grid
	// or one long stacked column, so the two halves' relative width is a
	// user preference, not a fixed layout decision.
	//
	// HSplit's own MinSize is the *sum* of both children's widths, so
	// without the HScroll wrapper below, any wide content on either side
	// (a long path, an unusually wide button row) would add directly onto
	// the window's minimum width instead of just scrolling -- the same
	// root cause as the wrapping fixes in resultsview.go/mainwindow.go,
	// worth guarding here too since it compounds two children's widths
	// instead of one label's.
	split := container.NewHSplit(left, right)
	split.Offset = 0.4
	return container.NewHScroll(split)
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

// selectWorkspace applies a saved workspace (built/managed on the
// Workspace Builder tab) directly from the Start tab, so switching between
// a few regular search areas doesn't require a trip to the full tab.
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
// Workspace Builder tab, so both stay in sync.
func (t *startTab) refreshWorkspaces() {
	if t.workspaceSelect == nil {
		return
	}
	t.workspaceSelect.SetOptions(workspaceNames(t.app.wsStore.List()))
}

// selectSavedSearch loads a saved search (built/managed on the Favorite
// Searches tab) into the quick fields and the Search Builder's real
// fields, mirroring selectWorkspace -- loads it, doesn't run it, so the
// user can still adjust before clicking Search Now.
func (t *startTab) selectSavedSearch(name string) {
	s, ok := t.app.ssStore.Get(name)
	if !ok {
		return
	}
	t.app.builder.fileEntry.SetText(s.FilePattern)
	t.app.builder.contentCombo.SetText(s.ContentPattern)
	t.quickFileEntry.SetText(s.FilePattern)
	t.quickContent.SetText(s.ContentPattern)
}

// refreshSavedSearches reloads the quick-select's options from the store --
// called on build and whenever a search is saved/deleted from the
// Favorite Searches tab, so both stay in sync.
func (t *startTab) refreshSavedSearches() {
	if t.savedSearchSelect == nil {
		return
	}
	searches := t.app.ssStore.List()
	names := make([]string, len(searches))
	for i, s := range searches {
		names[i] = s.Name
	}
	t.savedSearchSelect.SetOptions(names)
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
		return "Searching: (nothing selected -- see Workspace Builder)"
	}
	return "Searching: " + strings.Join(parts, " + ")
}

// clear wipes both the persisted recent-search history and the live
// results shown here and on Detailed Results -- once this panel showed
// its own frozen snapshot, "Clear History" only needed to touch config,
// but now that it's a live view over app.searchResults (same data
// Detailed Results shows), leaving that in place would leave stale cards
// on screen after "clearing" them.
func (t *startTab) clear() {
	t.app.cfg.Recent.LastFilePattern = ""
	t.app.cfg.Recent.ContentPatterns = nil
	t.app.cfg.Recent.Paths = nil
	t.app.cfg.RecentResults = nil
	t.app.cfg.Save()

	t.app.searchResults = nil
	t.view.clear()
	t.app.results.clear()

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
